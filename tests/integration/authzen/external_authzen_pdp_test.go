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

type externalAuthZENPDPConnectionResponse struct {
	ID                       string                                      `json:"id"`
	Name                     string                                      `json:"name"`
	Description              string                                      `json:"description,omitempty"`
	Type                     string                                      `json:"type"`
	Endpoint                 string                                      `json:"endpoint"`
	BatchEndpoint            string                                      `json:"batchEndpoint,omitempty"`
	TimeoutMS                int                                         `json:"timeoutMs"`
	RetryCount               int                                         `json:"retryCount"`
	SubjectProperties        string                                      `json:"subjectProperties,omitempty"`
	SubjectPropertyMappings  string                                      `json:"subjectPropertyMappings,omitempty"`
	SubjectAttributeMappings []externalAuthZENPDPSubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
	FailOpen                 bool                                        `json:"failOpen,omitempty"`
}

type externalAuthZENPDPSubjectAttributeMapping struct {
	UserType   string                                  `json:"userType"`
	Attributes []externalAuthZENPDPSubjectAttributeRow `json:"attributes"`
}

type externalAuthZENPDPSubjectAttributeRow struct {
	Attribute    string `json:"attribute"`
	PDPAttribute string `json:"pdpAttribute,omitempty"`
}

type externalAuthZENPDPConnectionSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type externalAuthZENPDPDependenciesResponse struct {
	TotalResults int                               `json:"totalResults"`
	Count        int                               `json:"count"`
	Summary      map[string]int                    `json:"summary"`
	Usages       []externalAuthZENPDPResourceUsage `json:"usages"`
}

type externalAuthZENPDPResourceUsage struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

const externalAuthZENPDPResourceIdentifier = "https://authzen-external-api.example.com"

type externalAuthZENPDPResourceServerUpdate struct {
	Name                string                                      `json:"name"`
	Description         string                                      `json:"description,omitempty"`
	Identifier          string                                      `json:"identifier,omitempty"`
	OUID                string                                      `json:"ouId"`
	AuthorizationEngine externalAuthZENPDPAuthorizationEngineConfig `json:"authorizationEngine,omitempty"`
}

type externalAuthZENPDPAuthorizationEngineConfig struct {
	Type       string                                     `json:"type,omitempty"`
	Properties externalAuthZENPDPAuthorizationEngineProps `json:"properties,omitempty"`
}

type externalAuthZENPDPAuthorizationEngineProps struct {
	ExternalPDPConnectionID string `json:"externalPDPConnectionId,omitempty"`
}

type externalAuthZENPDPBatchRequest struct {
	Evaluations []struct {
		Subject  map[string]interface{} `json:"subject"`
		Resource map[string]interface{} `json:"resource"`
		Action   map[string]interface{} `json:"action"`
		Context  map[string]interface{} `json:"context,omitempty"`
	} `json:"evaluations"`
}

type externalAuthZENPDPBatchResponse struct {
	Evaluations []evaluationResponse `json:"evaluations"`
}

func TestExternalAuthZENPDPIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ExternalAuthZENPDPIntegrationSuite))
}

type ExternalAuthZENPDPIntegrationSuite struct {
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

func (s *ExternalAuthZENPDPIntegrationSuite) SetupSuite() {
	s.pdpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/access/v1/evaluations":
			var request externalAuthZENPDPBatchRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			response := externalAuthZENPDPBatchResponse{
				Evaluations: make([]evaluationResponse, 0, len(request.Evaluations)),
			}
			for _, evaluation := range request.Evaluations {
				s.Contains(
					[]string{s.rsID, externalAuthZENPDPResourceIdentifier},
					evaluation.Resource["type"],
				)
				actionName, _ := evaluation.Action["name"].(string)
				decision := actionName == "read" || actionName == "write"
				if actionName == "attribute-check" {
					subjectProperties, _ := evaluation.Subject["properties"].(map[string]interface{})
					resourceProperties, _ := evaluation.Resource["properties"].(map[string]interface{})
					actionProperties, _ := evaluation.Action["properties"].(map[string]interface{})
					decision = subjectProperties["preferred_username"] == "external-authzen-user" &&
						resourceProperties["classification"] == "confidential" &&
						actionProperties["risk"] == "low" && evaluation.Context["tenant"] == "acme"
				}
				response.Evaluations = append(response.Evaluations, evaluationResponse{
					Decision: decision,
					Context:  map[string]interface{}{"source": "external-pdp"},
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
		Handle: "external-authzen-pdp-test-ou",
		Name:   "External AuthZEN PDP Test OU",
	})
	s.Require().NoError(err)
	s.ouID = ouID
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: "external-authzen-person",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err)
	s.userTypeID = userTypeID
	userID, err := testutils.CreateUser(testutils.User{
		Type:       "external-authzen-person",
		OUID:       s.ouID,
		Attributes: json.RawMessage(`{"username":"external-authzen-user"}`),
	})
	s.Require().NoError(err)
	s.userID = userID

	resourceServer, err := createResourceServer(testutils.ResourceServer{
		Name:       "External AuthZEN API",
		Identifier: externalAuthZENPDPResourceIdentifier,
		OUID:       s.ouID,
	})
	s.Require().NoError(err)
	s.Require().Equal(externalAuthZENPDPResourceIdentifier, resourceServer.Identifier)
	s.rsID = resourceServer.ID
	for _, actionConfig := range []testutils.Action{
		{Name: "Read external bookings", Handle: "read"},
		{Name: "Write external bookings", Handle: "write"},
		{Name: "Delete external bookings", Handle: "delete"},
		{Name: "Evaluate booking attributes", Handle: "attribute-check"},
	} {
		createdAction, actionErr := createAction(s.rsID, "", actionConfig)
		s.Require().NoError(actionErr)
		s.actionIDs = append(s.actionIDs, createdAction.ID)
	}

	connectionBody := map[string]interface{}{
		"name":                    "External AuthZEN PDP",
		"description":             "External PDP used by the AuthZEN integration suite",
		"endpoint":                s.pdpServer.URL + "/access/v1/evaluation",
		"batchEndpoint":           s.pdpServer.URL + "/access/v1/evaluations",
		"timeoutMs":               750,
		"retryCount":              2,
		"subjectProperties":       "username",
		"subjectPropertyMappings": "username: preferred_username",
		"subjectAttributeMappings": []externalAuthZENPDPSubjectAttributeMapping{{
			UserType: "user",
			Attributes: []externalAuthZENPDPSubjectAttributeRow{{
				Attribute:    "username",
				PDPAttribute: "preferred_username",
			}},
		}},
		"failOpen": true,
	}
	connectionPayload, err := json.Marshal(connectionBody)
	s.Require().NoError(err)
	request, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/connections/external-authzen-pdp", bytes.NewReader(connectionPayload))
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, response.StatusCode, string(body))
	var connection externalAuthZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.connectionID = connection.ID

	updateBody, err := json.Marshal(externalAuthZENPDPResourceServerUpdate{
		Name:       "External AuthZEN API",
		Identifier: externalAuthZENPDPResourceIdentifier,
		OUID:       s.ouID,
		AuthorizationEngine: externalAuthZENPDPAuthorizationEngineConfig{
			Type: "external_authzen_pdp",
			Properties: externalAuthZENPDPAuthorizationEngineProps{
				ExternalPDPConnectionID: s.connectionID,
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
		Name:         "External AuthZEN Token Test App",
		Description:  "M2M application for external AuthZEN token issuance testing",
		OUID:         s.ouID,
		Type:         "m2m",
		ClientID:     "external_authzen_token_test_client",
		ClientSecret: "external_authzen_token_test_secret",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "external_authzen_token_test_client",
				"clientSecret":            "external_authzen_token_test_secret",
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)

	s.roleID, err = testutils.CreateRole(testutils.Role{
		Name:        "External AuthZEN Token Test Role",
		Description: "Permission for external AuthZEN token issuance testing",
		OUID:        s.ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: s.rsID,
			Permissions:      []string{"read", "write", "delete", "attribute-check"},
		}},
		Assignments: []testutils.Assignment{{ID: s.appID, Type: "app"}},
	})
	s.Require().NoError(err)
}

func (s *ExternalAuthZENPDPIntegrationSuite) TearDownSuite() {
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
		request, err := http.NewRequest(http.MethodDelete, testutils.TestServerURL+"/connections/external-authzen-pdp/"+s.connectionID, nil)
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPConnectionEvaluatesAccess() {
	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{{
		Subject:  subject{Type: "user", ID: s.userID},
		Resource: resource{Type: externalAuthZENPDPResourceIdentifier, ID: "booking-1"},
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPBatchContinuesForUnknownUser() {
	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{
		{
			Subject:  subject{Type: "user", ID: "unknown-external-authzen-user"},
			Resource: resource{Type: externalAuthZENPDPResourceIdentifier, ID: "booking-unknown-user"},
			Action:   action{Name: "read"},
		},
		{
			Subject:  subject{Type: "user", ID: s.userID},
			Resource: resource{Type: externalAuthZENPDPResourceIdentifier, ID: "booking-known-user"},
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPAllDeniedIssuesTokenWithoutPermissions() {
	status, body, tokenResponse := s.requestClientCredentialsToken("delete")
	s.Require().Equal(http.StatusOK, status, string(body))
	s.NotEmpty(tokenResponse.AccessToken)
	s.Empty(tokenResponse.Scope)
}

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPFiltersMixedTokenPermissions() {
	status, body, tokenResponse := s.requestClientCredentialsToken("read write delete")
	s.Require().Equal(http.StatusOK, status, string(body))
	s.NotEmpty(tokenResponse.AccessToken)
	s.ElementsMatch([]string{"read", "write"}, strings.Fields(tokenResponse.Scope))

	claims, err := testutils.DecodeJWT(tokenResponse.AccessToken)
	s.Require().NoError(err)
	s.Equal(externalAuthZENPDPResourceIdentifier, claims.Aud)
}

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPForwardsEvaluationAttributesAndContext() {
	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{{
		Subject: subject{Type: "user", ID: s.userID},
		Resource: resource{
			Type:       externalAuthZENPDPResourceIdentifier,
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
	s.Equal("external-pdp", result.Evaluations[0].Context["source"])
}

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPConnectionSettingsAndUsagePersist() {
	request, err := http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+s.connectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var connection externalAuthZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.Equal(s.connectionID, connection.ID)
	s.Equal("External AuthZEN PDP", connection.Name)
	s.Equal("External PDP used by the AuthZEN integration suite", connection.Description)
	s.Equal("external-authzen-pdp", connection.Type)
	s.Equal(s.pdpServer.URL+"/access/v1/evaluation", connection.Endpoint)
	s.Equal(s.pdpServer.URL+"/access/v1/evaluations", connection.BatchEndpoint)
	s.Equal(750, connection.TimeoutMS)
	s.Equal(2, connection.RetryCount)
	s.Equal("username", connection.SubjectProperties)
	s.Equal("username: preferred_username", connection.SubjectPropertyMappings)
	s.Require().Len(connection.SubjectAttributeMappings, 1)
	s.Equal("user", connection.SubjectAttributeMappings[0].UserType)
	s.Require().Len(connection.SubjectAttributeMappings[0].Attributes, 1)
	s.Equal("preferred_username", connection.SubjectAttributeMappings[0].Attributes[0].PDPAttribute)
	s.True(connection.FailOpen)

	updatedPayload := mustJSON(map[string]interface{}{
		"name":                    "External AuthZEN PDP",
		"description":             "Updated external PDP connection",
		"endpoint":                s.pdpServer.URL + "/access/v1/evaluation",
		"batchEndpoint":           s.pdpServer.URL + "/access/v1/evaluations",
		"timeoutMs":               900,
		"retryCount":              3,
		"subjectProperties":       "username ouId",
		"subjectPropertyMappings": "username: preferred_username",
		"subjectAttributeMappings": []externalAuthZENPDPSubjectAttributeMapping{{
			UserType: "user",
			Attributes: []externalAuthZENPDPSubjectAttributeRow{{
				Attribute:    "username",
				PDPAttribute: "preferred_username",
			}},
		}},
		"failOpen": false,
	})
	request, err = http.NewRequest(
		http.MethodPut,
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+s.connectionID,
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
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+s.connectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))
	var updatedConnection externalAuthZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &updatedConnection))
	s.Equal("Updated external PDP connection", updatedConnection.Description)
	s.Equal(900, updatedConnection.TimeoutMS)
	s.Equal(3, updatedConnection.RetryCount)
	s.Equal("username ouId", updatedConnection.SubjectProperties)
	s.False(updatedConnection.FailOpen)

	request, err = http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/external-authzen-pdp",
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var summaries []externalAuthZENPDPConnectionSummary
	s.Require().NoError(json.Unmarshal(body, &summaries))
	s.True(containsExternalAuthZENPDPSummary(summaries, s.connectionID))

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

	var resourceServer externalAuthZENPDPResourceServerUpdate
	s.Require().NoError(json.Unmarshal(body, &resourceServer))
	s.Equal("external_authzen_pdp", resourceServer.AuthorizationEngine.Type)
	s.Equal(s.connectionID, resourceServer.AuthorizationEngine.Properties.ExternalPDPConnectionID)

	request, err = http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+s.connectionID+"/usages",
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var dependencies externalAuthZENPDPDependenciesResponse
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
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+s.connectionID,
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPConnectionValidationAndNotFound() {
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
			path:   "/connections/external-authzen-pdp",
			body:   "{",
			status: http.StatusBadRequest,
		},
		{
			name:   "missing batch endpoint",
			method: http.MethodPost,
			path:   "/connections/external-authzen-pdp",
			body: string(mustJSON(map[string]interface{}{
				"name":     "Missing Batch Endpoint PDP",
				"endpoint": s.pdpServer.URL + "/access/v1/evaluation",
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
			path:   "/connections/external-authzen-pdp/unknown-connection",
			status: http.StatusNotFound,
		},
		{
			name:   "unknown connection update",
			method: http.MethodPut,
			path:   "/connections/external-authzen-pdp/unknown-connection",
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
			path:   "/connections/external-authzen-pdp/unknown-connection/usages",
			status: http.StatusNotFound,
		},
		{
			name:   "unknown connection delete",
			method: http.MethodDelete,
			path:   "/connections/external-authzen-pdp/unknown-connection",
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPConnectionListSupportsCategoryAndPagination() {
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
	s.Equal("external-authzen-pdp", result.Connections[0].Type)
	s.Equal([]string{"authorization-pdp"}, result.Connections[0].Categories)
}

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPConnectionUpdateRejectsInvalidEndpoint() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Invalid Updated External PDP",
		"endpoint":      "ftp://pdp.example.com/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
	})
	request, err := http.NewRequest(
		http.MethodPut,
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+s.connectionID,
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

	connection := s.getExternalPDPConnection()
	s.Equal("External AuthZEN PDP", connection.Name)
}

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPConnectionExportImportRoundTrip() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Declarative External AuthZEN PDP",
		"description":   "External PDP declarative round trip",
		"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
		"timeoutMs":     1250,
		"retryCount":    4,
		"failOpen":      true,
	})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/connections/external-authzen-pdp",
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

	var connection externalAuthZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.Require().NotEmpty(connection.ID)
	defer func() {
		request, requestErr := http.NewRequest(
			http.MethodDelete,
			testutils.TestServerURL+"/connections/external-authzen-pdp/"+connection.ID,
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
	s.Contains(exportResponse.Resources, "type: external-authzen-pdp")
	s.Contains(exportResponse.Resources, "batchEndpoint:")
	s.Contains(exportResponse.Resources, "failOpen: true")

	deleteRequest, err := http.NewRequest(
		http.MethodDelete,
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+connection.ID,
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

	imported := s.getExternalPDPConnectionByID(connection.ID)
	s.Equal(connection.BatchEndpoint, imported.BatchEndpoint)
	s.Equal(1250, imported.TimeoutMS)
	s.Equal(4, imported.RetryCount)
	s.True(imported.FailOpen)

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
	invalidImportPayload := mustJSON(map[string]interface{}{
		"content": invalidImport,
		"options": map[string]interface{}{
			"upsert":          false,
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
	}
	s.Require().NoError(json.Unmarshal(body, &invalidImportResponse))
	s.Equal(1, invalidImportResponse.Summary.Failed)

	malformedImportPayload := mustJSON(map[string]interface{}{
		"content": "resource_type: connection\ntype: external-authzen-pdp\nname:\n  - invalid\n",
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPUnusedConnectionCanBeDeleted() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Unused External AuthZEN PDP",
		"endpoint":      s.pdpServer.URL + "/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
	})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/connections/external-authzen-pdp",
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

	var connection externalAuthZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	s.NotEmpty(connection.ID)

	request, err = http.NewRequest(
		http.MethodDelete,
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+connection.ID,
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
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+connection.ID,
		nil,
	)
	s.Require().NoError(err)
	response, err = testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	_ = response.Body.Close()
	s.Equal(http.StatusNotFound, response.StatusCode)
}

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPUnavailableFailsClosed() {
	original := s.getExternalPDPConnection()
	unavailable := original
	unavailable.Endpoint = s.pdpServer.URL + "/unavailable"
	unavailable.BatchEndpoint = s.pdpServer.URL + "/unavailable"
	unavailable.FailOpen = false
	s.updateExternalPDPConnection(unavailable)
	defer s.updateExternalPDPConnection(original)

	payload := mustJSON(evaluationsRequest{Evaluations: []evaluationRequest{{
		Subject:  subject{Type: "user", ID: s.userID},
		Resource: resource{Type: externalAuthZENPDPResourceIdentifier, ID: "booking-unavailable"},
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestResourceServerWithoutExternalPDPUsesDefaultEngine() {
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
	defer deleteExternalAuthZENPDPAction(resourceServer.ID, createdAction.ID)

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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPConnectionRejectsInvalidEndpoint() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Invalid External AuthZEN PDP",
		"endpoint":      "not-an-absolute-url",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
	})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/connections/external-authzen-pdp",
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPConnectionRejectsUnsupportedEndpointScheme() {
	payload := mustJSON(map[string]interface{}{
		"name":          "Unsupported Scheme External AuthZEN PDP",
		"endpoint":      "ftp://pdp.example.com/access/v1/evaluation",
		"batchEndpoint": s.pdpServer.URL + "/access/v1/evaluations",
	})
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/connections/external-authzen-pdp",
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

func (s *ExternalAuthZENPDPIntegrationSuite) TestExternalPDPDecisionAllowsClientCredentialsToken() {
	status, body, tokenResponse := s.requestClientCredentialsToken("read")
	s.Require().Equal(http.StatusOK, status, string(body))
	s.NotEmpty(tokenResponse.AccessToken)
	s.Equal("read", tokenResponse.Scope)

	claims, err := testutils.DecodeJWT(tokenResponse.AccessToken)
	s.Require().NoError(err)
	s.Equal(externalAuthZENPDPResourceIdentifier, claims.Aud)
}

func (s *ExternalAuthZENPDPIntegrationSuite) requestClientCredentialsToken(
	scope string,
) (int, []byte, testutils.TokenResponse) {
	form := "grant_type=client_credentials&resource=" + url.QueryEscape(externalAuthZENPDPResourceIdentifier) +
		"&scope=" + url.QueryEscape(scope)
	request, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(form),
	)
	s.Require().NoError(err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth("external_authzen_token_test_client", "external_authzen_token_test_secret")

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

func (s *ExternalAuthZENPDPIntegrationSuite) getExternalPDPConnection() externalAuthZENPDPConnectionResponse {
	return s.getExternalPDPConnectionByID(s.connectionID)
}

func (s *ExternalAuthZENPDPIntegrationSuite) getExternalPDPConnectionByID(
	connectionID string,
) externalAuthZENPDPConnectionResponse {
	request, err := http.NewRequest(
		http.MethodGet,
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+connectionID,
		nil,
	)
	s.Require().NoError(err)
	response, err := testutils.GetHTTPClient().Do(request)
	s.Require().NoError(err)
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, response.StatusCode, string(body))

	var connection externalAuthZENPDPConnectionResponse
	s.Require().NoError(json.Unmarshal(body, &connection))
	return connection
}

func (s *ExternalAuthZENPDPIntegrationSuite) updateExternalPDPConnection(
	connection externalAuthZENPDPConnectionResponse,
) {
	payload := mustJSON(map[string]interface{}{
		"name":                     connection.Name,
		"description":              connection.Description,
		"endpoint":                 connection.Endpoint,
		"batchEndpoint":            connection.BatchEndpoint,
		"timeoutMs":                connection.TimeoutMS,
		"retryCount":               connection.RetryCount,
		"subjectProperties":        connection.SubjectProperties,
		"subjectPropertyMappings":  connection.SubjectPropertyMappings,
		"subjectAttributeMappings": connection.SubjectAttributeMappings,
		"failOpen":                 connection.FailOpen,
	})
	request, err := http.NewRequest(
		http.MethodPut,
		testutils.TestServerURL+"/connections/external-authzen-pdp/"+s.connectionID,
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

func deleteExternalAuthZENPDPAction(resourceServerID, actionID string) {
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

func containsExternalAuthZENPDPSummary(
	summaries []externalAuthZENPDPConnectionSummary,
	connectionID string,
) bool {
	for _, summary := range summaries {
		if summary.ID == connectionID && summary.Name == "External AuthZEN PDP" {
			return true
		}
	}
	return false
}
