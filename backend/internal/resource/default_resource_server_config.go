// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/security"
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
func (h *DefaultResourceServerConfigHandler) Validate(ctx context.Context, incoming, _, _ any) error {
	cfg, _ := incoming.(DefaultResourceServerConfig)
	if cfg.ResourceServerID == "" {
		return nil
	}
	// Only the resource server's existence is checked, whichever organization unit owns it.
	if _, svcErr := h.resourceService.GetResourceServer(
		security.WithRuntimeContext(ctx), cfg.ResourceServerID); svcErr != nil {
		if svcErr.Code == ErrorResourceServerNotFound.Code {
			return errUnknownDefaultResourceServer
		}
		return errDefaultResourceServerLookupFailed
	}
	return nil
}

// Merge combines read-only and writable default resource server configurations.
func (h *DefaultResourceServerConfigHandler) Merge(readOnly, writable any) any {
	if h.IsLocked(readOnly) {
		return readOnly
	}
	if w, ok := writable.(DefaultResourceServerConfig); ok {
		return w
	}
	return DefaultResourceServerConfig{}
}

// Default returns nil: the section has no default.
func (*DefaultResourceServerConfigHandler) Default() any {
	return nil
}

// IsLocked reports whether a declarative document names the default resource server.
func (*DefaultResourceServerConfigHandler) IsLocked(readOnly any) bool {
	ro, ok := readOnly.(DefaultResourceServerConfig)
	return ok && ro.ResourceServerID != ""
}
