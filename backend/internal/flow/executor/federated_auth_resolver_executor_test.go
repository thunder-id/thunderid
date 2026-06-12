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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package executor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type FederatedAuthResolverTestSuite struct {
	suite.Suite
	mockFlowFactory   *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerInterfaceMock
	executor          *federatedAuthResolverExecutor
}

func TestFederatedAuthResolverSuite(t *testing.T) {
	suite.Run(t, new(FederatedAuthResolverTestSuite))
}

func newFederatedAuthResolverAuthenticatedUser() authnprovidermgr.AuthUser {
	var authUser authnprovidermgr.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`))
	return authUser
}

func (suite *FederatedAuthResolverTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())

	mockExec := createMockExecutor(suite.T(), ExecutorNameFederatedAuthResolver,
		common.ExecutorTypeAuthentication)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameFederatedAuthResolver,
		common.ExecutorTypeAuthentication,
		([]common.Input)(nil), ([]common.Input)(nil)).Return(mockExec)

	suite.executor = newFederatedAuthResolverExecutor(suite.mockFlowFactory, suite.mockAuthnProvider)
}

func (suite *FederatedAuthResolverTestSuite) TestNewFederatedAuthResolverExecutor() {
	assert.NotNil(suite.T(), suite.executor)
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_SingleCandidateMatch() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", OUID: "ou-1", OUHandle: "org-alpha", Type: "Customer"},
		{ID: "user-2", OUID: "ou-2", OUHandle: "org-beta", Type: "Customer"},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"ouHandle": "org-alpha"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
			"sub":                           "sub-123",
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "ouHandle", Type: "TEXT_INPUT", Required: true},
	})

	authenticatedAuthUser := newFederatedAuthResolverAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUserID: "user-1",
	}, map[string]interface{}{
		userAttributeSub: "sub-123",
	}, mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, authnprovidercm.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_NoCandidatesInRuntimeData() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"ouHandle": "org-alpha"},
		RuntimeData: map[string]string{},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
}

func (suite *FederatedAuthResolverTestSuite) executeWithCandidatesAndInput(
	candidates []*entityprovider.Entity, inputs map[string]string) (*common.ExecutorResponse, error) {
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  inputs,
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "ouHandle", Type: "TEXT_INPUT", Required: true},
	})

	return suite.executor.Execute(ctx)
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_NoMatchingCandidate() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", OUID: "ou-1", OUHandle: "org-alpha", Type: "Customer"},
		{ID: "user-2", OUID: "ou-2", OUHandle: "org-beta", Type: "Customer"},
	}

	resp, err := suite.executeWithCandidatesAndInput(candidates, map[string]string{"ouHandle": "org-gamma"})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrUserNotFound.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_MultipleCandidatesStillAmbiguous() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", OUID: "ou-1", OUHandle: "org-alpha", Type: "Customer"},
		{ID: "user-2", OUID: "ou-1", OUHandle: "org-alpha", Type: "Admin"},
	}

	resp, err := suite.executeWithCandidatesAndInput(candidates, map[string]string{"ouHandle": "org-alpha"})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.NotNil(suite.T(), resp.ForwardedData)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyCandidateUsers])
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_IndistinguishableCandidates() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", OUID: "ou-1", OUHandle: "org-alpha", Type: "Customer"},
		{ID: "user-2", OUID: "ou-1", OUHandle: "org-alpha", Type: "Customer"},
	}

	resp, err := suite.executeWithCandidatesAndInput(candidates, map[string]string{"ouHandle": "org-alpha"})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrFailedToIdentifyUser.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_RequiredInputsMissing() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_InvalidCandidatesJSON() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"ouHandle": "org-alpha"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: "invalid-json",
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_PassesSubToAuthnProvider() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", OUID: "ou-1", OUHandle: "org-alpha", Type: "Customer"},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"ouHandle": "org-alpha"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
			"sub":                           "federated-sub-123",
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "ouHandle", Type: "TEXT_INPUT", Required: true},
	})

	authenticatedAuthUser := newFederatedAuthResolverAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUserID: "user-1",
	}, map[string]interface{}{
		userAttributeSub: "federated-sub-123",
	}, mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, authnprovidercm.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_FailsWhenAuthnProviderReturnsError() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", OUID: "ou-1", OUHandle: "org-alpha", Type: "Customer"},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"ouHandle": "org-alpha"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
			"sub":                           "federated-sub-123",
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "ouHandle", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUserID: "user-1",
	}, map[string]interface{}{
		userAttributeSub: "federated-sub-123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(
		authnprovidermgr.AuthUser{}, (authnprovidercm.AuthenticatedClaims)(nil), &serviceerror.ServiceError{
			Code: "authentication_failed",
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserNotAuthenticated.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *FederatedAuthResolverTestSuite) TestExecute_IgnoresUnexpectedInputKeys() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", OUID: "ou-1", OUHandle: "org-alpha", Type: "Customer"},
		{ID: "user-2", OUID: "ou-2", OUHandle: "org-beta", Type: "Admin"},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	// Malicious client sends userID as an extra input
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs: map[string]string{
			"ouHandle": "org-alpha",
			"userID":   "user-2", // should be ignored
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
			"sub":                           "federated-sub-123",
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "ouHandle", Type: "TEXT_INPUT", Required: true},
	})

	authenticatedAuthUser := newFederatedAuthResolverAuthenticatedUser()
	// Should match user-1 (org-alpha), not user-2 despite userID injection
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUserID: "user-1",
	}, map[string]interface{}{
		userAttributeSub: "federated-sub-123",
	}, mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, authnprovidercm.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}
