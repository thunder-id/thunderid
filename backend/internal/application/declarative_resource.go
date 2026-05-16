/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package application

import (
	"context"
	"fmt"
	"testing"

	"encoding/json"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeApplication = "application"
	paramTypApplication     = "Application"
)

// applicationExporter implements declarativeresource.ResourceExporter for applications.
type applicationExporter struct {
	service ApplicationServiceInterface
}

// newApplicationExporter creates a new application exporter.
func newApplicationExporter(service ApplicationServiceInterface) *applicationExporter {
	return &applicationExporter{service: service}
}

// NewApplicationExporterForTest creates a new application exporter for testing purposes.
func NewApplicationExporterForTest(service ApplicationServiceInterface) *applicationExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newApplicationExporter(service)
}

// GetResourceType returns the resource type for applications.
func (e *applicationExporter) GetResourceType() string {
	return resourceTypeApplication
}

// GetParameterizerType returns the parameterizer type for applications.
func (e *applicationExporter) GetParameterizerType() string {
	return paramTypApplication
}

// GetAllResourceIDs retrieves all application IDs.
// In composite mode, this excludes declarative (YAML-based) applications.
func (e *applicationExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	apps, err := e.service.GetApplicationList(ctx)
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	ids := make([]string, 0, len(apps.Applications))
	for _, app := range apps.Applications {
		// Only include mutable (database-backed) applications
		if !app.IsReadOnly {
			ids = append(ids, app.ID)
		}
	}
	return ids, nil
}

// GetResourceByID retrieves an application by its ID.
func (e *applicationExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *serviceerror.ServiceError,
) {
	app, err := e.service.GetApplication(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return app, app.Name, nil
}

// ValidateResource validates an application resource.
func (e *applicationExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	app, ok := resource.(*model.Application)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeApplication, id)
	}

	if err := declarativeresource.ValidateResourceName(
		app.Name, resourceTypeApplication, id, "APP_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return app.Name, nil
}

// makeAppInboundConfig creates the inbound client declarative loader config for applications.
func makeAppInboundConfig(appService ApplicationServiceInterface) inboundmodel.DeclarativeLoaderConfig {
	return inboundmodel.DeclarativeLoaderConfig{
		ResourceType:  "Application",
		DirectoryName: "applications",
		Parser:        makeAppInboundParser(appService),
		Validator: func(p *inboundmodel.InboundClient) error {
			if p == nil {
				return fmt.Errorf("parsed profile is nil")
			}
			return nil
		},
	}
}

// makeAppInboundParser returns a parser that converts application YAML bytes into an InboundClient.
func makeAppInboundParser(appService ApplicationServiceInterface) func([]byte) (*inboundmodel.InboundClient, error) {
	return func(data []byte) (*inboundmodel.InboundClient, error) {
		appDTO, err := parseToApplicationDTO(data)
		if err != nil {
			return nil, err
		}
		validatedApp, _, svcErr := appService.ValidateApplication(context.Background(), appDTO)
		if svcErr != nil {
			return nil, fmt.Errorf("error validating application '%s': %v", appDTO.Name, svcErr)
		}

		profile := toInboundClient(validatedApp)

		for _, inbound := range validatedApp.InboundAuthConfig {
			if oauthProfile := buildOAuthProfileFromProcessed(inbound); oauthProfile != nil {
				if profile.Properties == nil {
					profile.Properties = make(map[string]interface{})
				}
				profile.Properties[inboundclient.PropOAuthProfile] = *oauthProfile
				break
			}
		}
		return &profile, nil
	}
}

// parseToApplicationDTO unmarshals YAML bytes into an ApplicationDTO.
func parseToApplicationDTO(data []byte) (*model.ApplicationDTO, error) {
	var appRequest model.ApplicationRequestWithID
	err := yaml.Unmarshal(data, &appRequest)
	if err != nil {
		return nil, err
	}

	appDTO := model.ApplicationDTO{
		ID:          appRequest.ID,
		OUID:        appRequest.OUID,
		Name:        appRequest.Name,
		Description: appRequest.Description,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			AuthFlowID:                appRequest.AuthFlowID,
			RegistrationFlowID:        appRequest.RegistrationFlowID,
			IsRegistrationFlowEnabled: appRequest.IsRegistrationFlowEnabled,
			RecoveryFlowID:            appRequest.RecoveryFlowID,
			IsRecoveryFlowEnabled:     appRequest.IsRecoveryFlowEnabled,
			ThemeID:                   appRequest.ThemeID,
			LayoutID:                  appRequest.LayoutID,
			Assertion:                 appRequest.Assertion,
			Certificate:               appRequest.Certificate,
			AllowedUserTypes:          appRequest.AllowedUserTypes,
			LoginConsent:              appRequest.LoginConsent,
		},
		Template:  appRequest.Template,
		URL:       appRequest.URL,
		LogoURL:   appRequest.LogoURL,
		TosURI:    appRequest.TosURI,
		PolicyURI: appRequest.PolicyURI,
		Contacts:  appRequest.Contacts,
		Metadata:  appRequest.Metadata,
	}
	if len(appRequest.InboundAuthConfig) > 0 {
		inboundAuthConfigDTOs := make([]inboundmodel.InboundAuthConfigWithSecret, 0)
		for _, config := range appRequest.InboundAuthConfig {
			if config.Type != inboundmodel.OAuthInboundAuthType || config.OAuthConfig == nil {
				continue
			}

			inboundAuthConfigDTO := inboundmodel.InboundAuthConfigWithSecret{
				Type: config.Type,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:                           config.OAuthConfig.ClientID,
					ClientSecret:                       config.OAuthConfig.ClientSecret,
					RedirectURIs:                       config.OAuthConfig.RedirectURIs,
					GrantTypes:                         config.OAuthConfig.GrantTypes,
					ResponseTypes:                      config.OAuthConfig.ResponseTypes,
					TokenEndpointAuthMethod:            config.OAuthConfig.TokenEndpointAuthMethod,
					PKCERequired:                       config.OAuthConfig.PKCERequired,
					PublicClient:                       config.OAuthConfig.PublicClient,
					RequirePushedAuthorizationRequests: config.OAuthConfig.RequirePushedAuthorizationRequests,
					DPoPBoundAccessTokens:              config.OAuthConfig.DPoPBoundAccessTokens,
					Token:                              config.OAuthConfig.Token,
					Scopes:                             config.OAuthConfig.Scopes,
					UserInfo:                           config.OAuthConfig.UserInfo,
					ScopeClaims:                        config.OAuthConfig.ScopeClaims,
					Certificate:                        config.OAuthConfig.Certificate,
				},
			}
			inboundAuthConfigDTOs = append(inboundAuthConfigDTOs, inboundAuthConfigDTO)
		}
		appDTO.InboundAuthConfig = inboundAuthConfigDTOs
	}
	return &appDTO, nil
}

// GetResourceRules returns the parameterization rules for applications.
func (e *applicationExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables: []string{
			"InboundAuthConfig[].OAuthConfig.ClientID",
			"InboundAuthConfig[].OAuthConfig.ClientSecret",
		},
		ArrayVariables: []string{
			"InboundAuthConfig[].OAuthConfig.RedirectURIs",
		},
	}
}

// GetResourceRulesForResource returns parameterization rules tailored to the specific application
// instance. Public clients do not have a client secret, so the ClientSecret variable is excluded
// from their export to avoid injecting an empty or invalid placeholder into the YAML template.
func (e *applicationExporter) GetResourceRulesForResource(resource interface{}) *declarativeresource.ResourceRules {
	app, ok := resource.(*model.Application)
	if !ok {
		return e.GetResourceRules()
	}

	for _, inbound := range app.InboundAuthConfig {
		if inbound.OAuthConfig != nil && inbound.OAuthConfig.PublicClient {
			return &declarativeresource.ResourceRules{
				Variables: []string{
					"InboundAuthConfig[].OAuthConfig.ClientID",
				},
				ArrayVariables: []string{
					"InboundAuthConfig[].OAuthConfig.RedirectURIs",
				},
			}
		}
	}

	return e.GetResourceRules()
}

// makeAppDeclarativeConfig creates the declarative loader config for loading application
// identity data into the entity file store.
func makeAppDeclarativeConfig(appService ApplicationServiceInterface) entity.DeclarativeLoaderConfig {
	return entity.DeclarativeLoaderConfig{
		Directory: "applications",
		Category:  entity.EntityCategoryApp,
		Parser:    makeAppEntityParser(appService),
	}
}

// makeAppEntityParser creates a parser that converts application YAML into an entity.
func makeAppEntityParser(
	appService ApplicationServiceInterface,
) func(data []byte) (*entity.Entity, json.RawMessage, json.RawMessage, error) {
	return func(data []byte) (*entity.Entity, json.RawMessage, json.RawMessage, error) {
		if appService == nil {
			return nil, nil, nil, fmt.Errorf("application service is required for declarative entity parsing")
		}

		appDTO, err := parseToApplicationDTO(data)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse application YAML: %w", err)
		}

		_, inboundAuthConfig, svcErr := appService.ValidateApplication(context.Background(), appDTO)
		if svcErr != nil {
			return nil, nil, nil, fmt.Errorf("error validating application '%s': %v", appDTO.Name, svcErr)
		}

		var clientID, clientSecret string
		if inboundAuthConfig != nil && inboundAuthConfig.OAuthConfig != nil {
			clientID = inboundAuthConfig.OAuthConfig.ClientID
			clientSecret = inboundAuthConfig.OAuthConfig.ClientSecret
		}

		sysAttrsJSON, err := buildSystemAttributes(appDTO, clientID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to build system attributes: %w", err)
		}

		sysCredsJSON, err := buildSystemCredentials(clientSecret)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to build system credentials: %w", err)
		}

		e := &entity.Entity{
			ID:               appDTO.ID,
			Category:         entity.EntityCategoryApp,
			Type:             "application",
			State:            entity.EntityStateActive,
			OUID:             appDTO.OUID,
			SystemAttributes: sysAttrsJSON,
		}

		return e, nil, sysCredsJSON, nil
	}
}
