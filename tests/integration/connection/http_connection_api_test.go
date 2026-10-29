// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	httpConnPath = "/connections/http"

	httpConnInvalidFormatCode      = "CON-1007"
	httpConnInvalidAuthTypeCode    = "CON-1012"
	httpConnNotFoundCode           = "CON-1015"
	httpConnDuplicateCode          = "CON-1016"
	httpConnInvalidNameCode        = "CON-1017"
	httpConnInvalidURLCode         = "CON-1018"
	httpConnInvalidTimeoutCode     = "CON-1019"
	httpConnInvalidHeadersCode     = "CON-1020"
	httpConnInvalidAuthCode        = "CON-1021"
	httpConnInvalidDescriptionCode = "CON-1022"

	httpConnDefaultTimeoutMS = 10000
	httpConnMaxTimeoutMS     = 20000

	// Stored property names of the shared outbound authentication object.
	httpConnStoredType     = "authentication_type"
	httpConnStoredUsername = "authentication_username"
	httpConnStoredPassword = "authentication_password"

	// Secret values used by this suite. No response body and no stored record may ever contain them.
	httpConnBasicUser      = "http-conn-api-user"
	httpConnBasicPassword  = "http-conn-api-basic-secret"
	httpConnBasicPassword2 = "http-conn-api-basic-secret-rotated"
	httpConnBearerToken    = "http-conn-api-bearer-secret"
	httpConnAPIKeyValue    = "http-conn-api-key-secret"
	httpConnAPIKeyValue2   = "http-conn-api-key-secret-second"

	httpConnAPIKeyHeader       = "X-Tenant-Key"
	httpConnAPIKeyHeader2      = "X-Region-Key"
	httpConnStoredBearerToken  = "authentication_token"
	httpConnStoredAPIKeyHeader = "authentication_" + httpConnAPIKeyHeader
)

var httpConnSecrets = []string{httpConnBasicPassword, httpConnBasicPassword2, httpConnBearerToken,
	httpConnAPIKeyValue, httpConnAPIKeyValue2, httpConnDeclarativePassword}

type httpConnHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// httpConnAuthentication mirrors outboundauth.Authentication.
type httpConnAuthentication struct {
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// httpConnRequest mirrors the connection package's httpConnectionRequest. Cases that must send an
// explicit zero timeout use a map instead, since omitempty drops it.
type httpConnRequest struct {
	Name           string                  `json:"name"`
	OUID           string                  `json:"ouId,omitempty"`
	Description    string                  `json:"description,omitempty"`
	URL            string                  `json:"url"`
	TimeoutMS      int                     `json:"timeoutMs,omitempty"`
	Headers        []httpConnHeader        `json:"headers,omitempty"`
	Authentication *httpConnAuthentication `json:"authentication,omitempty"`
}

// httpConnResponse mirrors the connection package's httpConnectionResponse.
type httpConnResponse struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	OUID           string                 `json:"ouId"`
	Description    string                 `json:"description"`
	Type           string                 `json:"type"`
	URL            string                 `json:"url"`
	TimeoutMS      int                    `json:"timeoutMs"`
	Headers        []httpConnHeader       `json:"headers"`
	Authentication httpConnAuthentication `json:"authentication"`
}

type httpConnStoredProperty struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret"`
}

// httpConnStored mirrors the PROPERTIES column of HTTP_CONNECTION.
type httpConnStored struct {
	URL            string                   `json:"url"`
	Authentication []httpConnStoredProperty `json:"authentication"`
}

func (p httpConnStored) property(name string) (httpConnStoredProperty, bool) {
	for _, prop := range p.Authentication {
		if prop.Name == name {
			return prop, true
		}
	}
	return httpConnStoredProperty{}, false
}

func basicConnAuth(username, password string) *httpConnAuthentication {
	props := map[string]string{"username": username}
	if password != "" {
		props["password"] = password
	}
	return &httpConnAuthentication{Type: "basic", Properties: props}
}

func bearerConnAuth(token string) *httpConnAuthentication {
	props := map[string]string{}
	if token != "" {
		props["token"] = token
	}
	return &httpConnAuthentication{Type: "bearer", Properties: props}
}

func apiKeyConnAuth(headers map[string]string) *httpConnAuthentication {
	return &httpConnAuthentication{Type: "api_key", Properties: headers}
}

func noneConnAuth() *httpConnAuthentication {
	return &httpConnAuthentication{Type: "none"}
}

// HTTPConnectionAPITestSuite covers the /connections/http management API. Nothing calls a
// connection yet, so AC2.2 is proven by the stored record, which only SQLite lets the harness read.
type HTTPConnectionAPITestSuite struct {
	suite.Suite
	ouID       string
	createdIDs []string
}

func TestHTTPConnectionAPITestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionAPITestSuite))
}

// TestStoredRowChecksNeedSQLite reports, as a skip, that the stored-row checks in this suite did
// not run, so a database the harness cannot read never passes them silently.
func (s *HTTPConnectionAPITestSuite) TestStoredRowChecksNeedSQLite() {
	if dbType := testutils.GetDBType(); dbType != "sqlite" {
		s.T().Skipf("Stored-row checks need SQLite; the configuration database is %s", dbType)
	}
}

func (s *HTTPConnectionAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "http-conn-api-ou", Name: "HTTP Connection API OU",
	})
	s.Require().NoError(err, "failed to create organization unit")
	s.ouID = ouID
}

func (s *HTTPConnectionAPITestSuite) TearDownSuite() {
	for i := len(s.createdIDs) - 1; i >= 0; i-- {
		res, err := doRequest(http.MethodDelete, httpConnPath+"/"+s.createdIDs[i], nil)
		if err != nil {
			s.T().Logf("failed to delete HTTP connection %s: %v", s.createdIDs[i], err)
			continue
		}
		if res.status != http.StatusNoContent {
			s.T().Logf("failed to delete HTTP connection %s: %d %s", s.createdIDs[i], res.status, string(res.body))
		}
	}
	if s.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(s.ouID); err != nil {
			s.T().Logf("failed to delete organization unit: %v", err)
		}
	}
}

// post sends a create request and tracks anything it created for teardown, whatever the test expected.
func (s *HTTPConnectionAPITestSuite) post(body interface{}) httpResult {
	s.T().Helper()
	res, err := doRequest(http.MethodPost, httpConnPath, ownedBy(s.T(), s.ouID, body))
	s.Require().NoError(err, "Create HTTP connection request failed")
	if res.status == http.StatusCreated {
		var created httpConnResponse
		if res.decode(&created) == nil && created.ID != "" {
			s.createdIDs = append(s.createdIDs, created.ID)
		}
	}
	return res
}

func (s *HTTPConnectionAPITestSuite) create(body interface{}) (httpConnResponse, httpResult) {
	s.T().Helper()
	res := s.post(body)
	s.Require().Equal(http.StatusCreated, res.status, "Create HTTP connection should succeed: %s", string(res.body))
	s.assertNoSecretLeaked(res, "Create response")
	var created httpConnResponse
	s.Require().NoError(res.decode(&created), "Create response should decode")
	s.Require().NotEmpty(created.ID, "Created connection should have an ID")
	return created, res
}

func (s *HTTPConnectionAPITestSuite) get(id string) httpConnResponse {
	s.T().Helper()
	res, err := doRequest(http.MethodGet, httpConnPath+"/"+id, nil)
	s.Require().NoError(err, "Get HTTP connection request failed")
	s.Require().Equal(http.StatusOK, res.status, "Get HTTP connection should succeed: %s", string(res.body))
	s.assertNoSecretLeaked(res, "Get response")
	var fetched httpConnResponse
	s.Require().NoError(res.decode(&fetched), "Get response should decode")
	return fetched
}

func (s *HTTPConnectionAPITestSuite) put(id string, body interface{}) httpResult {
	s.T().Helper()
	res, err := doRequest(http.MethodPut, httpConnPath+"/"+id, body)
	s.Require().NoError(err, "Update HTTP connection request failed")
	s.assertNoSecretLeaked(res, "Update response")
	return res
}

func (s *HTTPConnectionAPITestSuite) requireUpdated(id string, body interface{}, context string) httpConnResponse {
	s.T().Helper()
	res := s.put(id, body)
	s.Require().Equal(http.StatusOK, res.status, "%s should succeed: %s", context, string(res.body))
	var updated httpConnResponse
	s.Require().NoError(res.decode(&updated), "%s: update response should decode", context)
	return updated
}

// storedRow returns the raw PROPERTIES column of a connection, and false when the configuration
// database is not readable from the harness (only SQLite is).
func (s *HTTPConnectionAPITestSuite) storedRow(id string) (string, bool) {
	s.T().Helper()
	if testutils.GetDBType() != "sqlite" {
		return "", false
	}
	// id is a server-generated UUID, so it is safe to inline.
	raw, err := testutils.QueryConfigDB(
		fmt.Sprintf(`SELECT PROPERTIES FROM "HTTP_CONNECTION" WHERE ID = '%s';`, id))
	s.Require().NoError(err, "Reading the stored HTTP connection failed")
	s.Require().NotEmpty(raw, "The HTTP connection %s should have a stored row", id)
	return raw, true
}

func (s *HTTPConnectionAPITestSuite) stored(id string) (httpConnStored, bool) {
	s.T().Helper()
	raw, ok := s.storedRow(id)
	if !ok {
		return httpConnStored{}, false
	}
	var props httpConnStored
	s.Require().NoError(json.Unmarshal([]byte(raw), &props), "Stored properties should be JSON: %s", raw)
	return props, true
}

// storedPassword returns the stored ciphertext of the password, whether it is stored, and whether
// the database could be read at all.
func (s *HTTPConnectionAPITestSuite) storedPassword(id string) (value string, present, readable bool) {
	s.T().Helper()
	props, ok := s.stored(id)
	if !ok {
		return "", false, false
	}
	prop, present := props.property(httpConnStoredPassword)
	return prop.Value, present, true
}

func (s *HTTPConnectionAPITestSuite) assertNoSecretLeaked(res httpResult, context string) {
	s.T().Helper()
	body := string(res.body)
	for _, secret := range httpConnSecrets {
		s.NotContains(body, secret, "%s must not contain a stored secret value", context)
	}
}

// assertStoredAuth checks the stored authentication type, and that the password, when wantPassword
// is set, is held only as an encrypted secret property and nowhere in plain text.
func (s *HTTPConnectionAPITestSuite) assertStoredAuth(id, wantType string, wantPassword bool) {
	s.T().Helper()
	raw, ok := s.storedRow(id)
	if !ok {
		return
	}
	for _, secret := range httpConnSecrets {
		s.NotContains(raw, secret, "The stored record must not contain a secret in plain text")
	}
	props, _ := s.stored(id)
	typeProp, ok := props.property(httpConnStoredType)
	s.True(ok, "The stored record should hold the authentication type")
	s.Equal(wantType, typeProp.Value, "Stored authentication type")
	password, ok := props.property(httpConnStoredPassword)
	s.Equal(wantPassword, ok, "Whether the stored record holds a password")
	if ok {
		s.True(password.IsSecret, "The stored password should be marked secret")
		s.NotEmpty(password.Value, "The stored password should have a value")
		s.NotEqual(maskedSecretValue, password.Value, "The stored password must not be the mask")
	}
}

// assertKeptSecret checks that a secret an update kept is still the original. A kept secret is
// decrypted and encrypted afresh, so its ciphertext differs; under the same key it has the original's
// length, which the mask or any other value would not.
func (s *HTTPConnectionAPITestSuite) assertKeptSecret(original, kept, context string) {
	s.T().Helper()
	s.NotEmpty(kept, "%s: the secret should still be stored", context)
	s.Len(kept, len(original), "%s: the stored secret should be the original, not a new value", context)
}

func (s *HTTPConnectionAPITestSuite) assertError(res httpResult, status int, code, context string) {
	s.T().Helper()
	s.Equal(status, res.status, "%s: unexpected status, body %s", context, string(res.body))
	s.Equal(code, res.errorCode(), "%s: unexpected error code", context)
}

func (s *HTTPConnectionAPITestSuite) TestCreateAppliesDefaults() {
	created, _ := s.create(httpConnRequest{
		Name:        "http-conn-api-defaults",
		Description: "defaults",
		URL:         "https://ext.example.com/defaults",
		Headers:     []httpConnHeader{{Name: "X-Tenant", Value: "t-1"}, {Name: "X-Trace", Value: "on"}},
	})

	s.Equal("http", created.Type, "Connection type should be http")
	s.Equal("https://ext.example.com/defaults", created.URL, "URL should round-trip")
	s.Equal(httpConnDefaultTimeoutMS, created.TimeoutMS, "Omitted timeout should take the 10000 ms default")
	s.Equal(httpConnAuthentication{Type: "none"}, created.Authentication,
		"Omitted authentication should be type none with no properties")

	fetched := s.get(created.ID)
	s.Equal(created, fetched, "A read should return what create returned")
	s.Equal([]httpConnHeader{{Name: "X-Tenant", Value: "t-1"}, {Name: "X-Trace", Value: "on"}}, fetched.Headers,
		"Plain headers are not secrets and should round-trip in clear, in order")
	s.assertStoredAuth(created.ID, "none", false)

	// An explicit zero is the same as an omitted timeout, and an empty type the same as none.
	created, _ = s.create(map[string]interface{}{
		"name": "http-conn-api-defaults-zero", "url": "https://ext.example.com", "timeoutMs": 0,
		"authentication": map[string]interface{}{"type": ""},
	})
	s.Equal(httpConnDefaultTimeoutMS, created.TimeoutMS, "timeoutMs 0 should take the default")
	s.Equal("none", created.Authentication.Type, "An empty type should be none")
	s.NotNil(created.Headers, "headers should be an empty array, not null")
	s.Empty(created.Headers, "No plain headers were given")
}

func (s *HTTPConnectionAPITestSuite) TestBasicPasswordIsMaskedAndUsernameIsNot() {
	created, _ := s.create(httpConnRequest{
		Name: "http-conn-api-basic", URL: "https://ext.example.com/basic", TimeoutMS: 2500,
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})
	s.Equal(2500, created.TimeoutMS, "Explicit timeout should round-trip")
	want := httpConnAuthentication{Type: "basic",
		Properties: map[string]string{"username": httpConnBasicUser, "password": maskedSecretValue}}
	s.Equal(want, created.Authentication, "Create should show the username and mask the password")
	s.Equal(want, s.get(created.ID).Authentication, "Get should show the username and mask the password")
	s.assertStoredAuth(created.ID, "basic", true)
}

func (s *HTTPConnectionAPITestSuite) TestListsShowConnectionWithoutSecrets() {
	created, _ := s.create(httpConnRequest{
		Name: "http-conn-api-list", Description: "listed", URL: "https://ext.example.com/list",
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})

	res, err := doRequest(http.MethodGet, httpConnPath, nil)
	s.Require().NoError(err, "Type list request failed")
	s.Require().Equal(http.StatusOK, res.status, "GET /connections/http should succeed: %s", string(res.body))
	s.assertNoSecretLeaked(res, "GET /connections/http")
	var summaries []connectionInstance
	s.Require().NoError(res.decode(&summaries), "GET /connections/http should be an array of summaries")
	var summary *connectionInstance
	for i := range summaries {
		if summaries[i].ID == created.ID {
			summary = &summaries[i]
		}
	}
	s.Require().NotNil(summary, "GET /connections/http should list the created connection")
	s.Equal("http-conn-api-list", summary.Name, "Summary name should match")
	s.Equal("listed", summary.Description, "Summary description should match")

	res, err = doRequest(http.MethodGet, "/connections?category=http&limit=100", nil)
	s.Require().NoError(err, "Category list request failed")
	s.Require().Equal(http.StatusOK, res.status, "List by category http should succeed: %s", string(res.body))
	s.assertNoSecretLeaked(res, "GET /connections?category=http")
	var list connectionListResponse
	s.Require().NoError(res.decode(&list), "Category list should decode")
	var found *connectionInstance
	for i := range list.Connections {
		s.Equal("http", list.Connections[i].Type, "category=http should list only http connections")
		if list.Connections[i].ID == created.ID {
			found = &list.Connections[i]
		}
	}
	s.Require().NotNil(found, "category=http should list the created connection")
	s.Equal("http-conn-api-list", found.Name, "Listed name should match")
	s.Equal("listed", found.Description, "Listed description should match")
	s.Equal([]string{"http"}, found.Categories, "HTTP connections belong to the http category")

	res, err = doRequest(http.MethodGet, "/connections?category=identity-provider&limit=100", nil)
	s.Require().NoError(err, "Identity provider list request failed")
	s.Require().Equal(http.StatusOK, res.status, "List by identity-provider should succeed")
	s.Require().NoError(res.decode(&list), "Identity provider list should decode")
	s.False(containsID(list.Connections, created.ID), "An HTTP connection is not an identity provider")
}

func (s *HTTPConnectionAPITestSuite) TestOmittedPasswordWithSameTargetIsKept() {
	const name = "http-conn-api-keep"
	created, _ := s.create(httpConnRequest{
		Name: name, URL: "https://ext.example.com/keep", Authentication: basicConnAuth(httpConnBasicUser,
			httpConnBasicPassword),
	})
	original, _, readable := s.storedPassword(created.ID)

	for _, tc := range []struct {
		context string
		url     string
	}{
		{"Omitted password, new path", "https://ext.example.com/keep/v2"},
		// Default ports are normalized and hosts compare case-insensitively, so these are the same origin.
		{"Explicit default port", "https://ext.example.com:443/keep/v3"},
		{"Host in other case", "https://EXT.example.com/keep/v4"},
	} {
		updated := s.requireUpdated(created.ID, httpConnRequest{
			Name: name, Description: tc.context, URL: tc.url, Authentication: basicConnAuth(httpConnBasicUser, ""),
		}, tc.context)
		s.Equal(tc.context, updated.Description, "%s: description should be updated", tc.context)
		s.Equal(map[string]string{"username": httpConnBasicUser, "password": maskedSecretValue},
			s.get(created.ID).Authentication.Properties, "%s: the kept password reads masked", tc.context)
		if readable {
			kept, present, _ := s.storedPassword(created.ID)
			s.True(present, "%s: the password should still be stored", tc.context)
			s.assertKeptSecret(original, kept, tc.context)
		}
	}

	s.requireUpdated(created.ID, httpConnRequest{
		Name: name, URL: "https://ext.example.com/keep",
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	}, "Supplying the password again")
	s.assertStoredAuth(created.ID, "basic", true)
}

func (s *HTTPConnectionAPITestSuite) TestCredentialTargetChangeWithoutPasswordIsRejected() {
	const name = "http-conn-api-target"
	const targetURL = "https://ext.example.com:8443/target"
	created, _ := s.create(httpConnRequest{
		Name: name, URL: targetURL, Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})
	before := s.get(created.ID)
	beforeRow, readable := s.storedRow(created.ID)

	for _, tc := range []struct {
		context  string
		url      string
		username string
	}{
		{"Username change", targetURL, "another-user"},
		{"Host change", "https://attacker.example.net:8443/target", httpConnBasicUser},
		{"Subdomain change", "https://api.ext.example.com:8443/target", httpConnBasicUser},
		{"Port change", "https://ext.example.com:9443/target", httpConnBasicUser},
		{"Port change to the default", "https://ext.example.com/target", httpConnBasicUser},
	} {
		res := s.put(created.ID, httpConnRequest{
			Name: name, URL: tc.url, Authentication: basicConnAuth(tc.username, ""),
		})
		s.assertError(res, http.StatusBadRequest, httpConnInvalidAuthCode, tc.context+" with the password omitted")
	}
	s.Equal(before, s.get(created.ID), "A rejected update must leave the connection unchanged")
	if readable {
		afterRow, _ := s.storedRow(created.ID)
		s.Equal(beforeRow, afterRow, "A rejected update must leave the stored record unchanged")
	}

	// A leading-zero port is the same port, so the origin is unchanged.
	s.requireUpdated(created.ID, httpConnRequest{
		Name: name, URL: "https://ext.example.com:08443/other", Authentication: basicConnAuth(httpConnBasicUser, ""),
	}, "Path-only change with a leading-zero port")

	updated := s.requireUpdated(created.ID, httpConnRequest{
		Name: name, URL: "https://moved.example.com/target",
		Authentication: basicConnAuth("another-user", httpConnBasicPassword2),
	}, "Origin and username change with a new password")
	s.Equal("https://moved.example.com/target", updated.URL, "URL should be the new origin")
	s.Equal("another-user", updated.Authentication.Properties["username"], "Username should be updated")
	s.assertStoredAuth(created.ID, "basic", true)
}

func (s *HTTPConnectionAPITestSuite) TestAuthenticationTypeChange() {
	const name = "http-conn-api-type"
	const typeURL = "https://ext.example.com/type"
	created, _ := s.create(httpConnRequest{Name: name, URL: typeURL})
	before := s.get(created.ID)

	res := s.put(created.ID, httpConnRequest{
		Name: name, URL: typeURL, Authentication: basicConnAuth(httpConnBasicUser, ""),
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidAuthCode, "none to basic without a password")
	s.Equal(before, s.get(created.ID), "A rejected type change must leave the connection unchanged")

	s.requireUpdated(created.ID, httpConnRequest{
		Name: name, URL: typeURL, Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	}, "none to basic with a password")
	s.assertStoredAuth(created.ID, "basic", true)

	updated := s.requireUpdated(created.ID, httpConnRequest{Name: name, URL: typeURL, Authentication: noneConnAuth()},
		"basic to none")
	s.Equal(httpConnAuthentication{Type: "none"}, updated.Authentication, "none should carry no properties")
	// Dropping to none discards the stored password, so switching back must supply it again.
	s.assertStoredAuth(created.ID, "none", false)

	res = s.put(created.ID, httpConnRequest{
		Name: name, URL: typeURL, Authentication: basicConnAuth(httpConnBasicUser, ""),
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidAuthCode,
		"Back to basic with the password omitted after it was discarded")
}

func (s *HTTPConnectionAPITestSuite) TestHTTPURLRequiresTypeNone() {
	res := s.post(httpConnRequest{
		Name: "http-conn-api-plain-basic", URL: "http://ext.example.com/plain",
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidURLCode, "Create with an http URL and basic")
	for method, auth := range map[string]*httpConnAuthentication{
		"bearer":  bearerConnAuth(httpConnBearerToken),
		"api_key": apiKeyConnAuth(map[string]string{httpConnAPIKeyHeader: httpConnAPIKeyValue}),
	} {
		res = s.post(httpConnRequest{Name: "http-conn-api-plain-" + method, URL: "http://ext.example.com/plain",
			Authentication: auth})
		s.assertError(res, http.StatusBadRequest, httpConnInvalidURLCode, "Create with an http URL and "+method)
	}

	const name = "http-conn-api-plain-none"
	created, _ := s.create(httpConnRequest{
		Name: name, URL: "http://localhost:9000/dev", Authentication: noneConnAuth(),
	})
	s.Equal("http://localhost:9000/dev", created.URL, "An http URL with none should be accepted")

	res = s.put(created.ID, httpConnRequest{
		Name: name, URL: "http://localhost:9000/dev",
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidURLCode, "Update adding basic to an http URL")

	const secured = "http-conn-api-plain-downgrade"
	created, _ = s.create(httpConnRequest{
		Name: secured, URL: "https://ext.example.com/plain",
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})
	res = s.put(created.ID, httpConnRequest{
		Name: secured, URL: "http://ext.example.com/plain",
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword2),
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidURLCode, "Update moving a basic connection to http")
	s.Equal("https://ext.example.com/plain", s.get(created.ID).URL, "A rejected downgrade must leave the URL unchanged")

	s.requireUpdated(created.ID, httpConnRequest{
		Name: secured, URL: "http://ext.example.com/plain", Authentication: noneConnAuth(),
	}, "Update to an http URL dropping the credential")
	s.assertStoredAuth(created.ID, "none", false)
}

func (s *HTTPConnectionAPITestSuite) TestURLIsStoredCanonical() {
	for i, tc := range []struct {
		in   string
		want string
	}{
		{"HTTPS://Ext.Example.COM:443/Path?q=1", "https://ext.example.com/Path?q=1"},
		{"http://EXT.example.com:80/x", "http://ext.example.com/x"},
		{"https://ext.example.com:08443/x", "https://ext.example.com:8443/x"},
		{"https://[::1]:443/x", "https://[::1]/x"},
		{"  https://ext.example.com/trimmed  ", "https://ext.example.com/trimmed"},
	} {
		created, _ := s.create(httpConnRequest{Name: fmt.Sprintf("http-conn-api-canonical-%d", i), URL: tc.in})
		s.Equal(tc.want, created.URL, "%q should be stored canonical", tc.in)
		s.Equal(tc.want, s.get(created.ID).URL, "%q should read back canonical", tc.in)
	}
}

func (s *HTTPConnectionAPITestSuite) TestInvalidURLIsRejected() {
	for _, tc := range []struct {
		context string
		url     string
	}{
		{"Missing URL", ""},
		{"Not a URL", "not a url"},
		{"Relative URL", "/extension"},
		{"ftp URL", "ftp://ext.example.com/x"},
		{"URL without a host", "https:///path"},
		{"Non-numeric port", "https://ext.example.com:port/x"},
		{"Port 0", "https://ext.example.com:0/x"},
		{"Port above 65535", "https://ext.example.com:65536/x"},
		{"Empty host label", "https://ext..example.com/x"},
		{"Host label starting with a hyphen", "https://-ext.example.com/x"},
		{"Host with an invalid character", "https://ext!.example.com/x"},
		{"Host label over 63 characters", "https://" + strings.Repeat("a", 64) + ".example.com/x"},
		// Userinfo would be returned in clear on every read, outside the secret masking.
		{"Userinfo with a password", "https://user:" + httpConnBasicPassword + "@ext.example.com/x"},
		{"Userinfo with a user only", "https://user@ext.example.com/x"},
		{"Empty userinfo", "https://@ext.example.com/x"},
	} {
		res := s.post(httpConnRequest{Name: "http-conn-api-badurl", URL: tc.url})
		s.assertError(res, http.StatusBadRequest, httpConnInvalidURLCode, tc.context)
		s.assertNoSecretLeaked(res, tc.context)
	}

	created, _ := s.create(httpConnRequest{Name: "http-conn-api-badurl", URL: "https://ext.example.com/x"})
	res := s.put(created.ID, httpConnRequest{Name: "http-conn-api-badurl", URL: "https://user:pw@ext.example.com/x"})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidURLCode, "Update with userinfo in the URL")
	s.Equal("https://ext.example.com/x", s.get(created.ID).URL, "A rejected update must leave the URL unchanged")
}

func (s *HTTPConnectionAPITestSuite) TestNameAndDescriptionValidation() {
	const url = "https://ext.example.com"
	for _, tc := range []struct {
		context string
		body    httpConnRequest
		code    string
	}{
		{"Missing name", httpConnRequest{URL: url}, httpConnInvalidNameCode},
		{"Blank name", httpConnRequest{Name: "   ", URL: url}, httpConnInvalidNameCode},
		{"Name of 256 characters", httpConnRequest{Name: strings.Repeat("n", 256), URL: url}, httpConnInvalidNameCode},
		{"Description of 501 characters", httpConnRequest{
			Name: "http-conn-api-desc-long", Description: strings.Repeat("d", 501), URL: url,
		}, httpConnInvalidDescriptionCode},
	} {
		res := s.post(tc.body)
		s.assertError(res, http.StatusBadRequest, tc.code, tc.context)
	}

	// The bounds are in characters, not bytes.
	longName := "http-conn-api-" + strings.Repeat("é", 255-len("http-conn-api-"))
	created, _ := s.create(httpConnRequest{Name: longName, Description: strings.Repeat("ü", 500), URL: url})
	fetched := s.get(created.ID)
	s.Equal(longName, fetched.Name, "A 255-character name should be stored whole")
	s.Equal(strings.Repeat("ü", 500), fetched.Description, "A 500-character description should be stored whole")

	res := s.put(created.ID, httpConnRequest{Name: "", URL: url})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidNameCode, "Update with an empty name")
	res = s.put(created.ID, httpConnRequest{Name: longName, Description: strings.Repeat("ü", 501), URL: url})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidDescriptionCode, "Update with a 501-character description")
}

func (s *HTTPConnectionAPITestSuite) TestTimeoutValidation() {
	created, _ := s.create(httpConnRequest{
		Name: "http-conn-api-timeout-max", URL: "https://ext.example.com", TimeoutMS: httpConnMaxTimeoutMS,
	})
	s.Equal(httpConnMaxTimeoutMS, created.TimeoutMS, "The maximum timeout should be accepted")

	created, _ = s.create(httpConnRequest{
		Name: "http-conn-api-timeout-min", URL: "https://ext.example.com", TimeoutMS: 1,
	})
	s.Equal(1, created.TimeoutMS, "The minimum timeout should be accepted")

	for _, timeout := range []int{httpConnMaxTimeoutMS + 1, 60000, -1} {
		res := s.post(httpConnRequest{
			Name: "http-conn-api-timeout-bad", URL: "https://ext.example.com", TimeoutMS: timeout,
		})
		s.assertError(res, http.StatusBadRequest, httpConnInvalidTimeoutCode,
			fmt.Sprintf("Create with timeout %d", timeout))
	}

	res := s.put(created.ID, httpConnRequest{
		Name: "http-conn-api-timeout-min", URL: "https://ext.example.com", TimeoutMS: httpConnMaxTimeoutMS + 1,
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidTimeoutCode, "Update with a timeout above the maximum")
	s.Equal(1, s.get(created.ID).TimeoutMS, "A rejected update must leave the timeout unchanged")
}

func (s *HTTPConnectionAPITestSuite) TestHeaderValidationAndReplacement() {
	const url = "https://ext.example.com"
	cases := []struct {
		context string
		headers []httpConnHeader
	}{
		{"Header name repeated in other case", []httpConnHeader{{Name: "X-A", Value: "1"}, {Name: "x-a", Value: "2"}}},
		{"Header name with a space", []httpConnHeader{{Name: "Bad Header", Value: "1"}}},
		{"Empty header name", []httpConnHeader{{Name: "", Value: "1"}}},
		{"Header value with a line break", []httpConnHeader{{Name: "X-A", Value: "1\r\nX-Injected: 2"}}},
	}
	// Credentials belong in authentication, and the client manages the rest itself; any case is refused.
	for _, reserved := range []string{
		"Authorization", "proxy-authorization", "COOKIE", "Host", "Content-Length", "content-type",
		"Transfer-Encoding", "Connection", "te", "Trailer", "Upgrade", "Keep-Alive", "EXPECT",
		"x-API-key", "API-KEY", "x-Auth-Token", "X-ACCESS-TOKEN",
	} {
		cases = append(cases, struct {
			context string
			headers []httpConnHeader
		}{"Reserved header " + reserved, []httpConnHeader{{Name: reserved, Value: "v"}}})
	}
	for _, tc := range cases {
		res := s.post(httpConnRequest{Name: "http-conn-api-badheader", URL: url, Headers: tc.headers})
		s.assertError(res, http.StatusBadRequest, httpConnInvalidHeadersCode, tc.context)
	}

	const name = "http-conn-api-headers"
	created, _ := s.create(httpConnRequest{
		Name: name, URL: url, Headers: []httpConnHeader{{Name: "X-One", Value: "1"}, {Name: "X-Two", Value: "2"}},
	})
	s.requireUpdated(created.ID, httpConnRequest{
		Name: name, URL: url, Headers: []httpConnHeader{{Name: "X-Three", Value: "3"}},
	}, "Replacing the headers")
	s.Equal([]httpConnHeader{{Name: "X-Three", Value: "3"}}, s.get(created.ID).Headers,
		"A PUT should replace the whole header list")

	s.requireUpdated(created.ID, httpConnRequest{Name: name, URL: url}, "Omitting the headers")
	fetched := s.get(created.ID)
	s.NotNil(fetched.Headers, "headers should be an empty array, not null")
	s.Empty(fetched.Headers, "Omitted headers on a PUT mean none")

	res := s.put(created.ID, httpConnRequest{
		Name: name, URL: url, Headers: []httpConnHeader{{Name: "Authorization", Value: "Bearer x"}},
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidHeadersCode, "Update with a reserved header")
	res = s.put(created.ID, httpConnRequest{
		Name: name, URL: url, Headers: []httpConnHeader{{Name: "x-auth-TOKEN", Value: "t"}},
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidHeadersCode, "Update with a credential header name")

	// A plain header may not repeat, in any case, a header the api_key authentication sets.
	res = s.post(httpConnRequest{
		Name: "http-conn-api-badheader", URL: url,
		Headers:        []httpConnHeader{{Name: "x-tenant-KEY", Value: "plain"}},
		Authentication: apiKeyConnAuth(map[string]string{httpConnAPIKeyHeader: httpConnAPIKeyValue}),
	})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidHeadersCode, "Plain header repeating an API-key header")
}

func (s *HTTPConnectionAPITestSuite) TestAuthenticationValidation() {
	const url = "https://ext.example.com"
	for _, tc := range []struct {
		context string
		auth    *httpConnAuthentication
		code    string
	}{
		{"Unknown type", &httpConnAuthentication{Type: "digest"}, httpConnInvalidAuthTypeCode},
		{"bearer without a token", bearerConnAuth(""), httpConnInvalidAuthCode},
		{"api_key without headers", apiKeyConnAuth(nil), httpConnInvalidAuthCode},
		{"api_key with an HTTP-managed header", apiKeyConnAuth(map[string]string{"Host": httpConnAPIKeyValue}),
			httpConnInvalidAuthCode},
		{"basic without properties", &httpConnAuthentication{Type: "basic"}, httpConnInvalidAuthCode},
		{"basic without a username", basicConnAuth("", httpConnBasicPassword), httpConnInvalidAuthCode},
		{"basic with a blank username", basicConnAuth("   ", httpConnBasicPassword), httpConnInvalidAuthCode},
		{"basic without a password", basicConnAuth(httpConnBasicUser, ""), httpConnInvalidAuthCode},
		{"basic with a blank password", basicConnAuth(httpConnBasicUser, "   "), httpConnInvalidAuthCode},
		// RFC 7617: the user-id cannot contain a colon, or the server would split the credential elsewhere.
		{"basic username with a colon", basicConnAuth("a:b", httpConnBasicPassword), httpConnInvalidAuthCode},
	} {
		res := s.post(httpConnRequest{Name: "http-conn-api-badauth", URL: url, Authentication: tc.auth})
		s.assertError(res, http.StatusBadRequest, tc.code, tc.context)
	}

	// The type is matched case-insensitively.
	created, _ := s.create(httpConnRequest{
		Name: "http-conn-api-auth-case", URL: url,
		Authentication: &httpConnAuthentication{Type: "BASIC",
			Properties: map[string]string{"username": httpConnBasicUser, "password": httpConnBasicPassword}},
	})
	s.Equal("basic", created.Authentication.Type, "An upper-case type should be stored as basic")
}

func (s *HTTPConnectionAPITestSuite) TestMalformedBodyIsRejected() {
	res, err := doRawRequest(http.MethodPost, httpConnPath, `{"name": "http-conn-api-bad-json", "url": `)
	s.Require().NoError(err, "Malformed body request failed")
	s.assertError(res, http.StatusBadRequest, httpConnInvalidFormatCode, "Create with malformed JSON")
}

func (s *HTTPConnectionAPITestSuite) TestDuplicateNameIsRejected() {
	first, _ := s.create(httpConnRequest{Name: "http-conn-api-dup", URL: "https://ext.example.com/a"})

	res := s.post(httpConnRequest{Name: "http-conn-api-dup", URL: "https://ext.example.com/b"})
	s.assertError(res, http.StatusConflict, httpConnDuplicateCode, "Create with a taken name")

	second, _ := s.create(httpConnRequest{Name: "http-conn-api-dup-2", URL: "https://ext.example.com/c"})
	res = s.put(second.ID, httpConnRequest{Name: "http-conn-api-dup", URL: "https://ext.example.com/c"})
	s.assertError(res, http.StatusConflict, httpConnDuplicateCode, "Rename onto a taken name")
	s.Equal("http-conn-api-dup-2", s.get(second.ID).Name, "A rejected rename must leave the name unchanged")

	s.requireUpdated(first.ID, httpConnRequest{Name: "http-conn-api-dup", URL: "https://ext.example.com/a2"},
		"Updating a connection under its own name")
}

func (s *HTTPConnectionAPITestSuite) TestDeleteThenNotFound() {
	created, _ := s.create(httpConnRequest{
		Name: "http-conn-api-delete", URL: "https://ext.example.com/delete",
		Authentication: basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
	})

	res, err := doRequest(http.MethodDelete, httpConnPath+"/"+created.ID, nil)
	s.Require().NoError(err, "Delete request failed")
	s.Require().Equal(http.StatusNoContent, res.status, "Delete should succeed: %s", string(res.body))
	s.Empty(res.body, "A 204 should carry no body")

	res, err = doRequest(http.MethodGet, httpConnPath+"/"+created.ID, nil)
	s.Require().NoError(err, "Get after delete request failed")
	s.assertError(res, http.StatusNotFound, httpConnNotFoundCode, "Get after delete")

	// Deleting a connection that is already gone is a no-op that succeeds.
	res, err = doRequest(http.MethodDelete, httpConnPath+"/"+created.ID, nil)
	s.Require().NoError(err, "Second delete request failed")
	s.Equal(http.StatusNoContent, res.status, "A second delete should succeed: %s", string(res.body))
	s.Empty(res.body, "A 204 should carry no body")

	if testutils.GetDBType() == "sqlite" {
		count, err := testutils.QueryConfigDB(
			fmt.Sprintf(`SELECT COUNT(*) FROM "HTTP_CONNECTION" WHERE ID = '%s';`, created.ID))
		s.Require().NoError(err, "Counting the deleted row failed")
		s.Equal("0", count, "Delete must remove the stored record, encrypted password included")
	}
}

func (s *HTTPConnectionAPITestSuite) TestUnknownConnectionIsNotFound() {
	const unknownID = "00000000-0000-0000-0000-00000000c0de"
	for _, tc := range []struct {
		method string
		suffix string
		body   interface{}
	}{
		{http.MethodGet, "", nil},
		{http.MethodPut, "", httpConnRequest{Name: "http-conn-api-ghost", URL: "https://ext.example.com"}},
		{http.MethodGet, "/usages", nil},
	} {
		res, err := doRequest(tc.method, httpConnPath+"/"+unknownID+tc.suffix, tc.body)
		s.Require().NoError(err, "%s%s unknown connection request failed", tc.method, tc.suffix)
		s.assertError(res, http.StatusNotFound, httpConnNotFoundCode, tc.method+tc.suffix+" unknown connection")
	}

	res, err := doRequest(http.MethodDelete, httpConnPath+"/"+unknownID, nil)
	s.Require().NoError(err, "Delete unknown connection request failed")
	s.Equal(http.StatusNoContent, res.status, "Deleting an unknown connection should be a no-op: %s",
		string(res.body))

	created, _ := s.create(httpConnRequest{Name: "http-conn-api-isolation", URL: "https://ext.example.com"})
	res, err = doRequest(http.MethodGet, "/connections/authzen-pdp/"+created.ID, nil)
	s.Require().NoError(err, "Cross-type get failed")
	s.Equal(http.StatusNotFound, res.status, "An HTTP connection must not be readable as an AuthZEN PDP")
}

// storedSecretValue returns the stored ciphertext of an authentication property, and whether the
// configuration database could be read at all. The property must be stored, as a secret.
func (s *HTTPConnectionAPITestSuite) storedSecretValue(id, property string) (string, bool) {
	s.T().Helper()
	props, ok := s.stored(id)
	if !ok {
		return "", false
	}
	prop, present := props.property(property)
	s.Require().True(present, "The stored record should hold %s", property)
	s.True(prop.IsSecret, "%s should be stored as a secret", property)
	s.NotEqual(maskedSecretValue, prop.Value, "%s must not be stored as the mask", property)
	return prop.Value, true
}

// AC2.9: bearer and api_key connections store every secret encrypted and read it back masked,
// with the api_key header names in clear.
func (s *HTTPConnectionAPITestSuite) TestBearerAndAPIKeySecretsAreMasked() {
	bearer, _ := s.create(httpConnRequest{
		Name: "http-conn-api-bearer", URL: "https://ext.example.com/bearer",
		Authentication: bearerConnAuth(httpConnBearerToken),
	})
	s.Equal(httpConnAuthentication{Type: "bearer", Properties: map[string]string{"token": maskedSecretValue}},
		bearer.Authentication, "The create response masks the token")
	s.Equal(bearer.Authentication, s.get(bearer.ID).Authentication, "A read masks the token")
	s.storedSecretValue(bearer.ID, httpConnStoredBearerToken)
	s.assertStoredAuth(bearer.ID, "bearer", false)

	apiKey, _ := s.create(httpConnRequest{
		Name: "http-conn-api-apikey", URL: "https://ext.example.com/apikey",
		Authentication: apiKeyConnAuth(map[string]string{
			"x-tenant-key": httpConnAPIKeyValue, httpConnAPIKeyHeader2: httpConnAPIKeyValue2}),
	})
	want := httpConnAuthentication{Type: "api_key", Properties: map[string]string{
		httpConnAPIKeyHeader: maskedSecretValue, httpConnAPIKeyHeader2: maskedSecretValue}}
	s.Equal(want, apiKey.Authentication, "Header names read in clear and canonical, every value masked")
	s.Equal(want, s.get(apiKey.ID).Authentication, "A read masks every API-key value")
	s.storedSecretValue(apiKey.ID, httpConnStoredAPIKeyHeader)
	s.assertStoredAuth(apiKey.ID, "api_key", false)
}

// AC2.11: the mask sent as a password or token is refused on create and on update, and the stored
// secret is unchanged.
func (s *HTTPConnectionAPITestSuite) TestMaskedPasswordOrTokenIsRefused() {
	const url = "https://ext.example.com/masked"
	for method, auth := range map[string]*httpConnAuthentication{
		"basic":  basicConnAuth(httpConnBasicUser, maskedSecretValue),
		"bearer": bearerConnAuth(maskedSecretValue),
	} {
		res := s.post(httpConnRequest{Name: "http-conn-api-masked-create", URL: url, Authentication: auth})
		s.assertError(res, http.StatusBadRequest, httpConnInvalidAuthCode, "Create with a masked "+method+" secret")
	}

	for _, tc := range []struct {
		method   string
		stored   *httpConnAuthentication
		masked   *httpConnAuthentication
		property string
	}{
		{"basic", basicConnAuth(httpConnBasicUser, httpConnBasicPassword),
			basicConnAuth(httpConnBasicUser, maskedSecretValue), httpConnStoredPassword},
		{"bearer", bearerConnAuth(httpConnBearerToken), bearerConnAuth(maskedSecretValue),
			httpConnStoredBearerToken},
	} {
		name := "http-conn-api-masked-" + tc.method
		created, _ := s.create(httpConnRequest{Name: name, URL: url, Authentication: tc.stored})
		before := s.get(created.ID)
		original, readable := s.storedSecretValue(created.ID, tc.property)

		// The read sent back unchanged carries the mask.
		res := s.put(created.ID, httpConnRequest{Name: name, Description: "changed", URL: url,
			Authentication: tc.masked})
		s.assertError(res, http.StatusBadRequest, httpConnInvalidAuthCode, "Update with a masked "+tc.method+" secret")
		s.Equal(before, s.get(created.ID), "A refused update must leave the connection unchanged")
		if readable {
			kept, _ := s.storedSecretValue(created.ID, tc.property)
			s.Equal(original, kept, "The stored %s secret must be unchanged", tc.method)
		}
	}
}

// An api_key header sent as the mask keeps that header's stored value on update while the origin is
// unchanged; a header left out is removed; on create the mask is refused.
func (s *HTTPConnectionAPITestSuite) TestMaskedAPIKeyValueKeepsTheStoredValueOnUpdate() {
	const name = "http-conn-api-apikey-keep"
	const url = "https://ext.example.com/apikey-keep"
	res := s.post(httpConnRequest{Name: name, URL: url,
		Authentication: apiKeyConnAuth(map[string]string{httpConnAPIKeyHeader: maskedSecretValue})})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidAuthCode, "Create with a masked API-key value")

	created, _ := s.create(httpConnRequest{Name: name, URL: url, Authentication: apiKeyConnAuth(map[string]string{
		httpConnAPIKeyHeader: httpConnAPIKeyValue, httpConnAPIKeyHeader2: httpConnAPIKeyValue2})})
	original, readable := s.storedSecretValue(created.ID, httpConnStoredAPIKeyHeader)

	updated := s.requireUpdated(created.ID, httpConnRequest{Name: name, URL: url + "/v2",
		Authentication: apiKeyConnAuth(map[string]string{httpConnAPIKeyHeader: maskedSecretValue})},
		"Update keeping one API-key header by its mask")
	s.Equal(map[string]string{httpConnAPIKeyHeader: maskedSecretValue}, updated.Authentication.Properties,
		"The header left out is removed")
	if readable {
		kept, _ := s.storedSecretValue(created.ID, httpConnStoredAPIKeyHeader)
		s.assertKeptSecret(original, kept, "The masked API-key header")
	}

	res = s.put(created.ID, httpConnRequest{Name: name, URL: "https://moved.example.com/apikey",
		Authentication: apiKeyConnAuth(map[string]string{httpConnAPIKeyHeader: maskedSecretValue})})
	s.assertError(res, http.StatusBadRequest, httpConnInvalidAuthCode, "Masked API-key value with a new origin")
}

func (s *HTTPConnectionAPITestSuite) TestMetaDescribesTheFourMethods() {
	res, err := doRequest(http.MethodGet, "/connections/meta?vendor=http", nil)
	s.Require().NoError(err, "Meta request failed")
	s.Require().Equal(http.StatusOK, res.status, "Meta for vendor http should succeed: %s", string(res.body))
	var meta connectionMetaResponse
	s.Require().NoError(res.decode(&meta), "Meta response should decode")
	s.Require().Len(meta.Vendors, 1, "Filtering by vendor should return one entry")
	s.Equal("http", meta.Vendors[0].Vendor, "The entry should describe the http vendor")

	methods := meta.Vendors[0].Authentication.Methods
	index := map[string]int{}
	types := make([]string, 0, len(methods))
	for i, method := range methods {
		index[method.Type] = i
		types = append(types, method.Type)
		s.NotEmpty(method.DisplayName, "%s should have a display name", method.Type)
	}
	s.Require().ElementsMatch([]string{"none", "basic", "bearer", "api_key"}, types,
		"HTTP connections offer none, basic, bearer and api_key")
	s.Empty(methods[index["none"]].Fields, "none has no fields")
	basic := methods[index["basic"]].Fields
	s.Require().Len(basic, 2, "basic has a username and a password")
	s.Equal("username", basic[0].Key, "The username comes first")
	s.True(basic[0].Required, "The username is required")
	s.False(basic[0].Credential, "The username is not a credential")
	s.Equal("password", basic[1].Key, "The password comes second")
	s.True(basic[1].Required, "The password is required")
	s.True(basic[1].Credential, "The password is a credential")
	bearer := methods[index["bearer"]].Fields
	s.Require().Len(bearer, 1, "bearer has a token")
	s.Equal("token", bearer[0].Key, "The bearer field is the token")
	s.True(bearer[0].Credential, "The token is a credential")
}
