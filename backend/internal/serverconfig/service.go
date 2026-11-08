// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"sync"
	"time"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const loggerComponentName = "ServerConfigService"

// ServerConfigService defines the interface for the server config service.
type ServerConfigService interface {
	ListConfigNames(ctx context.Context) ([]ConfigName, *common.ServiceError)
	GetConfig(ctx context.Context, name ConfigName) (ServerConfigLayers, *common.ServiceError)
	GetReadOnlyConfig(ctx context.Context, name string) (any, *common.ServiceError)
	GetWritableConfig(ctx context.Context, name string) (any, *common.ServiceError)
	GetMergedConfig(ctx context.Context, name string) (any, *common.ServiceError)
	SetConfig(ctx context.Context, name ConfigName, value json.RawMessage) *common.ServiceError
	// UpdateConfig changes a section's writable layer from what it holds now, through updater, and
	// writes it only while no other write changed it since it was read, on any node, trying again a few
	// times when one did. Two updates that change different parts of a section both survive.
	UpdateConfig(ctx context.Context, name ConfigName, updater ConfigUpdater) *common.ServiceError
}

// ConfigUpdater works out a section's next writable layer from the one it holds, decoded as its handler
// decodes it. An error refuses the update as an invalid value.
type ConfigUpdater interface {
	Update(current any) (any, error)
}

// maxConfigUpdateAttempts bounds how many times an update reads the section again after another write
// changed it, and configUpdateBackoff is how long it waits first, longer each time, so two nodes
// updating at once do not keep meeting.
const (
	maxConfigUpdateAttempts = 10
	configUpdateBackoff     = 5 * time.Millisecond
)

// serverConfigService is the default implementation of ServerConfigService.
type serverConfigService struct {
	authzService sysauthz.SystemAuthorizationServiceInterface
	store        serverConfigStoreInterface
	handlers     map[ConfigName]ServerConfigHandlerInterface
	// observability receives a SERVER_CONFIG_UPDATED event per write. Optional.
	observability providers.ObservabilityProvider
	bus           eventbus.EventBusInterface
	logger        *log.Logger
	configMu      sync.Mutex
	// managed says which sections a control plane deployed, which a write through SetConfig leaves as they
	// are. Nil where nothing is deployed from one.
	managed managedresource.StoreInterface
}

// newServerConfigService creates a new instance of serverConfigService. Handlers are injected at
// construction, one per supported section. The store may be the mutable, declarative, or composite
// implementation depending on the configured store mode.
func newServerConfigService(store serverConfigStoreInterface,
	handlers map[ConfigName]ServerConfigHandlerInterface,
	observability providers.ObservabilityProvider, bus eventbus.EventBusInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface) ServerConfigService {
	return &serverConfigService{
		authzService:  authzService,
		store:         store,
		handlers:      handlers,
		observability: observability,
		bus:           bus,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// ListConfigNames returns the configuration section names this server serves.
//
// A name is listed only when a handler is registered for it. A server built without the service a
// section configures, such as a control plane, which runs no sessions, registers no handler for it,
// and listing the name would offer a section that every read of then fails.
func (s *serverConfigService) ListConfigNames(_ context.Context) ([]ConfigName, *common.ServiceError) {
	names := make([]ConfigName, 0, len(supportedConfigNames))
	for _, name := range supportedConfigNames {
		if handler, ok := s.handlers[name]; ok && handler != nil {
			names = append(names, name)
		}
	}
	return names, nil
}

// GetConfig resolves the readOnly (declarative), writable (db), and merged (effective) layers of a
// section. The stored layers are decoded once and merged into the effective value.
func (s *serverConfigService) GetConfig(ctx context.Context,
	name ConfigName) (ServerConfigLayers, *common.ServiceError) {
	handler, svcErr := s.handlerFor(ctx, name)
	if svcErr != nil {
		return ServerConfigLayers{}, svcErr
	}

	rawLayers, err := s.store.GetServerConfig(ctx, name)
	if err != nil {
		s.logger.Error(ctx, "Failed to get server config from store", log.Error(err))
		return ServerConfigLayers{}, &common.InternalServerError
	}

	readOnly, writable, svcErr := s.decodeLayers(ctx, name, handler, rawLayers)
	if svcErr != nil {
		return ServerConfigLayers{}, svcErr
	}
	return ServerConfigLayers{
		ReadOnly: readOnly,
		Writable: writable,
		Merged:   handler.Merge(readOnly, writable),
	}, nil
}

// GetReadOnlyConfig returns the decoded read-only (declarative) layer of a section.
func (s *serverConfigService) GetReadOnlyConfig(ctx context.Context, name string) (any, *common.ServiceError) {
	return s.getConfigLayer(ctx, name, "read-only")
}

// GetWritableConfig returns the decoded writable (db) layer of a section.
func (s *serverConfigService) GetWritableConfig(ctx context.Context, name string) (any, *common.ServiceError) {
	return s.getConfigLayer(ctx, name, "writable")
}

// GetMergedConfig returns the merged (effective) value of a section, combining its read-only and writable
// layers.
func (s *serverConfigService) GetMergedConfig(ctx context.Context, name string) (any, *common.ServiceError) {
	layers, svcErr := s.GetConfig(ctx, ConfigName(name))
	if svcErr != nil {
		return nil, svcErr
	}
	return layers.Merged, nil
}

// getConfigLayer decodes a single stored layer of a section, selected by layer. A store or decode failure
// is an internal error; the name is validated via handlerFor.
func (s *serverConfigService) getConfigLayer(ctx context.Context, name string,
	layer string) (any, *common.ServiceError) {
	configName := ConfigName(name)
	handler, svcErr := s.handlerFor(ctx, configName)
	if svcErr != nil {
		return nil, svcErr
	}
	rawLayers, err := s.store.GetServerConfig(ctx, configName)
	if err != nil {
		s.logger.Error(ctx, "Failed to get server config from store", log.Error(err))
		return nil, &common.InternalServerError
	}
	var raw json.RawMessage
	switch layer {
	case "read-only":
		raw = rawLayers.ReadOnly
	case "writable":
		raw = rawLayers.Writable
	}
	return s.decodeLayer(ctx, configName, handler, raw, layer)
}

// SetConfig decodes and validates an incoming value against the current layers and persists the raw
// bytes to the writable layer. A bad incoming value is a client error; the stored bytes are kept as-is.
func (s *serverConfigService) SetConfig(ctx context.Context,
	name ConfigName, value json.RawMessage) *common.ServiceError {
	svcErr := s.checkWriteAccess(ctx, name)
	if svcErr == nil {
		svcErr = s.refuseManaged(ctx, name)
	}
	if svcErr == nil {
		svcErr = s.setConfig(ctx, name, value)
	}
	s.publishServerConfigEvent(ctx, name, svcErr)
	return svcErr
}

// refuseManaged refuses a write to a section a control plane deployed, unless the write is the managed
// import's. Only SetConfig is refused: a section a control plane deployed still takes the updates the
// server makes itself, such as another resource's origins, through UpdateConfig.
func (s *serverConfigService) refuseManaged(ctx context.Context, name ConfigName) *common.ServiceError {
	managed, err := managedresource.IsReadOnly(ctx, s.managed, string(name), managedresource.TypeServerConfig)
	if err != nil {
		s.logger.Error(ctx, "Failed to read whether a control plane deployed the configuration section",
			log.String("name", string(name)), log.Error(err))
		return &common.InternalServerError
	}
	if managed {
		return &ErrorManagedConfig
	}
	return nil
}

// setConfig writes a section without publishing its audit event.
func (s *serverConfigService) setConfig(ctx context.Context,
	name ConfigName, value json.RawMessage) (svcErr *common.ServiceError) {
	handler, svcErr := s.handlerFor(ctx, name)
	if svcErr != nil {
		return svcErr
	}
	// Only a known section is recorded; the name comes from the request path.
	defer func() { s.publishConfigWrite(ctx, name, svcErr) }()

	incoming, err := handler.Decode(value)
	if err != nil {
		s.logger.Debug(ctx, "Config value decode failed", log.String("name", string(name)), log.Error(err))
		return &ErrorInvalidConfigValue
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	rawLayers, err := s.store.GetServerConfig(ctx, name)
	if err != nil {
		s.logger.Error(ctx, "Failed to get current server config", log.Error(err))
		return &common.InternalServerError
	}
	readOnly, writable, svcErr := s.decodeLayers(ctx, name, handler, rawLayers)
	if svcErr != nil {
		return svcErr
	}

	if err := handler.Validate(ctx, incoming, readOnly, writable); err != nil {
		s.logger.Debug(ctx, "Config value validation failed", log.String("name", string(name)), log.Error(err))
		return &ErrorInvalidConfigValue
	}

	if err := s.store.UpsertServerConfig(ctx, ServerConfig{Name: name, Value: value}); err != nil {
		s.logger.Error(ctx, "Failed to upsert server config", log.Error(err))
		return &common.InternalServerError
	}
	return nil
}

// UpdateConfig changes a section's writable layer from the one it holds, with a conditional write that
// loses to any write made since the read, and reads and tries again when it does.
func (s *serverConfigService) UpdateConfig(ctx context.Context, name ConfigName,
	updater ConfigUpdater) *common.ServiceError {
	svcErr := s.updateConfig(ctx, name, updater)
	s.publishServerConfigEvent(ctx, name, svcErr)
	return svcErr
}

// updateConfig changes a section from the one it holds without publishing its audit event.
func (s *serverConfigService) updateConfig(ctx context.Context, name ConfigName,
	updater ConfigUpdater) (svcErr *common.ServiceError) {
	handler, svcErr := s.handlerFor(ctx, name)
	if svcErr != nil {
		return svcErr
	}
	defer func() { s.publishConfigWrite(ctx, name, svcErr) }()

	s.configMu.Lock()
	defer s.configMu.Unlock()
	for attempt := 0; attempt < maxConfigUpdateAttempts; attempt++ {
		rawLayers, err := s.store.GetServerConfig(ctx, name)
		if err != nil {
			s.logger.Error(ctx, "Failed to get current server config", log.Error(err))
			return &common.InternalServerError
		}
		readOnly, writable, svcErr := s.decodeLayers(ctx, name, handler, rawLayers)
		if svcErr != nil {
			return svcErr
		}
		next, err := updater.Update(writable)
		if err != nil {
			s.logger.Debug(ctx, "Config update refused", log.String("name", string(name)), log.Error(err))
			return &ErrorInvalidConfigValue
		}
		value, err := json.Marshal(next)
		if err != nil {
			return &common.InternalServerError
		}
		incoming, err := handler.Decode(value)
		if err != nil {
			return &ErrorInvalidConfigValue
		}
		if err := handler.Validate(ctx, incoming, readOnly, writable); err != nil {
			s.logger.Debug(ctx, "Config value validation failed", log.String("name", string(name)), log.Error(err))
			return &ErrorInvalidConfigValue
		}
		written, err := s.store.ReplaceServerConfigIf(ctx, ServerConfig{Name: name, Value: value}, rawLayers.Writable)
		if err != nil {
			s.logger.Error(ctx, "Failed to write server config", log.Error(err))
			return &common.InternalServerError
		}
		if written {
			return nil
		}
		s.logger.Debug(ctx, "Server config changed while it was updated; reading it again",
			log.String("name", string(name)), log.Int("attempt", attempt+1))
		// #nosec G404 -- spreads retries apart; nothing depends on it being unpredictable.
		jitter := time.Duration(rand.Int64N(int64(configUpdateBackoff)))
		// The wait ends with the request, so a caller that gave up releases the section's lock at once.
		wait := time.NewTimer(time.Duration(attempt+1)*configUpdateBackoff + jitter)
		select {
		case <-ctx.Done():
			wait.Stop()
			return &ErrorConfigWriteConflict
		case <-wait.C:
		}
	}
	return &ErrorConfigWriteConflict
}

// publishServerConfigEvent publishes the audit event of an attempt to change the section name, after the
// change was stored or refused. The target is the section, never its value, since a section may hold
// secrets.
func (s *serverConfigService) publishServerConfigEvent(ctx context.Context, name ConfigName,
	svcErr *common.ServiceError) {
	// A failed attempt keeps the name the caller asked for, even one that is not a section, as forensic
	// evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: audit.TargetTypeServerConfig, ID: string(name)}
	eventbus.TryPublish(ctx, s.bus,
		eventbus.NewEvent(ctx, audit.EventServerConfigUpdate).With(
			audit.ChangePayload(ctx, target, audit.ActionUpdate, svcErr)))
}

// publishConfigWrite records who wrote which section and the outcome. It never records the value,
// since a section may hold secrets.
func (s *serverConfigService) publishConfigWrite(ctx context.Context, name ConfigName,
	svcErr *common.ServiceError) {
	actor := security.GetSubject(ctx)
	if svcErr == nil {
		s.logger.Debug(ctx, "Server config written", log.String("name", string(name)),
			log.MaskedString("actor", actor))
	}

	if s.observability == nil || !s.observability.IsEnabled() {
		return
	}
	eventType, status := event.EventTypeServerConfigUpdated, providers.StatusSuccess
	if svcErr != nil {
		eventType, status = event.EventTypeServerConfigUpdateFailed, providers.StatusFailure
	}
	evt := event.NewEvent(syscontext.GetTraceID(ctx), string(eventType), event.ComponentServerConfig).
		WithStatus(status).
		WithData(event.DataKey.ConfigName, string(name))
	if actor != "" {
		evt.WithData(event.DataKey.ActorSub, actor)
	}
	if svcErr != nil {
		evt.WithData(event.DataKey.Error, svcErr.Code)
	}
	s.observability.PublishEvent(ctx, evt)
}

// decodeLayers decodes the stored readOnly and writable layers into typed values. A failure here is an
// internal invariant violation — stored values were validated at write or load time — not a client error.
func (s *serverConfigService) decodeLayers(ctx context.Context, name ConfigName,
	handler ServerConfigHandlerInterface, rawLayers storeLayers) (any, any, *common.ServiceError) {
	readOnly, err := handler.Decode(rawLayers.ReadOnly)
	if err != nil {
		s.logger.Error(ctx, "Failed to decode read-only server config layer",
			log.String("name", string(name)), log.Error(err))
		return nil, nil, &common.InternalServerError
	}
	writable, err := handler.Decode(rawLayers.Writable)
	if err != nil {
		s.logger.Error(ctx, "Failed to decode writable server config layer",
			log.String("name", string(name)), log.Error(err))
		return nil, nil, &common.InternalServerError
	}
	return readOnly, writable, nil
}

// decodeLayer decodes a single stored layer into its typed value. A failure here is an internal invariant
// violation — stored values were validated at write or load time — not a client error.
func (s *serverConfigService) decodeLayer(ctx context.Context, name ConfigName,
	handler ServerConfigHandlerInterface, raw json.RawMessage, layer string) (any, *common.ServiceError) {
	decoded, err := handler.Decode(raw)
	if err != nil {
		s.logger.Error(ctx, "Failed to decode server config layer",
			log.String("name", string(name)), log.String("layer", layer), log.Error(err))
		return nil, &common.InternalServerError
	}
	return decoded, nil
}

// handlerFor returns the registered handler for a section. An unsupported name is a client error; a
// supported name with no registered handler is a wiring error.
func (s *serverConfigService) handlerFor(ctx context.Context,
	name ConfigName) (ServerConfigHandlerInterface, *common.ServiceError) {
	if !name.IsValid() {
		return nil, &ErrorUnsupportedConfigName
	}
	handler, ok := s.handlers[name]
	if !ok || handler == nil {
		s.logger.Error(ctx, "No handler registered for supported config name", log.String("name", string(name)))
		return nil, &common.InternalServerError
	}
	return handler, nil
}

// checkWriteAccess refuses a caller who may not write a configuration section through the server
// configuration API or import.
func (s *serverConfigService) checkWriteAccess(ctx context.Context, name ConfigName) *common.ServiceError {
	allowed, svcErr := s.authzService.IsActionAllowed(ctx, security.ActionUpdateServerConfig,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeServerConfig, ResourceID: string(name)})
	if svcErr != nil {
		return &common.InternalServerError
	}
	if !allowed {
		return &common.ErrorUnauthorized
	}
	return nil
}
