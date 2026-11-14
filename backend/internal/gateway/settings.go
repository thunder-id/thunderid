// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// settingsValue is the gateway server-config section: how this deployment deploys, changed at
// runtime without a restart.
type settingsValue struct {
	// AutoApply makes a change captured on this control plane deploy to the first gateway of the
	// pipeline. Unset, gateway.auto_apply decides. With projects set, it is the default each project's
	// auto sync inherits.
	AutoApply *bool `json:"autoApply,omitempty"`
	// Projects sets, by project id, how each project is synced after a capture. Set, each project is
	// synced on its own: its own changes and the shared ones go to its first gateway.
	Projects map[string]ProjectSyncSetting `json:"projects,omitempty"`
}

// ProjectSyncSetting is how one project is synced after a capture.
type ProjectSyncSetting struct {
	// AutoSync deploys the project's changes to its first gateway after a capture. Unset, AutoApply
	// decides.
	AutoSync *bool `json:"autoSync,omitempty"`
	// FirstGateway is the gateway the project's pipeline starts at, by id. Unset, the first stage of the
	// configured pipeline.
	FirstGateway string `json:"firstGateway,omitempty"`
}

// isEmpty reports whether the value sets nothing.
func (v settingsValue) isEmpty() bool {
	return v.AutoApply == nil && len(v.Projects) == 0
}

// over lays v over base, field by field.
func (v settingsValue) over(base settingsValue) settingsValue {
	if v.AutoApply != nil {
		base.AutoApply = v.AutoApply
	}
	if v.Projects != nil {
		base.Projects = v.Projects
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

// Validate checks the incoming value is a gateway section.
func (h settingsHandler) Validate(_ context.Context, incoming, _, _ any) error {
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

// Default returns nil: Merge lays the section over gateway.auto_apply itself.
func (h settingsHandler) Default() any {
	return nil
}

// IsLocked reports whether a declarative document sets the section.
func (h settingsHandler) IsLocked(readOnly any) bool {
	ro, ok := readOnly.(settingsValue)
	return ok && !ro.isEmpty()
}

// autoApplySetting reports whether a captured change deploys to the first gateway, and how each
// project is synced.
type autoApplySetting interface {
	enabled(ctx context.Context) bool
	// projectSettings is each project's sync setting, by project id; none while projects are not synced
	// on their own.
	projectSettings(ctx context.Context) map[string]ProjectSyncSetting
}

// autoApplySource reads the gateway section on every capture rather than once at startup. A node that
// caches server configuration sees a change made on another node once its cached copy expires. When
// the section cannot be read, gateway.auto_apply applies.
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
	section := s.section(ctx)
	if section.AutoApply == nil {
		return s.base
	}
	return *section.AutoApply
}

func (s autoApplySource) projectSettings(ctx context.Context) map[string]ProjectSyncSetting {
	return s.section(ctx).Projects
}

// section reads the gateway section, or an empty one when there is no reader or it cannot be read.
func (s autoApplySource) section(ctx context.Context) settingsValue {
	if s.reader == nil {
		return settingsValue{}
	}
	value, svcErr := s.reader.GetMergedConfig(ctx, string(serverconfig.ConfigNameGateway))
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read the gateway server config; using gateway.auto_apply",
			log.String("errorCode", svcErr.Code))
		return settingsValue{}
	}
	section, _ := value.(settingsValue)
	return section
}
