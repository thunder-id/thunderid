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

package interceptor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type ChallengeTokenInterceptorTestSuite struct {
	suite.Suite
	interceptor *challengeTokenInterceptor
}

func TestChallengeTokenInterceptorSuite(t *testing.T) {
	suite.Run(t, new(ChallengeTokenInterceptorTestSuite))
}

func (s *ChallengeTokenInterceptorTestSuite) SetupTest() {
	reg, err := Initialize(InterceptorDependencies{FlowFactory: &stubFlowFactory{}}, config.FlowConfig{})
	assert.NoError(s.T(), err)
	ic, err := reg.GetInterceptor(ChallengeTokenInterceptor)
	assert.NoError(s.T(), err)
	s.interceptor = ic.(*challengeTokenInterceptor)
}

// --- Execute mode dispatch ---

func (s *ChallengeTokenInterceptorTestSuite) TestExecute_PreRequestMode_ValidatesToken() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestExecute_PostRequestMode_RotatesToken() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePostRequest,
		ExecutionID: "exec-1",
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
	assert.NotEmpty(s.T(), result.ChallengeToken)
}

func (s *ChallengeTokenInterceptorTestSuite) TestExecute_UnknownMode_ReturnsFail() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreNode,
		ExecutionID: "exec-1",
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFail, result.Status)
}

// --- validateChallengeToken ---

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_NoStoredHash_Passes() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_EmptyIncomingToken_Fails() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFail, result.Status)
	assert.Equal(s.T(), &ErrorChallengeTokenInvalid, result.Error)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_InvalidToken_Fails() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash:           cryptolib.HashToken("correct-token"),
			common.InterceptorDataKeyChallengeTokenIn: "wrong-token",
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFail, result.Status)
	assert.Equal(s.T(), &ErrorChallengeTokenInvalid, result.Error)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_ValidToken_Passes() {
	token := "valid-token"
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash:           cryptolib.HashToken(token),
			common.InterceptorDataKeyChallengeTokenIn: token,
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

// --- shouldSkipValidation ---

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_SkipChallengeValidationPolicy_Passes() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
		CurrentNode: &challengeTokenStubNode{
			executionPolicy: &core.ExecutionPolicy{SkipChallengeValidation: true},
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_AllowSegmentRestart_Passes() {
	ctx := &core.InterceptorContext{
		Mode:                common.InterceptorModePreRequest,
		ExecutionID:         "exec-1",
		AllowSegmentRestart: true,
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_NodeWithNilPolicy_DoesNotSkip() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
		CurrentNode: &challengeTokenStubNode{
			executionPolicy: nil,
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFail, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_PolicyWithoutSkipFlag_DoesNotSkip() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
		CurrentNode: &challengeTokenStubNode{
			executionPolicy: &core.ExecutionPolicy{SkipChallengeValidation: false},
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFail, result.Status)
}

// --- rotateChallengeToken ---

func (s *ChallengeTokenInterceptorTestSuite) TestRotate_GeneratesNewToken() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePostRequest,
		ExecutionID: "exec-1",
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)

	newToken := result.ChallengeToken
	assert.NotEmpty(s.T(), newToken)

	storedHash := ctx.SharedData[sharedDataKeyChallengeTokenHash]
	assert.NotEmpty(s.T(), storedHash)
	assert.True(s.T(), cryptolib.ValidateTokenHash(newToken, storedHash))
}

func (s *ChallengeTokenInterceptorTestSuite) TestRotate_ClearsIncomingToken() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePostRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			common.InterceptorDataKeyChallengeTokenIn: "old-incoming-token",
		},
	}

	_, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Empty(s.T(), ctx.SharedData[common.InterceptorDataKeyChallengeTokenIn])
}

func (s *ChallengeTokenInterceptorTestSuite) TestRotate_OverwritesPreviousHash() {
	ctx := &core.InterceptorContext{
		Mode:        common.InterceptorModePostRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: "old-hash",
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.NotEqual(s.T(), "old-hash", ctx.SharedData[sharedDataKeyChallengeTokenHash])

	newToken := result.ChallengeToken
	assert.True(s.T(), cryptolib.ValidateTokenHash(newToken, ctx.SharedData[sharedDataKeyChallengeTokenHash]))
}

// --- Stubs ---

// stubFlowFactory implements core.FlowFactoryInterface for testing.
type stubFlowFactory struct{}

var _ core.FlowFactoryInterface = (*stubFlowFactory)(nil)

func (f *stubFlowFactory) CreateInterceptor(name string, isDefault bool,
	priority int) core.InterceptorInterface {
	return &stubInterceptor{name: name, isDefault: isDefault, priority: priority}
}

func (f *stubFlowFactory) CreateNode(string, string, map[string]interface{}, bool, bool) (core.NodeInterface, error) {
	return nil, nil
}

func (f *stubFlowFactory) CreateGraph(string, common.FlowType) core.GraphInterface { return nil }

func (f *stubFlowFactory) CreateExecutor(
	string, common.ExecutorType, []common.Input, []common.Input) core.ExecutorInterface {
	return nil
}

func (f *stubFlowFactory) CreateInterceptorUnit(name string, mode common.InterceptorMode,
	scope common.InterceptorScope, applyTo []string,
	properties map[string]interface{}) core.InterceptorUnitInterface {
	return &stubInterceptorUnit{name: name, mode: mode}
}

func (f *stubFlowFactory) CloneNode(core.NodeInterface) (core.NodeInterface, error) { return nil, nil }

func (f *stubFlowFactory) CloneNodes(map[string]core.NodeInterface) (map[string]core.NodeInterface, error) {
	return nil, nil
}

// stubInterceptorUnit implements core.InterceptorUnitInterface for testing.
type stubInterceptorUnit struct {
	name string
	mode common.InterceptorMode
}

var _ core.InterceptorUnitInterface = (*stubInterceptorUnit)(nil)

func (u *stubInterceptorUnit) GetName() string                            { return u.name }
func (u *stubInterceptorUnit) GetMode() common.InterceptorMode            { return u.mode }
func (u *stubInterceptorUnit) GetScope() common.InterceptorScope          { return "" }
func (u *stubInterceptorUnit) GetApplyTo() []string                       { return nil }
func (u *stubInterceptorUnit) GetProperties() map[string]interface{}      { return nil }
func (u *stubInterceptorUnit) GetInterceptor() core.InterceptorInterface  { return nil }
func (u *stubInterceptorUnit) SetName(_ string)                           {}
func (u *stubInterceptorUnit) SetMode(_ common.InterceptorMode)           {}
func (u *stubInterceptorUnit) SetScope(_ common.InterceptorScope)         {}
func (u *stubInterceptorUnit) SetApplyTo(_ []string)                      {}
func (u *stubInterceptorUnit) SetProperties(_ map[string]interface{})     {}
func (u *stubInterceptorUnit) SetInterceptor(_ core.InterceptorInterface) {}

// challengeTokenStubNode implements core.NodeInterface for challenge token tests.
type challengeTokenStubNode struct {
	executionPolicy *core.ExecutionPolicy
}

var _ core.NodeInterface = (*challengeTokenStubNode)(nil)

func (n *challengeTokenStubNode) GetID() string                         { return "stub-node" }
func (n *challengeTokenStubNode) GetType() common.NodeType              { return common.NodeTypeTaskExecution }
func (n *challengeTokenStubNode) GetProperties() map[string]interface{} { return nil }
func (n *challengeTokenStubNode) Execute(_ *core.NodeContext) (*common.NodeResponse, *serviceerror.ServiceError) {
	return nil, nil
}
func (n *challengeTokenStubNode) ShouldExecute(_ *core.NodeContext) bool    { return true }
func (n *challengeTokenStubNode) SetCondition(_ *core.NodeCondition)        {}
func (n *challengeTokenStubNode) GetCondition() *core.NodeCondition         { return nil }
func (n *challengeTokenStubNode) IsStartNode() bool                         { return false }
func (n *challengeTokenStubNode) SetAsStartNode()                           {}
func (n *challengeTokenStubNode) IsFinalNode() bool                         { return false }
func (n *challengeTokenStubNode) SetAsFinalNode()                           {}
func (n *challengeTokenStubNode) GetNextNodeList() []string                 { return nil }
func (n *challengeTokenStubNode) SetNextNodeList(_ []string)                {}
func (n *challengeTokenStubNode) AddNextNode(_ string)                      {}
func (n *challengeTokenStubNode) RemoveNextNode(_ string)                   {}
func (n *challengeTokenStubNode) GetPreviousNodeList() []string             { return nil }
func (n *challengeTokenStubNode) SetPreviousNodeList(_ []string)            {}
func (n *challengeTokenStubNode) AddPreviousNode(_ string)                  {}
func (n *challengeTokenStubNode) RemovePreviousNode(_ string)               {}
func (n *challengeTokenStubNode) GetExecutionPolicy() *core.ExecutionPolicy { return n.executionPolicy }
