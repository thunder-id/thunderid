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

package application

import (
	"context"
	"fmt"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/application/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

// applicationTools provides MCP tools for managing  applications.
type applicationTools struct {
	appService ApplicationServiceInterface
}

// registerMCPTools registers all application MCP tools with the server.
func registerMCPTools(server *mcp.Server, appService ApplicationServiceInterface) {
	tools := &applicationTools{
		appService: appService,
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_list_applications",
		Description: `List all registered applications.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Applications",
			ReadOnlyHint: true,
		},
	}, tools.listApplications)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_get_application_by_id",
		Description: `Retrieve full details of an application by ID including OAuth settings, ` +
			`customizations, and flow associations.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Application by ID",
			ReadOnlyHint: true,
		},
	}, tools.getApplicationByID)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_get_application_by_client_id",
		Description: `Retrieve full details of an application by client_id including OAuth ` +
			`settings, customizations, and flow associations.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Application by Client ID",
			ReadOnlyHint: true,
		},
	}, tools.getApplicationByClientID)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_create_application",
		Description: `Create a new application optionally with OAuth configuration.

Use get_application_templates to get pre-configured minimal templates for common app types (SPA, Mobile, Server, M2M).

Prerequisites:
- Create flows first using create_flow if custom authentication/registration flows are needed.
- Use thunderid_list_themes to pick a theme and set themeId before creating the application
  (skip for M2M — no login page).

Behavior:
- If auth_flow_id is omitted, the default authentication flow is used.
- If user_attributes are omitted in token configs, defaults are applied.
- themeId (Go: InboundAuthProfile.ThemeID) controls the login page appearance. It is a top-level
  field in the request body, not nested under inboundAuthProfile.`,
		InputSchema: getCreateAppSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Create Application",
			IdempotentHint: false,
		},
	}, tools.createApplication)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_update_application",
		Description: `Update an existing application (full replacement).

Provide the COMPLETE application object to update the application.

Workflow:
1. Use get_application_by_id to get current state
2. Modify the fields you want to change
3. Send the complete object back

Any field not provided will be reset to empty/default.`,
		InputSchema: getUpdateAppSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Update Application",
			IdempotentHint: true,
		},
	}, tools.updateApplication)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_get_application_templates",
		Description: `Get minimal OAuth configuration templates for common application types.

Templates contain ONLY the required fields to create each app type. ` +
			`Optional fields with service-layer defaults are omitted.

Theme: SPA, Mobile, and Server templates include a themeId field that controls the ` +
			`visual appearance of the ThunderID-hosted login page. ` +
			`Call thunderid_list_themes first to see available themes, then replace the ` +
			`<THEME_ID> placeholder with the ID of the desired theme. ` +
			`Omit themeId to use the system default theme. ` +
			`M2M templates do not include themeId — M2M apps have no login page.

Prompt the user for any missing required placeholders.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Application Templates",
			ReadOnlyHint: true,
		},
	}, tools.getApplicationTemplates)
}

// listApplications handles the list_applications tool call.
func (t *applicationTools) listApplications(
	ctx context.Context,
	req *mcp.CallToolRequest,
	_ any,
) (*mcp.CallToolResult, model.ApplicationListOutput, error) {
	listResponse, svcErr := t.appService.GetApplicationList(ctx)
	if svcErr != nil {
		return nil, model.ApplicationListOutput{},
			fmt.Errorf("failed to list applications: %s", svcErr.ErrorDescription)
	}

	return nil, model.ApplicationListOutput{
		TotalCount:   listResponse.TotalResults,
		Applications: listResponse.Applications,
	}, nil
}

// getApplicationByID handles the get_application_by_id tool call.
func (t *applicationTools) getApplicationByID(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *model.Application, error) {
	app, svcErr := t.appService.GetApplication(ctx, input.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get application: %s", svcErr.ErrorDescription)
	}

	return nil, app, nil
}

// getApplicationByClientID handles the get_application_by_client_id tool call.
func (t *applicationTools) getApplicationByClientID(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input model.ClientIDInput,
) (*mcp.CallToolResult, *model.Application, error) {
	// Get OAuth application to find app ID
	oauthApp, svcErr := t.appService.GetOAuthApplication(ctx, input.ClientID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get OAuth application: %s", svcErr.ErrorDescription)
	}

	// Get full application details
	app, svcErr := t.appService.GetApplication(ctx, oauthApp.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get application: %s", svcErr.ErrorDescription)
	}

	return nil, app, nil
}

// createApplication handles the create_application tool call.
func (t *applicationTools) createApplication(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input model.ApplicationDTO,
) (*mcp.CallToolResult, *model.ApplicationDTO, error) {
	createdApp, svcErr := t.appService.CreateApplication(ctx, &input)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to create application: %s", svcErr.ErrorDescription)
	}

	return nil, createdApp, nil
}

// updateApplication handles the update_application tool call with complete replacement.
func (t *applicationTools) updateApplication(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input model.ApplicationDTO,
) (*mcp.CallToolResult, *model.ApplicationDTO, error) {
	updatedApp, svcErr := t.appService.UpdateApplication(ctx, input.ID, &input)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update application: %s", svcErr.ErrorDescription)
	}

	return nil, updatedApp, nil
}

// getApplicationTemplates handles the get_application_templates tool call.
// Returns pre-configured templates with placeholder values for common application types.
func (t *applicationTools) getApplicationTemplates(
	ctx context.Context,
	req *mcp.CallToolRequest,
	_ any,
) (*mcp.CallToolResult, map[string]model.ApplicationDTO, error) {
	templates := map[string]model.ApplicationDTO{
		"spa": {
			OUID: "<OU_ID>",
			Name: "<APP_NAME>",
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				ThemeID: "<THEME_ID>",
			},
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						RedirectURIs: []string{"<REDIRECT_URI>"},
						GrantTypes: []oauth2const.GrantType{
							oauth2const.GrantTypeAuthorizationCode,
							oauth2const.GrantTypeRefreshToken,
						},
						TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodNone,
						PKCERequired:            true,
						PublicClient:            true,
						Scopes:                  model.DefaultScopes,
						Token: &inboundmodel.OAuthTokenConfig{
							IDToken: &inboundmodel.IDTokenConfig{
								UserAttributes: model.DefaultUserAttributes,
							},
						},
						UserInfo: &inboundmodel.UserInfoConfig{
							ResponseType:   inboundmodel.UserInfoResponseTypeJSON,
							UserAttributes: model.DefaultUserAttributes,
						},
					},
				},
			},
		},
		"mobile": {
			OUID: "<OU_ID>",
			Name: "<APP_NAME>",
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				ThemeID: "<THEME_ID>",
			},
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						RedirectURIs: []string{"<CUSTOM_SCHEME>://callback"},
						GrantTypes: []oauth2const.GrantType{
							oauth2const.GrantTypeAuthorizationCode,
							oauth2const.GrantTypeRefreshToken,
						},
						TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodNone,
						PKCERequired:            true,
						PublicClient:            true,
						Scopes:                  model.DefaultScopes,
						Token: &inboundmodel.OAuthTokenConfig{
							IDToken: &inboundmodel.IDTokenConfig{
								UserAttributes: model.DefaultUserAttributes,
							},
						},
						UserInfo: &inboundmodel.UserInfoConfig{
							ResponseType:   inboundmodel.UserInfoResponseTypeJSON,
							UserAttributes: model.DefaultUserAttributes,
						},
					},
				},
			},
		},
		"server": {
			OUID: "<OU_ID>",
			Name: "<APP_NAME>",
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				ThemeID: "<THEME_ID>",
			},
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						RedirectURIs: []string{"<REDIRECT_URI>"},
						GrantTypes: []oauth2const.GrantType{
							oauth2const.GrantTypeAuthorizationCode,
							oauth2const.GrantTypeRefreshToken,
						},
						TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
						PKCERequired:            true,
						Scopes:                  model.DefaultScopes,
						Token: &inboundmodel.OAuthTokenConfig{
							IDToken: &inboundmodel.IDTokenConfig{
								UserAttributes: model.DefaultUserAttributes,
							},
						},
						UserInfo: &inboundmodel.UserInfoConfig{
							ResponseType:   inboundmodel.UserInfoResponseTypeJSON,
							UserAttributes: model.DefaultUserAttributes,
						},
					},
				},
			},
		},
		"m2m": {
			OUID: "<OU_ID>",
			Name: "<APP_NAME>",
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						GrantTypes:              []oauth2const.GrantType{oauth2const.GrantTypeClientCredentials},
						TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethodClientSecretBasic,
					},
				},
			},
		},
	}

	return nil, templates, nil
}

// getCommonSchemaModifiers returns the common schema modifiers for ApplicationDTO.
func getCommonSchemaModifiers() []func(*jsonschema.Schema) {
	return []func(*jsonschema.Schema){
		tool.WithEnum("inbound_auth_config.config", "grant_types", oauth2const.GetSupportedGrantTypes()),
		tool.WithEnum("inbound_auth_config.config", "response_types", oauth2const.GetSupportedResponseTypes()),
		tool.WithEnum("inbound_auth_config.config", "token_endpoint_auth_method",
			oauth2const.GetSupportedTokenEndpointAuthMethods()),
		tool.WithEnum("inbound_auth_config", "type", []string{string(inboundmodel.OAuthInboundAuthType)}),
	}
}

// getCreateAppSchema generates the schema for create_application tool.
func getCreateAppSchema() *jsonschema.Schema {
	modifiers := getCommonSchemaModifiers()
	modifiers = append(modifiers, tool.WithRemove("", "id"))
	return tool.GenerateSchema[model.ApplicationDTO](modifiers...)
}

// getUpdateAppSchema generates the schema for update_application tool.
func getUpdateAppSchema() *jsonschema.Schema {
	modifiers := getCommonSchemaModifiers()
	modifiers = append(modifiers, tool.WithRequired("", "id"))
	return tool.GenerateSchema[model.ApplicationDTO](modifiers...)
}
