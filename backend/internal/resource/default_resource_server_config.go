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

package resource

import (
	"context"
	"encoding/json"
)

// DefaultResourceServerConfig contains the default resource server configuration.
type DefaultResourceServerConfig struct {
	ResourceServerID string `json:"resourceServerId" yaml:"resourceServerId"`
}

// DefaultResourceServerConfigHandler handles default resource server configuration.
type DefaultResourceServerConfigHandler struct {
	resourceService ResourceServiceInterface
}

// NewDefaultResourceServerConfigHandler creates a default resource server configuration handler.
func NewDefaultResourceServerConfigHandler(
	resourceService ResourceServiceInterface,
) *DefaultResourceServerConfigHandler {
	if resourceService == nil {
		panic("default resource server config handler requires a non-nil resource service")
	}
	return &DefaultResourceServerConfigHandler{resourceService: resourceService}
}

// Decode parses a default resource server configuration.
func (*DefaultResourceServerConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return DefaultResourceServerConfig{}, nil
	}
	var cfg DefaultResourceServerConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate validates a default resource server configuration.
func (h *DefaultResourceServerConfigHandler) Validate(incoming, readOnly, _ any) error {
	cfg, _ := incoming.(DefaultResourceServerConfig)
	if ro, ok := readOnly.(DefaultResourceServerConfig); ok && ro.ResourceServerID != "" {
		return errDeclarativeDefaultLocked
	}
	if cfg.ResourceServerID == "" {
		return nil
	}
	if _, svcErr := h.resourceService.GetResourceServer(context.Background(), cfg.ResourceServerID); svcErr != nil {
		if svcErr.Code == ErrorResourceServerNotFound.Code {
			return errUnknownDefaultResourceServer
		}
		return errDefaultResourceServerLookupFailed
	}
	return nil
}

// Merge combines read-only and writable default resource server configurations.
func (*DefaultResourceServerConfigHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(DefaultResourceServerConfig); ok && ro.ResourceServerID != "" {
		return ro
	}
	if w, ok := writable.(DefaultResourceServerConfig); ok {
		return w
	}
	return DefaultResourceServerConfig{}
}
