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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
)

// ValidateApplicationWrapperTestSuite tests the validateApplicationWrapper function.
// ParseToApplicationDTOTestSuite tests the parseToApplicationDTO function.
type ParseToApplicationDTOTestSuite struct {
	suite.Suite
}

func TestParseToApplicationDTOTestSuite(t *testing.T) {
	suite.Run(t, new(ParseToApplicationDTOTestSuite))
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_AllFieldsParsed() {
	// Test that all fields including new ones are parsed correctly
	yamlData := `
id: test-app-001
name: Test Application
description: A test application
auth_flow_id: flow-123
registration_flow_id: flow-reg-456
is_registration_flow_enabled: true
theme_id: theme-blue
layout_id: layout-standard
template: web
url: https://example.com
logo_url: https://example.com/logo.png
tos_uri: https://example.com/tos
policy_uri: https://example.com/policy
contacts:
  - admin@example.com
  - support@example.com
assertion:
  validity_period: 3600
  user_attributes:
    - email
    - username
certificate:
  type: PEM
  value: |
    -----BEGIN CERTIFICATE-----
    MIIDazCCAlOgAwIBAgI...
    -----END CERTIFICATE-----
allowed_user_types:
  - internal
  - external
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), appDTO)
	assert.Equal(s.T(), "test-app-001", appDTO.ID)
	assert.Equal(s.T(), "Test Application", appDTO.Name)
	assert.Equal(s.T(), "A test application", appDTO.Description)
	assert.Equal(s.T(), "flow-123", appDTO.AuthFlowID)
	assert.Equal(s.T(), "flow-reg-456", appDTO.RegistrationFlowID)
	assert.True(s.T(), appDTO.IsRegistrationFlowEnabled)
	assert.Equal(s.T(), "theme-blue", appDTO.ThemeID)
	assert.Equal(s.T(), "layout-standard", appDTO.LayoutID)
	assert.Equal(s.T(), "web", appDTO.Template)
	assert.Equal(s.T(), "https://example.com", appDTO.URL)
	assert.Equal(s.T(), "https://example.com/logo.png", appDTO.LogoURL)
	assert.Equal(s.T(), "https://example.com/tos", appDTO.TosURI)
	assert.Equal(s.T(), "https://example.com/policy", appDTO.PolicyURI)
	assert.Equal(s.T(), 2, len(appDTO.Contacts))
	assert.Contains(s.T(), appDTO.Contacts, "admin@example.com")
	assert.Contains(s.T(), appDTO.Contacts, "support@example.com")
	assert.NotNil(s.T(), appDTO.Assertion)
	assert.Equal(s.T(), int64(3600), appDTO.Assertion.ValidityPeriod)
	assert.Equal(s.T(), 2, len(appDTO.AllowedUserTypes))
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_MinimalFields() {
	// Test with only required fields
	yamlData := `
id: minimal-app
name: Minimal App
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), appDTO)
	assert.Equal(s.T(), "minimal-app", appDTO.ID)
	assert.Equal(s.T(), "Minimal App", appDTO.Name)
	assert.Equal(s.T(), "", appDTO.Description)
	assert.Equal(s.T(), "", appDTO.ThemeID)
	assert.Equal(s.T(), "", appDTO.LayoutID)
	assert.Equal(s.T(), "", appDTO.Template)
	assert.Equal(s.T(), "", appDTO.TosURI)
	assert.Equal(s.T(), "", appDTO.PolicyURI)
	assert.Equal(s.T(), 0, len(appDTO.Contacts))
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_ThemeAndLayoutParsed() {
	// Test that theme_id and layout_id are properly parsed
	yamlData := `
id: themed-app
name: Themed Application
theme_id: modern-theme
layout_id: two-column
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "themed-app", appDTO.ID)
	assert.Equal(s.T(), "modern-theme", appDTO.ThemeID)
	assert.Equal(s.T(), "two-column", appDTO.LayoutID)
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_URIFieldsParsed() {
	// Test that URI fields (tos_uri, policy_uri) are properly parsed
	yamlData := `
id: legal-app
name: Legal Application
tos_uri: https://example.com/terms-of-service
policy_uri: https://example.com/privacy-policy
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "legal-app", appDTO.ID)
	assert.Equal(s.T(), "https://example.com/terms-of-service", appDTO.TosURI)
	assert.Equal(s.T(), "https://example.com/privacy-policy", appDTO.PolicyURI)
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_ContactsParsed() {
	// Test that contacts array is properly parsed
	yamlData := `
id: contact-app
name: Contact Application
contacts:
  - primary@example.com
  - secondary@example.com
  - tertiary@example.com
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), 3, len(appDTO.Contacts))
	assert.Equal(s.T(), "primary@example.com", appDTO.Contacts[0])
	assert.Equal(s.T(), "secondary@example.com", appDTO.Contacts[1])
	assert.Equal(s.T(), "tertiary@example.com", appDTO.Contacts[2])
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_InvalidYAML() {
	// Test parsing invalid YAML
	yamlData := `
id: test-app
name: Test App
invalid yaml: [unclosed bracket
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.NotNil(s.T(), err)
	assert.Nil(s.T(), appDTO)
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_WithOAuthConfig() {
	// Test parsing application with OAuth inbound auth config
	yamlData := `
id: oauth-app
name: OAuth Application
auth_flow_id: flow-123
url: https://example.com
inbound_auth_config:
  - type: oauth2
    config:
      client_id: client-123
      client_secret: secret-456
      redirect_uris:
        - https://example.com/callback
      grant_types:
        - authorization_code
        - refresh_token
      response_types:
        - code
      token_endpoint_auth_method: client_secret_basic
      pkce_required: true
      public_client: false
      scopes:
        - openid
        - email
        - profile
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "oauth-app", appDTO.ID)
	assert.Equal(s.T(), "OAuth Application", appDTO.Name)

	// Verify OAuth configuration was parsed correctly
	assert.Len(s.T(), appDTO.InboundAuthConfig, 1)
	assert.Equal(s.T(), inboundmodel.OAuthInboundAuthType, appDTO.InboundAuthConfig[0].Type)

	oauthConfig := appDTO.InboundAuthConfig[0].OAuthConfig
	assert.NotNil(s.T(), oauthConfig)
	assert.Equal(s.T(), "client-123", oauthConfig.ClientID)
	assert.Equal(s.T(), "secret-456", oauthConfig.ClientSecret)
	assert.Contains(s.T(), oauthConfig.RedirectURIs, "https://example.com/callback")
	assert.Contains(s.T(), oauthConfig.GrantTypes, oauth2const.GrantType("authorization_code"))
	assert.Contains(s.T(), oauthConfig.GrantTypes, oauth2const.GrantType("refresh_token"))
	assert.True(s.T(), oauthConfig.PKCERequired)
	assert.False(s.T(), oauthConfig.PublicClient)
	assert.Contains(s.T(), oauthConfig.Scopes, "openid")
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_TemplateAndOtherFieldsCombination() {
	// Test template field with other new fields
	yamlData := `
id: template-app
name: Template Application
template: single-page-app
theme_id: corporate-theme
layout_id: horizontal
tos_uri: https://example.com/tos
policy_uri: https://example.com/policy
contacts:
  - admin@example.com
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "template-app", appDTO.ID)
	assert.Equal(s.T(), "single-page-app", appDTO.Template)
	assert.Equal(s.T(), "corporate-theme", appDTO.ThemeID)
	assert.Equal(s.T(), "horizontal", appDTO.LayoutID)
	assert.Equal(s.T(), "https://example.com/tos", appDTO.TosURI)
	assert.Equal(s.T(), "https://example.com/policy", appDTO.PolicyURI)
	assert.Equal(s.T(), 1, len(appDTO.Contacts))
	assert.Equal(s.T(), "admin@example.com", appDTO.Contacts[0])
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_EmptyContacts() {
	// Test that empty contacts is handled correctly
	yamlData := `
id: no-contact-app
name: No Contact App
contacts: []
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), 0, len(appDTO.Contacts))
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_PreservesOrderAndTypes() {
	// Test that field types are preserved correctly
	yamlData := `
id: type-test-app
name: Type Test App
is_registration_flow_enabled: false
theme_id: ""
layout_id: default
contacts:
  - test@example.com
allowed_user_types:
  - internal
  - external
  - guest
`

	appDTO, err := parseToApplicationDTO([]byte(yamlData))

	assert.Nil(s.T(), err)
	assert.False(s.T(), appDTO.IsRegistrationFlowEnabled)
	assert.Equal(s.T(), "", appDTO.ThemeID)
	assert.Equal(s.T(), "default", appDTO.LayoutID)
	assert.Equal(s.T(), 3, len(appDTO.AllowedUserTypes))
	assert.Contains(s.T(), appDTO.AllowedUserTypes, "guest")
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_WithMetadata() {
	yamlData := []byte(`
id: app-123
name: Test App
metadata:
  env: production
  team: platform
`)

	dto, err := parseToApplicationDTO(yamlData)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), dto)
	assert.Equal(s.T(), "app-123", dto.ID)
	assert.Equal(s.T(), "Test App", dto.Name)
	assert.NotNil(s.T(), dto.Metadata)
	assert.Equal(s.T(), "production", dto.Metadata["env"])
	assert.Equal(s.T(), "platform", dto.Metadata["team"])
}

func (s *ParseToApplicationDTOTestSuite) TestMakeAppEntityParser_PublicClientWithoutSecretStoresClientID() {
	yamlData := []byte(`
id: public-oauth-app
name: Public OAuth Application
inbound_auth_config:
  - type: oauth2
    config:
      client_id: public-client-id-123
      pkce_required: true
      public_client: true
`)

	mockAppService := NewApplicationServiceInterfaceMock(s.T())
	mockAppService.EXPECT().ValidateApplication(mock.Anything, mock.Anything).Return(
		&model.ApplicationProcessedDTO{ID: "public-oauth-app", Name: "Public OAuth Application"},
		&inboundmodel.InboundAuthConfigWithSecret{
			Type: inboundmodel.OAuthInboundAuthType,
			OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
				ClientID: "public-client-id-123",
			},
		},
		nil,
	)

	parser := makeAppEntityParser(mockAppService)
	entityObj, _, sysCredsJSON, err := parser(yamlData)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), entityObj)
	assert.Nil(s.T(), sysCredsJSON)

	var sysAttrs map[string]interface{}
	err = json.Unmarshal(entityObj.SystemAttributes, &sysAttrs)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "Public OAuth Application", sysAttrs[fieldName])
	assert.Equal(s.T(), "public-client-id-123", sysAttrs[fieldClientID])
}

func (s *ParseToApplicationDTOTestSuite) TestMakeAppEntityParser_ConfidentialClientStoresSecretAndClientID() {
	yamlData := []byte(`
id: confidential-oauth-app
name: Confidential OAuth Application
inbound_auth_config:
  - type: oauth2
    config:
      client_id: confidential-client-id-123
      client_secret: confidential-secret-456
`)

	mockAppService := NewApplicationServiceInterfaceMock(s.T())
	mockAppService.EXPECT().ValidateApplication(mock.Anything, mock.Anything).Return(
		&model.ApplicationProcessedDTO{ID: "confidential-oauth-app", Name: "Confidential OAuth Application"},
		&inboundmodel.InboundAuthConfigWithSecret{
			Type: inboundmodel.OAuthInboundAuthType,
			OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
				ClientID:     "confidential-client-id-123",
				ClientSecret: "confidential-secret-456",
			},
		},
		nil,
	)

	parser := makeAppEntityParser(mockAppService)
	entityObj, _, sysCredsJSON, err := parser(yamlData)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), entityObj)
	assert.NotNil(s.T(), sysCredsJSON)

	var sysAttrs map[string]interface{}
	err = json.Unmarshal(entityObj.SystemAttributes, &sysAttrs)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "confidential-client-id-123", sysAttrs[fieldClientID])

	var sysCreds map[string]interface{}
	err = json.Unmarshal(sysCredsJSON, &sysCreds)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "confidential-secret-456", sysCreds[fieldClientSecret])
}

func (s *ParseToApplicationDTOTestSuite) TestMakeAppEntityParser_UsesValidatedGeneratedClientCredentials() {
	yamlData := []byte(`
id: generated-credentials-app
name: Generated Credentials App
inbound_auth_config:
  - type: oauth2
    config:
      grant_types:
        - client_credentials
`)

	mockAppService := NewApplicationServiceInterfaceMock(s.T())
	mockAppService.EXPECT().ValidateApplication(mock.Anything, mock.Anything).Run(
		func(_ context.Context, app *model.ApplicationDTO) {
			assert.Equal(s.T(), "generated-credentials-app", app.ID)
			assert.Equal(s.T(), "Generated Credentials App", app.Name)
		},
	).Return(
		&model.ApplicationProcessedDTO{ID: "generated-credentials-app", Name: "Generated Credentials App"},
		&inboundmodel.InboundAuthConfigWithSecret{
			Type: inboundmodel.OAuthInboundAuthType,
			OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
				ClientID:     "generated-client-id",
				ClientSecret: "generated-client-secret",
			},
		},
		nil,
	)

	parser := makeAppEntityParser(mockAppService)
	entityObj, _, sysCredsJSON, err := parser(yamlData)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), entityObj)
	assert.NotNil(s.T(), sysCredsJSON)

	var sysAttrs map[string]interface{}
	err = json.Unmarshal(entityObj.SystemAttributes, &sysAttrs)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "generated-client-id", sysAttrs[fieldClientID])

	var sysCreds map[string]interface{}
	err = json.Unmarshal(sysCredsJSON, &sysCreds)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "generated-client-secret", sysCreds[fieldClientSecret])
}

func (s *ParseToApplicationDTOTestSuite) TestMakeAppEntityParser_SelectsOAuthConfigWhenFirstInboundIsNotOAuth() {
	yamlData := []byte(`
id: mixed-inbound-app
name: Mixed Inbound App
inbound_auth_config:
  - type: saml
    config: {}
  - type: oauth2
    config:
      client_id: oauth-client-id
      client_secret: oauth-client-secret
`)

	mockAppService := NewApplicationServiceInterfaceMock(s.T())
	mockAppService.EXPECT().ValidateApplication(mock.Anything, mock.Anything).Run(
		func(_ context.Context, app *model.ApplicationDTO) {
			assert.Len(s.T(), app.InboundAuthConfig, 1)
			assert.Equal(s.T(), inboundmodel.OAuthInboundAuthType, app.InboundAuthConfig[0].Type)
			assert.Equal(s.T(), "oauth-client-id", app.InboundAuthConfig[0].OAuthConfig.ClientID)
		},
	).Return(
		&model.ApplicationProcessedDTO{ID: "mixed-inbound-app", Name: "Mixed Inbound App"},
		&inboundmodel.InboundAuthConfigWithSecret{
			Type: inboundmodel.OAuthInboundAuthType,
			OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
				ClientID:     "oauth-client-id",
				ClientSecret: "oauth-client-secret",
			},
		},
		nil,
	)

	parser := makeAppEntityParser(mockAppService)
	entityObj, _, sysCredsJSON, err := parser(yamlData)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), entityObj)
	assert.NotNil(s.T(), sysCredsJSON)

	var sysAttrs map[string]interface{}
	err = json.Unmarshal(entityObj.SystemAttributes, &sysAttrs)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "oauth-client-id", sysAttrs[fieldClientID])

	var sysCreds map[string]interface{}
	err = json.Unmarshal(sysCredsJSON, &sysCreds)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "oauth-client-secret", sysCreds[fieldClientSecret])
}

func (s *ParseToApplicationDTOTestSuite) TestParseToApplicationDTO_OUHandlePassedThrough() {
	yamlData := []byte("id: app-1\nname: My App\nou_handle: default\n")

	appDTO, err := parseToApplicationDTO(yamlData)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "default", appDTO.OUHandle)
	assert.Empty(s.T(), appDTO.OUID)
}
