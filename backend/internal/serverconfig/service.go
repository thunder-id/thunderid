/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package serverconfig

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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
	logger   *log.Logger
}

// newServerConfigService creates a new instance of serverConfigService. Handlers are injected at
// construction, one per supported section. The store may be the mutable, declarative, or composite
// implementation depending on the configured store mode.
func newServerConfigService(store serverConfigStoreInterface,
	handlers map[ConfigName]ServerConfigHandlerInterface) ServerConfigService {
	return &serverConfigService{
		store:    store,
		handlers: handlers,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// ListConfigNames returns the supported configuration section names.
func (s *serverConfigService) ListConfigNames(_ context.Context) ([]ConfigName, *common.ServiceError) {
	names := make([]ConfigName, len(supportedConfigNames))
	copy(names, supportedConfigNames)
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
	handler, svcErr := s.handlerFor(ctx, name)
	if svcErr != nil {
		return svcErr
	}

	incoming, err := handler.Decode(value)
	if err != nil {
		s.logger.Debug(ctx, "Config value decode failed", log.String("name", string(name)), log.Error(err))
		return &ErrorInvalidConfigValue
	}

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
