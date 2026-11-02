// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"sync"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
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
}

// serverConfigService is the default implementation of ServerConfigService.
type serverConfigService struct {
	store    serverConfigStoreInterface
	handlers map[ConfigName]ServerConfigHandlerInterface
	// observability receives a SERVER_CONFIG_UPDATED event per write. Optional.
	observability providers.ObservabilityProvider
	logger        *log.Logger
	configMu      sync.Mutex
}

// newServerConfigService creates a new instance of serverConfigService. Handlers are injected at
// construction, one per supported section. The store may be the mutable, declarative, or composite
// implementation depending on the configured store mode.
func newServerConfigService(store serverConfigStoreInterface,
	handlers map[ConfigName]ServerConfigHandlerInterface,
	observability providers.ObservabilityProvider) ServerConfigService {
	return &serverConfigService{
		store:         store,
		handlers:      handlers,
		observability: observability,
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

	if err := handler.Validate(incoming, readOnly, writable); err != nil {
		s.logger.Debug(ctx, "Config value validation failed", log.String("name", string(name)), log.Error(err))
		return &ErrorInvalidConfigValue
	}

	if err := s.store.UpsertServerConfig(ctx, ServerConfig{Name: name, Value: value}); err != nil {
		s.logger.Error(ctx, "Failed to upsert server config", log.Error(err))
		return &common.InternalServerError
	}
	return nil
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
