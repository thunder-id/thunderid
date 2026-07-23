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
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
		FlowType:    providers.FlowTypeAuthentication,
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
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
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
		FlowType:    providers.FlowTypeAuthentication,
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
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
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
		FlowType:          providers.FlowTypeAuthentication,
		UserInputs:        map[string]string{},
		RuntimeData:       map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{},
		ExecutionHistory:  map[string]*providers.NodeExecutionRecord{},
		Graph:             mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
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
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		FlowType:    providers.FlowTypeAuthentication,
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
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)
	content := s.getContextContent(dbModel)
	s.NotNil(content.Token)

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

	s.NoError(err)
	s.Equal("test-flow-id", resultCtx.ExecutionID)
	s.Equal("test-app-id", resultCtx.AppID)
	s.True(resultCtx.AuthenticatedUser.IsAuthenticated)
	s.Equal("user-456", resultCtx.AuthenticatedUser.UserID)
	s.Equal(testToken, resultCtx.AuthenticatedUser.Token)
}

func (s *ModelTestSuite) TestToEngineContext_WithoutToken() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

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

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

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
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication).Maybe()

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ctx := EngineContext{
				ExecutionID: "test-flow-id",
				AppID:       tc.appID,
				FlowType:    providers.FlowTypeAuthentication,
				AuthenticatedUser: authncm.AuthenticatedUser{
					IsAuthenticated: tc.userID != "",
					UserID:          tc.userID,
					Attributes:      map[string]interface{}{},
				},
				UserInputs:       tc.inputs,
				RuntimeData:      tc.runtime,
				ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
				Graph:            mockGraph,
			}

			dbModel := &FlowContextDB{}
			err := dbModel.FromEngineContext(ctx)
			s.NoError(err)

			// Context should be plain JSON (encryption is the service's responsibility)
			s.Contains(dbModel.Context, `"appId"`)

			resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)
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
		FlowType:      providers.FlowTypeAuthentication,
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
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{
			"node1": {NodeID: "node1"},
		},
		Graph: mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
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
	testAvailableAttributes := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {
				AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
					IsVerified: true,
				},
			},
			"phoneNumber": {
				AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
					IsVerified: false,
				},
			},
		},
		Verifications: map[string]*providers.VerificationResponse{},
	}
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		Verbose:     true,
		FlowType:    providers.FlowTypeAuthentication,
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
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
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
		FlowType:    providers.FlowTypeAuthentication,
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
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)
	s.NotNil(dbModel)
	s.Equal("test-flow-id", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.True(content.IsAuthenticated)
	s.Nil(content.AvailableAttributes)
}

func (s *ModelTestSuite) TestToEngineContext_WithAvailableAttributes() {
	testAvailableAttributes := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {
				AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
					IsVerified: true,
				},
			},
			"address": {
				AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
					IsVerified: false,
				},
			},
		},
		Verifications: map[string]*providers.VerificationResponse{},
	}
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:     context.Background(),
		ExecutionID: "test-flow-id",
		AppID:       "test-app-id",
		FlowType:    providers.FlowTypeAuthentication,
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
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)
	content := s.getContextContent(dbModel)
	s.NotNil(content.AvailableAttributes)

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

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
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

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

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

	s.NoError(err)
	s.Equal("test-flow-id", resultCtx.ExecutionID)
	s.True(resultCtx.AuthenticatedUser.IsAuthenticated)
	s.Equal("user-987", resultCtx.AuthenticatedUser.UserID)
	s.Nil(resultCtx.AuthenticatedUser.AvailableAttributes)
}

func (s *ModelTestSuite) TestAvailableAttributesSerializationRoundTrip() {
	testCases := []struct {
		name       string
		attributes *providers.AttributesResponse
	}{
		{
			name: "Single attribute",
			attributes: &providers.AttributesResponse{
				Attributes: map[string]*providers.AttributeResponse{
					"email": {
						AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
							IsVerified: true,
						},
					},
				},
				Verifications: map[string]*providers.VerificationResponse{},
			},
		},
		{
			name: "Multiple attributes",
			attributes: &providers.AttributesResponse{
				Attributes: map[string]*providers.AttributeResponse{
					"email": {
						AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
							IsVerified: true,
						},
					},
					"phone": {
						AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
							IsVerified: false,
						},
					},
					"address": {
						AssuranceMetadataResponse: &providers.AssuranceMetadataResponse{
							IsVerified: true,
						},
					},
				},
				Verifications: map[string]*providers.VerificationResponse{},
			},
		},
	}

	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id").Maybe()
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication).Maybe()

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ctx := EngineContext{
				Context:     context.Background(),
				ExecutionID: "test-flow-id",
				AppID:       "test-app-id",
				FlowType:    providers.FlowTypeAuthentication,
				AuthenticatedUser: authncm.AuthenticatedUser{
					IsAuthenticated:     true,
					UserID:              "user-123",
					AvailableAttributes: tc.attributes,
					Attributes:          map[string]interface{}{},
				},
				UserInputs:       map[string]string{},
				RuntimeData:      map[string]string{},
				ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
				Graph:            mockGraph,
			}

			dbModel := &FlowContextDB{}
			err := dbModel.FromEngineContext(ctx)
			s.NoError(err)
			content := s.getContextContent(dbModel)
			s.NotNil(content.AvailableAttributes)

			resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)
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
		FlowType:         providers.FlowTypeAuthentication,
		CurrentSegmentID: "seg-1",
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
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
		FlowType:         providers.FlowTypeAuthentication,
		CurrentSegmentID: "",
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)

	content := s.getContextContent(dbModel)
	s.Nil(content.CurrentSegmentID)
}

func (s *ModelTestSuite) TestToEngineContext_WithCurrentSegmentID() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

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

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

	s.NoError(err)
	s.Equal("seg-1", resultCtx.CurrentSegmentID)
}

func (s *ModelTestSuite) TestToEngineContext_MissingCurrentSegmentID_IsEmpty() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

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

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

	s.NoError(err)
	s.Equal("", resultCtx.CurrentSegmentID)
}

func (s *ModelTestSuite) TestCurrentSegmentID_RoundTrip() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "test-exec-id",
		FlowType:         providers.FlowTypeAuthentication,
		CurrentSegmentID: "seg-2",
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)
	s.NoError(err)
	s.Equal("seg-2", resultCtx.CurrentSegmentID)
}

// MergeRuntimeData tests

func (s *ModelTestSuite) TestMergeRuntimeData_IntoExistingMap() {
	ctx := &EngineContext{
		RuntimeData: map[string]string{"existing": "value"},
	}

	ctx.mergeRuntimeData(map[string]string{"new": "data", "another": "entry"})

	s.Len(ctx.RuntimeData, 3)
	s.Equal("value", ctx.RuntimeData["existing"])
	s.Equal("data", ctx.RuntimeData["new"])
	s.Equal("entry", ctx.RuntimeData["another"])
}

func (s *ModelTestSuite) TestMergeRuntimeData_NilRuntimeData() {
	ctx := &EngineContext{
		RuntimeData: nil,
	}

	ctx.mergeRuntimeData(map[string]string{"key": "value"})

	s.NotNil(ctx.RuntimeData)
	s.Equal("value", ctx.RuntimeData["key"])
}

func (s *ModelTestSuite) TestMergeRuntimeData_OverwritesExistingKeys() {
	ctx := &EngineContext{
		RuntimeData: map[string]string{"key": "old"},
	}

	ctx.mergeRuntimeData(map[string]string{"key": "new"})

	s.Equal("new", ctx.RuntimeData["key"])
}

func (s *ModelTestSuite) TestMergeRuntimeData_EmptyInput() {
	ctx := &EngineContext{
		RuntimeData: map[string]string{"existing": "value"},
	}

	ctx.mergeRuntimeData(map[string]string{})

	s.Len(ctx.RuntimeData, 1)
	s.Equal("value", ctx.RuntimeData["existing"])
}

func (s *ModelTestSuite) TestMergeRuntimeData_NilInput() {
	ctx := &EngineContext{
		RuntimeData: map[string]string{"existing": "value"},
	}

	ctx.mergeRuntimeData(nil)

	s.Len(ctx.RuntimeData, 1)
	s.Equal("value", ctx.RuntimeData["existing"])
}

// InterceptorSharedData serialization tests

func (s *ModelTestSuite) TestFromEngineContext_WithInterceptorSharedData() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:               context.Background(),
		ExecutionID:           "test-exec-id",
		FlowType:              providers.FlowTypeAuthentication,
		UserInputs:            map[string]string{},
		RuntimeData:           map[string]string{},
		ExecutionHistory:      map[string]*providers.NodeExecutionRecord{},
		Graph:                 mockGraph,
		InterceptorSharedData: map[string]string{"challenge": "abc123"},
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)
	s.NotNil(dbModel)

	content := s.getContextContent(dbModel)
	s.NotNil(content.InterceptorSharedData)
	s.Contains(*content.InterceptorSharedData, "abc123")
}

func (s *ModelTestSuite) TestFromEngineContext_NilInterceptorSharedData() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:               context.Background(),
		ExecutionID:           "test-exec-id",
		FlowType:              providers.FlowTypeAuthentication,
		UserInputs:            map[string]string{},
		RuntimeData:           map[string]string{},
		ExecutionHistory:      map[string]*providers.NodeExecutionRecord{},
		Graph:                 mockGraph,
		InterceptorSharedData: nil,
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)
	content := s.getContextContent(dbModel)
	// nil map is marshaled as "null", same pattern as RuntimeData
	s.NotNil(content.InterceptorSharedData)
	s.Equal("null", *content.InterceptorSharedData)
}

func (s *ModelTestSuite) TestToEngineContext_WithInterceptorSharedData() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	sharedDataJSON := `{"token":"xyz"}`

	content := flowContextContent{
		GraphID:               "test-graph-id",
		InterceptorSharedData: &sharedDataJSON,
		UserInputs:            func() *string { v := `{}`; return &v }(),
		RuntimeData:           func() *string { v := `{}`; return &v }(),
		ExecutionHistory:      func() *string { v := `{}`; return &v }(),
	}
	ctxJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "test-exec-id",
		Context:     string(ctxJSON),
	}

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

	s.NoError(err)
	s.NotNil(resultCtx.InterceptorSharedData)
	s.Equal("xyz", resultCtx.InterceptorSharedData["token"])
}

func (s *ModelTestSuite) TestToEngineContext_NilInterceptorSharedData() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	content := flowContextContent{
		GraphID:               "test-graph-id",
		InterceptorSharedData: nil,
		UserInputs:            func() *string { v := `{}`; return &v }(),
		RuntimeData:           func() *string { v := `{}`; return &v }(),
		ExecutionHistory:      func() *string { v := `{}`; return &v }(),
	}
	ctxJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "test-exec-id",
		Context:     string(ctxJSON),
	}

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

	s.NoError(err)
	// When InterceptorSharedData is nil in content, it initializes to empty map (same as RuntimeData)
	s.NotNil(resultCtx.InterceptorSharedData)
	s.Empty(resultCtx.InterceptorSharedData)
}

func (s *ModelTestSuite) TestInterceptorSharedData_RoundTrip() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:               context.Background(),
		ExecutionID:           "test-exec-id",
		FlowType:              providers.FlowTypeAuthentication,
		UserInputs:            map[string]string{},
		RuntimeData:           map[string]string{},
		ExecutionHistory:      map[string]*providers.NodeExecutionRecord{},
		Graph:                 mockGraph,
		InterceptorSharedData: map[string]string{"reqCount": "3"},
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)
	s.NoError(err)

	s.NotNil(resultCtx.InterceptorSharedData)
	s.Equal("3", resultCtx.InterceptorSharedData["reqCount"])
}

// FrameStack tests

func (s *ModelTestSuite) TestFrameStack_PushAndDepth() {
	ctx := &EngineContext{}
	ctx.pushFrame("call-node-1")
	s.Equal(1, ctx.frameDepth())
}

func (s *ModelTestSuite) TestFrameStack_PushAndPop() {
	mockCallerGraph := coremock.NewGraphInterfaceMock(s.T())
	mockCalleeGraph := coremock.NewGraphInterfaceMock(s.T())

	ctx := &EngineContext{
		Graph:    mockCallerGraph,
		FlowType: providers.FlowTypeAuthentication,
		RuntimeData: map[string]string{
			"caller-key": "caller-value",
		},
	}

	ctx.pushFrame("call-node-1")

	// Simulate switching to callee state
	ctx.Graph = mockCalleeGraph
	ctx.FlowType = providers.FlowTypeRegistration
	ctx.RuntimeData = map[string]string{"callee-key": "callee-value"}

	s.Equal(1, ctx.frameDepth())

	popped := ctx.popFrame()
	s.NotNil(popped)
	s.Equal(0, ctx.frameDepth())
	// Caller state restored
	s.Equal(mockCallerGraph, ctx.Graph)
	s.Equal(providers.FlowTypeAuthentication, ctx.FlowType)
	s.Equal("caller-value", ctx.RuntimeData["caller-key"])
}

func (s *ModelTestSuite) TestFrameStack_PopEmpty() {
	ctx := &EngineContext{}
	result := ctx.popFrame()
	s.Nil(result)
	s.Equal(0, ctx.frameDepth())
}

func (s *ModelTestSuite) TestSharedRuntimeData_SetAndGet() {
	ctx := &EngineContext{}
	ctx.setSharedRuntimeData("myKey", "myValue")
	val, ok := ctx.getSharedRuntimeData("myKey")
	s.True(ok)
	s.Equal("myValue", val)
}

func (s *ModelTestSuite) TestSharedRuntimeData_GetMissing() {
	ctx := &EngineContext{}
	val, ok := ctx.getSharedRuntimeData("nonexistent")
	s.False(ok)
	s.Equal("", val)
}

func (s *ModelTestSuite) TestSharedRuntimeData_NilMap() {
	ctx := &EngineContext{sharedRuntimeData: nil}
	val, ok := ctx.getSharedRuntimeData("anyKey")
	s.False(ok)
	s.Equal("", val)
}

func (s *ModelTestSuite) TestToEngineContext_BackwardsCompatibility_NoFrameStack() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	userInputs := `{}`
	runtimeData := `{}`
	executionHistory := `{}`

	content := flowContextContent{
		AppID:            "test-app-id",
		GraphID:          "graph-id",
		UserInputs:       &userInputs,
		RuntimeData:      &runtimeData,
		ExecutionHistory: &executionHistory,
		FrameStack:       nil,
	}
	ctxJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "exec-id",
		Context:     string(ctxJSON),
	}

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)

	s.NoError(err)
	s.Equal(0, resultCtx.frameDepth())
}

func (s *ModelTestSuite) TestToEngineContext_WithFrameStack_RoundTrip() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("callee-graph-id")
	mockGraph.On("GetType").Return(providers.FlowTypeRegistration)

	mockCallerGraph := coremock.NewGraphInterfaceMock(s.T())
	mockCallerGraph.On("GetID").Return("caller-graph-id")
	mockCallerGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	// Build an engine context with the callee as active graph and push the caller frame
	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-roundtrip",
		FlowType:         providers.FlowTypeAuthentication,
		AppID:            "app-id",
		Graph:            mockCallerGraph,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}

	ctx.pushFrame("call-node-1")
	// Simulate callee state as active
	ctx.Graph = mockGraph
	ctx.FlowType = providers.FlowTypeRegistration

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)

	resolver := graphResolverFunc(func(_ context.Context, _ string) (core.GraphInterface, error) {
		return mockCallerGraph, nil
	})

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, resolver)
	s.NoError(err)
	s.Equal(1, resultCtx.frameDepth())
}

func (s *ModelTestSuite) TestToEngineContext_WithSharedRuntimeData_RoundTrip() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:           context.Background(),
		ExecutionID:       "exec-srd",
		FlowType:          providers.FlowTypeAuthentication,
		AppID:             "app-id",
		Graph:             mockGraph,
		UserInputs:        map[string]string{},
		RuntimeData:       map[string]string{},
		ExecutionHistory:  map[string]*providers.NodeExecutionRecord{},
		sharedRuntimeData: map[string]string{"shared-key": "shared-value"},
	}

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)

	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)
	s.NoError(err)

	s.NotNil(resultCtx.sharedRuntimeData)
	val, ok := resultCtx.getSharedRuntimeData("shared-key")
	s.True(ok)
	s.Equal("shared-value", val)
}

func (s *ModelTestSuite) TestToEngineContext_WithFrameStack_NilResolver_Empty() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("callee-graph-id")
	mockGraph.On("GetType").Return(providers.FlowTypeRegistration)

	mockCallerGraph := coremock.NewGraphInterfaceMock(s.T())
	mockCallerGraph.On("GetID").Return("caller-graph-id")

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-nil-resolver",
		FlowType:         providers.FlowTypeAuthentication,
		AppID:            "app-id",
		Graph:            mockCallerGraph,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}

	ctx.pushFrame("call-node-1")
	ctx.Graph = mockGraph
	ctx.FlowType = providers.FlowTypeRegistration

	dbModel := &FlowContextDB{}
	err := dbModel.FromEngineContext(ctx)
	s.NoError(err)

	// nil resolver: frame stack should be ignored, no error
	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)
	s.NoError(err)
	s.Equal(0, resultCtx.frameDepth())
}

// --- InitiatorRequest round-trip ---

func (s *ModelTestSuite) TestInitiatorRequest_RoundTrip() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("graph-init-req")
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-init-req",
		FlowType:         providers.FlowTypeAuthentication,
		AppID:            "app-id",
		Graph:            mockGraph,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}
	ctx.SetInitiatorRequest(&providers.InitiatorRequest{
		Headers:     map[string][]string{"X-Request-Id": {"req-123"}, "Content-Type": {"application/json"}},
		QueryParams: map[string][]string{"client_id": {"my-client"}, "scope": {"openid"}},
	})

	dbModel := &FlowContextDB{}
	s.NoError(dbModel.FromEngineContext(ctx))

	restored, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)
	s.NoError(err)

	req := restored.GetInitiatorRequest()
	s.Require().NotNil(req)
	s.Equal([]string{"req-123"}, req.Headers["X-Request-Id"])
	s.Equal([]string{"application/json"}, req.Headers["Content-Type"])
	s.Equal([]string{"my-client"}, req.QueryParams["client_id"])
	s.Equal([]string{"openid"}, req.QueryParams["scope"])
}

func (s *ModelTestSuite) TestInitiatorRequest_NilRoundTrip() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("graph-nil-init")
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-nil-init",
		FlowType:         providers.FlowTypeAuthentication,
		AppID:            "app-id",
		Graph:            mockGraph,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}
	// initiatorRequest is nil by default

	dbModel := &FlowContextDB{}
	s.NoError(dbModel.FromEngineContext(ctx))

	restored, err := dbModel.ToEngineContext(context.Background(), mockGraph, nil)
	s.NoError(err)
	s.Nil(restored.GetInitiatorRequest())
}

// --- serializeFrameStack ---

func (s *ModelTestSuite) TestSerializeFrameStack_EmptyStack() {
	dbModel := &FlowContextDB{}
	result, err := dbModel.serializeFrameStack(nil)
	s.NoError(err)
	s.Nil(result)

	result, err = dbModel.serializeFrameStack([]*frame{})
	s.NoError(err)
	s.Nil(result)
}

func (s *ModelTestSuite) TestSerializeFrameStack_NilGraphError() {
	dbModel := &FlowContextDB{}
	f := &frame{graph: nil, resumeCallNodeID: "call-1"}
	_, err := dbModel.serializeFrameStack([]*frame{f})
	s.Error(err)
}

func (s *ModelTestSuite) TestSerializeFrameStack_EmptyGraphIDError() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetID").Return("")

	dbModel := &FlowContextDB{}
	f := &frame{graph: mockGraph, resumeCallNodeID: "call-1"}
	_, err := dbModel.serializeFrameStack([]*frame{f})
	s.Error(err)
}

func (s *ModelTestSuite) TestSerializeFrameStack_WithCurrentNode() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetID").Return("graph-1")
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1")

	dbModel := &FlowContextDB{}
	f := &frame{graph: mockGraph, currentNode: mockNode, resumeCallNodeID: "call-1"}
	result, err := dbModel.serializeFrameStack([]*frame{f})
	s.NoError(err)
	s.NotNil(result)

	var frames []serializedFrame
	s.NoError(json.Unmarshal([]byte(*result), &frames))
	s.Len(frames, 1)
	s.NotNil(frames[0].CurrentNodeID)
	s.Equal("node-1", *frames[0].CurrentNodeID)
}

func (s *ModelTestSuite) TestSerializeFrameStack_WithOptionalFields() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetID").Return("graph-1")

	dbModel := &FlowContextDB{}
	f := &frame{
		graph:            mockGraph,
		currentAction:    "action-1",
		currentSegmentID: "seg-1",
		runtimeData:      map[string]string{"key": "val"},
		resumeCallNodeID: "call-1",
	}
	result, err := dbModel.serializeFrameStack([]*frame{f})
	s.NoError(err)
	s.NotNil(result)

	var frames []serializedFrame
	s.NoError(json.Unmarshal([]byte(*result), &frames))
	s.Len(frames, 1)
	s.NotNil(frames[0].CurrentAction)
	s.Equal("action-1", *frames[0].CurrentAction)
	s.NotNil(frames[0].CurrentSegmentID)
	s.Equal("seg-1", *frames[0].CurrentSegmentID)
	s.NotNil(frames[0].RuntimeData)
}

// --- deserializeFrameStack ---

func (s *ModelTestSuite) TestDeserializeFrameStack_InvalidJSON() {
	bad := "not-json"
	content := flowContextContent{FrameStack: &bad}
	dbModel := &FlowContextDB{}
	resolver := graphResolverFunc(func(_ context.Context, _ string) (core.GraphInterface, error) {
		return nil, nil
	})
	_, err := dbModel.deserializeFrameStack(context.Background(), content, resolver)
	s.Error(err)
}

func (s *ModelTestSuite) TestDeserializeFrameStack_ResolveGraphError() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetID").Return("graph-1")
	f := &frame{graph: mockGraph, resumeCallNodeID: "call-1"}

	dbModel := &FlowContextDB{}
	serialized, err := dbModel.serializeFrameStack([]*frame{f})
	s.NoError(err)
	s.NotNil(serialized)

	content := flowContextContent{FrameStack: serialized}
	resolver := graphResolverFunc(func(_ context.Context, _ string) (core.GraphInterface, error) {
		return nil, errors.New("resolve error")
	})
	_, err = dbModel.deserializeFrameStack(context.Background(), content, resolver)
	s.Error(err)
}

func (s *ModelTestSuite) TestDeserializeFrameStack_WithCurrentNodeAndOptionalFields() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetID").Return("graph-1")
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1")
	mockGraph.On("GetNode", "node-1").Return(mockNode, true)

	dbModel := &FlowContextDB{}
	f := &frame{
		graph:            mockGraph,
		currentNode:      mockNode,
		currentAction:    "my-action",
		currentSegmentID: "my-seg",
		runtimeData:      map[string]string{"k": "v"},
		resumeCallNodeID: "call-1",
	}
	serialized, err := dbModel.serializeFrameStack([]*frame{f})
	s.NoError(err)
	s.NotNil(serialized)

	content := flowContextContent{FrameStack: serialized}
	resolver := graphResolverFunc(func(_ context.Context, _ string) (core.GraphInterface, error) {
		return mockGraph, nil
	})
	frames, err := dbModel.deserializeFrameStack(context.Background(), content, resolver)
	s.NoError(err)
	s.Len(frames, 1)
	s.Equal(mockNode, frames[0].currentNode)
	s.Equal("my-action", frames[0].currentAction)
	s.Equal("my-seg", frames[0].currentSegmentID)
	s.Equal("v", frames[0].runtimeData["k"])
}

func (s *ModelTestSuite) TestDeserializeFrameStack_InvalidRuntimeDataJSON() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)

	bad := "not-json"
	frames := []serializedFrame{{GraphID: "graph-1", RuntimeData: &bad}}
	b, _ := json.Marshal(frames)
	frameStackStr := string(b)
	content := flowContextContent{FrameStack: &frameStackStr}

	dbModel := &FlowContextDB{}
	resolver := graphResolverFunc(func(_ context.Context, _ string) (core.GraphInterface, error) {
		return mockGraph, nil
	})
	_, err := dbModel.deserializeFrameStack(context.Background(), content, resolver)
	s.Error(err)
}

func (s *ModelTestSuite) TestDeserializeFrameStack_NodeIDNotInGraph() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetType").Return(providers.FlowTypeAuthentication)
	mockGraph.On("GetNode", "missing-node").Return(nil, false)

	nodeID := "missing-node"
	frames := []serializedFrame{{GraphID: "graph-1", CurrentNodeID: &nodeID}}
	b, _ := json.Marshal(frames)
	frameStackStr := string(b)
	content := flowContextContent{FrameStack: &frameStackStr}

	dbModel := &FlowContextDB{}
	resolver := graphResolverFunc(func(_ context.Context, _ string) (core.GraphInterface, error) {
		return mockGraph, nil
	})
	result, err := dbModel.deserializeFrameStack(context.Background(), content, resolver)
	s.NoError(err)
	s.Len(result, 1)
	s.Nil(result[0].currentNode)
}
