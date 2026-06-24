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

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "ServerConfigService"

// ServerConfigService defines the interface for the server config service.
type ServerConfigService interface {
	GetConfig(ctx context.Context, name ConfigName) (json.RawMessage, *serviceerror.ServiceError)
	SetConfig(ctx context.Context, name ConfigName, value json.RawMessage) *serviceerror.ServiceError
	SetConfigs(ctx context.Context, configs map[ConfigName]json.RawMessage) *serviceerror.ServiceError
	ListConfigs(ctx context.Context) (map[ConfigName]json.RawMessage, *serviceerror.ServiceError)
	RegisterValidator(name ConfigName, validator ServerConfigValidatorInterface)
}

// serverConfigService is the default implementation of ServerConfigService.
type serverConfigService struct {
	store      serverConfigStoreInterface
	validators map[ConfigName]ServerConfigValidatorInterface
	logger     *log.Logger
}

// newServerConfigService creates a new instance of serverConfigService with injected dependencies.
func newServerConfigService(store serverConfigStoreInterface) ServerConfigService {
	return &serverConfigService{
		store:      store,
		validators: make(map[ConfigName]ServerConfigValidatorInterface),
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// RegisterValidator registers a value validator for a config name. It is called at boot time from
// the composition root, before the service starts handling requests.
func (s *serverConfigService) RegisterValidator(name ConfigName, validator ServerConfigValidatorInterface) {
	s.validators[name] = validator
}

// GetConfig retrieves the raw JSON value of a config section by name.
func (s *serverConfigService) GetConfig(ctx context.Context,
	name ConfigName) (json.RawMessage, *serviceerror.ServiceError) {
	if !name.IsValid() {
		return nil, &ErrorUnsupportedConfigName
	}

	cfg, err := s.store.GetServerConfigByName(ctx, name)
	if err != nil {
		s.logger.Error(ctx, "Failed to get server config from store", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if cfg == nil {
		return nil, &ErrorConfigNotFound
	}
	return cfg.Value, nil
}

// SetConfig validates and persists a single config section.
func (s *serverConfigService) SetConfig(ctx context.Context,
	name ConfigName, value json.RawMessage) *serviceerror.ServiceError {
	if svcErr := s.validateConfig(ctx, name, value); svcErr != nil {
		return svcErr
	}

	if err := s.store.UpsertServerConfig(ctx, ServerConfig{Name: name, Value: value}); err != nil {
		s.logger.Error(ctx, "Failed to upsert server config", log.Error(err))
		return &serviceerror.InternalServerError
	}
	return nil
}

// SetConfigs validates all the given config sections first and, only if all pass, persists them in
// a single transaction. A single invalid section rejects the whole request with no write performed.
func (s *serverConfigService) SetConfigs(ctx context.Context,
	configs map[ConfigName]json.RawMessage) *serviceerror.ServiceError {
	toWrite := make([]ServerConfig, 0, len(configs))
	for name, value := range configs {
		if svcErr := s.validateConfig(ctx, name, value); svcErr != nil {
			return svcErr
		}
		toWrite = append(toWrite, ServerConfig{Name: name, Value: value})
	}

	if len(toWrite) == 0 {
		return nil
	}

	if err := s.store.UpsertServerConfigs(ctx, toWrite); err != nil {
		s.logger.Error(ctx, "Failed to upsert server configs", log.Error(err))
		return &serviceerror.InternalServerError
	}
	return nil
}

// ListConfigs retrieves all set config sections as a map of name to raw JSON value.
func (s *serverConfigService) ListConfigs(ctx context.Context) (
	map[ConfigName]json.RawMessage, *serviceerror.ServiceError) {
	configs, err := s.store.GetServerConfigList(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list server configs", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	result := make(map[ConfigName]json.RawMessage, len(configs))
	for _, cfg := range configs {
		result[cfg.Name] = cfg.Value
	}
	return result, nil
}

// validateConfig runs the name gate and the registered validator for a config section. A supported
// name with no registered validator is a server misconfiguration and is rejected, never persisted.
func (s *serverConfigService) validateConfig(ctx context.Context,
	name ConfigName, value json.RawMessage) *serviceerror.ServiceError {
	if !name.IsValid() {
		return &ErrorUnsupportedConfigName
	}

	validator, ok := s.validators[name]
	if !ok || validator == nil {
		s.logger.Error(ctx, "No validator registered for supported config name", log.String("name", string(name)))
		return &serviceerror.InternalServerError
	}

	if err := validator.Validate(value); err != nil {
		s.logger.Debug(ctx, "Config value validation failed", log.String("name", string(name)), log.Error(err))
		return &ErrorInvalidConfigValue
	}
	return nil
}
