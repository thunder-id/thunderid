// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ApplyResult reports what an apply did.
type ApplyResult struct {
	GatewayID string `json:"gatewayId"`
	// Import is the data plane's own answer, passed through.
	Import *ImportResult `json:"import,omitempty"`
	// UnresolvedVariables names the placeholders this control plane had no value for. They are left
	// out of the payload rather than sent empty, so the gateway fills them from its own environment.
	// Reporting them is how an operator learns which values that gateway has to hold.
	UnresolvedVariables []string `json:"unresolvedVariables,omitempty"`
}

// applier posts configuration to a data plane.
type applier interface {
	Apply(ctx context.Context, gw *Gateway, secret, content string, variables map[string]string) (*ImportResult, error)
}

// Apply sends this control plane's current configuration to a registered gateway.
//
// There is no version history here: what is applied is what the control plane holds now.
func (s *service) Apply(ctx context.Context, id string) (*ApplyResult, *tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorInvalidGatewayID
	}
	if s.exporter == nil || s.dataPlane == nil {
		s.logger.Error(ctx, "Apply is not wired on this server")
		return nil, &tidcommon.InternalServerError
	}

	gw, err := s.store.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if gw == nil {
		return nil, &ErrorGatewayNotFound
	}

	secret, svcErr := credentialOf(gw)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to recover the gateway credential", log.String("gatewayId", id))
		return nil, svcErr
	}

	exported, svcErr := s.exporter.ExportResources(ctx, everything())
	if svcErr != nil {
		return nil, svcErr
	}
	content := export.CombineResources(exported.Files)
	if strings.TrimSpace(content) == "" {
		return nil, &ErrorNothingToApply
	}

	variables, unresolved := variablesFrom(exported.EnvFile)

	result, err := s.dataPlane.Apply(ctx, gw, secret, content, variables)
	if err != nil {
		// The message can carry what the data plane echoed back, so it is logged rather than returned.
		s.logger.Error(ctx, "Failed to apply to the gateway",
			log.String("gatewayId", id), log.Error(err))
		return nil, &ErrorGatewayUnreachable
	}

	s.logger.Info(ctx, "Applied the configuration to a gateway", log.String("gatewayId", id))
	return &ApplyResult{GatewayID: id, Import: result, UnresolvedVariables: unresolved}, nil
}

// everything asks the exporter for every resource of every type.
func everything() *export.ExportRequest {
	all := []string{"*"}
	return &export.ExportRequest{
		Agents: all, Applications: all, Connections: all, UserTypes: all, AgentTypes: all,
		OrganizationUnits: all, Users: all, Groups: all, ResourceServers: all, Roles: all,
		Flows: all, Translations: all, Layouts: all, Themes: all, ServerConfigs: all,
		CredentialConfigurations: all, PresentationDefinitions: all,
	}
}

// variablesFrom reads the exported environment file, which carries a value for every placeholder the
// export could resolve and an empty one for the rest.
//
// The empty ones are deliberately not sent. An empty value would resolve the placeholder to nothing
// and write an empty credential; leaving it out lets the gateway supply it from its environment.
func variablesFrom(env *export.EnvironmentFile) (map[string]string, []string) {
	variables := map[string]string{}
	unresolved := []string{}
	if env == nil {
		return variables, unresolved
	}
	for _, line := range strings.Split(env.Content, "\n") {
		name, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(name) == "" {
			continue
		}
		if value == "" {
			unresolved = append(unresolved, name)
			continue
		}
		variables[name] = value
	}
	sort.Strings(unresolved)
	return variables, unresolved
}

// credentialOf recovers the stored client secret.
func credentialOf(gw *Gateway) (string, *tidcommon.ServiceError) {
	properties, err := cmodels.DeserializePropertiesFromJSON(gw.ClientSecret)
	if err != nil || len(properties) == 0 {
		return "", &ErrorGatewayCredentialUnreadable
	}
	secret, err := properties[0].GetValue()
	if err != nil || secret == "" {
		return "", &ErrorGatewayCredentialUnreadable
	}
	return secret, nil
}
