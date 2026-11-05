// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// errDeclarativeSettingsLocked is returned when a write targets a gateway section set declaratively.
var errDeclarativeSettingsLocked = errors.New("gateway is set declaratively and cannot be overridden")

// settingsValue is the gateway server-config section: how this deployment deploys, changed at
// runtime without a restart.
type settingsValue struct {
	// AutoApply makes a change captured on this control plane deploy to the first environment of the
	// pipeline. Unset, gateway.auto_apply decides.
	AutoApply *bool `json:"autoApply,omitempty"`
}

// isEmpty reports whether the value sets nothing.
func (v settingsValue) isEmpty() bool {
	return v.AutoApply == nil
}

// over lays v over base, field by field.
func (v settingsValue) over(base settingsValue) settingsValue {
	if v.AutoApply != nil {
		base.AutoApply = v.AutoApply
	}
	return base
}

// settingsHandler decodes, validates and merges the gateway section over the deployment
// configuration.
type settingsHandler struct {
	base settingsValue
}

// NewSettingsConfigHandler builds the gateway server-config handler over gateway.auto_apply.
func NewSettingsConfigHandler() serverconfig.ServerConfigHandlerInterface {
	autoApply := false
	if config.IsServerRuntimeInitialized() {
		autoApply = config.GetServerRuntime().Config.Gateway.IsAutoApplyEnabled()
	}
	return settingsHandler{base: settingsValue{AutoApply: &autoApply}}
}

// Decode parses a raw section value. Empty input is an absent layer. Unknown fields are refused, so a
// misspelled key is not silently dropped.
func (h settingsHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return settingsValue{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value settingsValue
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// Validate refuses a write while a declarative document sets the section.
func (h settingsHandler) Validate(_ context.Context, incoming, readOnly, _ any) error {
	if ro, ok := readOnly.(settingsValue); ok && !ro.isEmpty() {
		return errDeclarativeSettingsLocked
	}
	if _, ok := incoming.(settingsValue); !ok {
		return fmt.Errorf("%s: unexpected config type %T", serverconfig.ConfigNameGateway, incoming)
	}
	return nil
}

// Merge lays the declarative document over the deployment configuration when one exists, otherwise
// the writable layer.
func (h settingsHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(settingsValue); ok && !ro.isEmpty() {
		return ro.over(h.base)
	}
	wr, _ := writable.(settingsValue)
	return wr.over(h.base)
}

// autoApplySetting reports whether a captured change deploys to the first environment.
type autoApplySetting interface {
	enabled(ctx context.Context) bool
}

// autoApplySource reads the gateway section on every capture, so a change on any node is seen. When it
// cannot be read, gateway.auto_apply applies.
type autoApplySource struct {
	base   bool
	reader serverconfig.ServerConfigService
	logger *log.Logger
}

// newAutoApplySource reads auto apply from the gateway section over gateway.auto_apply. Without a
// reader, gateway.auto_apply alone decides.
func newAutoApplySource(reader serverconfig.ServerConfigService) autoApplySource {
	base := false
	if config.IsServerRuntimeInitialized() {
		base = config.GetServerRuntime().Config.Gateway.IsAutoApplyEnabled()
	}
	return autoApplySource{base: base, reader: reader,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayAutoApply"))}
}

func (s autoApplySource) enabled(ctx context.Context) bool {
	if s.reader == nil {
		return s.base
	}
	value, svcErr := s.reader.GetMergedConfig(ctx, string(serverconfig.ConfigNameGateway))
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read the gateway server config; using gateway.auto_apply",
			log.String("errorCode", svcErr.Code))
		return s.base
	}
	section, ok := value.(settingsValue)
	if !ok || section.AutoApply == nil {
		return s.base
	}
	return *section.AutoApply
}
