// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzen

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type authZENPDPConnectionResponse struct {
	ID                       string                              `json:"id"`
	Name                     string                              `json:"name"`
	Description              string                              `json:"description,omitempty"`
	Type                     string                              `json:"type"`
	Endpoint                 string                              `json:"endpoint"`
	BatchEndpoint            string                              `json:"batchEndpoint,omitempty"`
	TimeoutMS                int                                 `json:"timeoutMs"`
	RetryCount               int                                 `json:"retryCount"`
	Authentication           authZENPDPAuthenticationResponse    `json:"authentication"`
	SubjectAttributeMappings []authZENPDPSubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
}

type authZENPDPAuthenticationResponse struct {
	Scheme        string                `json:"scheme"`
	APIKeyHeaders []authZENPDPAPIHeader `json:"apiKeyHeaders,omitempty"`
}

type authZENPDPAPIHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type authZENPDPSubjectAttributeMapping struct {
	EntityType string                          `json:"entityType"`
	Attributes []authZENPDPSubjectAttributeRow `json:"attributes"`
}

type authZENPDPSubjectAttributeRow struct {
	Attribute    string `json:"attribute"`
	PDPAttribute string `json:"pdpAttribute,omitempty"`
}

type authZENPDPConnectionSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type authZENPDPDependenciesResponse struct {
	TotalResults int                       `json:"totalResults"`
	Count        int                       `json:"count"`
	Summary      map[string]int            `json:"summary"`
	Usages       []authZENPDPResourceUsage `json:"usages"`
}

type authZENPDPResourceUsage struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

const authZENPDPResourceIdentifier = "https://authzen-api.example.com"

type authZENPDPResourceServerUpdate struct {
	Name                string                              `json:"name"`
	Description         string                              `json:"description,omitempty"`
	Identifier          string                              `json:"identifier,omitempty"`
	OUID                string                              `json:"ouId"`
	AuthorizationEngine authZENPDPAuthorizationEngineConfig `json:"authorizationEngine,omitempty"`
}

type authZENPDPAuthorizationEngineConfig struct {
	Type       string                             `json:"type,omitempty"`
	Properties authZENPDPAuthorizationEngineProps `json:"properties,omitempty"`
}

type authZENPDPAuthorizationEngineProps struct {
	PDPConnectionID string `json:"pdpConnectionId,omitempty"`
}

type authZENPDPBatchRequest struct {
	Evaluations []struct {
		Subject  map[string]interface{} `json:"subject"`
		Resource map[string]interface{} `json:"resource"`
		Action   map[string]interface{} `json:"action"`
		Context  map[string]interface{} `json:"context,omitempty"`
	} `json:"evaluations"`
}

type authZENPDPBatchResponse struct {
	Evaluations []evaluationResponse `json:"evaluations"`
}

type authZENPDPSingleRequest struct {
	Subject  map[string]interface{} `json:"subject"`
	Resource map[string]interface{} `json:"resource"`
	Action   map[string]interface{} `json:"action"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

func TestAuthZENPDPIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AuthZENPDPIntegrationSuite))
}

type AuthZENPDPIntegrationSuite struct {
	suite.Suite
	pdpServer    *httptest.Server
	ouID         string
	userTypeID   string
	userID       string
	rsID         string
	actionIDs    []string
	connectionID string
	appID        string
	roleID       string
}

func (s *AuthZENPDPIntegrationSuite) SetupSuite() {
	s.pdpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Require().Equal("authzen-primary-key", r.Header.Get("X-AuthZEN-Key"))
		s.Require().Equal("authzen-tenant", r.Header.Get("X-AuthZEN-Tenant"))
		switch r.URL.Path {
		case "/access/v1/evaluation":
			var evaluation authZENPDPSingleRequest
			if err := json.NewDecoder(r.Body).Decode(&evaluation); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.Contains(
				[]string{s.rsID, authZENPDPResourceIdentifier},
				evaluation.Resource["type"],
			)
			actionName, _ := evaluation.Action["name"].(string)
			response := evaluationResponse{
				Decision: actionName == "read" || actionName == "write",
				Context:  map[string]interface{}{"source": "authzen-pdp"},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/access/v1/evaluations":
			var request authZENPDPBatchRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			response := authZENPDPBatchResponse{
				Evaluations: make([]evaluationResponse, 0, len(request.Evaluations)),
			}
			for _, evaluation := range request.Evaluations {
				s.Contains(
					[]string{s.rsID, authZENPDPResourceIdentifier},
					evaluation.Resource["type"],
				)
				actionName, _ := evaluation.Action["name"].(string)
				decision := actionName == "read" || actionName == "write"
				if actionName == "attribute-check" {
					subjectProperties, _ := evaluation.Subject["properties"].(map[string]interface{})
					resourceProperties, _ := evaluation.Resource["properties"].(map[string]interface{})
					actionProperties, _ := evaluation.Action["properties"].(map[string]interface{})
					decision = subjectProperties["preferred_username"] == "authzen-pdp-user" &&
						resourceProperties["classification"] == "confidential" &&
						actionProperties["risk"] == "low" && evaluation.Context["tenant"] == "acme"
				}
				response.Evaluations = append(response.Evaluations, evaluationResponse{
					Decision: decision,
					Context:  map[string]interface{}{"source": "authzen-pdp"},
				})
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "/unavailable":
			http.Error(w, "PDP unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "authzen-pdp-test-ou",
		Name:   "AuthZEN PDP Test OU",
	})
	s.Require().NoError(err)
	s.ouID = ouID
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: "authzen-pdp-person",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"email":    map[string]interface{}{"type": "string", "unique": true},
		},
	})
	s.Require().NoError(err)
	s.userTypeID = userTypeID
	userID, err := testutils.CreateUser(testutils.User{
		Type:       "authzen-pdp-person",
		OUID:       s.ouID,
		Attributes: json.RawMessage(`{"username":"authzen-pdp-user"}`),
	})
	s.Require().NoError(err)
	s.userID = userID

	resourceServer, err := createResourceServer(testutils.ResourceServer{
		Name:       "AuthZEN API",
		Identifier: authZENPDPResourceIdentifier,
		OUID:       s.ouID,
	})
	s.Require().NoError(err)
	s.Require().Equal(authZENPDPResourceIdentifier, resourceServer.Identifier)
	s.rsID = resourceServer.ID
	for _, actionConfig := range []testutils.Action{
		{Name: "Read bookings", Handle: "read"},
		{Name: "Write bookings", Handle: "write"},
		{Name: "Delete bookings", Handle: "delete"},
		{Name: "Evaluate booking attributes", Handle: "attribute-check"},
	} {
		createdAction, actionErr := createAction(s.rsID, "", actionConfig)
		s.Require().NoError(actionErr)
		s.actionIDs = append(s.actionIDs, createdAction.ID)
	}

	connectionBody := map[string]interface{}{
		"name":          "AuthZEN PDP",
		"description":   "AuthZEN PDP used by the AuthZEN integration suite",
		"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
		"timeoutMs":     750,
		"retryCount":    2,
		"authentication": map[string]interface{}{
			"scheme": "API_KEY",
			"apiKeyHeaders": []authZENPDPAPIHeader{
				{Name: "X-AuthZEN-Key", Value: "authzen-primary-key"},
				{Name: "X-AuthZEN-Tenant", Value: "authzen-tenant"},
			},
		},
		"subjectAttributeMappings": []authZENPDPSubjectAttributeMapping{{
			EntityType: "authzen-pdp-person",
			Attributes: []authZENPDPSubjectAttributeRow{{
				Attribute:    "username",
				PDPAttribute: "preferred_username",
			}},
		}},
	}
	connectionPayload, err := json.Marshal(connectionBody)
	s.Require().NoError(err)
	request, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/connections/authzen-pdp", bytes.NewReader(connectionPayload))
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, response.StatusCode, string(body))
	s.NotContains(string(body), `"subjectCategory"`)
	var connection authZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.connectionID = connection.ID

	updateBody, err := json.Marshal(authZENPDPResourceServerUpdate{
		Name:       "AuthZEN API",
		Identifier: authZENPDPResourceIdentifier,
		OUID:       s.ouID,
		AuthorizationEngine: authZENPDPAuthorizationEngineConfig{
			Type: "authzen_pdp",
			Properties: authZENPDPAuthorizationEngineProps{
				PDPConnectionID: s.connectionID,
			},
		},
	})
	s.Require().NoError(err)
	request, err = http.NewRequest(http.MethodPut, testutils.TestServerURL+"/resource-servers/"+s.rsID, bytes.NewReader(updateBody))
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	s.appID, err = testutils.CreateApplication(testutils.Application{
		Name:         "AuthZEN Token Test App",
		Description:  "M2M application for AuthZEN token issuance testing",
		OUID:         s.ouID,
		Type:         "m2m",
		ClientID:     "authzen_token_test_client",
		ClientSecret: "authzen_token_test_secret",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "authzen_token_test_client",
				"clientSecret":            "authzen_token_test_secret",
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)

	s.roleID, err = testutils.CreateRole(testutils.Role{
		Name:        "AuthZEN Token Test Role",
		Description: "Permission for AuthZEN token issuance testing",
		OUID:        s.ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: s.rsID,
			Permissions:      []string{"read", "write", "delete", "attribute-check"},
		}},
		Assignments: []testutils.Assignment{{ID: s.appID, Type: "app"}},
	})
	s.Require().NoError(err)
}

func (s *AuthZENPDPIntegrationSuite) TearDownSuite() {
	if s.roleID != "" {
		_ = testutils.DeleteRole(s.roleID)
	}
	if s.appID != "" {
		_ = testutils.DeleteApplication(s.appID)
	}
	for _, actionID := range s.actionIDs {
		request, err := http.NewRequest(
			http.MethodDelete,
			testutils.TestServerURL+"/resource-servers/"+s.rsID+"/actions/"+actionID,
			nil,
		)
		if err == nil {
			response, requestErr := testutils.GetHTTPClient().Do(request)
			if requestErr == nil {
				_ = response.Body.Close()
			}
		}
	}
	if s.rsID != "" {
		_ = testutils.DeleteResourceServer(s.rsID)
	}
	if s.userID != "" {
		_ = testutils.DeleteUser(s.userID)
	}
	if s.userTypeID != "" {
		_ = testutils.DeleteUserType(s.userTypeID)
	}
	if s.connectionID != "" {
		request, err := http.NewRequest(http.MethodDelete, testutils.TestServerURL+"/connections/authzen-pdp/"+s.connectionID, nil)
		if err == nil {
			response, requestErr := testutils.GetHTTPClient().Do(request)
			if requestErr == nil {
				_ = response.Body.Close()
			}
		}
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
	if s.pdpServer != nil {
		s.pdpServer.Close()
	}
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPConnectionEvaluatesAccess() {
	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{{
		Subject:  subject{Type: "user", ID: s.userID},
		Resource: resource{Type: authZENPDPResourceIdentifier, ID: "booking-1"},
		Action:   action{Name: "read"},
	}}})

	request, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/access/v1/evaluations", bytes.NewReader(payload))
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var result evaluationsResponse
	s.Require().NoError(json.Unmarshal(body, &result))
	s.Require().Len(result.Evaluations, 1)
	s.True(result.Evaluations[0].Decision)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPBatchContinuesForUnknownUser() {
	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{
		{
			Subject:  subject{Type: "user", ID: "unknown-authzen-pdp-user"},
			Resource: resource{Type: authZENPDPResourceIdentifier, ID: "booking-unknown-user"},
			Action:   action{Name: "read"},
		},
		{
			Subject:  subject{Type: "user", ID: s.userID},
			Resource: resource{Type: authZENPDPResourceIdentifier, ID: "booking-known-user"},
			Action:   action{Name: "write"},
		},
	}})

	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/access/v1/evaluations",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var result evaluationsResponse
	s.Require().NoError(json.Unmarshal(body, &result))
	s.Require().Len(result.Evaluations, 2)
	s.False(result.Evaluations[0].Decision)
	s.True(result.Evaluations[1].Decision)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPBatchFallsBackToSingleEndpoint() {
	original := s.getAuthZENPDPConnection()
	fallback := original
	fallback.BatchEndpoint = ""
	s.updateAuthZENPDPConnection(fallback)
	defer s.updateAuthZENPDPConnection(original)

	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{
		{
			Subject:  subject{Type: "user", ID: s.userID},
			Resource: resource{Type: authZENPDPResourceIdentifier, ID: "booking-single-read"},
			Action:   action{Name: "read"},
		},
		{
			Subject:  subject{Type: "user", ID: s.userID},
			Resource: resource{Type: authZENPDPResourceIdentifier, ID: "booking-single-write"},
			Action:   action{Name: "write"},
		},
	}})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/access/v1/evaluations",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var result evaluationsResponse
	s.Require().NoError(json.Unmarshal(body, &result))
	s.Require().Len(result.Evaluations, 2)
	s.True(result.Evaluations[0].Decision)
	s.True(result.Evaluations[1].Decision)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPAllDeniedIssuesTokenWithoutPermissions() {
	status, body, tokenResponse := s.requestClientCredentialsToken("delete")
	s.Require().Equal(http.StatusOK, status, string(body))
	s.NotEmpty(tokenResponse.AccessToken)
	s.Empty(tokenResponse.Scope)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPFiltersMixedTokenPermissions() {
	status, body, tokenResponse := s.requestClientCredentialsToken("read write delete")
	s.Require().Equal(http.StatusOK, status, string(body))
	s.NotEmpty(tokenResponse.AccessToken)
	s.ElementsMatch([]string{"read", "write"}, strings.Fields(tokenResponse.Scope))

	claims, err := testutils.DecodeJWT(tokenResponse.AccessToken)
	s.Require().NoError(err)
	s.Equal(authZENPDPResourceIdentifier, claims.Aud)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPForwardsEvaluationAttributesAndContext() {
	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{{
		Subject: subject{Type: "user", ID: s.userID},
		Resource: resource{
			Type:       authZENPDPResourceIdentifier,
			ID:         "booking-attributes",
			Properties: map[string]interface{}{"classification": "confidential"},
		},
		Action: action{
			Name:       "attribute-check",
			Properties: map[string]interface{}{"risk": "low"},
		},
		Context: map[string]interface{}{"tenant": "acme"},
	}}})

	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/access/v1/evaluations",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var result evaluationsResponse
	s.Require().NoError(json.Unmarshal(body, &result))
	s.Require().Len(result.Evaluations, 1)
	s.True(result.Evaluations[0].Decision)
	s.Equal("authzen-pdp", result.Evaluations[0].Context["source"])
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPConnectionSettingsAndUsagePersist() {
	request, err := http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/authzen-pdp/"+s.connectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var connection authZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.Equal(s.connectionID, connection.ID)
	s.Equal("AuthZEN PDP", connection.Name)
	s.Equal("AuthZEN PDP used by the AuthZEN integration suite", connection.Description)
	s.Equal("authzen-pdp", connection.Type)
	s.Equal(s.pdpServer.URL+"/access/v1/evaluation", connection.Endpoint)
	s.Equal(s.pdpServer.URL+"/access/v1/evaluations", connection.BatchEndpoint)
	s.Equal(750, connection.TimeoutMS)
	s.Equal(2, connection.RetryCount)
	s.Equal("API_KEY", connection.Authentication.Scheme)
	s.Require().Len(connection.Authentication.APIKeyHeaders, 2)
	s.Equal("X-Authzen-Key", connection.Authentication.APIKeyHeaders[0].Name)
	s.Equal("******", connection.Authentication.APIKeyHeaders[0].Value)
	s.Equal("X-Authzen-Tenant", connection.Authentication.APIKeyHeaders[1].Name)
	s.Equal("******", connection.Authentication.APIKeyHeaders[1].Value)
	s.Require().Len(connection.SubjectAttributeMappings, 1)
	s.Equal("authzen-pdp-person", connection.SubjectAttributeMappings[0].EntityType)
	s.Require().Len(connection.SubjectAttributeMappings[0].Attributes, 1)
	s.Equal("preferred_username", connection.SubjectAttributeMappings[0].Attributes[0].PDPAttribute)

	updatedPayload := mustJSON(map[string]interface{}{
		"name":          "AuthZEN PDP",
		"description":   "Updated AuthZEN PDP connection",
		"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
		"timeoutMs":     900,
		"retryCount":    3,
		"subjectAttributeMappings": []authZENPDPSubjectAttributeMapping{{
			EntityType: "authzen-pdp-person",
			Attributes: []authZENPDPSubjectAttributeRow{
				{Attribute: "username", PDPAttribute: "preferred_username"},
				{Attribute: "email"},
			},
		}},
	})
	request, err = http.NewRequest(
		http.MethodPut,
		testutils.TestServerURL+"/connections/authzen-pdp/"+s.connectionID,
		bytes.NewReader(updatedPayload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	request, err = http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/authzen-pdp/"+s.connectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))
	var updatedConnection authZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &updatedConnection))
	s.Equal("Updated AuthZEN PDP connection", updatedConnection.Description)
	s.Equal(900, updatedConnection.TimeoutMS)
	s.Equal(3, updatedConnection.RetryCount)
	s.Require().Len(updatedConnection.SubjectAttributeMappings, 1)
	s.Equal("authzen-pdp-person", updatedConnection.SubjectAttributeMappings[0].EntityType)
	s.Require().Len(updatedConnection.SubjectAttributeMappings[0].Attributes, 2)

	request, err = http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/authzen-pdp",
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var summaries []authZENPDPConnectionSummary
	s.Require().NoError(json.Unmarshal(body, &summaries))
	s.True(containsAuthZENPDPSummary(summaries, s.connectionID))

	request, err = http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/resource-servers/"+s.rsID,
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var resourceServer authZENPDPResourceServerUpdate
	s.Require().NoError(json.Unmarshal(body, &resourceServer))
	s.Equal("authzen_pdp", resourceServer.AuthorizationEngine.Type)
	s.Equal(s.connectionID, resourceServer.AuthorizationEngine.Properties.PDPConnectionID)

	request, err = http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/authzen-pdp/"+s.connectionID+"/usages",
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var dependencies authZENPDPDependenciesResponse
	s.Require().NoError(json.Unmarshal(body, &dependencies))
	s.Equal(1, dependencies.TotalResults)
	s.Equal(1, dependencies.Count)
	s.Equal(1, dependencies.Summary["resourceServer"])
	s.Require().Len(dependencies.Usages, 1)
	s.Equal(s.rsID, dependencies.Usages[0].ID)
	s.Equal("resourceServer", dependencies.Usages[0].ResourceType)
	s.Equal("restrict", dependencies.Usages[0].BehaviorOnDelete)

	request, err = http.NewRequest(
		http.MethodDelete,
		testutils.TestServerURL+"/connections/authzen-pdp/"+s.connectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Equal(http.StatusConflict, response.StatusCode, string(body))
}

func (s *AuthZENPDPIntegrationSuite) TestDeclarativeAuthZENPDPConnectionUsesCompositeStore() {
	const declarativeConnectionID = "decl-authzen-pdp-1"

	request, err := http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/authzen-pdp/"+declarativeConnectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var connection authZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.Equal(declarativeConnectionID, connection.ID)
	s.Equal("Declarative File AuthZEN PDP", connection.Name)
	s.Equal(1000, connection.TimeoutMS)
	s.Equal(1, connection.RetryCount)

	request, err = http.NewRequest(http.MethodGet, testutils.TestServerURL+"/connections/authzen-pdp", nil)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var connections []authZENPDPConnectionSummary
	s.Require().NoError(json.Unmarshal(body, &connections))
	declarativeConnectionFound := false
	for _, summary := range connections {
		if summary.ID == declarativeConnectionID && summary.Name == connection.Name {
			declarativeConnectionFound = true
			break
		}
	}
	s.True(declarativeConnectionFound)

	updatePayload := mustJSON(map[string]interface{}{
		"name":          connection.Name,
		"endpoint":      connection.Endpoint,
		"batchEndpoint": connection.BatchEndpoint,
	})
	request, err = http.NewRequest(
		http.MethodPut,
		testutils.TestServerURL+"/connections/authzen-pdp/"+declarativeConnectionID,
		bytes.NewReader(updatePayload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Equal(http.StatusBadRequest, response.StatusCode, string(body))

	request, err = http.NewRequest(
		http.MethodDelete,
		testutils.TestServerURL+"/connections/authzen-pdp/"+declarativeConnectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Equal(http.StatusBadRequest, response.StatusCode, string(body))
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPConnectionValidationAndNotFound() {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{
			name:   "malformed create body",
			method: http.MethodPost,
			path:   "/connections/authzen-pdp",
			body:   "{",
			status: http.StatusBadRequest,
		},
		{
			name:   "optional batch endpoint",
			method: http.MethodPost,
			path:   "/connections/authzen-pdp",
			body: string(mustJSON(map[string]interface{}{
				"name":     "Missing Batch Endpoint PDP",
				"endpoint": s.pdpServer.URL + "/access/v1/evaluation",
			})),
			status: http.StatusCreated,
		},
		{
			name:   "unknown subject mapping attribute",
			method: http.MethodPost,
			path:   "/connections/authzen-pdp",
			body: string(mustJSON(map[string]interface{}{
				"name":          "Invalid Mapping Attribute PDP",
				"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
				"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
				"subjectAttributeMappings": []authZENPDPSubjectAttributeMapping{{
					EntityType: "authzen-pdp-person",
					Attributes: []authZENPDPSubjectAttributeRow{{Attribute: "unknown"}},
				}},
			})),
			status: http.StatusBadRequest,
		},
		{
			name:   "unknown subject mapping type",
			method: http.MethodPost,
			path:   "/connections/authzen-pdp",
			body: string(mustJSON(map[string]interface{}{
				"name":          "Invalid Mapping Type PDP",
				"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
				"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
				"subjectAttributeMappings": []authZENPDPSubjectAttributeMapping{{
					EntityType: "unknown-authzen-pdp-person",
					Attributes: []authZENPDPSubjectAttributeRow{{Attribute: "email"}},
				}},
			})),
			status: http.StatusBadRequest,
		},
		{
			name:   "invalid connection category",
			method: http.MethodGet,
			path:   "/connections?category=unsupported",
			status: http.StatusBadRequest,
		},
		{
			name:   "invalid list limit",
			method: http.MethodGet,
			path:   "/connections?category=authorization-pdp&limit=invalid",
			status: http.StatusBadRequest,
		},
		{
			name:   "unknown connection",
			method: http.MethodGet,
			path:   "/connections/authzen-pdp/unknown-connection",
			status: http.StatusNotFound,
		},
		{
			name:   "unknown connection update",
			method: http.MethodPut,
			path:   "/connections/authzen-pdp/unknown-connection",
			body: string(mustJSON(map[string]interface{}{
				"name":          "Unknown Connection",
				"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
				"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
			})),
			status: http.StatusNotFound,
		},
		{
			name:   "unknown connection usages",
			method: http.MethodGet,
			path:   "/connections/authzen-pdp/unknown-connection/usages",
			status: http.StatusNotFound,
		},
		{
			name:   "unknown connection delete",
			method: http.MethodDelete,
			path:   "/connections/authzen-pdp/unknown-connection",
			status: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			var bodyReader io.Reader
			if test.body != "" {
				bodyReader = strings.NewReader(test.body)
			}
			request, err := http.NewRequest(test.method, testutils.TestServerURL+test.path, bodyReader)
			s.Require().NoError(err)
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response, err := testutils.GetHTTPClient().Do(request)
			s.Require().NoError(err)
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			s.Require().NoError(readErr)
			s.Equal(test.status, response.StatusCode, string(body))
		})
	}
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPConnectionListSupportsCategoryAndPagination() {
	request, err := http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections?category=authorization-pdp&limit=1&offset=0",
		nil,
	)
	s.Require().NoError(err)
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var result struct {
		TotalResults int `json:"totalResults"`
		StartIndex   int `json:"startIndex"`
		Count        int `json:"count"`
		Connections  []struct {
			ID         string   `json:"id"`
			Type       string   `json:"type"`
			Categories []string `json:"categories"`
		} `json:"connections"`
	}
	s.Require().NoError(json.Unmarshal(body, &result))
	s.GreaterOrEqual(result.TotalResults, 1)
	s.Equal(1, result.StartIndex)
	s.Equal(1, result.Count)
	s.Require().Len(result.Connections, 1)
	s.Equal(s.connectionID, result.Connections[0].ID)
	s.Equal("authzen-pdp", result.Connections[0].Type)
	s.Equal([]string{"authorization-pdp"}, result.Connections[0].Categories)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPConnectionUpdateRejectsInvalidEndpoint() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Invalid Updated AuthZEN PDP",
		"endpoint":      "ftp://pdp.example.com/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
	})
	request, err := http.NewRequest(
		http.MethodPut,
		testutils.TestServerURL+"/connections/authzen-pdp/"+s.connectionID,
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Equal(http.StatusBadRequest, response.StatusCode, string(body))

	connection := s.getAuthZENPDPConnection()
	s.Equal("AuthZEN PDP", connection.Name)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPConnectionExportImportRoundTrip() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Declarative AuthZEN PDP",
		"description":   "AuthZEN PDP declarative round trip",
		"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
		"timeoutMs":     1250,
		"retryCount":    4,
	})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/connections/authzen-pdp",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusCreated, response.StatusCode, string(body))

	var connection authZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.Require().NotEmpty(connection.ID)
	defer func() {
		request, requestErr := http.NewRequest(
			http.MethodDelete,
			testutils.TestServerURL+"/connections/authzen-pdp/"+connection.ID,
			nil,
		)
		if requestErr == nil {
			response, responseErr := testutils.GetHTTPClient().Do(request)
			if responseErr == nil {
				_ = response.Body.Close()
			}
		}
	}()

	exportPayload := mustJSON(map[string]interface{}{
		"connections": []string{connection.ID},
	})
	request, err = http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/export",
		bytes.NewReader(exportPayload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var exportResponse struct {
		Resources string `json:"resources"`
	}
	s.Require().NoError(json.Unmarshal(body, &exportResponse))
	s.Contains(exportResponse.Resources, "resource_type: connection")
	s.Contains(exportResponse.Resources, "type: authzen-pdp")
	s.Contains(exportResponse.Resources, "batchEndpoint:")

	deleteRequest, err := http.NewRequest(
		http.MethodDelete,
		testutils.TestServerURL+"/connections/authzen-pdp/"+connection.ID,
		nil,
	)
	s.Require().NoError(err)
	deleteResponse, err := testutils.GetHTTPClient().Do(deleteRequest)
	s.Require().NoError(err)
	_ = deleteResponse.Body.Close()
	s.Require().Equal(http.StatusNoContent, deleteResponse.StatusCode)

	importOptions := map[string]interface{}{
		"upsert":          false,
		"continueOnError": false,
		"target":          "runtime",
	}
	importPayload := mustJSON(map[string]interface{}{
		"content": exportResponse.Resources,
		"options": importOptions,
	})
	request, err = http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/import",
		bytes.NewReader(importPayload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var importResponse struct {
		Summary struct {
			Imported int `json:"imported"`
			Failed   int `json:"failed"`
		} `json:"summary"`
	}
	s.Require().NoError(json.Unmarshal(body, &importResponse))
	s.Equal(1, importResponse.Summary.Imported)
	s.Equal(0, importResponse.Summary.Failed)

	imported := s.getAuthZENPDPConnectionByID(connection.ID)
	s.Equal(connection.BatchEndpoint, imported.BatchEndpoint)
	s.Equal(1250, imported.TimeoutMS)
	s.Equal(4, imported.RetryCount)

	importOptions["upsert"] = true
	request, err = http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/import",
		bytes.NewReader(mustJSON(map[string]interface{}{
			"content": exportResponse.Resources,
			"dryRun":  true,
			"options": importOptions,
		})),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	request, err = http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/import",
		bytes.NewReader(mustJSON(map[string]interface{}{
			"content": exportResponse.Resources,
			"dryRun":  true,
			"options": map[string]interface{}{
				"upsert":          false,
				"continueOnError": false,
				"target":          "runtime",
			},
		})),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var dryRunResponse struct {
		Summary struct {
			Imported int `json:"imported"`
			Failed   int `json:"failed"`
		} `json:"summary"`
	}
	s.Require().NoError(json.Unmarshal(body, &dryRunResponse))
	s.Equal(1, dryRunResponse.Summary.Imported)
	s.Equal(0, dryRunResponse.Summary.Failed)

	invalidImport := strings.Replace(
		exportResponse.Resources,
		"endpoint: "+connection.Endpoint,
		"endpoint: ftp://pdp.example.com/access/v1/evaluation",
		1,
	)
	s.Require().NotEqual(exportResponse.Resources, invalidImport)
	invalidImportPayload := mustJSON(map[string]interface{}{
		"content": invalidImport,
		"options": map[string]interface{}{
			"upsert":          true,
			"continueOnError": true,
			"target":          "runtime",
		},
	})
	request, err = http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/import",
		bytes.NewReader(invalidImportPayload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))
	var invalidImportResponse struct {
		Summary struct {
			Failed int `json:"failed"`
		} `json:"summary"`
		Results []struct {
			Code string `json:"code"`
		} `json:"results"`
	}
	s.Require().NoError(json.Unmarshal(body, &invalidImportResponse))
	s.Equal(1, invalidImportResponse.Summary.Failed)
	s.Require().Len(invalidImportResponse.Results, 1)
	s.Equal("CON-1006", invalidImportResponse.Results[0].Code)

	malformedImportPayload := mustJSON(map[string]interface{}{
		"content": "resource_type: connection\ntype: authzen-pdp\nname:\n  - invalid\n",
		"options": map[string]interface{}{
			"upsert":          false,
			"continueOnError": true,
			"target":          "runtime",
		},
	})
	request, err = http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/import",
		bytes.NewReader(malformedImportPayload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, readErr = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(readErr)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))
	s.Require().NoError(json.Unmarshal(body, &invalidImportResponse))
	s.Equal(1, invalidImportResponse.Summary.Failed)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPUnusedConnectionCanBeDeleted() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Unused AuthZEN PDP",
		"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
	})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/connections/authzen-pdp",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, response.StatusCode, string(body))

	var connection authZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.NotEmpty(connection.ID)

	request, err = http.NewRequest(
		http.MethodDelete,
		testutils.TestServerURL+"/connections/authzen-pdp/"+connection.ID,
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusNoContent, response.StatusCode, string(body))

	request, err = http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/authzen-pdp/"+connection.ID,
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	_ = response.Body.Close()
	s.Equal(http.StatusNotFound, response.StatusCode)
}

func (s *AuthZENPDPIntegrationSuite) TestResourceServerCanSwitchToRBAC() {
	update := func(engineType, connectionID string) {
		payload := mustJSON(authZENPDPResourceServerUpdate{
			Name: "AuthZEN API", Identifier: authZENPDPResourceIdentifier, OUID: s.ouID,
			AuthorizationEngine: authZENPDPAuthorizationEngineConfig{
				Type:       engineType,
				Properties: authZENPDPAuthorizationEngineProps{PDPConnectionID: connectionID},
			},
		})
		request, err := http.NewRequest(http.MethodPut,
			testutils.TestServerURL+"/resource-servers/"+s.rsID, bytes.NewReader(payload))
		s.Require().NoError(err)
		request.Header.Set("Content-Type", "application/json")
		response, err := testutils.GetHTTPClient().Do(request)
		s.Require().NoError(err)
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		s.Require().NoError(err)
		s.Require().Equal(http.StatusOK, response.StatusCode, string(body))
		var result authZENPDPResourceServerUpdate
		s.Require().NoError(json.Unmarshal(body, &result))
		s.Equal(engineType, result.AuthorizationEngine.Type)
		if engineType == "rbac" {
			s.Empty(result.AuthorizationEngine.Properties.PDPConnectionID)
		}
	}
	defer update("authzen_pdp", s.connectionID)
	update("rbac", s.connectionID)
	request, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/resource-servers/"+s.rsID, nil)
	s.Require().NoError(err)
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	defer response.Body.Close()
	var result authZENPDPResourceServerUpdate
	s.Require().Equal(http.StatusOK, response.StatusCode)
	s.Require().NoError(json.NewDecoder(response.Body).Decode(&result))
	s.Equal("rbac", result.AuthorizationEngine.Type)
	s.Empty(result.AuthorizationEngine.Properties.PDPConnectionID)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPUnavailableFailsClosed() {
	original := s.getAuthZENPDPConnection()
	unavailable := original
	unavailable.Endpoint = s.pdpServer.URL + "/unavailable"
	unavailable.BatchEndpoint = s.pdpServer.URL + "/unavailable"
	s.updateAuthZENPDPConnection(unavailable)
	defer s.updateAuthZENPDPConnection(original)

	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{{
		Subject:  subject{Type: "user", ID: s.userID},
		Resource: resource{Type: authZENPDPResourceIdentifier, ID: "booking-unavailable"},
		Action:   action{Name: "read"},
	}}})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/access/v1/evaluations",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusInternalServerError, response.StatusCode, string(body))
}

func (s *AuthZENPDPIntegrationSuite) TestResourceServerWithoutAuthZENPDPUsesDefaultEngine() {
	resourceServer, err := createResourceServer(testutils.ResourceServer{
		Name:       "Default Authorization Engine API",
		Identifier: "https://default-authz-api.example.com",
		OUID:       s.ouID,
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteResourceServer(resourceServer.ID) }()

	createdAction, err := createAction(resourceServer.ID, "", testutils.Action{
		Name:   "Read default-engine resource",
		Handle: "read",
	})
	s.Require().NoError(err)
	defer deleteAuthZENPDPAction(resourceServer.ID, createdAction.ID)

	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Default Authorization Engine Test Role",
		Description: "Permission evaluated by the default authorization engine",
		OUID:        s.ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: resourceServer.ID,
			Permissions:      []string{"read"},
		}},
		Assignments: []testutils.Assignment{{ID: s.userID, Type: "user"}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(roleID) }()

	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{{
		Subject:  subject{Type: "user", ID: s.userID},
		Resource: resource{Type: resourceServer.Identifier, ID: "default-resource"},
		Action:   action{Name: "read"},
	}}})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/access/v1/evaluations",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var result evaluationsResponse
	s.Require().NoError(json.Unmarshal(body, &result))
	s.Require().Len(result.Evaluations, 1)
	s.True(result.Evaluations[0].Decision)
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPConnectionRejectsInvalidEndpoint() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Invalid AuthZEN PDP",
		"endpoint":      "not-an-absolute-url",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
	})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/connections/authzen-pdp",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, response.StatusCode, string(body))
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPConnectionRejectsUnsupportedEndpointScheme() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Unsupported Scheme AuthZEN PDP",
		"endpoint":      "ftp://pdp.example.com/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
	})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/connections/authzen-pdp",
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, response.StatusCode, string(body))
}

func (s *AuthZENPDPIntegrationSuite) TestAuthZENPDPDecisionAllowsClientCredentialsToken() {
	status, body, tokenResponse := s.requestClientCredentialsToken("read")
	s.Require().Equal(http.StatusOK, status, string(body))
	s.NotEmpty(tokenResponse.AccessToken)
	s.Equal("read", tokenResponse.Scope)

	claims, err := testutils.DecodeJWT(tokenResponse.AccessToken)
	s.Require().NoError(err)
	s.Equal(authZENPDPResourceIdentifier, claims.Aud)
}

func (s *AuthZENPDPIntegrationSuite) requestClientCredentialsToken(
	scope string,
) (int, []byte, testutils.TokenResponse) {
	form := "grant_type=client_credentials&resource=" + url.QueryEscape(authZENPDPResourceIdentifier) +
		"&scope=" + url.QueryEscape(scope)
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(form),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth("authzen_token_test_client", "authzen_token_test_secret")

	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)

	var tokenResponse testutils.TokenResponse
	if response.StatusCode == http.StatusOK {
		s.Require().NoError(json.Unmarshal(body, &tokenResponse))
	}
	return response.StatusCode, body, tokenResponse
}

func (s *AuthZENPDPIntegrationSuite) getAuthZENPDPConnection() authZENPDPConnectionResponse {
	return s.getAuthZENPDPConnectionByID(s.connectionID)
}

func (s *AuthZENPDPIntegrationSuite) getAuthZENPDPConnectionByID(
	connectionID string,
) authZENPDPConnectionResponse {
	request, err := http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/authzen-pdp/"+connectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var connection authZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	return connection
}

func (s *AuthZENPDPIntegrationSuite) updateAuthZENPDPConnection(
	connection authZENPDPConnectionResponse,
) {
	payload := mustJSON(map[string]interface{}{
		"name":                     connection.Name,
		"description":              connection.Description,
		"endpoint":                 connection.Endpoint,
		"batchEndpoint":            connection.BatchEndpoint,
		"timeoutMs":                connection.TimeoutMS,
		"retryCount":               connection.RetryCount,
		"subjectAttributeMappings": connection.SubjectAttributeMappings,
	})
	request, err := http.NewRequest(
		http.MethodPut,
		testutils.TestServerURL+"/connections/authzen-pdp/"+s.connectionID,
		bytes.NewReader(payload),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))
}

func deleteAuthZENPDPAction(resourceServerID, actionID string) {
	request, err := http.NewRequest(
		http.MethodDelete,
		testutils.TestServerURL+"/resource-servers/"+resourceServerID+"/actions/"+actionID,
		nil,
	)
	if err != nil {
		return
	}
	response, err := testutils.GetHTTPClient().Do(request)
	if err == nil {
		_ = response.Body.Close()
	}
}

func containsAuthZENPDPSummary(
	summaries []authZENPDPConnectionSummary,
	connectionID string,
) bool {
	for _, summary := range summaries {
		if summary.ID == connectionID && summary.Name == "AuthZEN PDP" {
			return true
		}
	}
	return false
}
