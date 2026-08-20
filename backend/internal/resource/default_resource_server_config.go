// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
func (h *DefaultResourceServerConfigHandler) Validate(ctx context.Context, incoming, readOnly, _ any) error {
	cfg, _ := incoming.(DefaultResourceServerConfig)
	if ro, ok := readOnly.(DefaultResourceServerConfig); ok && ro.ResourceServerID != "" {
		return errDeclarativeDefaultLocked
	}
	if cfg.ResourceServerID == "" {
		return nil
	}
	// Read under the deployment the value is being set for: on a multi-tenant control plane the same
	// id names a different row per tenant, so a lookup without it finds nothing and every tenant's
	// default would be rejected as unknown.
	if _, svcErr := h.resourceService.GetResourceServer(ctx, cfg.ResourceServerID); svcErr != nil {
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
