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

package authzen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	authzenServerURL = testutils.TestServerURL

	authzenOUHandle                = "authzen-test-ou"
	authzenUserTypeName            = "authzen-person"
	authzenResourceIdentifier      = "authzen-booking-api"
	authzenOtherResourceIdentifier = "authzen-invoice-api"
)

type AuthZENAPITestSuite struct {
	suite.Suite

	ouID             string
	userTypeID       string
	userID           string
	deniedUserID     string
	resourceServerID string
	otherServerID    string
	resourceID       string
	roleID           string

	readPermission    string
	writePermission   string
	approvePermission string
	otherPermission   string
}

func TestAuthZENAPITestSuite(t *testing.T) {
	suite.Run(t, new(AuthZENAPITestSuite))
}

func (ts *AuthZENAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      authzenOUHandle,
		Name:        "AuthZEN Test OU",
		Description: "Organization unit for AuthZEN integration tests",
	})
	ts.Require().NoError(err, "create AuthZEN test OU")
	ts.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: authzenUserTypeName,
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create AuthZEN user type")
	ts.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		Type:       authzenUserTypeName,
		OUID:       ts.ouID,
		Attributes: json.RawMessage(`{"username": "authzen-user"}`),
	})
	ts.Require().NoError(err, "create AuthZEN subject user")
	ts.userID = userID

	deniedUserID, err := testutils.CreateUser(testutils.User{
		Type:       authzenUserTypeName,
		OUID:       ts.ouID,
		Attributes: json.RawMessage(`{"username": "authzen-denied-user"}`),
	})
	ts.Require().NoError(err, "create AuthZEN denied subject user")
	ts.deniedUserID = deniedUserID

	resourceServer, err := createResourceServer(testutils.ResourceServer{
		Name:        "AuthZEN Booking API",
		Description: "Resource server for AuthZEN integration tests",
		Identifier:  authzenResourceIdentifier,
		OUID:        ts.ouID,
	})
	ts.Require().NoError(err, "create AuthZEN resource server")
	ts.resourceServerID = resourceServer.ID

	otherServer, err := createResourceServer(testutils.ResourceServer{
		Name:        "AuthZEN Invoice API",
		Description: "Second resource server for AuthZEN integration tests",
		Identifier:  authzenOtherResourceIdentifier,
		OUID:        ts.ouID,
	})
	ts.Require().NoError(err, "create second AuthZEN resource server")
	ts.otherServerID = otherServer.ID

	readAction, err := createAction(
		ts.resourceServerID,
		"",
		testutils.Action{Name: "Read bookings", Handle: "read", Description: "Read bookings"},
	)
	ts.Require().NoError(err, "create read action")
	ts.readPermission = readAction.Permission

	writeAction, err := createAction(
		ts.resourceServerID,
		"",
		testutils.Action{Name: "Write bookings", Handle: "write", Description: "Write bookings"},
	)
	ts.Require().NoError(err, "create write action")
	ts.writePermission = writeAction.Permission

	otherAction, err := createAction(
		ts.otherServerID,
		"",
		testutils.Action{Name: "Read invoices", Handle: "invoice-read", Description: "Read invoices"},
	)
	ts.Require().NoError(err, "create invoice read action")
	ts.otherPermission = otherAction.Permission

	resource, err := createResource(ts.resourceServerID, createResourceRequest{
		Name:        "Booking",
		Handle:      "booking",
		Description: "Booking resource",
	})
	ts.Require().NoError(err, "create AuthZEN resource")
	ts.resourceID = resource.ID

	approveAction, err := createAction(
		ts.resourceServerID,
		ts.resourceID,
		testutils.Action{Name: "Approve booking", Handle: "approve", Description: "Approve booking"},
	)
	ts.Require().NoError(err, "create approve action")
	ts.approvePermission = approveAction.Permission

	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "AuthZEN Booking Reader",
		Description: "Role granting selected AuthZEN permissions",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: ts.resourceServerID,
				Permissions:      []string{ts.readPermission, ts.approvePermission},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.userID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "create AuthZEN test role")
	ts.roleID = roleID
}

func (ts *AuthZENAPITestSuite) TearDownSuite() {
	if ts.roleID != "" {
		_ = testutils.DeleteRole(ts.roleID)
	}
	if ts.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerID)
	}
	if ts.otherServerID != "" {
		_ = testutils.DeleteResourceServer(ts.otherServerID)
	}
	if ts.deniedUserID != "" {
		_ = testutils.DeleteUser(ts.deniedUserID)
	}
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.userTypeID != "" {
		_ = testutils.DeleteUserType(ts.userTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

func (ts *AuthZENAPITestSuite) TestMetadataDiscovery() {
	resp, body := ts.doRequest(http.MethodGet, "/.well-known/authzen-configuration", nil)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var metadata metadataResponse
	ts.Require().NoError(json.Unmarshal(body, &metadata))
	ts.Equal(authzenServerURL, metadata.PolicyDecisionPoint)
	ts.Equal(authzenServerURL+"/access/v1/evaluation", metadata.AccessEvaluationEndpoint)
	ts.Equal(authzenServerURL+"/access/v1/evaluations", metadata.AccessEvaluationsEndpoint)
	ts.Equal(authzenServerURL+"/access/v1/search/action", metadata.SearchActionEndpoint)
}

func (ts *AuthZENAPITestSuite) TestMetadataDiscoveryDoesNotRequireDirectAuthSecret() {
	resp, body := ts.doUnauthenticatedRequest(http.MethodGet, "/.well-known/authzen-configuration", nil, nil)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var metadata metadataResponse
	ts.Require().NoError(json.Unmarshal(body, &metadata))
	ts.Equal(authzenServerURL, metadata.PolicyDecisionPoint)
}

func (ts *AuthZENAPITestSuite) TestAccessEndpointsRequireDirectAuthSecret() {
	accessEndpoints := []struct {
		name string
		path string
		body []byte
	}{
		{
			name: "single evaluation",
			path: "/access/v1/evaluation",
			body: mustJSON(evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
				Action:   action{Name: ts.readPermission},
			}),
		},
		{
			name: "batch evaluations",
			path: "/access/v1/evaluations",
			body: mustJSON(evaluationsRequest{
				Evaluations: []evaluationRequest{
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
						Action:   action{Name: ts.readPermission},
					},
				},
			}),
		},
		{
			name: "action search",
			path: "/access/v1/search/action",
			body: mustJSON(searchActionRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
			}),
		},
	}

	for _, endpoint := range accessEndpoints {
		ts.Run(endpoint.name+" missing secret", func() {
			resp, body := ts.doUnauthenticatedRequest(http.MethodPost, endpoint.path, endpoint.body, nil)
			ts.Require().Equal(http.StatusUnauthorized, resp.StatusCode, string(body))

			var errResp testutils.ErrorResponse
			ts.Require().NoError(json.Unmarshal(body, &errResp))
			ts.Equal("AUTH-4010", errResp.Code)
		})

		ts.Run(endpoint.name+" wrong secret", func() {
			resp, body := ts.doUnauthenticatedRequest(http.MethodPost, endpoint.path, endpoint.body, map[string]string{
				testutils.DirectAuthHeaderName: "wrong-secret",
			})
			ts.Require().Equal(http.StatusUnauthorized, resp.StatusCode, string(body))

			var errResp testutils.ErrorResponse
			ts.Require().NoError(json.Unmarshal(body, &errResp))
			ts.Equal("AUTH-4010", errResp.Code)
		})

		ts.Run(endpoint.name+" valid secret", func() {
			resp, body := ts.doUnauthenticatedRequest(http.MethodPost, endpoint.path, endpoint.body, map[string]string{
				testutils.DirectAuthHeaderName: testutils.DirectAuthHeaderValue,
			})
			ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
		})
	}
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessAllowedAndDenied() {
	allowed := ts.evaluate(evaluationRequest{
		Subject: subject{Type: "user", ID: ts.userID},
		Resource: resource{
			Type: authzenResourceIdentifier,
			ID:   ts.resourceID,
		},
		Action: action{Name: ts.readPermission},
	})
	ts.True(allowed.Decision)
	ts.Empty(allowed.Context)

	denied := ts.evaluate(evaluationRequest{
		Subject: subject{Type: "user", ID: ts.userID},
		Resource: resource{
			Type: authzenResourceIdentifier,
			ID:   ts.resourceID,
		},
		Action: action{Name: ts.writePermission},
	})
	ts.False(denied.Decision)
	ts.Equal("Subject is not authorized to perform the requested action", denied.Context["reason"])
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessDeniedUser() {
	denied := ts.evaluate(evaluationRequest{
		Subject:  subject{Type: "user", ID: ts.deniedUserID},
		Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
		Action:   action{Name: ts.readPermission},
	})
	ts.False(denied.Decision)
	ts.Equal("Subject is not authorized to perform the requested action", denied.Context["reason"])

	unknownAction := ts.evaluate(evaluationRequest{
		Subject:  subject{Type: "user", ID: ts.deniedUserID},
		Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
		Action:   action{Name: "archive"},
	})
	ts.False(unknownAction.Decision)
	ts.Equal(
		"Action archive is not registered on the resource server",
		errorMessage(unknownAction.Context),
	)
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessResourceIDDoesNotScopeCurrentDecisions() {
	tests := []struct {
		name       string
		resourceID string
	}{
		{
			name:       "created resource id",
			resourceID: ts.resourceID,
		},
		{
			name:       "generic resource id",
			resourceID: "booking-1",
		},
		{
			name:       "different resource id",
			resourceID: "booking-2",
		},
		{
			name:       "empty resource id",
			resourceID: "",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			result := ts.evaluate(evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: tc.resourceID},
				Action:   action{Name: ts.approvePermission},
			})
			ts.True(result.Decision)
		})
	}
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessRejectsCrossResourceServerPermission() {
	tests := []struct {
		name         string
		resourceType string
		permission   string
	}{
		{
			name:         "invoice permission against booking server",
			resourceType: authzenResourceIdentifier,
			permission:   ts.otherPermission,
		},
		{
			name:         "booking permission against invoice server",
			resourceType: authzenOtherResourceIdentifier,
			permission:   ts.readPermission,
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			result := ts.evaluate(evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: tc.resourceType, ID: "resource-1"},
				Action:   action{Name: tc.permission},
			})
			ts.False(result.Decision)
			ts.Contains(errorMessage(result.Context), "is not registered on the resource server")
		})
	}
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessRejectsUnknownSubject() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluation", mustJSON(evaluationRequest{
		Subject:  subject{Type: "user", ID: "00000000-0000-0000-0000-000000000000"},
		Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
		Action:   action{Name: ts.readPermission},
	}))
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, string(body))

	var errResp errorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("Invalid subject", errResp.Error)
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessReturnsDecisionForUnknownResourceAndAction() {
	unknownResource := ts.evaluate(evaluationRequest{
		Subject:  subject{Type: "user", ID: ts.userID},
		Resource: resource{Type: "authzen-unknown", ID: "booking-1"},
		Action:   action{Name: "authzen-unknown:read"},
	})
	ts.False(unknownResource.Decision)
	ts.Equal("Resource not found", errorMessage(unknownResource.Context))

	unknownAction := ts.evaluate(evaluationRequest{
		Subject:  subject{Type: "user", ID: ts.userID},
		Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
		Action:   action{Name: "archive"},
	})
	ts.False(unknownAction.Decision)
	ts.Equal(
		"Action archive is not registered on the resource server",
		errorMessage(unknownAction.Context),
	)
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessValidationErrors() {
	tests := []struct {
		name  string
		body  string
		error string
	}{
		{
			name:  "malformed JSON",
			body:  `{`,
			error: "Invalid request format",
		},
		{
			name:  "empty request body",
			body:  ``,
			error: "Invalid request format",
		},
		{
			name: "wrong subject data type",
			body: fmt.Sprintf(`{"subject":"%s","resource":{"type":%q,"id":"booking-1"},"action":{"name":%q}}`,
				ts.userID, authzenResourceIdentifier, ts.readPermission),
			error: "Invalid request format",
		},
		{
			name: "empty subject id",
			body: fmt.Sprintf(`{"subject":{"type":"user","id":""},"resource":{"type":%q,"id":"booking-1"},"action":{"name":%q}}`,
				authzenResourceIdentifier, ts.readPermission),
			error: "Missing subject",
		},
		{
			name: "missing subject",
			body: fmt.Sprintf(`{"resource":{"type":%q,"id":"booking-1"},"action":{"name":%q}}`,
				authzenResourceIdentifier, ts.readPermission),
			error: "Missing subject",
		},
		{
			name: "empty resource type",
			body: fmt.Sprintf(`{"subject":{"type":"user","id":%q},"resource":{"type":"","id":"booking-1"},"action":{"name":%q}}`,
				ts.userID, ts.readPermission),
			error: "Missing resource",
		},
		{
			name: "missing resource",
			body: fmt.Sprintf(`{"subject":{"type":"user","id":%q},"action":{"name":%q}}`,
				ts.userID, ts.readPermission),
			error: "Missing resource",
		},
		{
			name: "empty action name",
			body: fmt.Sprintf(`{"subject":{"type":"user","id":%q},"resource":{"type":%q,"id":"booking-1"},"action":{"name":""}}`,
				ts.userID, authzenResourceIdentifier),
			error: "Missing action",
		},
		{
			name: "missing action",
			body: fmt.Sprintf(`{"subject":{"type":"user","id":%q},"resource":{"type":%q,"id":"booking-1"}}`,
				ts.userID, authzenResourceIdentifier),
			error: "Missing action",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluation", []byte(tc.body))
			ts.Equal(http.StatusBadRequest, resp.StatusCode, string(body))

			var errResp errorResponse
			ts.Require().NoError(json.Unmarshal(body, &errResp))
			ts.Equal(tc.error, errResp.Error)
		})
	}
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessIgnoresUnknownJSONFields() {
	body := fmt.Sprintf(`{
		"subject": {"type": "user", "id": %q, "tenant": "ignored"},
		"resource": {"type": %q, "id": %q, "owner": "ignored"},
		"action": {"name": %q, "risk": "ignored"},
		"unknown": "ignored"
	}`, ts.userID, authzenResourceIdentifier, ts.resourceID, ts.readPermission)

	resp, payload := ts.doRequest(http.MethodPost, "/access/v1/evaluation", []byte(body))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(payload))

	var result evaluationResponse
	ts.Require().NoError(json.Unmarshal(payload, &result))
	ts.True(result.Decision)
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessContentTypeIsNotEnforced() {
	payload := mustJSON(evaluationRequest{
		Subject:  subject{Type: "user", ID: ts.userID},
		Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
		Action:   action{Name: ts.readPermission},
	})

	tests := []struct {
		name        string
		contentType *string
	}{
		{
			name: "without content type",
		},
		{
			name:        "text plain content type",
			contentType: stringPtr("text/plain"),
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			resp, body := ts.doRawRequest(http.MethodPost, "/access/v1/evaluation", payload, tc.contentType)
			ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

			var result evaluationResponse
			ts.Require().NoError(json.Unmarshal(body, &result))
			ts.True(result.Decision)
		})
	}
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessFieldCombinations() {
	tests := []struct {
		name           string
		request        evaluationRequest
		expectedStatus int
		expectedError  string
		expectedAllow  bool
	}{
		{
			name: "subject with type",
			request: evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
				Action:   action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusOK,
			expectedAllow:  true,
		},
		{
			name: "subject without type",
			request: evaluationRequest{
				Subject:  subject{ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
				Action:   action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusOK,
			expectedAllow:  true,
		},
		{
			name: "subject without id",
			request: evaluationRequest{
				Subject:  subject{Type: "user"},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
				Action:   action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing subject",
		},
		{
			name: "subject without type or id",
			request: evaluationRequest{
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
				Action:   action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing subject",
		},
		{
			name: "resource with type and id",
			request: evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
				Action:   action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusOK,
			expectedAllow:  true,
		},
		{
			name: "resource without id",
			request: evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier},
				Action:   action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusOK,
			expectedAllow:  true,
		},
		{
			name: "resource without type",
			request: evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{ID: ts.resourceID},
				Action:   action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing resource",
		},
		{
			name: "resource without type or id",
			request: evaluationRequest{
				Subject: subject{Type: "user", ID: ts.userID},
				Action:  action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing resource",
		},
		{
			name: "action with name",
			request: evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
				Action:   action{Name: ts.readPermission},
			},
			expectedStatus: http.StatusOK,
			expectedAllow:  true,
		},
		{
			name: "action without name",
			request: evaluationRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing action",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluation", mustJSON(tc.request))
			ts.Require().Equal(tc.expectedStatus, resp.StatusCode, string(body))

			if tc.expectedStatus == http.StatusOK {
				var result evaluationResponse
				ts.Require().NoError(json.Unmarshal(body, &result))
				ts.Equal(tc.expectedAllow, result.Decision)
				return
			}

			var errResp errorResponse
			ts.Require().NoError(json.Unmarshal(body, &errResp))
			ts.Equal(tc.expectedError, errResp.Error)
		})
	}
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessBatchPreservesOrderAndItemErrors() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluations", mustJSON(evaluationsRequest{
		Evaluations: []evaluationRequest{
			{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: ts.readPermission},
			},
			{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: ts.writePermission},
			},
			{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: "archive"},
			},
			{
				Subject:  subject{Type: "agent", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: ts.readPermission},
			},
		},
	}))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var result evaluationsResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Len(result.Evaluations, 4)
	ts.True(result.Evaluations[0].Decision)
	ts.False(result.Evaluations[1].Decision)
	ts.Equal("Subject is not authorized to perform the requested action", result.Evaluations[1].Context["reason"])
	ts.False(result.Evaluations[2].Decision)
	ts.Equal("Action archive is not registered on the resource server",
		errorMessage(result.Evaluations[2].Context))
	ts.False(result.Evaluations[3].Decision)
	ts.Equal("Invalid subject", errorMessage(result.Evaluations[3].Context))
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessBatchCombinations() {
	tests := []struct {
		name      string
		request   evaluationsRequest
		decisions []bool
	}{
		{
			name: "one evaluation only",
			request: evaluationsRequest{
				Evaluations: []evaluationRequest{
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
						Action:   action{Name: ts.readPermission},
					},
				},
			},
			decisions: []bool{true},
		},
		{
			name: "multiple evaluations all allowed",
			request: evaluationsRequest{
				Evaluations: []evaluationRequest{
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
						Action:   action{Name: ts.readPermission},
					},
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: "booking-2"},
						Action:   action{Name: ts.approvePermission},
					},
				},
			},
			decisions: []bool{true, true},
		},
		{
			name: "multiple evaluations all denied",
			request: evaluationsRequest{
				Evaluations: []evaluationRequest{
					{
						Subject:  subject{Type: "user", ID: ts.deniedUserID},
						Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
						Action:   action{Name: ts.readPermission},
					},
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
						Action:   action{Name: ts.writePermission},
					},
				},
			},
			decisions: []bool{false, false},
		},
		{
			name: "duplicate evaluations",
			request: evaluationsRequest{
				Evaluations: []evaluationRequest{
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
						Action:   action{Name: ts.readPermission},
					},
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
						Action:   action{Name: ts.readPermission},
					},
				},
			},
			decisions: []bool{true, true},
		},
		{
			name: "different resources and actions",
			request: evaluationsRequest{
				Evaluations: []evaluationRequest{
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
						Action:   action{Name: ts.readPermission},
					},
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: "booking-2"},
						Action:   action{Name: ts.writePermission},
					},
					{
						Subject:  subject{Type: "user", ID: ts.userID},
						Resource: resource{Type: authzenResourceIdentifier, ID: "booking-3"},
						Action:   action{Name: ts.approvePermission},
					},
				},
			},
			decisions: []bool{true, false, true},
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluations", mustJSON(tc.request))
			ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

			var result evaluationsResponse
			ts.Require().NoError(json.Unmarshal(body, &result))
			ts.Require().Len(result.Evaluations, len(tc.decisions))
			for i, expectedDecision := range tc.decisions {
				ts.Equal(expectedDecision, result.Evaluations[i].Decision)
			}
		})
	}
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessBatchReturnsPerItemFieldValidationErrors() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluations", mustJSON(evaluationsRequest{
		Evaluations: []evaluationRequest{
			{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: ts.readPermission},
			},
			{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
			},
			{
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: ts.readPermission},
			},
			{
				Subject: subject{Type: "user", ID: ts.userID},
				Action:  action{Name: ts.readPermission},
			},
		},
	}))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var result evaluationsResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Len(result.Evaluations, 4)
	ts.True(result.Evaluations[0].Decision)
	ts.False(result.Evaluations[1].Decision)
	ts.Equal("Missing action", errorMessage(result.Evaluations[1].Context))
	ts.False(result.Evaluations[2].Decision)
	ts.Equal("Missing subject", errorMessage(result.Evaluations[2].Context))
	ts.False(result.Evaluations[3].Decision)
	ts.Equal("Missing resource", errorMessage(result.Evaluations[3].Context))
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessBatchKeepsDecisionsSeparateForDifferentSubjects() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluations", mustJSON(evaluationsRequest{
		Evaluations: []evaluationRequest{
			{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: ts.readPermission},
			},
			{
				Subject:  subject{Type: "user", ID: ts.deniedUserID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: ts.readPermission},
			},
			{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
				Action:   action{Name: ts.approvePermission},
			},
		},
	}))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var result evaluationsResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Len(result.Evaluations, 3)
	ts.True(result.Evaluations[0].Decision)
	ts.False(result.Evaluations[1].Decision)
	ts.True(result.Evaluations[2].Decision)
}

func (ts *AuthZENAPITestSuite) TestEvaluateAccessBatchRejectsEmptyEvaluations() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluations", mustJSON(evaluationsRequest{}))
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, string(body))

	var errResp errorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("Missing evaluations", errResp.Error)
}

func (ts *AuthZENAPITestSuite) TestSearchActionReturnsAllowedActionsOnly() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/search/action", mustJSON(searchActionRequest{
		Subject:  subject{Type: "user", ID: ts.userID},
		Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
	}))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var result searchActionResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.ElementsMatch([]action{
		{Name: ts.readPermission},
		{Name: ts.approvePermission},
	}, result.Results)
}

func (ts *AuthZENAPITestSuite) TestSearchActionDeniedUserReturnsEmptyResults() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/search/action", mustJSON(searchActionRequest{
		Subject:  subject{Type: "user", ID: ts.deniedUserID},
		Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
	}))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var result searchActionResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Empty(result.Results)
}

func (ts *AuthZENAPITestSuite) TestSearchActionResourceIDDoesNotScopeCurrentResults() {
	tests := []struct {
		name       string
		resourceID string
	}{
		{
			name:       "created resource id",
			resourceID: ts.resourceID,
		},
		{
			name:       "nonexistent resource id",
			resourceID: "missing-booking",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			resp, body := ts.doRequest(http.MethodPost, "/access/v1/search/action", mustJSON(searchActionRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: tc.resourceID},
			}))
			ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

			var result searchActionResponse
			ts.Require().NoError(json.Unmarshal(body, &result))
			ts.ElementsMatch([]action{
				{Name: ts.readPermission},
				{Name: ts.approvePermission},
			}, result.Results)
		})
	}
}

func (ts *AuthZENAPITestSuite) TestSearchActionUnknownResourceReturnsBadRequest() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/search/action", mustJSON(searchActionRequest{
		Subject:  subject{Type: "user", ID: ts.userID},
		Resource: resource{Type: "authzen-unknown", ID: "booking-1"},
	}))
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, string(body))

	var errResp errorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("Invalid resource", errResp.Error)
}

func (ts *AuthZENAPITestSuite) TestSearchActionValidationErrors() {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/search/action", mustJSON(searchActionRequest{
		Resource: resource{Type: authzenResourceIdentifier, ID: "booking-1"},
	}))
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, string(body))

	var errResp errorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("Missing subject", errResp.Error)

	resp, body = ts.doRequest(http.MethodPost, "/access/v1/search/action", mustJSON(searchActionRequest{
		Subject: subject{Type: "user", ID: ts.userID},
	}))
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, string(body))
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("Missing resource", errResp.Error)
}

func (ts *AuthZENAPITestSuite) TestWrongMethodsReturnMethodNotAllowed() {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "get single evaluation",
			method: http.MethodGet,
			path:   "/access/v1/evaluation",
		},
		{
			name:   "delete search action",
			method: http.MethodDelete,
			path:   "/access/v1/search/action",
		},
		{
			name:   "post metadata",
			method: http.MethodPost,
			path:   "/.well-known/authzen-configuration",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			resp, body := ts.doRequest(tc.method, tc.path, nil)
			ts.Require().Equal(http.StatusMethodNotAllowed, resp.StatusCode, string(body))
		})
	}
}

func (ts *AuthZENAPITestSuite) TestSearchActionFieldCombinations() {
	tests := []struct {
		name           string
		request        searchActionRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "subject with type",
			request: searchActionRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "subject without type",
			request: searchActionRequest{
				Subject:  subject{ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "subject without id",
			request: searchActionRequest{
				Subject:  subject{Type: "user"},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing subject",
		},
		{
			name: "resource with type and id",
			request: searchActionRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier, ID: ts.resourceID},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "resource without id",
			request: searchActionRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{Type: authzenResourceIdentifier},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "resource without type",
			request: searchActionRequest{
				Subject:  subject{Type: "user", ID: ts.userID},
				Resource: resource{ID: ts.resourceID},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing resource",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			resp, body := ts.doRequest(http.MethodPost, "/access/v1/search/action", mustJSON(tc.request))
			ts.Require().Equal(tc.expectedStatus, resp.StatusCode, string(body))

			if tc.expectedStatus == http.StatusOK {
				var result searchActionResponse
				ts.Require().NoError(json.Unmarshal(body, &result))
				ts.NotEmpty(result.Results)
				return
			}

			var errResp errorResponse
			ts.Require().NoError(json.Unmarshal(body, &errResp))
			ts.Equal(tc.expectedError, errResp.Error)
		})
	}
}

func (ts *AuthZENAPITestSuite) TestOptionsRequests() {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "single evaluation",
			path: "/access/v1/evaluation",
		},
		{
			name: "batch evaluations",
			path: "/access/v1/evaluations",
		},
		{
			name: "search action",
			path: "/access/v1/search/action",
		},
		{
			name: "metadata",
			path: "/.well-known/authzen-configuration",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			resp, body := ts.doRequest(http.MethodOptions, tc.path, nil)
			ts.Require().Equal(http.StatusNoContent, resp.StatusCode, string(body))
		})
	}
}

func (ts *AuthZENAPITestSuite) evaluate(request evaluationRequest) evaluationResponse {
	resp, body := ts.doRequest(http.MethodPost, "/access/v1/evaluation", mustJSON(request))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var result evaluationResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	return result
}

func (ts *AuthZENAPITestSuite) doRequest(method string, path string, body []byte) (*http.Response, []byte) {
	return ts.doRequestWithHeaders(method, path, body, nil)
}

func (ts *AuthZENAPITestSuite) doRawRequest(
	method string,
	path string,
	body []byte,
	contentType *string,
) (*http.Response, []byte) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, authzenServerURL+path, reader)
	ts.Require().NoError(err)
	if contentType != nil {
		req.Header.Set("Content-Type", *contentType)
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp, respBody
}

func (ts *AuthZENAPITestSuite) doRequestWithHeaders(
	method string,
	path string,
	body []byte,
	headers map[string]string,
) (*http.Response, []byte) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, authzenServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp, respBody
}

func (ts *AuthZENAPITestSuite) doUnauthenticatedRequest(
	method string,
	path string,
	body []byte,
	headers map[string]string,
) (*http.Response, []byte) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, authzenServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp, respBody
}

func createResourceServer(request testutils.ResourceServer) (*testutils.ResourceServer, error) {
	var response testutils.ResourceServer
	if err := doJSON(http.MethodPost, "/resource-servers", request, http.StatusCreated, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func createResource(resourceServerID string, request createResourceRequest) (*resourceResponse, error) {
	var response resourceResponse
	if err := doJSON(http.MethodPost, fmt.Sprintf("/resource-servers/%s/resources", resourceServerID),
		request, http.StatusCreated, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func createAction(resourceServerID string, resourceID string, request testutils.Action) (*testutils.Action, error) {
	path := fmt.Sprintf("/resource-servers/%s/actions", resourceServerID)
	if resourceID != "" {
		path = fmt.Sprintf("/resource-servers/%s/resources/%s/actions", resourceServerID, resourceID)
	}

	var response testutils.Action
	if err := doJSON(http.MethodPost, path, request, http.StatusCreated, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func doJSON(method string, path string, request interface{}, expectedStatus int, response interface{}) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, authzenServerURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("expected status %d, got %d. Response: %s", expectedStatus, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(body))
	}
	return nil
}

func mustJSON(value interface{}) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}

func errorMessage(context map[string]interface{}) string {
	errContext, ok := context["error"].(map[string]interface{})
	if !ok {
		return ""
	}
	message, _ := errContext["message"].(string)
	return message
}

func stringPtr(value string) *string {
	return &value
}
