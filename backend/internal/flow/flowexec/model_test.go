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

	managerpkg "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/system/config"
	"github.com/asgardeo/thunder/tests/mocks/flow/coremock"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	// Setup test config with encryption key
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "2729a7928c79371e5f312167269294a14bb0660fd166b02a408a20fa73271580",
			},
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/test/thunderid/home", testConfig)
	if err != nil {
		t.Fatalf("failed to initialize server runtime: %v", err)
	}

	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) getContextContent(dbModel *FlowContextDB) flowContextContent {
	err := dbModel.decrypt(context.Background())
	s.NoError(err)
	var content flowContextContent
	err = json.Unmarshal([]byte(dbModel.Context), &content)
	s.NoError(err)
	return content
}

// buildAuthUser creates an AuthUser with the given identity fields and a single verified auth history entry.
func buildAuthUser(userID, userType, ouID string) managerpkg.AuthUser {
	data, _ := json.Marshal(map[string]interface{}{
		"authHistory": []map[string]interface{}{{
			"authType":   "password",
			"isVerified": true,
		}},
		"userHistory": []map[string]interface{}{{
			"userId":           userID,
			"userType":         userType,
			"ouId":             ouID,
			"isValuesIncluded": true,
		}},
		"userState": "exists",
	})
	var authUser managerpkg.AuthUser
	_ = json.Unmarshal(data, &authUser)
	return authUser
}

func (s *ModelTestSuite) TestFromEngineContext_WithAuthUser() {
	// Setup
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	authUser := buildAuthUser("user-123", "", "")

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "test-flow-id",
		AppID:            "test-app-id",
		Verbose:          true,
		FlowType:         common.FlowTypeAuthentication,
		UserInputs:       map[string]string{"username": "testuser"},
		RuntimeData:      map[string]string{"key": "value"},
		AuthUser:         authUser,
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	// Execute
	dbModel, err := FromEngineContext(ctx)

	// Verify
	s.NoError(err)
	s.NotNil(dbModel)
	s.Equal("test-flow-id", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.Equal("test-app-id", content.AppID)
	s.True(content.Verbose)
	s.NotNil(content.AuthUser, "AuthUser should be serialized when set")
}

func (s *ModelTestSuite) TestFromEngineContext_WithEmptyAuthUser() {
	// Setup
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "test-flow-id",
		AppID:            "test-app-id",
		Verbose:          false,
		FlowType:         common.FlowTypeAuthentication,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		AuthUser:         managerpkg.AuthUser{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	// Execute
	dbModel, err := FromEngineContext(ctx)

	// Verify
	s.NoError(err)
	s.NotNil(dbModel)
	s.Equal("test-flow-id", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.Nil(content.AuthUser, "AuthUser should be nil when not set")
}

func (s *ModelTestSuite) TestToEngineContext_WithAuthUser() {
	// Setup
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	authUser := buildAuthUser("user-456", "admin", "org-xyz")

	ctx := EngineContext{
		Context:          context.Background(),
		ExecutionID:      "test-flow-id",
		AppID:            "test-app-id",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         authUser,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)
	s.NoError(err)

	// Execute - Convert back to EngineContext
	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)

	// Verify
	s.NoError(err)
	s.Equal("test-flow-id", resultCtx.ExecutionID)
	s.Equal("test-app-id", resultCtx.AppID)
	s.True(resultCtx.AuthUser.IsAuthenticated())
	s.Equal("user-456", resultCtx.AuthUser.GetUserID())
	s.Equal("org-xyz", resultCtx.AuthUser.GetOUID())
	s.Equal("admin", resultCtx.AuthUser.GetUserType())
}

func (s *ModelTestSuite) TestToEngineContext_WithoutAuthUser() {
	// Setup: plain JSON content with no AuthUser field
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	userInputs := `{"username":"testuser"}`
	runtimeData := `{"key":"value"}`
	executionHistory := `{}`

	content := flowContextContent{
		AppID:            "test-app-id",
		Verbose:          true,
		GraphID:          "test-graph-id",
		UserInputs:       &userInputs,
		RuntimeData:      &runtimeData,
		ExecutionHistory: &executionHistory,
		AuthUser:         nil,
	}
	contextJSON, _ := json.Marshal(content)
	dbModel := &FlowContextDB{
		ExecutionID: "test-flow-id",
		Context:     string(contextJSON),
	}

	// Execute
	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)

	// Verify
	s.NoError(err)
	s.Equal("test-flow-id", resultCtx.ExecutionID)
	s.False(resultCtx.AuthUser.IsSet(), "AuthUser should be empty when not in content")
	s.False(resultCtx.AuthUser.IsAuthenticated())
	s.Equal("", resultCtx.AuthUser.GetUserID())
}

func (s *ModelTestSuite) TestAuthUserEncryptionDecryptionRoundTrip() {
	// Setup
	testUsers := []struct {
		userID   string
		userType string
		ouID     string
	}{
		{"user-simple", "", ""},
		{"user-with-type", "admin", ""},
		{"user-full", "member", "org-abc"},
	}

	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id").Maybe()
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication).Maybe()

	for _, tc := range testUsers {
		s.Run("UserID: "+tc.userID, func() {
			authUser := buildAuthUser(tc.userID, tc.userType, tc.ouID)

			ctx := EngineContext{
				Context:          context.Background(),
				ExecutionID:      "test-flow-id",
				AppID:            "test-app-id",
				FlowType:         common.FlowTypeAuthentication,
				AuthUser:         authUser,
				UserInputs:       map[string]string{},
				RuntimeData:      map[string]string{},
				ExecutionHistory: map[string]*common.NodeExecutionRecord{},
				Graph:            mockGraph,
			}

			// Convert to DB model (encrypts entire context)
			dbModel, err := FromEngineContext(ctx)
			s.NoError(err)

			// Verify context is encrypted
			s.Contains(dbModel.Context, `"ct"`, "context should be encrypted")

			// Decrypt and convert back to EngineContext
			s.NoError(dbModel.decrypt(context.Background()))
			resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)
			s.NoError(err)

			// Verify AuthUser identity fields are restored
			s.Equal(tc.userID, resultCtx.AuthUser.GetUserID())
			s.Equal(tc.ouID, resultCtx.AuthUser.GetOUID())
			s.Equal(tc.userType, resultCtx.AuthUser.GetUserType())
		})
	}
}

func (s *ModelTestSuite) TestDecrypt_WithInvalidCiphertext() {
	// Context is valid JSON with the encrypted envelope structure but has corrupted ciphertext.
	testCases := []struct {
		name    string
		context string
	}{
		{
			name:    "invalid base64 in ct field",
			context: `{"alg":"AES-GCM","ct":"not-valid-base64!!!","kid":"key-1"}`,
		},
		{
			name:    "empty ct field",
			context: `{"alg":"AES-GCM","ct":"","kid":"key-1"}`,
		},
		{
			name:    "ct too short to contain nonce",
			context: `{"alg":"AES-GCM","ct":"dGVzdA==","kid":"key-1"}`,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			dbModel := &FlowContextDB{ExecutionID: "test-flow-id", Context: tc.context}
			err := dbModel.decrypt(context.Background())
			s.Error(err)
		})
	}
}

func (s *ModelTestSuite) TestGetGraphID_WithDecryptionFailure() {
	// Verifies that GetGraphID propagates a decryption error when the context
	// looks encrypted but cannot be decrypted.
	dbModel := &FlowContextDB{
		ExecutionID: "test-flow-id",
		Context:     `{"alg":"AES-GCM","ct":"not-valid-base64!!!","kid":"key-1"}`,
	}

	graphID, err := dbModel.GetGraphID(context.Background())
	s.Error(err)
	s.Empty(graphID)
}

func (s *ModelTestSuite) TestToEngineContext_WithDecryptionFailure() {
	// Verifies that ToEngineContext propagates a decryption error when the context
	// looks encrypted but cannot be decrypted.
	mockGraph := coremock.NewGraphInterfaceMock(s.T())

	dbModel := &FlowContextDB{
		ExecutionID: "test-flow-id",
		Context:     `{"alg":"AES-GCM","ct":"not-valid-base64!!!","kid":"key-1"}`,
	}

	result, err := dbModel.ToEngineContext(context.Background(), mockGraph)
	s.Error(err)
	s.Equal(EngineContext{}, result)
}

func (s *ModelTestSuite) TestDecrypt_WithDecryptionFailure() {
	// Verifies that decrypt returns an error (and does not panic) when
	// the context looks encrypted but the ciphertext is invalid.
	dbModel := &FlowContextDB{
		ExecutionID: "test-flow-id",
		Context:     `{"alg":"AES-GCM","ct":"not-valid-base64!!!","kid":"key-1"}`,
	}

	s.True(dbModel.isEncrypted(), "context should be detected as encrypted before attempt")

	err := dbModel.decrypt(context.Background())
	s.Error(err)
	// Context should remain unchanged (still in its original encrypted form)
	s.True(dbModel.isEncrypted(), "context should remain encrypted after failed decryption")
}

func (s *ModelTestSuite) TestContextEncryptionRoundTrip() {
	// Tests that the entire context JSON is encrypted and all fields survive the round trip.
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
			var authUser managerpkg.AuthUser
			if tc.userID != "" {
				authUser = buildAuthUser(tc.userID, "", "")
			}

			ctx := EngineContext{
				ExecutionID:      "test-flow-id",
				AppID:            tc.appID,
				FlowType:         common.FlowTypeAuthentication,
				AuthUser:         authUser,
				UserInputs:       tc.inputs,
				RuntimeData:      tc.runtime,
				ExecutionHistory: map[string]*common.NodeExecutionRecord{},
				Graph:            mockGraph,
			}

			// Encrypt
			dbModel, err := FromEngineContext(ctx)
			s.NoError(err)

			// Verify context is encrypted: ciphertext envelope present, plain fields hidden
			s.Contains(dbModel.Context, `"ct"`, "encrypted context should contain ciphertext field")
			s.NotContains(dbModel.Context, tc.appID, "appId should not be visible in encrypted context")

			// Decrypt and verify all fields are restored
			err = dbModel.decrypt(context.Background())
			s.NoError(err)
			s.NotContains(dbModel.Context, `"ct"`, "decrypted context should not contain ciphertext field")
			s.Contains(dbModel.Context, `"appId"`, "decrypted context should expose plain appId field")

			resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)
			s.NoError(err)
			s.Equal(tc.appID, resultCtx.AppID)
			s.Equal(tc.userID, resultCtx.AuthUser.GetUserID())
			s.Equal(len(tc.inputs), len(resultCtx.UserInputs))
			s.Equal(len(tc.runtime), len(resultCtx.RuntimeData))
		})
	}
}

func (s *ModelTestSuite) TestIsEncrypted() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")

	ctx := EngineContext{
		ExecutionID:      "test-flow-id",
		AppID:            "test-app-id",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         managerpkg.AuthUser{},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)
	s.NoError(err)

	// Freshly created DB model should have encrypted context
	s.True(dbModel.isEncrypted(), "freshly encrypted context should be detected as encrypted")

	// After decryption, should no longer be detected as encrypted
	err = dbModel.decrypt(context.Background())
	s.NoError(err)
	s.False(dbModel.isEncrypted(), "decrypted context should not be detected as encrypted")

	// Plain JSON without "alg" field should not be detected as encrypted
	plainModel := &FlowContextDB{
		ExecutionID: "plain-id",
		Context:     `{"appId":"test","graphId":"graph-1","isAuthenticated":false}`,
	}
	s.False(plainModel.isEncrypted(), "plain JSON context should not be detected as encrypted")

	// Non-JSON string should not be detected as encrypted
	invalidModel := &FlowContextDB{ExecutionID: "invalid-id", Context: "not-json-at-all"}
	s.False(invalidModel.isEncrypted(), "non-JSON string should not be detected as encrypted")
}

func (s *ModelTestSuite) TestDecrypt_WithEncryptedContext() {
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("test-graph-id")
	mockGraph.On("GetType").Return(common.FlowTypeAuthentication)

	authUser := buildAuthUser("user-ensure", "", "")

	ctx := EngineContext{
		ExecutionID:      "test-flow-id",
		AppID:            "ensure-decrypt-app",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         authUser,
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}

	dbModel, err := FromEngineContext(ctx)
	s.NoError(err)
	s.True(dbModel.isEncrypted())

	err = dbModel.decrypt(context.Background())
	s.NoError(err)
	s.False(dbModel.isEncrypted(), "context should be decrypted after decrypt")

	// Verify context is usable after decrypt
	resultCtx, err := dbModel.ToEngineContext(context.Background(), mockGraph)
	s.NoError(err)
	s.Equal("ensure-decrypt-app", resultCtx.AppID)
	s.Equal("user-ensure", resultCtx.AuthUser.GetUserID())
}

func (s *ModelTestSuite) TestDecrypt_WithPlainContext() {
	// Plain JSON context should pass through decrypt unchanged
	plainJSON := `{"appId":"test-app","graphId":"graph-1","isAuthenticated":false}`
	dbModel := &FlowContextDB{ExecutionID: "test-flow-id", Context: plainJSON}

	s.False(dbModel.isEncrypted())
	originalContext := dbModel.Context

	err := dbModel.decrypt(context.Background())
	s.NoError(err)
	s.Equal(originalContext, dbModel.Context, "plain context should be unchanged by decrypt")
}

func (s *ModelTestSuite) TestFromEngineContext_PreservesOtherFields() {
	// Setup
	mockGraph := coremock.NewGraphInterfaceMock(s.T())
	mockGraph.On("GetID").Return("graph-123")

	authUser := buildAuthUser("user-abc", "admin", "org-xyz")
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
		AuthUser: authUser,
		ExecutionHistory: map[string]*common.NodeExecutionRecord{
			"node1": {NodeID: "node1"},
		},
		Graph: mockGraph,
	}

	// Execute
	dbModel, err := FromEngineContext(ctx)

	// Verify all fields are preserved
	s.NoError(err)
	s.Equal("flow-123", dbModel.ExecutionID)

	content := s.getContextContent(dbModel)
	s.Equal("app-123", content.AppID)
	s.True(content.Verbose)
	s.NotNil(content.CurrentAction)
	s.Equal(currentAction, *content.CurrentAction)
	s.Equal("graph-123", content.GraphID)
	s.NotNil(content.UserInputs)
	s.NotNil(content.RuntimeData)
	s.NotNil(content.ExecutionHistory)
	s.NotNil(content.AuthUser, "AuthUser should be serialized")

	// Verify AuthUser round-trips correctly
	var restoredAuthUser managerpkg.AuthUser
	err = json.Unmarshal([]byte(*content.AuthUser), &restoredAuthUser)
	s.NoError(err)
	s.Equal("user-abc", restoredAuthUser.GetUserID())
	s.Equal("org-xyz", restoredAuthUser.GetOUID())
	s.Equal("admin", restoredAuthUser.GetUserType())
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
