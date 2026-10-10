// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// storeValue writes a variable or a secret to this deployment's store, and returns a cleanup that
// removes it.
func (s *ApplicationImportExportSuite) storeValue(collection, name, value string) func() {
	s.T().Helper()
	body, err := json.Marshal(map[string]string{"name": name, "value": value})
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/"+collection, bytes.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	_ = resp.Body.Close()
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "the %s %s was not stored", collection, name)
	return func() {
		del, err := http.NewRequest(http.MethodDelete, testServerURL+"/"+collection+"/"+name, nil)
		if err != nil {
			return
		}
		if resp, err := testutils.GetHTTPClient().Do(del); err == nil {
			_ = resp.Body.Close()
		}
	}
}

func (s *ApplicationImportExportSuite) clientCredentialsToken(clientID, clientSecret string) int {
	s.T().Helper()
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// A control plane's configuration names the client ID and secret rather than carrying them. Import
// puts in what this deployment's store holds, so the application authenticates with those values,
// and the secret is stored as any typed-in one: the reference text is not what verifies.
func (s *ApplicationImportExportSuite) TestImportResolvesReferencesFromTheVariableStore() {
	prefix := "APP_REF_" + s.handleSuffix
	clientID := "app-ref-client-" + s.handleSuffix
	clientSecret := "app-ref-secret-" + s.handleSuffix
	defer s.storeValue("variables", prefix+"_CLIENT_ID", clientID)()
	defer s.storeValue("secrets", prefix+"_CLIENT_SECRET", clientSecret)()

	importResp, err := s.importApps(appImportRequest{
		Content: fmt.Sprintf(`resource_type: application
name: App Ref %s
type: m2m
ouId: %s
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: var:%s_CLIENT_ID
      clientSecret: sec:%s_CLIENT_SECRET
      grantTypes:
        - client_credentials
      tokenEndpointAuthMethod: client_secret_basic
`, s.handleSuffix, s.ouID, prefix, prefix),
		Options: appImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	defer func() { _ = deleteApplication(importResp.Results[0].ResourceID) }()

	s.Equal(http.StatusOK, s.clientCredentialsToken(clientID, clientSecret),
		"the application did not authenticate with the values the store holds")
	s.NotEqual(http.StatusOK, s.clientCredentialsToken(clientID, "sec:"+prefix+"_CLIENT_SECRET"),
		"the reference text authenticated in place of the secret")
}

// A reference this deployment holds no value for refuses the application, naming the reference,
// rather than storing the reference text in its place. That holds for a variable and a secret alike.
func (s *ApplicationImportExportSuite) TestImportRefusesAnApplicationWhoseReferenceIsNotHeld() {
	for _, field := range []string{"clientId", "clientSecret"} {
		s.Run(field, func() {
			missing := "var:APP_REF_MISSING_" + s.handleSuffix
			clientID, clientSecret := missing, "some-secret"
			if field == "clientSecret" {
				missing = "sec:APP_REF_MISSING_" + s.handleSuffix
				clientID, clientSecret = "app-ref-missing-"+s.handleSuffix, missing
			}

			importResp, err := s.importApps(appImportRequest{
				Content: fmt.Sprintf(`resource_type: application
name: App Ref Missing %s %s
type: m2m
ouId: %s
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: %s
      clientSecret: %s
      grantTypes:
        - client_credentials
      tokenEndpointAuthMethod: client_secret_basic
`, field, s.handleSuffix, s.ouID, clientID, clientSecret),
				Options: appImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
			})
			s.Require().NoError(err)
			s.Require().Len(importResp.Results, 1)
			s.Equal("failed", importResp.Results[0].Status)
			s.Equal("IMP-1006", importResp.Results[0].Code)
			s.Contains(importResp.Results[0].Message, missing)
			s.Zero(importResp.Summary.Imported)
		})
	}
}

// A control plane exports an address list as a list of one reference. The variable it names holds this
// deployment's addresses as a JSON list, and import writes them as the application's items.
func (s *ApplicationImportExportSuite) TestImportExpandsAListVariableIntoRedirectURIs() {
	prefix := "APP_LIST_REF_" + s.handleSuffix
	defer s.storeValue("variables", prefix+"_CLIENT_ID", "app-list-ref-client-"+s.handleSuffix)()
	defer s.storeValue("secrets", prefix+"_CLIENT_SECRET", "app-list-ref-secret-"+s.handleSuffix)()
	defer s.storeValue("variables", prefix+"_REDIRECT_URIS",
		`["https://one.list-ref.test/callback","https://two.list-ref.test/callback"]`)()

	importResp, err := s.importApps(appImportRequest{
		Content: fmt.Sprintf(`resource_type: application
name: App List Ref %s
type: fullstack
ouId: %s
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: var:%s_CLIENT_ID
      clientSecret: sec:%s_CLIENT_SECRET
      redirectUris:
        - var:%s_REDIRECT_URIS
      grantTypes:
        - authorization_code
      responseTypes:
        - code
      tokenEndpointAuthMethod: client_secret_basic
`, s.handleSuffix, s.ouID, prefix, prefix, prefix),
		Options: appImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	defer func() { _ = deleteApplication(importResp.Results[0].ResourceID) }()

	app, err := getApplicationByID(importResp.Results[0].ResourceID)
	s.Require().NoError(err)
	s.Require().Len(app.InboundAuthConfig, 1)
	s.Require().NotNil(app.InboundAuthConfig[0].OAuthAppConfig)
	s.Equal([]string{"https://one.list-ref.test/callback", "https://two.list-ref.test/callback"},
		app.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs)
}
