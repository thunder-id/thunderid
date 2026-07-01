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

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type ChallengeTokenInterceptorTestSuite struct {
	suite.Suite
	interceptor *challengeTokenInterceptor
}

func TestChallengeTokenInterceptorSuite(t *testing.T) {
	suite.Run(t, new(ChallengeTokenInterceptorTestSuite))
}

func (s *ChallengeTokenInterceptorTestSuite) SetupTest() {
	reg, err := Initialize(InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}, engineconfig.FlowConfig{})
	assert.NoError(s.T(), err)
	ic, err := reg.GetInterceptor(ChallengeTokenInterceptor)
	assert.NoError(s.T(), err)
	s.interceptor = ic.(*challengeTokenInterceptor)
}

// --- Execute mode dispatch ---

func (s *ChallengeTokenInterceptorTestSuite) TestExecute_PreRequestMode_ValidatesToken() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestExecute_PostRequestMode_RotatesToken() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePostRequest,
		ExecutionID: "exec-1",
		FlowStatus:  providers.FlowStatusIncomplete,
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
	assert.NotEmpty(s.T(), result.ChallengeToken)
}

func (s *ChallengeTokenInterceptorTestSuite) TestExecute_UnknownMode_ReturnsFail() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFailure, result.Status)
}

// --- validateChallengeToken ---

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_NoStoredHash_Passes() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_EmptyIncomingToken_Fails() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFailure, result.Status)
	assert.Equal(s.T(), &ErrorChallengeTokenInvalid, result.Error)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_InvalidToken_Fails() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash:           cryptolib.HashToken("correct-token"),
			common.InterceptorDataKeyChallengeTokenIn: "wrong-token",
		},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFailure, result.Status)
	assert.Equal(s.T(), &ErrorChallengeTokenInvalid, result.Error)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_ValidToken_Passes() {
	token := "valid-token"
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreRequest,
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
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
		ExecutionPolicy: &providers.ExecutionPolicy{SkipChallengeValidation: true},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_AllowSegmentRestart_Passes() {
	ctx := &core.InterceptorContext{
		Mode:                providers.InterceptorModePreRequest,
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
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
		ExecutionPolicy: nil,
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFailure, result.Status)
}

func (s *ChallengeTokenInterceptorTestSuite) TestValidate_PolicyWithoutSkipFlag_DoesNotSkip() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		SharedData: map[string]string{
			sharedDataKeyChallengeTokenHash: cryptolib.HashToken("some-token"),
		},
		ExecutionPolicy: &providers.ExecutionPolicy{SkipChallengeValidation: false},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFailure, result.Status)
}

// --- rotateChallengeToken ---

func (s *ChallengeTokenInterceptorTestSuite) TestRotate_GeneratesNewToken() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePostRequest,
		ExecutionID: "exec-1",
		FlowStatus:  providers.FlowStatusIncomplete,
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
		Mode:        providers.InterceptorModePostRequest,
		ExecutionID: "exec-1",
		FlowStatus:  providers.FlowStatusIncomplete,
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
		Mode:        providers.InterceptorModePostRequest,
		ExecutionID: "exec-1",
		FlowStatus:  providers.FlowStatusIncomplete,
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

// --- Mock helpers ---

func newMockFlowFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *coremock.FlowFactoryInterfaceMock {
	factoryMock := coremock.NewFlowFactoryInterfaceMock(t)

	// CreateInterceptor is called by newChallengeTokenInterceptor.
	baseMock := coremock.NewInterceptorInterfaceMock(t)
	baseMock.On("GetName").Return(ChallengeTokenInterceptor).Maybe()
	baseMock.On("IsDefault").Return(true).Maybe()
	baseMock.On("GetPriority").Return(PriorityDefault).Maybe()
	factoryMock.On("CreateInterceptor", ChallengeTokenInterceptor, true, PriorityDefault).Return(baseMock).Maybe()

	// CreateInterceptorUnit is called by initDefaultInterceptorUnits.
	preUnit := coremock.NewInterceptorUnitInterfaceMock(t)
	preUnit.On("GetName").Return(ChallengeTokenInterceptor).Maybe()
	preUnit.On("GetMode").Return(providers.InterceptorModePreRequest).Maybe()
	preUnit.On("Clone").Return(preUnit).Maybe()
	preUnit.On("GetInterceptor").Return(nil).Maybe()
	preUnit.On("GetScope").Return(providers.InterceptorScope("")).Maybe()
	preUnit.On("GetApplyTo").Return([]string(nil)).Maybe()
	preUnit.On("SetInterceptor", mock.Anything).Return().Maybe()

	postUnit := coremock.NewInterceptorUnitInterfaceMock(t)
	postUnit.On("GetName").Return(ChallengeTokenInterceptor).Maybe()
	postUnit.On("GetMode").Return(providers.InterceptorModePostRequest).Maybe()
	postUnit.On("Clone").Return(postUnit).Maybe()
	postUnit.On("GetInterceptor").Return(nil).Maybe()
	postUnit.On("GetScope").Return(providers.InterceptorScope("")).Maybe()
	postUnit.On("GetApplyTo").Return([]string(nil)).Maybe()
	postUnit.On("SetInterceptor", mock.Anything).Return().Maybe()

	factoryMock.On("CreateInterceptorUnit",
		ChallengeTokenInterceptor, providers.InterceptorModePreRequest,
		providers.InterceptorScope(""), []string(nil), map[string]interface{}(nil)).Return(preUnit).Maybe()
	factoryMock.On("CreateInterceptorUnit",
		ChallengeTokenInterceptor, providers.InterceptorModePostRequest,
		providers.InterceptorScope(""), []string(nil), map[string]interface{}(nil)).Return(postUnit).Maybe()

	return factoryMock
}
