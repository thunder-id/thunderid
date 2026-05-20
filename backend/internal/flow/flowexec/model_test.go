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

package flowexec

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	testUserID789 = "user-789"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) getContextContent(dbModel *FlowContextDB) flowContextContent {
	var content flowContextContent
	err := json.Unmarshal([]byte(dbModel.Context), &content)
	s.NoError(err)
	return content
}

func (s *ModelTestSuite) TestFromEngineContext_WithToken() {
	testToken := "test-token-123456"
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		Verbose:     true,
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "testuser",
		},
		RuntimeData: map[string]string{
			"key": "value",
		},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			Token:           testToken,
			Attributes: map[string]interface{}{
				"email": "test@example.com",
			},
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)

	s.NoError(err)
	s.NotNil(dbModel)
	s.Equal("test-flow-id", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.Equal("test-app-id", content.AppID)
	s.True(content.Verbose)
	s.True(content.IsAuthenticated)
	s.NotNil(content.UserID)
	s.Equal("user-123", *content.UserID)
	s.NotNil(content.Token)
	s.Equal(testToken, *content.Token)
}

func (s *ModelTestSuite) TestFromEngineContext_WithoutToken() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		Verbose:     false,
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "testuser",
		},
		RuntimeData: map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			Token:           "",
			Attributes:      map[string]interface{}{},
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)

	s.NoError(err)
	s.NotNil(dbModel)
	s.Equal("test-flow-id", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.True(content.IsAuthenticated)
	s.Nil(content.Token)
}

func (s *ModelTestSuite) TestFromEngineContext_WithEmptyAuthenticatedUser() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:           context.Background(),
		ExecutionID:       "test-flow-id",
		AppID:             "test-app-id",
		Verbose:           false,
		FlowType:          common.FlowTypeAuthentication,
		UserInputs:        map[string]string{},
		RuntimeData:       map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{},
		ExecutionHistory:  map[string]*common.NodeExecutionRecord{},
		Graph:             mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)

	s.NoError(err)
	s.NotNil(dbModel)

	content := s.getContextContent(dbModel)
	s.False(content.IsAuthenticated)
	s.Nil(content.UserID)
	s.Nil(content.Token)
}

func (s *ModelTestSuite) TestToEngineContext_WithToken() {
	testToken := "test-token-xyz789"
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-456",
			Token:           testToken,
			Attributes: map[string]interface{}{
				"role": "admin",
			},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)
	s.NoError(err)
	content := s.getContextContent(dbModel)
	s.NotNil(content.Token)

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)

	s.NoError(err)
	s.Equal("test-flow-id", resultCtx.ExecutionID)
	s.Equal("test-app-id", resultCtx.AppID)
	s.True(resultCtx.AuthenticatedUser.IsAuthenticated)
	s.Equal("user-456", resultCtx.AuthenticatedUser.UserID)
	s.Equal(testToken, resultCtx.AuthenticatedUser.Token)
}

func (s *ModelTestSuite) TestToEngineContext_WithoutToken() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	userInputs := `{"username":"testuser"}`
	runtimeData := `{"key":"value"}`
	userAttributes := `{"email":"test@example.com"}`
	executionHistory := `{}`
	userID := testUserID789

	content := flowContextContent{
		AppID:            "test-app-id",
		Verbose:          true,
		GraphID:          "test-graph-id",
		IsAuthenticated:  true,
		UserID:           &userID,
		UserInputs:       &userInputs,
		RuntimeData:      &runtimeData,
		UserAttributes:   &userAttributes,
		ExecutionHistory: &executionHistory,
		Token:            nil,
	}
	contextJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "test-flow-id",
		Context:     string(contextJSON),
	}

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)

	s.NoError(err)
	s.Equal("test-flow-id", resultCtx.ExecutionID)
	s.True(resultCtx.AuthenticatedUser.IsAuthenticated)
	s.Equal(testUserID789, resultCtx.AuthenticatedUser.UserID)
	s.Equal("", resultCtx.AuthenticatedUser.Token)
}

func (s *ModelTestSuite) TestGetGraphID() {
	userInputs := `{}`
	content := flowContextContent{
		GraphID:          "expected-graph-id",
		UserInputs:       &userInputs,
		RuntimeData:      &userInputs,
		ExecutionHistory: &userInputs,
	}
	contextJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "test-flow-id",
		Context:     string(contextJSON),
	}

	graphID, err := dbModel.GetGraphID(context.Background())

	s.NoError(err)
	s.Equal("expected-graph-id", graphID)
}

func (s *ModelTestSuite) TestGetGraphID_InvalidJSON() {
	dbModel := &FlowContextDB{
		ExecutionID: "test-flow-id",
		Context:     "not-valid-json",
	}

	graphID, err := dbModel.GetGraphID(context.Background())
	s.Error(err)
	s.Empty(graphID)
}

func (s *ModelTestSuite) TestContextRoundTrip() {
	testCases := []struct {
		name    string
		appID   string
		userID  string
		inputs  map[string]string
		runtime map[string]string
	}{
		{
			name:    "full context",
			appID:   "app-full-context",
			userID:  "user-full-context",
			inputs:  map[string]string{"username": "testuser", "password": "secret"},
			runtime: map[string]string{"state": "abc123", "nonce": "xyz789"},
		},
		{
			name:    "minimal context",
			appID:   "app-minimal",
			userID:  "",
			inputs:  map[string]string{},
			runtime: map[string]string{},
		},
	}

	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id").Maybe()
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication).Maybe()

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ctx := EngineContext{
				ExecutionID: "test-flow-id",
				AppID:       tc.appID,
				FlowType:    common.FlowTypeAuthentication,
				AuthenticatedUser: authncm.AuthenticatedUser{
					IsAuthenticated: tc.userID != "",
					UserID:          tc.userID,
					Attributes:      map[string]interface{}{},
				},
				UserInputs:       tc.inputs,
				RuntimeData:      tc.runtime,
				ExecutionHistory: map[string]*common.NodeExecutionRecord{},
				Graph:            mockGraph,
			}

			dbModel, err := FromEngineContext(ctx)
			s.NoError(err)

			// Context should be plain JSON (encryption is the service's responsibility)
			s.Contains(dbModel.Context, `"appId"`)

			resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)
			s.NoError(err)
			s.Equal(tc.appID, resultCtx.AppID)
			s.Equal(tc.userID, resultCtx.AuthenticatedUser.UserID)
			s.Equal(len(tc.inputs), len(resultCtx.UserInputs))
			s.Equal(len(tc.runtime), len(resultCtx.RuntimeData))
		})
	}
}

func (s *ModelTestSuite) TestFromEngineContext_PreservesOtherFields() {
	testToken := "test-token-preserve-fields"
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("graph-123")

	currentAction := "test-action"
	ctx := EngineContext{
		Context:       context.Background(),
		ExecutionID:   "flow-123",
		AppID:         "app-123",
		Verbose:       true,
		FlowType:      common.FlowTypeAuthentication,
		CurrentAction: currentAction,
		UserInputs: map[string]string{
			"input1": "value1",
			"input2": "value2",
		},
		RuntimeData: map[string]string{
			"runtime1": "val1",
		},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-abc",
			OUID:            "org-xyz",
			UserType:        "admin",
			Token:           testToken,
			Attributes: map[string]interface{}{
				"attr1": "value1",
			},
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{
			"node1": {NodeID: "node1"},
		},
		Graph: mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)

	s.NoError(err)
	s.Equal("flow-123", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.Equal("app-123", content.AppID)
	s.True(content.Verbose)
	s.NotNil(content.CurrentAction)
	s.Equal(currentAction, *content.CurrentAction)
	s.Equal("graph-123", content.GraphID)
	s.True(content.IsAuthenticated)
	s.NotNil(content.UserID)
	s.Equal("user-abc", *content.UserID)
	s.NotNil(content.OUID)
	s.Equal("org-xyz", *content.OUID)
	s.NotNil(content.UserType)
	s.Equal("admin", *content.UserType)
	s.NotNil(content.UserInputs)
	s.NotNil(content.RuntimeData)
	s.NotNil(content.UserAttributes)
	s.NotNil(content.ExecutionHistory)
	s.NotNil(content.Token)
}

func (s *ModelTestSuite) TestFromEngineContext_WithAvailableAttributes() {
	testAvailableAttributes := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {
				AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
					IsVerified: true,
				},
			},
			"phoneNumber": {
				AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
					IsVerified: false,
				},
			},
		},
		Verifications: map[string]*authnprovidercm.VerificationResponse{},
	}
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		Verbose:     true,
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "testuser",
		},
		RuntimeData: map[string]string{
			"key": "value",
		},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated:     true,
			UserID:              "user-123",
			AvailableAttributes: testAvailableAttributes,
			Attributes: map[string]interface{}{
				"email": "test@example.com",
			},
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)

	s.NoError(err)
	s.NotNil(dbModel)
	s.Equal("test-flow-id", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.Equal("test-app-id", content.AppID)
	s.True(content.Verbose)
	s.True(content.IsAuthenticated)
	s.NotNil(content.UserID)
	s.Equal("user-123", *content.UserID)
	s.NotNil(content.AvailableAttributes)
	s.Greater(len(*content.AvailableAttributes), 0)
	s.Contains(*content.AvailableAttributes, "\"email\"")
	s.Contains(*content.AvailableAttributes, "\"phoneNumber\"")
}

func (s *ModelTestSuite) TestFromEngineContext_WithoutAvailableAttributes() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		Verbose:     false,
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "testuser",
		},
		RuntimeData: map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated:     true,
			UserID:              "user-123",
			AvailableAttributes: nil,
			Attributes:          map[string]interface{}{},
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)

	s.NoError(err)
	s.NotNil(dbModel)
	s.Equal("test-flow-id", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.True(content.IsAuthenticated)
	s.Nil(content.AvailableAttributes)
}

func (s *ModelTestSuite) TestToEngineContext_WithAvailableAttributes() {
	testAvailableAttributes := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {
				AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
					IsVerified: true,
				},
			},
			"address": {
				AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
					IsVerified: false,
				},
			},
		},
		Verifications: map[string]*authnprovidercm.VerificationResponse{},
	}
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated:     true,
			UserID:              "user-456",
			AvailableAttributes: testAvailableAttributes,
			Attributes: map[string]interface{}{
				"role": "admin",
			},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)
	s.NoError(err)
	content := s.getContextContent(dbModel)
	s.NotNil(content.AvailableAttributes)

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)

	s.NoError(err)
	s.Equal("test-flow-id", resultCtx.ExecutionID)
	s.Equal("test-app-id", resultCtx.AppID)
	s.True(resultCtx.AuthenticatedUser.IsAuthenticated)
	s.Equal("user-456", resultCtx.AuthenticatedUser.UserID)
	s.NotNil(resultCtx.AuthenticatedUser.AvailableAttributes)
	s.Len(resultCtx.AuthenticatedUser.AvailableAttributes.Attributes, 2)
	s.Contains(resultCtx.AuthenticatedUser.AvailableAttributes.Attributes, "email")
	s.Contains(resultCtx.AuthenticatedUser.AvailableAttributes.Attributes, "address")
	s.True(resultCtx.AuthenticatedUser.AvailableAttributes.Attributes["email"].AssuranceMetadataResponse.IsVerified)
	s.False(resultCtx.AuthenticatedUser.AvailableAttributes.Attributes["address"].AssuranceMetadataResponse.IsVerified)
}

func (s *ModelTestSuite) TestToEngineContext_WithoutAvailableAttributes() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	userInputs := `{"username":"testuser"}`
	runtimeData := `{"key":"value"}`
	userAttributes := `{"email":"test@example.com"}`
	executionHistory := `{}`
	userID := "user-987"

	content := flowContextContent{
		AppID:               "test-flow-id",
		Verbose:             true,
		GraphID:             "test-graph-id",
		IsAuthenticated:     true,
		UserID:              &userID,
		UserInputs:          &userInputs,
		RuntimeData:         &runtimeData,
		UserAttributes:      &userAttributes,
		ExecutionHistory:    &executionHistory,
		AvailableAttributes: nil,
	}
	contextJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "test-flow-id",
		Context:     string(contextJSON),
	}

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)

	s.NoError(err)
	s.Equal("test-flow-id", resultCtx.ExecutionID)
	s.True(resultCtx.AuthenticatedUser.IsAuthenticated)
	s.Equal("user-987", resultCtx.AuthenticatedUser.UserID)
	s.Nil(resultCtx.AuthenticatedUser.AvailableAttributes)
}

func (s *ModelTestSuite) TestAvailableAttributesSerializationRoundTrip() {
	testCases := []struct {
		name       string
		attributes *authnprovidercm.AttributesResponse
	}{
		{
			name: "Single attribute",
			attributes: &authnprovidercm.AttributesResponse{
				Attributes: map[string]*authnprovidercm.AttributeResponse{
					"email": {
						AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
							IsVerified: true,
						},
					},
				},
				Verifications: map[string]*authnprovidercm.VerificationResponse{},
			},
		},
		{
			name: "Multiple attributes",
			attributes: &authnprovidercm.AttributesResponse{
				Attributes: map[string]*authnprovidercm.AttributeResponse{
					"email": {
						AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
							IsVerified: true,
						},
					},
					"phone": {
						AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
							IsVerified: false,
						},
					},
					"address": {
						AssuranceMetadataResponse: &authnprovidercm.AssuranceMetadataResponse{
							IsVerified: true,
						},
					},
				},
				Verifications: map[string]*authnprovidercm.VerificationResponse{},
			},
		},
	}

	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id").Maybe()
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication).Maybe()

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ctx := EngineContext{
				Context:     context.Background(),
				ExecutionID: "test-flow-id",
				AppID:       "test-app-id",
				FlowType:    common.FlowTypeAuthentication,
				AuthenticatedUser: authncm.AuthenticatedUser{
					IsAuthenticated:     true,
					UserID:              "user-123",
					AvailableAttributes: tc.attributes,
					Attributes:          map[string]interface{}{},
				},
				UserInputs:       map[string]string{},
				RuntimeData:      map[string]string{},
				ExecutionHistory: map[string]*common.NodeExecutionRecord{},
				Graph:            mockGraph,
			}

			dbModel, err := FromEngineContext(ctx)
			s.NoError(err)
			content := s.getContextContent(dbModel)
			s.NotNil(content.AvailableAttributes)

			resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)
			s.NoError(err)

			s.NotNil(resultCtx.AuthenticatedUser.AvailableAttributes)
			s.Len(resultCtx.AuthenticatedUser.AvailableAttributes.Attributes, len(tc.attributes.Attributes))
			for attrName, attrMetadata := range tc.attributes.Attributes {
				s.Contains(resultCtx.AuthenticatedUser.AvailableAttributes.Attributes, attrName)
				expectedVerified := attrMetadata.AssuranceMetadataResponse.IsVerified
				actualVerified := resultCtx.AuthenticatedUser.AvailableAttributes.Attributes[attrName].
					AssuranceMetadataResponse.IsVerified
				s.Equal(expectedVerified, actualVerified)
			}
		})
	}
}

func (s *ModelTestSuite) TestFromEngineContext_WithCurrentSegmentID() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "test-exec-id",
		FlowType:         common.FlowTypeAuthentication,
		CurrentSegmentID: "seg-1",
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)
	s.NoError(err)

	content := s.getContextContent(dbModel)
	s.NotNil(content.CurrentSegmentID)
	s.Equal("seg-1", *content.CurrentSegmentID)
}

func (s *ModelTestSuite) TestFromEngineContext_EmptyCurrentSegmentID_OmitsField() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "test-exec-id",
		FlowType:         common.FlowTypeAuthentication,
		CurrentSegmentID: "",
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)
	s.NoError(err)

	content := s.getContextContent(dbModel)
	s.Nil(content.CurrentSegmentID)
}

func (s *ModelTestSuite) TestToEngineContext_WithCurrentSegmentID() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	segID := "seg-1"
	content := flowContextContent{
		GraphID:          "test-graph-id",
		CurrentSegmentID: &segID,
		UserInputs:       func() *string { v := `{}`; return &v }(),
		RuntimeData:      func() *string { v := `{}`; return &v }(),
		ExecutionHistory: func() *string { v := `{}`; return &v }(),
	}
	ctxJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "test-exec-id",
		Context:     string(ctxJSON),
	}

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)

	s.NoError(err)
	s.Equal("seg-1", resultCtx.CurrentSegmentID)
}

func (s *ModelTestSuite) TestToEngineContext_MissingCurrentSegmentID_IsEmpty() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	content := flowContextContent{
		GraphID:          "test-graph-id",
		CurrentSegmentID: nil,
		UserInputs:       func() *string { v := `{}`; return &v }(),
		RuntimeData:      func() *string { v := `{}`; return &v }(),
		ExecutionHistory: func() *string { v := `{}`; return &v }(),
	}
	ctxJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "test-exec-id",
		Context:     string(ctxJSON),
	}

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)

	s.NoError(err)
	s.Equal("", resultCtx.CurrentSegmentID)
}

func (s *ModelTestSuite) TestCurrentSegmentID_RoundTrip() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "test-exec-id",
		FlowType:         common.FlowTypeAuthentication,
		CurrentSegmentID: "seg-2",
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)
	s.NoError(err)

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)
	s.NoError(err)
	s.Equal("seg-2", resultCtx.CurrentSegmentID)
}
