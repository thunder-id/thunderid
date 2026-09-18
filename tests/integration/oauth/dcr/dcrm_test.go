// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// DCRMTestSuite covers the RFC 7592 client configuration endpoint: reading, updating and deleting
// a dynamically registered client as an administrative caller holding the system permission.
type DCRMTestSuite struct {
	suite.Suite
	registeredAppIDs []string
}

func TestDCRMTestSuite(t *testing.T) {
	suite.Run(t, new(DCRMTestSuite))
}

func (ts *DCRMTestSuite) TearDownSuite() {
	for _, appID := range ts.registeredAppIDs {
		if appID != "" {
			if err := testutils.DeleteApplication(appID); err != nil {
				ts.T().Logf("Failed to delete application during teardown: %v", err)
			}
		}
	}
}

// UC-1: registration assigns the client identity. Management is administrative, so no per-client
// management credential is handed to the registrant.
func (ts *DCRMTestSuite) TestRegistrationReturnsClientIdentity() {
	response := ts.register("DCRM Registration Fields")
	ts.registeredAppIDs = append(ts.registeredAppIDs, response.AppID)

	ts.Assert().NotEmpty(response.ClientID)
	ts.Assert().NotZero(response.ClientIDIssuedAt)
	ts.Assert().Empty(response.RegistrationAccessToken,
		"registration must not issue a registration access token")
	ts.Assert().Empty(response.RegistrationClientURI,
		"registration must not advertise a client configuration URI")
}

// UC-2: an administrative caller reads a client's registration.
func (ts *DCRMTestSuite) TestReadClientRegistration() {
	registered := ts.register("DCRM Read Client")
	ts.registeredAppIDs = append(ts.registeredAppIDs, registered.AppID)

	read, status, _ := ts.manage(http.MethodGet, registered.ClientID, nil)

	ts.Require().Equal(http.StatusOK, status)
	ts.Assert().Equal(registered.ClientID, read.ClientID)
	ts.Assert().Equal("DCRM Read Client", read.ClientName)
	ts.Assert().Contains(read.RedirectURIs, "https://dcrm.example.com/callback")
	ts.Assert().Empty(read.ClientSecret, "the client secret is not readable and must not be returned")
}

// UC-3: a client updates its registered metadata, and the client identity survives the update.
func (ts *DCRMTestSuite) TestUpdateClientRegistration() {
	registered := ts.register("DCRM Update Client")
	ts.registeredAppIDs = append(ts.registeredAppIDs, registered.AppID)

	update := DCRRegistrationRequest{
		ClientID:                registered.ClientID,
		ClientName:              "DCRM Updated Client",
		RedirectURIs:            []string{"https://dcrm.example.com/updated"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
	}
	body, err := json.Marshal(update)
	ts.Require().NoError(err)

	updated, status, _ := ts.manage(http.MethodPut, registered.ClientID, body)

	ts.Require().Equal(http.StatusOK, status)
	ts.Assert().Equal(registered.ClientID, updated.ClientID, "the client_id must be preserved")
	ts.Assert().Equal("DCRM Updated Client", updated.ClientName)

	// The change is persisted, and the identity still holds on a subsequent read.
	read, status, _ := ts.manage(http.MethodGet, registered.ClientID, nil)
	ts.Require().Equal(http.StatusOK, status)
	ts.Assert().Equal(registered.ClientID, read.ClientID)
	ts.Assert().Equal("DCRM Updated Client", read.ClientName)
	ts.Assert().Contains(read.RedirectURIs, "https://dcrm.example.com/updated")
	ts.Assert().NotContains(read.RedirectURIs, "https://dcrm.example.com/callback")
}

// The client secret is preserved across an update. Since it cannot be read back, this is verified
// by exercising it: the original secret still authenticates at the token endpoint afterwards.
func (ts *DCRMTestSuite) TestUpdatePreservesClientSecret() {
	registered := ts.register("DCRM Secret Preserved")
	ts.registeredAppIDs = append(ts.registeredAppIDs, registered.AppID)
	ts.Require().NotEmpty(registered.ClientSecret)

	update := DCRRegistrationRequest{
		ClientID:                registered.ClientID,
		ClientName:              "DCRM Secret Preserved Renamed",
		RedirectURIs:            []string{"https://dcrm.example.com/callback"},
		GrantTypes:              []string{"client_credentials"},
		TokenEndpointAuthMethod: "client_secret_basic",
	}
	body, err := json.Marshal(update)
	ts.Require().NoError(err)

	_, status, _ := ts.manage(http.MethodPut, registered.ClientID, body)
	ts.Require().Equal(http.StatusOK, status)

	ts.Assert().True(ts.clientCredentialsSucceeds(registered.ClientID, registered.ClientSecret),
		"the original client secret must still authenticate after an update")
}

// UC-4: an administrative caller deletes a registration, after which the client no longer
// resolves and every later request for it reports not found.
func (ts *DCRMTestSuite) TestDeleteClientRegistration() {
	registered := ts.register("DCRM Delete Client")

	_, status, _ := ts.manage(http.MethodDelete, registered.ClientID, nil)
	ts.Require().Equal(http.StatusNoContent, status)

	_, status, _ = ts.manage(http.MethodGet, registered.ClientID, nil)
	ts.Assert().Equal(http.StatusNotFound, status,
		"the registration must be gone once the client is deleted")
}

// UC-5: a request without administrative credentials is rejected and does not expose the
// registration.
func (ts *DCRMTestSuite) TestUnauthenticatedRequestIsRejected() {
	registered := ts.register("DCRM Unauthenticated")
	ts.registeredAppIDs = append(ts.registeredAppIDs, registered.AppID)

	_, status, _ := ts.manageAs(http.MethodGet, registered.ClientID, nil, false)
	ts.Assert().Equal(http.StatusUnauthorized, status)

	// The registration is untouched and still readable by an administrative caller.
	read, status, _ := ts.manage(http.MethodGet, registered.ClientID, nil)
	ts.Require().Equal(http.StatusOK, status)
	ts.Assert().Equal("DCRM Unauthenticated", read.ClientName)
}

// An update naming a different client is rejected.
func (ts *DCRMTestSuite) TestUpdateWithMismatchedClientIDIsRejected() {
	registered := ts.register("DCRM Mismatch Client")
	ts.registeredAppIDs = append(ts.registeredAppIDs, registered.AppID)

	body, err := json.Marshal(DCRRegistrationRequest{
		ClientID:     "some-other-client-id",
		ClientName:   "DCRM Mismatch Client",
		RedirectURIs: []string{"https://dcrm.example.com/callback"},
	})
	ts.Require().NoError(err)

	_, status, _ := ts.manage(http.MethodPut, registered.ClientID, body)
	ts.Assert().Equal(http.StatusBadRequest, status)
}

// An update carrying invalid client metadata is rejected.
func (ts *DCRMTestSuite) TestUpdateWithInvalidMetadataIsRejected() {
	registered := ts.register("DCRM Invalid Metadata")
	ts.registeredAppIDs = append(ts.registeredAppIDs, registered.AppID)

	body, err := json.Marshal(DCRRegistrationRequest{
		ClientID:     registered.ClientID,
		ClientName:   "DCRM Invalid Metadata",
		RedirectURIs: []string{"not-a-valid-uri"},
	})
	ts.Require().NoError(err)

	_, status, errResponse := ts.manage(http.MethodPut, registered.ClientID, body)
	ts.Require().Equal(http.StatusBadRequest, status)
	ts.Require().NotNil(errResponse)
	ts.Assert().NotEmpty(errResponse.Error)
}

// An administrative caller may address any registration, so an unknown client identifier is
// reported as not found rather than as an authorization failure.
func (ts *DCRMTestSuite) TestUnknownClientIsNotFound() {
	_, status, _ := ts.manage(http.MethodGet, "client-that-does-not-exist", nil)
	ts.Assert().Equal(http.StatusNotFound, status)
}

// UC-6: neither a read nor an update advertises a management credential, since management is
// administrative and the caller already holds one.
func (ts *DCRMTestSuite) TestManagementResponsesOmitClientConfigurationFields() {
	registered := ts.register("DCRM No Management Fields")
	ts.registeredAppIDs = append(ts.registeredAppIDs, registered.AppID)

	read, status, _ := ts.manage(http.MethodGet, registered.ClientID, nil)
	ts.Require().Equal(http.StatusOK, status)
	ts.Assert().Empty(read.RegistrationAccessToken)
	ts.Assert().Empty(read.RegistrationClientURI)

	body, err := json.Marshal(DCRRegistrationRequest{
		ClientID:                registered.ClientID,
		ClientName:              "DCRM No Management Fields",
		RedirectURIs:            []string{"https://dcrm.example.com/callback"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
	})
	ts.Require().NoError(err)

	updated, status, _ := ts.manage(http.MethodPut, registered.ClientID, body)
	ts.Require().Equal(http.StatusOK, status)
	ts.Assert().Empty(updated.RegistrationAccessToken)
	ts.Assert().Empty(updated.RegistrationClientURI)
}

// register creates a client through DCR and fails the test if registration does not succeed.
func (ts *DCRMTestSuite) register(clientName string) *DCRRegistrationResponse {
	request := DCRRegistrationRequest{
		ClientName:              clientName,
		RedirectURIs:            []string{"https://dcrm.example.com/callback"},
		GrantTypes:              []string{"authorization_code", "client_credentials"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
	}
	requestJSON, err := json.Marshal(request)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+dcrEndpoint, bytes.NewReader(requestJSON))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	ts.setAdminAuthorization(req)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 201 registering %q, got %d. Response: %s",
			clientName, resp.StatusCode, string(responseBody))
	}

	var response DCRRegistrationResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&response))
	ts.Require().NotEmpty(response.ClientID)
	return &response
}

// manage issues a request to the client configuration endpoint as an administrative caller, and
// returns the decoded response, the status and any error body.
func (ts *DCRMTestSuite) manage(method, clientID string, body []byte) (
	*DCRRegistrationResponse, int, *DCRErrorResponse) {
	return ts.manageAs(method, clientID, body, true)
}

// manageAs issues a client configuration request either as an administrative caller or with no
// credentials at all, so the authorization boundary can be exercised.
func (ts *DCRMTestSuite) manageAs(method, clientID string, body []byte, asAdmin bool) (
	*DCRRegistrationResponse, int, *DCRErrorResponse) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, testServerURL+dcrEndpoint+"/"+clientID, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if asAdmin {
		ts.setAdminAuthorization(req)
	}

	// A plain client is used so that only the credentials set above are attached.
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	if resp.StatusCode == http.StatusNoContent || len(responseBody) == 0 {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		var errResponse DCRErrorResponse
		if err := json.Unmarshal(responseBody, &errResponse); err != nil {
			return nil, resp.StatusCode, nil
		}
		return nil, resp.StatusCode, &errResponse
	}

	var response DCRRegistrationResponse
	ts.Require().NoError(json.Unmarshal(responseBody, &response))
	return &response, resp.StatusCode, nil
}

// clientCredentialsSucceeds reports whether the given credentials obtain a token, which is how the
// client secret is verified without reading it back.
func (ts *DCRMTestSuite) clientCredentialsSucceeds(clientID, clientSecret string) bool {
	form := "grant_type=client_credentials"
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token",
		bytes.NewReader([]byte(form)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	return resp.StatusCode == http.StatusOK
}

// setAdminAuthorization attaches an administrative access token, which DCR registration requires
// because it does not run in insecure mode.
func (ts *DCRMTestSuite) setAdminAuthorization(req *http.Request) {
	token, err := testutils.GetAccessToken()
	if err != nil {
		ts.T().Fatalf("Failed to obtain access token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
}
