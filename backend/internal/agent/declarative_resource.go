// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/agent/model"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/valueref"
	"github.com/thunder-id/thunderid/internal/system/varname"
)

const (
	resourceTypeAgent = "agent"
	paramTypeAgent    = "Agent"
)

// agentExporter implements declarativeresource.ResourceExporter for agents.
type agentExporter struct {
	service           AgentServiceInterface
	entityService     entity.EntityServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	// references writes a credential as a secret reference rather than a template placeholder, as a
	// control plane's export does for every other credential.
	references bool
}

// WriteValueReferences sets whether credentials are written as references or as placeholders.
func (e *agentExporter) WriteValueReferences(references bool) {
	e.references = references
}

func newAgentExporter(
	service AgentServiceInterface,
	entityService entity.EntityServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) *agentExporter {
	return &agentExporter{service: service, entityService: entityService, entityTypeService: entityTypeService}
}

// NewAgentExporterForTest exposes the exporter constructor for test packages.
func NewAgentExporterForTest(
	service AgentServiceInterface,
	entityService entity.EntityServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) *agentExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newAgentExporter(service, entityService, entityTypeService)
}

// GetResourceType returns the resource type identifier for agents.
func (e *agentExporter) GetResourceType() string {
	return resourceTypeAgent
}

// GetParameterizerType returns the parameterizer type name for agents.
func (e *agentExporter) GetParameterizerType() string {
	return paramTypeAgent
}

// GetAllResourceIDs returns IDs of all mutable (non-declarative) agents.
// In composite mode declarative agents are excluded so they are not re-exported.
func (e *agentExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	ids := []string{}

	for {
		agents, err := e.service.GetAgentList(ctx, limit, offset, nil, false)
		if err != nil {
			return nil, err
		}

		for _, a := range agents.Agents {
			if !a.IsReadOnly {
				ids = append(ids, a.ID)
			}
		}

		offset += len(agents.Agents)
		if len(agents.Agents) == 0 {
			break
		}
	}

	return ids, nil
}

// GetResourceByID retrieves an agent by its ID for export.
func (e *agentExporter) GetResourceByID(
	ctx context.Context, id string) (interface{}, string, *tidcommon.ServiceError) {
	a, err := e.service.GetAgent(ctx, id, true)
	if err != nil {
		return nil, "", err
	}
	// Decode Attributes from json.RawMessage into a map so the YAML marshaler can
	// serialize them correctly (json.RawMessage would encode as base64 in YAML).
	if len(a.Attributes) > 0 {
		var attrs map[string]interface{}
		if jsonErr := json.Unmarshal(a.Attributes, &attrs); jsonErr == nil {
			a.AttributesYAML = attrs
		}
	}
	// An unbounded lifespan is written as its sentinel rather than left out, since an import reads an
	// absent validUntil as leaving the target agent's lifespan unchanged.
	if a.Lifespan != nil && a.Lifespan.Phase == model.LifespanPhaseUnbounded {
		a.ValidUntil = authnprovidercm.UnboundedValidUntil
	}
	credentials, svcErr := e.exportableCredentials(ctx, a.ID, a.Type, a.Display)
	if svcErr != nil {
		return nil, "", svcErr
	}
	a.Credentials = credentials
	return a, a.Display, nil
}

// exportableCredentials describes the credentials an exported agent carries: one placeholder for each
// credential the agent's type declares and the agent has set, a secret reference where the export
// writes references and a template placeholder where it writes templates.
//
// A stored credential is a one-way hash, so its value never leaves. The placeholder names it
// instead, and the importing server fills it from its own secret provider or environment before
// hashing it, which leaves the imported agent able to sign in with it. A credential the agent has not
// set is left out rather than exported empty. Only credentials the type declares are considered, and
// the placeholder is written here because the parameterizer only walks a slice of properties and
// credentials are a map.
func (e *agentExporter) exportableCredentials(
	ctx context.Context, agentID, agentType, name string,
) (map[string]interface{}, *tidcommon.ServiceError) {
	credentials := map[string]interface{}{}
	if name == "" {
		// The placeholder is named after the agent, so there is nothing to name it after.
		return credentials, nil
	}

	declared, svcErr := e.entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryAgent, agentType,
		entitytype.AttributeFilter{AllowCredential: true})
	if svcErr != nil {
		return nil, svcErr
	}

	for _, attribute := range declared {
		stored, err := e.entityService.GetCredentialsByType(ctx, agentID, attribute.Attribute)
		if err != nil {
			log.GetLogger().Error(ctx, "Failed to read an agent's credentials for export",
				log.MaskedString(log.LoggerKeyEntityID, agentID), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if len(stored) == 0 {
			continue
		}
		variable := varname.DeriveVariableName(resourceTypeAgent, name, attribute.Attribute)
		if e.references {
			credentials[attribute.Attribute] = valueref.SecretReference(variable)
			continue
		}
		credentials[attribute.Attribute] = fmt.Sprintf("{{.%s}}", variable)
	}
	return credentials, nil
}

// ValidateResource validates an agent resource prior to export.
func (e *agentExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	a, ok := resource.(*model.AgentGetResponse)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeAgent, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
		a.Display, resourceTypeAgent, id, "AGT_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return a.Display, nil
}

// ViewResource shows an exported agent as a read of it returns it. The read carries the agent's
// client ID at its top level and never its client secret, so the view does the same. It also reports
// the lifespan as a read does: the unbounded sentinel export writes is shown as an unbounded phase with
// no date.
func (e *agentExporter) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	return declarativeresource.DecodeView(document, func(a *model.AgentGetResponse) (interface{}, error) {
		if a.ValidUntil != "" {
			if end, svcErr := parseLifespanEnd(a.ValidUntil); svcErr == nil {
				a.ValidUntil, a.Lifespan = lifespanView(&end, time.Now())
			}
		}
		var err error
		if len(a.AttributesYAML) > 0 {
			a.Attributes, err = json.Marshal(a.AttributesYAML)
		}
		for _, inbound := range a.InboundAuthConfig {
			if inbound.OAuthConfig == nil {
				continue
			}
			a.ClientID = inbound.OAuthConfig.ClientID
			inbound.OAuthConfig.ClientSecret = ""
		}
		return a, err
	})
}

// SecretFields names where an exported agent holds its client secret and its credentials: every key
// of the credentials map, since which credentials an agent has is its type's to say.
func (e *agentExporter) SecretFields() []string {
	return []string{"inboundAuthConfig[].config.clientSecret", "credentials.*"}
}

// GetResourceRules returns parameterization rules for agents with OAuth.
func (e *agentExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables: []string{
			"InboundAuthConfig[].OAuthConfig.ClientID",
		},
		SecretVariables: []string{
			"InboundAuthConfig[].OAuthConfig.ClientSecret",
		},
		ArrayVariables: []string{
			"InboundAuthConfig[].OAuthConfig.RedirectURIs",
		},
		DynamicPropertyFields: []string{"Credentials"},
	}
}

// GetResourceRulesForResource returns per-resource parameterization rules.
// Public clients omit ClientSecret. RedirectURIs are only parameterized when present
// (e.g. M2M agents using client_credentials have no redirect URIs).
func (e *agentExporter) GetResourceRulesForResource(resource interface{}) *declarativeresource.ResourceRules {
	a, ok := resource.(*model.AgentGetResponse)
	if !ok {
		return e.GetResourceRules()
	}

	isPublicClient := false
	hasRedirectURIs := false
	for _, inbound := range a.InboundAuthConfig {
		if inbound.OAuthConfig == nil {
			continue
		}
		if inbound.OAuthConfig.PublicClient {
			isPublicClient = true
		}
		if len(inbound.OAuthConfig.RedirectURIs) > 0 {
			hasRedirectURIs = true
		}
	}

	variables := []string{"InboundAuthConfig[].OAuthConfig.ClientID"}
	var secrets []string
	if !isPublicClient {
		secrets = append(secrets, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	}

	rules := &declarativeresource.ResourceRules{
		Variables: variables, SecretVariables: secrets, DynamicPropertyFields: []string{"Credentials"},
	}
	if hasRedirectURIs {
		rules.ArrayVariables = []string{"InboundAuthConfig[].OAuthConfig.RedirectURIs"}
	}
	return rules
}

// makeAgentDeclarativeConfig returns the entity loader configuration for agents.
func makeAgentDeclarativeConfig(agentSvc AgentServiceInterface) entity.DeclarativeLoaderConfig {
	return entity.DeclarativeLoaderConfig{
		Directory: "agents",
		Category:  providers.EntityCategoryAgent,
		Parser:    makeAgentEntityParser(agentSvc),
	}
}

// makeAgentEntityParser returns a parser that converts agent YAML into an providers.Entity.
func makeAgentEntityParser(
	agentSvc AgentServiceInterface,
) func([]byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
	return func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
		if agentSvc == nil {
			return nil, nil, nil, fmt.Errorf("agent service is required for declarative entity parsing")
		}

		var req model.AgentRequestWithID
		if err := yaml.Unmarshal(data, &req); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse agent YAML: %w", err)
		}

		var attributesJSON json.RawMessage
		if len(req.Attributes) > 0 {
			raw, err := json.Marshal(req.Attributes)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to marshal agent attributes: %w", err)
			}
			attributesJSON = raw
		}

		agent := &providers.Agent{
			ID:          req.ID,
			OUID:        req.OUID,
			OUHandle:    req.OUHandle,
			Type:        req.Type,
			Description: req.Description,
			LogoURL:     req.LogoURL,
			Owner:       req.Owner,
			Attributes:  attributesJSON,
			InboundAuthProfile: providers.InboundAuthProfile{
				AuthFlowID:                req.AuthFlowID,
				AuthFlowHandle:            req.AuthFlowHandle,
				RegistrationFlowID:        req.RegistrationFlowID,
				RegistrationFlowHandle:    req.RegistrationFlowHandle,
				IsRegistrationFlowEnabled: req.IsRegistrationFlowEnabled,
				RecoveryFlowID:            req.RecoveryFlowID,
				RecoveryFlowHandle:        req.RecoveryFlowHandle,
				IsRecoveryFlowEnabled:     req.IsRecoveryFlowEnabled,
				SignOutFlowID:             req.SignOutFlowID,
				SignOutFlowHandle:         req.SignOutFlowHandle,
				ThemeID:                   req.ThemeID,
				LayoutID:                  req.LayoutID,
				Assertion:                 req.Assertion,
				LoginConsent:              req.LoginConsent,
				AllowedUserTypes:          req.AllowedUserTypes,
				AllowedAgentTypes:         req.AllowedAgentTypes,
				PasskeyAllowedOrigins:     req.PasskeyAllowedOrigins,
				Attestation:               req.Attestation,
			},
			InboundAuthConfig: req.InboundAuthConfig,
		}
		clientID, clientSecret, _, svcErr := agentSvc.ValidateAgent(
			security.WithRuntimeContext(context.Background()), agent, req.ID)
		if svcErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to validate agent '%s': %v", req.ID, svcErr)
		}

		validUntil, err := resolveDeclarativeValidUntil(req.ValidUntil)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to validate agent '%s': %w", req.ID, err)
		}

		// agent.OUID may have been resolved from OUHandle by ValidateAgent.
		e, sysCredsJSON, buildErr := buildAgentEntity(
			req.ID, req.Type, agent.OUID, attributesJSON,
			req.Description, req.Owner, clientID, clientSecret, validUntil,
		)
		if buildErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to build agent entity for '%s': %w", req.ID, buildErr)
		}

		sysCredsJSON, err = mergeDeclaredCredentials(sysCredsJSON, req.Credentials)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to merge credentials of agent '%s': %w", req.ID, err)
		}

		return e, nil, sysCredsJSON, nil
	}
}

// mergeDeclaredCredentials adds an agent's declared credentials to its system credentials, which the
// loader hashes before they are stored. A plain-text value is hashed there; a stored-format list
// passes through. An empty value is left out, as the loader would drop it.
func mergeDeclaredCredentials(
	sysCredsJSON json.RawMessage, declared map[string]interface{},
) (json.RawMessage, error) {
	if len(declared) == 0 {
		return sysCredsJSON, nil
	}
	merged := map[string]interface{}{}
	if len(sysCredsJSON) > 0 {
		if err := json.Unmarshal(sysCredsJSON, &merged); err != nil {
			return nil, err
		}
	}
	for name, value := range declared {
		merged[name] = value
	}
	return json.Marshal(merged)
}

// makeAgentInboundConfig returns the inbound client loader configuration for agents.
func makeAgentInboundConfig(agentSvc AgentServiceInterface) inboundmodel.DeclarativeLoaderConfig {
	return inboundmodel.DeclarativeLoaderConfig{
		ResourceType:  "Agent",
		DirectoryName: "agents",
		Parser:        makeAgentInboundParser(agentSvc),
		Validator: func(p *inboundmodel.InboundClient) error {
			if p == nil {
				return fmt.Errorf("parsed inbound client is nil")
			}
			return nil
		},
	}
}

// makeAgentInboundParser returns a parser that converts agent YAML into an InboundClient.
func makeAgentInboundParser(agentSvc AgentServiceInterface) func([]byte) (*inboundmodel.InboundClient, error) {
	return func(data []byte) (*inboundmodel.InboundClient, error) {
		var req model.AgentRequestWithID
		if err := yaml.Unmarshal(data, &req); err != nil {
			return nil, fmt.Errorf("failed to parse agent YAML: %w", err)
		}

		agent := &providers.Agent{
			ID:          req.ID,
			OUID:        req.OUID,
			OUHandle:    req.OUHandle,
			Type:        req.Type,
			Description: req.Description,
			LogoURL:     req.LogoURL,
			Owner:       req.Owner,
			InboundAuthProfile: providers.InboundAuthProfile{
				AuthFlowID:                req.AuthFlowID,
				AuthFlowHandle:            req.AuthFlowHandle,
				RegistrationFlowID:        req.RegistrationFlowID,
				RegistrationFlowHandle:    req.RegistrationFlowHandle,
				IsRegistrationFlowEnabled: req.IsRegistrationFlowEnabled,
				RecoveryFlowID:            req.RecoveryFlowID,
				RecoveryFlowHandle:        req.RecoveryFlowHandle,
				IsRecoveryFlowEnabled:     req.IsRecoveryFlowEnabled,
				SignOutFlowID:             req.SignOutFlowID,
				SignOutFlowHandle:         req.SignOutFlowHandle,
				ThemeID:                   req.ThemeID,
				LayoutID:                  req.LayoutID,
				Assertion:                 req.Assertion,
				LoginConsent:              req.LoginConsent,
				AllowedUserTypes:          req.AllowedUserTypes,
				AllowedAgentTypes:         req.AllowedAgentTypes,
				PasskeyAllowedOrigins:     req.PasskeyAllowedOrigins,
				Attestation:               req.Attestation,
			},
			InboundAuthConfig: req.InboundAuthConfig,
		}
		_, _, resolvedClient, svcErr := agentSvc.ValidateAgent(
			security.WithRuntimeContext(context.Background()), agent, req.ID)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to validate agent '%s': %v", req.ID, svcErr)
		}

		resolvedClient.ID = req.ID
		resolvedClient.IsReadOnly = true
		setLogoProperty(&resolvedClient, req.LogoURL)

		oauthProfile := buildOAuthProfile(req.InboundAuthConfig)
		if oauthProfile != nil {
			if resolvedClient.Properties == nil {
				resolvedClient.Properties = make(map[string]interface{})
			}
			resolvedClient.Properties[inboundclient.PropOAuthProfile] = *oauthProfile
		}

		return &resolvedClient, nil
	}
}
