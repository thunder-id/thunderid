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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type appExportRequest struct {
	Applications []string `json:"applications,omitempty"`
}

// appExportResponse mirrors export.JSONExportResponse: all file bodies concatenated into
// `resources` plus an .env-format `environment_variables` blob.
type appExportResponse struct {
	Resources            string `json:"resources"`
	EnvironmentVariables string `json:"environment_variables"`
}

type appImportRequest struct {
	Content   string                 `json:"content"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	DryRun    bool                   `json:"dryRun,omitempty"`
	Options   appImportOptions       `json:"options"`
}

type appImportOptions struct {
	Upsert          bool   `json:"upsert"`
	ContinueOnError bool   `json:"continueOnError"`
	Target          string `json:"target"`
}

type appImportResponse struct {
	Summary appImportSummary `json:"summary"`
	Results []appImportItem  `json:"results"`
}

type appImportSummary struct {
	TotalDocuments int `json:"totalDocuments"`
	Imported       int `json:"imported"`
	Failed         int `json:"failed"`
}

type appImportItem struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	Operation    string `json:"operation,omitempty"`
	Status       string `json:"status"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
}

// ApplicationImportExportSuite verifies the export → import lifecycle for applications,
// with particular emphasis on inline-embedded InboundAuthProfile fields.
type ApplicationImportExportSuite struct {
	suite.Suite
	ouID               string
	handleSuffix       string
	authFlowID         string
	registrationFlowID string
}

func TestApplicationImportExportSuite(t *testing.T) {
	suite.Run(t, new(ApplicationImportExportSuite))
}

func (s *ApplicationImportExportSuite) SetupSuite() {
	s.handleSuffix = fmt.Sprintf("%d", time.Now().UnixNano())

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "app-ie-ou-" + s.handleSuffix,
		Name:        "App Import Export OU " + s.handleSuffix,
		Description: "OU for application import-export lifecycle tests",
		Parent:      nil,
	})
	s.Require().NoError(err)
	s.ouID = ouID

	authFlowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "AUTHENTICATION")
	s.Require().NoError(err)
	s.Require().NotEmpty(authFlowID)
	s.authFlowID = authFlowID

	regFlowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "REGISTRATION")
	s.Require().NoError(err)
	s.Require().NotEmpty(regFlowID)
	s.registrationFlowID = regFlowID
}

func (s *ApplicationImportExportSuite) TearDownSuite() {
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

// TestExportImportRoundTrip_ConfidentialOAuthApp populates every settable field on a
// confidential OAuth application, exports it, deletes it, re-imports, and asserts every
// field survives the round-trip.
func (s *ApplicationImportExportSuite) TestExportImportRoundTrip_ConfidentialOAuthApp() {
	// Name becomes the parameterizer's variable-name prefix; must be alphanumeric + spaces
	// (spaces → underscores). Dashes are not stripped and would produce invalid template names.
	appName := "App RT Conf " + s.handleSuffix

	original := Application{
		OUID:                      s.ouID,
		Name:                      appName,
		Description:               "Round-trip confidential application",
		Template:                  "web",
		URL:                       "https://app-rt-conf.example.com",
		LogoURL:                   "https://app-rt-conf.example.com/logo.png",
		TosURI:                    "https://app-rt-conf.example.com/tos",
		PolicyURI:                 "https://app-rt-conf.example.com/policy",
		Contacts:                  []string{"admin@example.com", "support@example.com"},
		AuthFlowID:                s.authFlowID,
		RegistrationFlowID:        s.registrationFlowID,
		IsRegistrationFlowEnabled: true,
		Assertion: &AssertionConfig{
			ValidityPeriod: 3600,
			UserAttributes: []string{"email"},
		},
		LoginConsent: &LoginConsentConfig{
			ValidityPeriod: 86400,
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "app-rt-conf-client-" + s.handleSuffix,
					ClientSecret:            "app-rt-conf-secret-" + s.handleSuffix,
					RedirectURIs:            []string{"https://app-rt-conf.example.com/callback"},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
					Scopes:                  []string{"openid", "profile"},
					AcrValues:               []string{"urn:thunder:acr:password"},
					Token: &OAuthTokenConfig{
						AccessToken: &AccessTokenConfig{
							ValidityPeriod: 1800,
							UserAttributes: []string{"email"},
						},
						IDToken: &IDTokenConfig{
							ValidityPeriod: 1200,
							UserAttributes: []string{"email"},
						},
					},
					UserInfo: &UserInfoConfig{
						ResponseType:   "JSON",
						UserAttributes: []string{"email"},
					},
					ScopeClaims: map[string][]string{
						"profile": {"email"},
					},
				},
			},
		},
	}

	createdID, err := createApplication(original)
	s.Require().NoError(err, "failed to create source application")

	getResp, err := s.appGet(createdID)
	s.Require().NoError(err)
	var pre Application
	s.Require().NoError(json.Unmarshal(getResp, &pre))

	exportResp, err := s.exportApps(appExportRequest{Applications: []string{createdID}})
	s.Require().NoError(err)
	s.Require().NotEmpty(exportResp.Resources, "expected exported YAML in resources field")
	yamlContent := exportResp.Resources

	// Bare-`:` regression check (inline-embedded fields must flatten, not nest under an empty key).
	for _, line := range strings.Split(yamlContent, "\n") {
		s.Assert().NotEqual(":", strings.TrimSpace(line),
			"exported YAML must not contain a bare `:` key")
	}

	s.Assert().Contains(yamlContent, "# resource_type: application")
	s.Assert().Contains(yamlContent, "id: "+createdID)
	s.Assert().Contains(yamlContent, "ou_id: "+s.ouID)
	s.Assert().Contains(yamlContent, "name: "+appName)
	s.Assert().Contains(yamlContent, "description: Round-trip confidential application")
	s.Assert().Contains(yamlContent, "template: web")

	// Inline-embedded fields appear at the top level (flattened, not nested).
	s.Assert().Contains(yamlContent, "auth_flow_id: "+s.authFlowID)
	s.Assert().Contains(yamlContent, "registration_flow_id: "+s.registrationFlowID)
	s.Assert().Contains(yamlContent, "is_registration_flow_enabled: true")
	s.Assert().Contains(yamlContent, "assertion:")
	s.Assert().Contains(yamlContent, "validity_period: 3600")
	s.Assert().Contains(yamlContent, "login_consent:")
	s.Assert().Contains(yamlContent, "validity_period: 86400")
	s.Assert().Contains(yamlContent, "inbound_auth_config:")
	s.Assert().Contains(yamlContent, "authorization_code")
	s.Assert().Contains(yamlContent, "token_endpoint_auth_method: client_secret_basic")
	s.Assert().NotContains(yamlContent, "app-rt-conf-secret-"+s.handleSuffix)
	s.Assert().Contains(yamlContent, "{{")

	s.Require().NoError(deleteApplication(createdID))

	vars := s.extractTemplateVariables(yamlContent, map[string]interface{}{
		"client_id":     "app-rt-conf-client-" + s.handleSuffix,
		"client_secret": "app-rt-conf-secret-" + s.handleSuffix,
		"redirect_uris": []string{"https://app-rt-conf.example.com/callback"},
	})

	importResp, err := s.importApps(appImportRequest{
		Content:   yamlContent,
		Options:   appImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
		Variables: vars,
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.TotalDocuments)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	s.Require().Equal(0, importResp.Summary.Failed)
	s.Require().Len(importResp.Results, 1)
	s.Assert().Equal("application", importResp.Results[0].ResourceType)
	s.Assert().Equal("success", importResp.Results[0].Status)
	importedID := importResp.Results[0].ResourceID
	s.Assert().Equal(createdID, importedID)
	defer func() { _ = deleteApplication(importedID) }()

	restoredBody, err := s.appGet(importedID)
	s.Require().NoError(err)
	var restored Application
	s.Require().NoError(json.Unmarshal(restoredBody, &restored))

	s.Assert().Equal(createdID, restored.ID)
	s.Assert().Equal(pre.Name, restored.Name)
	s.Assert().Equal(pre.Description, restored.Description)
	s.Assert().Equal(pre.OUID, restored.OUID)
	s.Assert().Equal(pre.Template, restored.Template)
	s.Assert().Equal(pre.URL, restored.URL)
	s.Assert().Equal(pre.LogoURL, restored.LogoURL)
	s.Assert().Equal(pre.TosURI, restored.TosURI)
	s.Assert().Equal(pre.PolicyURI, restored.PolicyURI)
	s.Assert().ElementsMatch(pre.Contacts, restored.Contacts)

	// Inline-embedded InboundAuthProfile fields — coverage for the inline-walker code path.
	s.Assert().Equal(pre.AuthFlowID, restored.AuthFlowID)
	s.Assert().Equal(pre.RegistrationFlowID, restored.RegistrationFlowID)
	s.Assert().Equal(pre.IsRegistrationFlowEnabled, restored.IsRegistrationFlowEnabled)
	s.Require().NotNil(restored.Assertion)
	s.Assert().Equal(pre.Assertion.ValidityPeriod, restored.Assertion.ValidityPeriod)
	s.Require().NotNil(restored.LoginConsent)
	s.Assert().Equal(pre.LoginConsent.ValidityPeriod, restored.LoginConsent.ValidityPeriod)

	s.Require().Len(restored.InboundAuthConfig, 1)
	cfg := restored.InboundAuthConfig[0].OAuthAppConfig
	s.Require().NotNil(cfg)
	s.Assert().Equal("app-rt-conf-client-"+s.handleSuffix, cfg.ClientID)
	s.Assert().Empty(cfg.ClientSecret)
	s.Assert().ElementsMatch([]string{"authorization_code", "refresh_token"}, cfg.GrantTypes)
	s.Assert().ElementsMatch([]string{"code"}, cfg.ResponseTypes)
	s.Assert().Equal("client_secret_basic", cfg.TokenEndpointAuthMethod)
	s.Assert().False(cfg.PublicClient)
	s.Assert().ElementsMatch([]string{"https://app-rt-conf.example.com/callback"}, cfg.RedirectURIs)
	s.Assert().ElementsMatch([]string{"openid", "profile"}, cfg.Scopes)
	s.Assert().ElementsMatch([]string{"urn:thunder:acr:password"}, cfg.AcrValues)
	s.Require().NotNil(cfg.Token)
	s.Require().NotNil(cfg.Token.AccessToken)
	s.Assert().Equal(int64(1800), cfg.Token.AccessToken.ValidityPeriod)
	s.Require().NotNil(cfg.Token.IDToken)
	s.Assert().Equal(int64(1200), cfg.Token.IDToken.ValidityPeriod)
	s.Require().NotNil(cfg.UserInfo)
	s.Assert().Equal("JSON", cfg.UserInfo.ResponseType)
	s.Assert().ElementsMatch([]string{"email"}, cfg.UserInfo.UserAttributes)
	s.Require().NotNil(cfg.ScopeClaims)
	s.Assert().ElementsMatch([]string{"email"}, cfg.ScopeClaims["profile"])
}

// TestExportImportRoundTrip_PublicClientAuthCode covers the public OAuth variant
// (authorization_code + PKCE + none auth). The PerResourceRuler must omit the ClientSecret
// template variable for public clients.
func (s *ApplicationImportExportSuite) TestExportImportRoundTrip_PublicClientAuthCode() {
	redirectURI := "https://app-rt-public.example.com/callback"
	appName := "App RT Public " + s.handleSuffix
	clientIDLiteral := "app-rt-public-client-" + s.handleSuffix

	original := Application{
		OUID:        s.ouID,
		Name:        appName,
		Description: "Round-trip public client application",
		AuthFlowID:  s.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                clientIDLiteral,
					RedirectURIs:            []string{redirectURI},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "none",
					PKCERequired:            true,
					PublicClient:            true,
				},
			},
		},
	}

	createdID, err := createApplication(original)
	s.Require().NoError(err)

	getResp, err := s.appGet(createdID)
	s.Require().NoError(err)
	var pre Application
	s.Require().NoError(json.Unmarshal(getResp, &pre))

	exportResp, err := s.exportApps(appExportRequest{Applications: []string{createdID}})
	s.Require().NoError(err)
	s.Require().NotEmpty(exportResp.Resources)
	yamlContent := exportResp.Resources

	for _, line := range strings.Split(yamlContent, "\n") {
		s.Assert().NotEqual(":", strings.TrimSpace(line),
			"exported YAML must not contain a bare `:` key")
	}

	s.Assert().Contains(yamlContent, "auth_flow_id: "+s.authFlowID)
	s.Assert().Contains(yamlContent, "public_client: true")
	s.Assert().Contains(yamlContent, "pkce_required: true")
	s.Assert().Contains(yamlContent, "token_endpoint_auth_method: none")
	s.Assert().Contains(yamlContent, "redirect_uris:")
	s.Assert().Contains(yamlContent, "{{- range .",
		"redirect_uris should be parameterized as a template range")

	// Public client carve-out: ClientSecret variable is omitted, literal client_id is replaced.
	s.Assert().NotContains(strings.ToLower(yamlContent), "client_secret")
	s.Assert().NotContains(yamlContent, "client_id: "+clientIDLiteral)
	s.Assert().Contains(yamlContent, "{{")

	s.Require().NoError(deleteApplication(createdID))

	vars := s.extractTemplateVariables(yamlContent, map[string]interface{}{
		"client_id":     clientIDLiteral,
		"redirect_uris": []string{redirectURI},
	})
	importResp, err := s.importApps(appImportRequest{
		Content:   yamlContent,
		Options:   appImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
		Variables: vars,
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	importedID := importResp.Results[0].ResourceID
	s.Assert().Equal(createdID, importedID)
	defer func() { _ = deleteApplication(importedID) }()

	restoredBody, err := s.appGet(importedID)
	s.Require().NoError(err)
	var restored Application
	s.Require().NoError(json.Unmarshal(restoredBody, &restored))

	s.Assert().Equal(pre.Name, restored.Name)
	s.Assert().Equal(pre.Description, restored.Description)
	s.Assert().Equal(pre.AuthFlowID, restored.AuthFlowID)
	s.Require().Len(restored.InboundAuthConfig, 1)
	cfg := restored.InboundAuthConfig[0].OAuthAppConfig
	s.Require().NotNil(cfg)
	s.Assert().Equal(clientIDLiteral, cfg.ClientID)
	s.Assert().Empty(cfg.ClientSecret)
	s.Assert().True(cfg.PublicClient)
	s.Assert().True(cfg.PKCERequired)
	s.Assert().Equal("none", cfg.TokenEndpointAuthMethod)
	s.Assert().ElementsMatch([]string{"authorization_code"}, cfg.GrantTypes)
	s.Assert().ElementsMatch([]string{"code"}, cfg.ResponseTypes)
	s.Assert().ElementsMatch([]string{redirectURI}, cfg.RedirectURIs)
}

// --- Helpers ---

func (s *ApplicationImportExportSuite) appGet(appID string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/applications/"+appID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /applications/%s failed: status=%d body=%s",
			appID, resp.StatusCode, string(body))
	}
	return body, nil
}

func (s *ApplicationImportExportSuite) exportApps(reqBody appExportRequest) (*appExportResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/export", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("export request failed with status %d: %s", resp.StatusCode, string(body))
	}
	var parsed appExportResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse export response: %w (body=%s)", err, string(body))
	}
	return &parsed, nil
}

func (s *ApplicationImportExportSuite) importApps(reqBody appImportRequest) (*appImportResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/import", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("import request failed with status %d: %s", resp.StatusCode, string(body))
	}
	var parsed appImportResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse import response: %w (body=%s)", err, string(body))
	}
	return &parsed, nil
}

// extractTemplateVariables walks the exported YAML and discovers the variable names emitted
// by the parameterizer. It handles two forms:
//
//  1. Scalar:   client_id: {{.X_CLIENT_ID}}
//  2. Array:    redirect_uris:
//                 {{- range .X_REDIRECT_URIS}}
//                 - {{.}}
//                 {{- end}}
//
// Values come from the caller-supplied map keyed by yaml field name. Scalar values are
// strings; array values are []string.
func (s *ApplicationImportExportSuite) extractTemplateVariables(
	yamlContent string, valuesByKey map[string]interface{},
) map[string]interface{} {
	out := make(map[string]interface{})
	lines := strings.Split(yamlContent, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "{{- range .") {
			end := strings.Index(trimmed, "}}")
			if end < 0 {
				continue
			}
			varRef := strings.TrimSpace(trimmed[len("{{- range ."):end])
			if varRef == "" || i == 0 {
				continue
			}
			prev := strings.TrimSpace(lines[i-1])
			key := strings.TrimSpace(strings.TrimSuffix(prev, ":"))
			if val, ok := valuesByKey[key]; ok {
				out[varRef] = val
			}
			continue
		}

		idx := strings.Index(trimmed, "{{.")
		if idx < 0 {
			continue
		}
		end := strings.Index(trimmed[idx:], "}}")
		if end < 0 {
			continue
		}
		varRef := strings.TrimSpace(trimmed[idx+3 : idx+end])
		if varRef == "" {
			continue
		}
		key := strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[0])
		if val, ok := valuesByKey[key]; ok {
			out[varRef] = val
		}
	}
	return out
}
