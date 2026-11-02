// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
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

	authFlowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	s.Require().NoError(err)
	s.Require().NotEmpty(authFlowID)
	s.authFlowID = authFlowID

	regFlowID, err := testutils.GetFlowIDByHandle("default-flow", "REGISTRATION")
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
							UserConfig: &AccessTokenSubConfig{
								ValidityPeriod: 1800,
								Attributes:     []string{"email"},
							},
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

	s.Assert().Contains(yamlContent, "resource_type: application")
	s.Assert().Contains(yamlContent, "id: "+createdID)
	s.Assert().Contains(yamlContent, "ouId: "+s.ouID)
	s.Assert().Contains(yamlContent, "name: "+appName)
	s.Assert().Contains(yamlContent, "description: Round-trip confidential application")
	s.Assert().Contains(yamlContent, "template: web")

	// Inline-embedded fields appear at the top level (flattened, not nested).
	s.Assert().Contains(yamlContent, "authFlowId: "+s.authFlowID)
	s.Assert().Contains(yamlContent, "registrationFlowId: "+s.registrationFlowID)
	s.Assert().Contains(yamlContent, "isRegistrationFlowEnabled: true")
	s.Assert().Contains(yamlContent, "assertion:")
	s.Assert().Contains(yamlContent, "validityPeriod: 3600")
	s.Assert().Contains(yamlContent, "loginConsent:")
	s.Assert().Contains(yamlContent, "validityPeriod: 86400")
	s.Assert().Contains(yamlContent, "inboundAuthConfig:")
	s.Assert().Contains(yamlContent, "authorization_code")
	s.Assert().Contains(yamlContent, "tokenEndpointAuthMethod: client_secret_basic")
	s.Assert().NotContains(yamlContent, "app-rt-conf-secret-"+s.handleSuffix)
	s.Assert().Contains(yamlContent, "{{")

	s.Require().NoError(deleteApplication(createdID))

	vars := s.extractTemplateVariables(yamlContent, map[string]interface{}{
		"clientId":     "app-rt-conf-client-" + s.handleSuffix,
		"clientSecret": "app-rt-conf-secret-" + s.handleSuffix,
		"redirectUris": []string{"https://app-rt-conf.example.com/callback"},
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
	s.Assert().Equal(int64(1800), cfg.Token.AccessToken.UserConfig.ValidityPeriod)
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

	s.Assert().Contains(yamlContent, "authFlowId: "+s.authFlowID)
	s.Assert().Contains(yamlContent, "publicClient: true")
	s.Assert().Contains(yamlContent, "pkceRequired: true")
	s.Assert().Contains(yamlContent, "tokenEndpointAuthMethod: none")
	s.Assert().Contains(yamlContent, "redirectUris:")
	s.Assert().Contains(yamlContent, "{{- range .",
		"redirect_uris should be parameterized as a template range")

	// Public client carve-out: ClientSecret variable is omitted, literal client_id is replaced.
	s.Assert().NotContains(strings.ToLower(yamlContent), "client_secret")
	s.Assert().NotContains(yamlContent, "clientId: "+clientIDLiteral)
	s.Assert().Contains(yamlContent, "{{")

	s.Require().NoError(deleteApplication(createdID))

	vars := s.extractTemplateVariables(yamlContent, map[string]interface{}{
		"clientId":     clientIDLiteral,
		"redirectUris": []string{redirectURI},
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

// TestExportImportRoundTrip_ClientCredentialsSubType covers the sub_type claim across the whole
// lifecycle: creation selects it, export carries the selection, and import restores it. Nothing else
// asserts that the selection survives an export, which is how a GitOps adoption inherits the claim.
func (s *ApplicationImportExportSuite) TestExportImportRoundTrip_ClientCredentialsSubType() {
	appName := "App RT CC " + s.handleSuffix
	clientID := "app-rt-cc-client-" + s.handleSuffix
	clientSecret := "app-rt-cc-secret-" + s.handleSuffix

	createdID, err := createApplication(Application{
		OUID:        s.ouID,
		Name:        appName,
		Description: "Round-trip client_credentials application",
		Type:        "m2m",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                clientID,
					ClientSecret:            clientSecret,
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	s.Require().NoError(err, "failed to create source application")

	getResp, err := s.appGet(createdID)
	s.Require().NoError(err)
	var pre Application
	s.Require().NoError(json.Unmarshal(getResp, &pre))
	s.Require().Contains(clientAttributesOf(pre), "sub_type",
		"creation must select sub_type for a client_credentials application")

	exportResp, err := s.exportApps(appExportRequest{Applications: []string{createdID}})
	s.Require().NoError(err)
	yamlContent := exportResp.Resources
	s.Assert().Contains(yamlContent, "sub_type", "the exported selection must carry the claim")

	s.Require().NoError(deleteApplication(createdID))

	// The exporter parameterizes redirectUris even for a client that has none, so the variable has to
	// be supplied as an empty list for the template to resolve.
	vars := s.extractTemplateVariables(yamlContent, map[string]interface{}{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"redirectUris": []string{},
	})
	importResp, err := s.importApps(appImportRequest{
		Content:   yamlContent,
		Options:   appImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
		Variables: vars,
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)

	importedID := importResp.Results[0].ResourceID
	s.Require().NotEmpty(importedID)
	defer func() { _ = deleteApplication(importedID) }()

	restoredResp, err := s.appGet(importedID)
	s.Require().NoError(err)
	var restored Application
	s.Require().NoError(json.Unmarshal(restoredResp, &restored))

	s.Assert().Contains(clientAttributesOf(restored), "sub_type",
		"the selection must survive the export and import round-trip")
}

// TestImportApplication_SeedsSubTypeWhenOmitted covers a hand-written declarative document that does
// not mention the claim. An import with target runtime creates the application through the same
// service as the API, so the claim is selected for it rather than being absent until edited.
func (s *ApplicationImportExportSuite) TestImportApplication_SeedsSubTypeWhenOmitted() {
	clientID := "app-imp-seed-client-" + s.handleSuffix
	yamlContent := fmt.Sprintf(`resource_type: application
name: App Imp Seed %s
type: m2m
ouId: %s
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: %s
      clientSecret: app-imp-seed-secret-%s
      grantTypes:
        - client_credentials
      tokenEndpointAuthMethod: client_secret_basic
`, s.handleSuffix, s.ouID, clientID, s.handleSuffix)

	importResp, err := s.importApps(appImportRequest{
		Content: yamlContent,
		Options: appImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)

	importedID := importResp.Results[0].ResourceID
	s.Require().NotEmpty(importedID)
	defer func() { _ = deleteApplication(importedID) }()

	restoredResp, err := s.appGet(importedID)
	s.Require().NoError(err)
	var restored Application
	s.Require().NoError(json.Unmarshal(restoredResp, &restored))

	s.Assert().Contains(clientAttributesOf(restored), "sub_type",
		"an imported client_credentials application must have the claim selected for it")
}

// clientAttributesOf returns the application's client-token attribute selection, or nil when unset.
func clientAttributesOf(app Application) []string {
	if len(app.InboundAuthConfig) == 0 {
		return nil
	}
	cfg := app.InboundAuthConfig[0].OAuthAppConfig
	if cfg == nil || cfg.Token == nil || cfg.Token.AccessToken == nil ||
		cfg.Token.AccessToken.ClientConfig == nil {
		return nil
	}
	return cfg.Token.AccessToken.ClientConfig.Attributes
}

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
//  1. Scalar:   clientId: {{.X_CLIENT_ID}}
//  2. Array:    redirectUris:
//     {{- range .X_REDIRECT_URIS}}
//     - {{.}}
//     {{- end}}
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

// sharingPolicyRequest posts to an application's sharing endpoints and returns status and body.
func (s *ApplicationImportExportSuite) sharingRequest(
	method, path string, body interface{},
) (int, map[string]interface{}) {
	s.T().Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	decoded := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return resp.StatusCode, decoded
}

// An export carries the sharing policies recorded for an application, and replaying the exported
// document records the same sharing rather than producing an application nobody can act for.
//
// Re-importing is keyed on the organization unit that issues each policy, so the replay replaces
// what that unit holds instead of colliding with it.
func (s *ApplicationImportExportSuite) TestExportImportRoundTrip_CarriesSharingPolicies() {
	appName := "App RT Sharing " + s.handleSuffix

	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "app-ie-child-" + s.handleSuffix,
		Name:   "App Import Export Child " + s.handleSuffix,
		Parent: &s.ouID,
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(childOUID) }()

	createdID, err := createApplication(Application{
		OUID: s.ouID, Name: appName, Description: "Round-trip with sharing policies",
		Template: "web", AuthFlowID: s.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                "app-rt-sharing-" + s.handleSuffix,
				ClientSecret:            "app-rt-sharing-secret",
				RedirectURIs:            []string{"https://app-rt-sharing.example.com/callback"},
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)
	defer func() { _ = deleteApplication(createdID) }()

	base := "/applications/" + createdID + "/sharing-policies"
	status, created := s.sharingRequest(http.MethodPost, base, map[string]interface{}{
		"childUnits": allChildUnits(childOUID),
	})
	s.Require().Equal(http.StatusCreated, status, "body: %v", created)
	policyID, _ := created["id"].(string)
	s.Require().NotEmpty(policyID)
	policyVersion := created["version"]

	exported, err := s.exportApps(appExportRequest{Applications: []string{createdID}})
	s.Require().NoError(err)
	s.Require().Contains(exported.Resources, "sharingPolicies",
		"the exported document carries the sharing")
	s.Require().Contains(exported.Resources, "childUnits", "including the section that was recorded")
	s.Require().Contains(exported.Resources, "mode: all", "in the mode it was recorded with")
	s.Require().Contains(exported.Resources, childOUID, "and the exclusion on it")

	// The export parameterizes the client credentials, so the replay has to supply them back.
	vars := s.extractTemplateVariables(exported.Resources, map[string]interface{}{
		"clientId":     "app-rt-sharing-" + s.handleSuffix,
		"clientSecret": "app-rt-sharing-secret",
		"redirectUris": []string{"https://app-rt-sharing.example.com/callback"},
	})

	// Replaying the document leaves the organization unit holding one policy, not two.
	imported, err := s.importApps(appImportRequest{
		Content:   exported.Resources,
		Variables: vars,
		Options:   appImportOptions{Upsert: true, ContinueOnError: false},
	})
	s.Require().NoError(err)
	s.Require().Zero(imported.Summary.Failed, "the replay must not fail: %+v", imported.Results)

	status, listed := s.sharingRequest(http.MethodGet, base, nil)
	s.Require().Equal(http.StatusOK, status)
	s.Equal(float64(1), listed["totalResults"],
		"the replay replaced the policy rather than adding a second")

	policies, _ := listed["policies"].([]interface{})
	s.Require().Len(policies, 1)
	s.Equal(policyID, policies[0].(map[string]interface{})["id"],
		"a replay reproduces the policy under its own id, so anything pointing at it survives")
	// The document says what the policy already says, so the replay writes nothing. An export,
	// import, export round has to agree with itself, and a version that moved would break that.
	s.Equal(policyVersion, policies[0].(map[string]interface{})["version"],
		"replaying an unchanged document is not an edit, so the version stays")
	childUnits, _ := policies[0].(map[string]interface{})["childUnits"].(map[string]interface{})
	s.Require().NotNil(childUnits)
	s.Equal("all", childUnits["mode"])
	s.Equal([]interface{}{childOUID}, childUnits["excludedOuIds"],
		"the exclusion survived the round trip")
}

// Deleting an application takes its sharing policies with it. A document brought back under the
// same id without any policy must be shared with nobody, rather than picking up the rows the
// deleted application left behind.
func (s *ApplicationImportExportSuite) TestDeleteApplicationRemovesItsSharingPolicies() {
	appName := "App Del Sharing " + s.handleSuffix
	createdID, err := createApplication(Application{
		OUID: s.ouID, Name: appName, Description: "Deleted with sharing policies",
		Template: "web", AuthFlowID: s.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                "app-del-sharing-" + s.handleSuffix,
				ClientSecret:            "app-del-sharing-secret",
				RedirectURIs:            []string{"https://app-del-sharing.example.com/callback"},
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)
	defer func() { _ = deleteApplication(createdID) }()

	base := "/applications/" + createdID + "/sharing-policies"
	status, created := s.sharingRequest(http.MethodPost, base,
		map[string]interface{}{"childUnits": allChildUnits()})
	s.Require().Equal(http.StatusCreated, status, "body: %v", created)

	exported, err := s.exportApps(appExportRequest{Applications: []string{createdID}})
	s.Require().NoError(err)
	vars := s.extractTemplateVariables(exported.Resources, map[string]interface{}{
		"clientId":     "app-del-sharing-" + s.handleSuffix,
		"clientSecret": "app-del-sharing-secret",
		"redirectUris": []string{"https://app-del-sharing.example.com/callback"},
	})
	content := testutils.WithoutTopLevelKey(exported.Resources, "sharingPolicies")
	s.Require().NotContains(content, "sharingPolicies", "the document brought back declares no policy")

	s.Require().NoError(deleteApplication(createdID))

	imported, err := s.importApps(appImportRequest{
		Content:   content,
		Variables: vars,
		Options:   appImportOptions{Upsert: true, ContinueOnError: false},
	})
	s.Require().NoError(err)
	s.Require().Zero(imported.Summary.Failed, "import results: %+v", imported.Results)
	s.Require().Equal(createdID, imported.Results[0].ResourceID,
		"the document brings the application back under its own id")

	status, listed := s.sharingRequest(http.MethodGet, base, nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", listed)
	s.Equal(float64(0), listed["totalResults"], "the deleted application's policies went with it")
}

// An export copied to make a second application in the same deployment still carries the source's
// policy ids. The copy's sharing is stored under ids of its own, and the source keeps its policy.
func (s *ApplicationImportExportSuite) TestImportOfACopiedDocumentGetsItsOwnSharing() {
	appName := "App Clone Src " + s.handleSuffix
	sourceID, err := createApplication(Application{
		OUID: s.ouID, Name: appName, Description: "Source of a copied document",
		Template: "web", AuthFlowID: s.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                "app-clone-src-" + s.handleSuffix,
				ClientSecret:            "app-clone-src-secret",
				RedirectURIs:            []string{"https://app-clone.example.com/callback"},
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)
	defer func() { _ = deleteApplication(sourceID) }()

	sourceBase := "/applications/" + sourceID + "/sharing-policies"
	status, sourcePolicy := s.sharingRequest(http.MethodPost, sourceBase,
		map[string]interface{}{"childUnits": allChildUnits()})
	s.Require().Equal(http.StatusCreated, status, "body: %v", sourcePolicy)

	exported, err := s.exportApps(appExportRequest{Applications: []string{sourceID}})
	s.Require().NoError(err)
	// The copy is a new application: no id of its own yet, another name, another client.
	content := testutils.WithoutTopLevelKey(exported.Resources, "id")
	content = strings.Replace(content, "name: "+appName, "name: App Clone Copy "+s.handleSuffix, 1)
	vars := s.extractTemplateVariables(content, map[string]interface{}{
		"clientId":     "app-clone-copy-" + s.handleSuffix,
		"clientSecret": "app-clone-copy-secret",
		"redirectUris": []string{"https://app-clone.example.com/callback"},
	})

	imported, err := s.importApps(appImportRequest{
		Content: content, Variables: vars,
		Options: appImportOptions{Upsert: true, ContinueOnError: false},
	})
	s.Require().NoError(err)
	s.Require().Zero(imported.Summary.Failed, "import results: %+v", imported.Results)
	copyID := imported.Results[0].ResourceID
	s.Require().NotEqual(sourceID, copyID, "the copy is a new application")
	defer func() { _ = deleteApplication(copyID) }()

	status, copied := s.sharingRequest(http.MethodGet, "/applications/"+copyID+"/sharing-policies", nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", copied)
	copiedPolicies, _ := copied["policies"].([]interface{})
	s.Require().Len(copiedPolicies, 1, "the copy carries the sharing the document declared")
	s.NotEqual(sourcePolicy["id"], copiedPolicies[0].(map[string]interface{})["id"],
		"under an id of its own")

	status, source := s.sharingRequest(http.MethodGet, sourceBase, nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", source)
	sourcePolicies, _ := source["policies"].([]interface{})
	s.Require().Len(sourcePolicies, 1)
	s.Equal(sourcePolicy["id"], sourcePolicies[0].(map[string]interface{})["id"], "the source keeps its policy")
}

// An application with no sharing policies exports a document that declares none, so nothing is
// invented on the way back in.
func (s *ApplicationImportExportSuite) TestExportOmitsSharingWhenThereIsNone() {
	appName := "App RT No Sharing " + s.handleSuffix

	createdID, err := createApplication(Application{
		OUID: s.ouID, Name: appName, Description: "Round-trip without sharing policies",
		Template: "web", AuthFlowID: s.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                "app-rt-nosharing-" + s.handleSuffix,
				ClientSecret:            "app-rt-nosharing-secret",
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)
	defer func() { _ = deleteApplication(createdID) }()

	exported, err := s.exportApps(appExportRequest{Applications: []string{createdID}})
	s.Require().NoError(err)
	s.NotContains(exported.Resources, "sharingPolicies")
}

// A reach not bounded by the issuer's own position in the tree is a deployment decision, so a
// document may carry one. An import is reviewed configuration, the same as a resource file; only
// an interactive create through the API is refused.
func (s *ApplicationImportExportSuite) TestImportCarriesADeploymentWideSharingPolicy() {
	clientID := "app-imp-allous-client-" + s.handleSuffix

	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "app-imp-allous-child-" + s.handleSuffix,
		Name:   "App Imp AllOus Child " + s.handleSuffix,
		Parent: &s.ouID,
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(childOUID) }()
	yamlContent := fmt.Sprintf(`resource_type: application
name: App Imp AllOus %s
type: m2m
ouId: %s
sharingPolicies:
  - allUnits: {}
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: %s
      clientSecret: app-imp-allous-secret-%s
      grantTypes:
        - client_credentials
      tokenEndpointAuthMethod: client_secret_basic
`, s.handleSuffix, s.ouID, clientID, s.handleSuffix)

	imported, err := s.importApps(appImportRequest{
		Content: yamlContent,
		Options: appImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, imported.Summary.Imported, "import results: %+v", imported.Results)

	appID := imported.Results[0].ResourceID
	s.Require().NotEmpty(appID)
	defer func() { _ = deleteApplication(appID) }()

	status, listed := s.sharingRequest(http.MethodGet,
		"/applications/"+appID+"/sharing-policies", nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", listed)
	policies, _ := listed["policies"].([]interface{})
	s.Require().Len(policies, 1, "the document's policy was recorded")

	policy := policies[0].(map[string]interface{})
	s.Contains(policy, "allUnits", "the policy reaches every organization unit")
	s.NotContains(policy, "childUnits")
	s.NotContains(policy, "otherTreeRoots")
	s.Equal(false, policy["isReadOnly"], "an imported policy is stored, not declared")

	// The same policy through the API is refused, which is the line the import is on the other
	// side of.
	status, refused := s.sharingRequest(http.MethodPost,
		"/applications/"+appID+"/sharing-policies",
		map[string]interface{}{"otherTreeRoots": map[string]interface{}{"mode": "all"}})
	s.Equal(http.StatusBadRequest, status, "body: %v", refused)
	s.Equal(ErrorDeploymentWideSharingCode, refused["code"])

	policyID, _ := policy["id"].(string)
	s.Require().NotEmpty(policyID)
	path := "/applications/" + appID + "/sharing-policies/" + policyID

	// What the API may not create, it may still maintain. The section is restated, which is what
	// lets its exclusions move.
	status, edited := s.sharingRequest(http.MethodPut, path, map[string]interface{}{
		"version":  policy["version"],
		"allUnits": map[string]interface{}{"excludedOuIds": []string{childOUID}},
	})
	s.Require().Equal(http.StatusOK, status, "body: %v", edited)
	editedAllUnits, _ := edited["allUnits"].(map[string]interface{})
	s.Require().NotNil(editedAllUnits)
	s.Equal([]interface{}{childOUID}, editedAllUnits["excludedOuIds"])

	// Handing the organization unit back is the same edit in the other direction.
	status, handedBack := s.sharingRequest(http.MethodPut, path, map[string]interface{}{
		"version":  edited["version"],
		"allUnits": map[string]interface{}{},
	})
	s.Require().Equal(http.StatusOK, status, "body: %v", handedBack)
	handedBackAllUnits, _ := handedBack["allUnits"].(map[string]interface{})
	s.Require().NotNil(handedBackAllUnits)
	s.Empty(handedBackAllUnits["excludedOuIds"])

	// Trading the section for another is still out of reach, so the edit cannot go where the
	// create could not.
	status, widened := s.sharingRequest(http.MethodPut, path, map[string]interface{}{
		"version":    handedBack["version"],
		"childUnits": allChildUnits(),
	})
	s.Equal(http.StatusBadRequest, status, "body: %v", widened)

	// An imported policy is stored, not declared, so it is deleted like any other.
	status, _ = s.sharingRequest(http.MethodDelete, path, nil)
	s.Require().Equal(http.StatusNoContent, status)
	status, _ = s.sharingRequest(http.MethodGet, path, nil)
	s.Equal(http.StatusNotFound, status, "the policy is gone")
}

// A shared application stays in its organization unit: its policies were issued against that unit.
func (s *ApplicationImportExportSuite) TestSharedApplicationCannotMove() {
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "app-move-child-" + s.handleSuffix,
		Name:   "App Move Child " + s.handleSuffix,
		Parent: &s.ouID,
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(childOUID) }()

	app := Application{
		OUID: s.ouID, Name: "App Move Shared " + s.handleSuffix, Template: "web", AuthFlowID: s.authFlowID,
	}
	appID, err := createApplication(app)
	s.Require().NoError(err)
	defer func() { _ = deleteApplication(appID) }()

	status, created := s.sharingRequest(http.MethodPost, "/applications/"+appID+"/sharing-policies",
		map[string]interface{}{"childUnits": allChildUnits()})
	s.Require().Equal(http.StatusCreated, status, "body: %v", created)

	app.OUID = childOUID
	err = updateApplication(appID, app)
	s.Require().Error(err)
	s.Contains(err.Error(), "got 400")
	s.Contains(err.Error(), "APP-1061")
}

// A policy field the format does not declare fails the document rather than being dropped, which
// would share the application with the unit the document meant to exclude.
func (s *ApplicationImportExportSuite) TestImportRefusesAMisspelledPolicyField() {
	yamlContent := fmt.Sprintf(`resource_type: application
name: App Imp Misspelled %s
type: m2m
ouId: %s
sharingPolicies:
  - childUnits:
      mode: all
      excludeOuIds: [some-unit]
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: app-imp-misspelled-%s
      clientSecret: app-imp-misspelled-secret-%s
      grantTypes:
        - client_credentials
      tokenEndpointAuthMethod: client_secret_basic
`, s.handleSuffix, s.ouID, s.handleSuffix, s.handleSuffix)

	imported, err := s.importApps(appImportRequest{
		Content: yamlContent,
		Options: appImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Equal(0, imported.Summary.Imported, "import results: %+v", imported.Results)
	s.Require().Equal(1, imported.Summary.Failed, "import results: %+v", imported.Results)
	s.Contains(imported.Results[0].Message, "excludeOuIds")
}
