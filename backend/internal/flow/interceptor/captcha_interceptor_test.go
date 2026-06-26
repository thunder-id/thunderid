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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/captcha"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/captchamock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type CaptchaInterceptorTestSuite struct {
	suite.Suite
	captchaService *captchamock.CaptchaServiceInterfaceMock
	interceptor    *captchaInterceptor
}

func TestCaptchaInterceptorSuite(t *testing.T) {
	suite.Run(t, new(CaptchaInterceptorTestSuite))
}

func (s *CaptchaInterceptorTestSuite) SetupTest() {
	s.captchaService = captchamock.NewCaptchaServiceInterfaceMock(s.T())
	s.interceptor = newCaptchaInterceptor(newCaptchaMockFlowFactory(s.T()), s.captchaService)
}

// --- Execute mode guard ---

func (s *CaptchaInterceptorTestSuite) TestExecute_NonPreNodeMode_FailsWithoutServiceCall() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		UserInputs:  map[string]string{defaultCaptchaFieldKey: "token"},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFailure, result.Status)
	s.captchaService.AssertNotCalled(s.T(), "Verify", mock.Anything, mock.Anything)
}

// --- Token presence ---

func (s *CaptchaInterceptorTestSuite) TestExecute_MissingToken_Passes() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		UserInputs:  map[string]string{},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
	s.captchaService.AssertNotCalled(s.T(), "Verify", mock.Anything, mock.Anything)
}

func (s *CaptchaInterceptorTestSuite) TestExecute_EmptyToken_Passes() {
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		UserInputs:  map[string]string{defaultCaptchaFieldKey: ""},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
	s.captchaService.AssertNotCalled(s.T(), "Verify", mock.Anything, mock.Anything)
}

// --- Verification verdicts ---

func (s *CaptchaInterceptorTestSuite) TestExecute_ValidToken_Passes() {
	s.captchaService.On("Verify", mock.Anything, "valid-token").
		Return(&captcha.VerificationResult{Success: true}, nil)
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		UserInputs:  map[string]string{defaultCaptchaFieldKey: "valid-token"},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *CaptchaInterceptorTestSuite) TestExecute_NegativeVerdict_Fails() {
	s.captchaService.On("Verify", mock.Anything, "bad-token").
		Return(&captcha.VerificationResult{Success: false}, nil)
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		UserInputs:  map[string]string{defaultCaptchaFieldKey: "bad-token"},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFailure, result.Status)
	assert.Equal(s.T(), &ErrorCaptchaInvalid, result.Error)
}

func (s *CaptchaInterceptorTestSuite) TestExecute_NilResult_Fails() {
	s.captchaService.On("Verify", mock.Anything, "bad-token").Return(nil, nil)
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		UserInputs:  map[string]string{defaultCaptchaFieldKey: "bad-token"},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusFailure, result.Status)
	assert.Equal(s.T(), &ErrorCaptchaInvalid, result.Error)
}

func (s *CaptchaInterceptorTestSuite) TestExecute_OperationalError_ReturnsError() {
	s.captchaService.On("Verify", mock.Anything, "any-token").
		Return(nil, &tidcommon.InternalServerError)
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		UserInputs:  map[string]string{defaultCaptchaFieldKey: "any-token"},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
}

// --- Field key resolution ---

func (s *CaptchaInterceptorTestSuite) TestExecute_CustomFieldKeyFromProperties_Passes() {
	s.captchaService.On("Verify", mock.Anything, "custom-token").
		Return(&captcha.VerificationResult{Success: true}, nil)
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		Properties:  map[string]interface{}{propertyKeyFieldKey: "my_captcha"},
		UserInputs:  map[string]string{"my_captcha": "custom-token"},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

func (s *CaptchaInterceptorTestSuite) TestExecute_NonStringFieldKeyProperty_FallsBackToDefault() {
	s.captchaService.On("Verify", mock.Anything, "default-token").
		Return(&captcha.VerificationResult{Success: true}, nil)
	ctx := &core.InterceptorContext{
		Mode:        providers.InterceptorModePreNode,
		ExecutionID: "exec-1",
		Properties:  map[string]interface{}{propertyKeyFieldKey: 123},
		UserInputs:  map[string]string{defaultCaptchaFieldKey: "default-token"},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
}

// --- Mock helpers ---

func newCaptchaMockFlowFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *coremock.FlowFactoryInterfaceMock {
	factoryMock := coremock.NewFlowFactoryInterfaceMock(t)

	baseMock := coremock.NewInterceptorInterfaceMock(t)
	baseMock.On("GetName").Return(CaptchaInterceptor).Maybe()
	baseMock.On("IsDefault").Return(false).Maybe()
	baseMock.On("GetPriority").Return(BasePriorityConfigurable).Maybe()
	factoryMock.On("CreateInterceptor", CaptchaInterceptor, false, BasePriorityConfigurable).
		Return(baseMock).Maybe()

	return factoryMock
}
