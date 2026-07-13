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

	"github.com/thunder-id/thunderid/tests/mocks/captchamock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type InterceptorRegistryTestSuite struct {
	suite.Suite
	registry InterceptorRegistryInterface
}

func TestInterceptorRegistrySuite(t *testing.T) {
	suite.Run(t, new(InterceptorRegistryTestSuite))
}

func (s *InterceptorRegistryTestSuite) SetupTest() {
	s.registry = newInterceptorRegistry()
}

// --- Registry ---

func (s *InterceptorRegistryTestSuite) TestNewRegistry_CreatesEmptyRegistry() {
	registry := newInterceptorRegistry()

	assert.NotNil(s.T(), registry)
	assert.False(s.T(), registry.IsRegistered("any"))
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptor_Successful() {
	ic := coremock.NewInterceptorInterfaceMock(s.T())
	ic.On("GetName").Return("TestIC").Maybe()

	s.registry.RegisterInterceptor(ic.GetName(), ic)

	assert.True(s.T(), s.registry.IsRegistered("TestIC"))
}

func (s *InterceptorRegistryTestSuite) TestRegisterInterceptor_Nil() {
	s.registry.RegisterInterceptor("", nil)

	assert.False(s.T(), s.registry.IsRegistered(""))
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptor_EmptyName() {
	ic := coremock.NewInterceptorInterfaceMock(s.T())
	ic.On("GetName").Return("").Maybe()

	s.registry.RegisterInterceptor(ic.GetName(), ic)

	assert.False(s.T(), s.registry.IsRegistered(""))
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptor_Duplicate() {
	ic1 := coremock.NewInterceptorInterfaceMock(s.T())
	ic1.On("GetName").Return("TestIC").Maybe()
	ic1.On("IsDefault").Return(true).Maybe()

	ic2 := coremock.NewInterceptorInterfaceMock(s.T())
	ic2.On("GetName").Return("TestIC").Maybe()
	ic2.On("IsDefault").Return(false).Maybe()

	s.registry.RegisterInterceptor(ic1.GetName(), ic1)
	s.registry.RegisterInterceptor(ic2.GetName(), ic2)

	// Should keep the first registration.
	result, err := s.registry.GetInterceptor("TestIC")
	assert.NoError(s.T(), err)
	assert.True(s.T(), result.IsDefault())
}

func (s *InterceptorRegistryTestSuite) TestGetInterceptor_Found() {
	ic := coremock.NewInterceptorInterfaceMock(s.T())
	ic.On("GetName").Return("TestIC").Maybe()

	s.registry.RegisterInterceptor(ic.GetName(), ic)

	result, err := s.registry.GetInterceptor("TestIC")

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "TestIC", result.GetName())
}

func (s *InterceptorRegistryTestSuite) TestGetInterceptor_NotFound() {
	_, err := s.registry.GetInterceptor("NonExistent")

	assert.Error(s.T(), err)
}

// --- registerInterceptors ---

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_EmptyList_RegistersAllBuiltIn() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerInterceptors(deps, registry, []string{})

	assert.NoError(s.T(), err)
	assert.True(s.T(), registry.IsRegistered(ChallengeTokenInterceptor))
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_SpecificName_RegistersOnlyThat() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerInterceptors(deps, registry, []string{ChallengeTokenInterceptor})

	assert.NoError(s.T(), err)
	assert.True(s.T(), registry.IsRegistered(ChallengeTokenInterceptor))
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_AlreadyRegistered_Skips() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerInterceptors(deps, registry, []string{ChallengeTokenInterceptor})
	assert.NoError(s.T(), err)

	err = registerInterceptors(deps, registry, []string{ChallengeTokenInterceptor})
	assert.NoError(s.T(), err)
	assert.True(s.T(), registry.IsRegistered(ChallengeTokenInterceptor))
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_UnknownName_ReturnsError() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerInterceptors(deps, registry, []string{"NonExistentInterceptor"})

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "unknown interceptor")
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_EmptyName_ReturnsError() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerInterceptors(deps, registry, []string{""})

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "empty interceptor name")
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_DuplicateName_ReturnsError() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerInterceptors(deps, registry, []string{
		ChallengeTokenInterceptor,
		ChallengeTokenInterceptor,
	})

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "duplicate interceptor name")
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_WhitespaceOnlyName_ReturnsError() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerInterceptors(deps, registry, []string{"   "})

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "empty interceptor name")
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_NilFlowFactory_ReturnsError() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: nil}

	err := registerInterceptors(deps, registry, []string{ChallengeTokenInterceptor})

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to register interceptor")
}

func (s *InterceptorRegistryTestSuite) TestregisterInterceptors_NameWithWhitespace_Trimmed() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerInterceptors(deps, registry, []string{"  " + ChallengeTokenInterceptor + "  "})

	assert.NoError(s.T(), err)
	assert.True(s.T(), registry.IsRegistered(ChallengeTokenInterceptor))
}

// --- sanitizeAndValidate ---

func (s *InterceptorRegistryTestSuite) TestSanitizeAndValidate_ValidNames_ReturnsAll() {
	result, err := sanitizeAndValidate([]string{ChallengeTokenInterceptor})

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), []string{ChallengeTokenInterceptor}, result)
}

func (s *InterceptorRegistryTestSuite) TestSanitizeAndValidate_EmptyName_ReturnsError() {
	_, err := sanitizeAndValidate([]string{""})

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "empty interceptor name")
}

func (s *InterceptorRegistryTestSuite) TestSanitizeAndValidate_DuplicateName_ReturnsError() {
	_, err := sanitizeAndValidate([]string{ChallengeTokenInterceptor, ChallengeTokenInterceptor})

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "duplicate interceptor name")
}

func (s *InterceptorRegistryTestSuite) TestSanitizeAndValidate_UnknownName_ReturnsError() {
	_, err := sanitizeAndValidate([]string{"UnknownInterceptor"})

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "unknown interceptor")
}

func (s *InterceptorRegistryTestSuite) TestSanitizeAndValidate_TrimsWhitespace() {
	result, err := sanitizeAndValidate([]string{"  " + ChallengeTokenInterceptor + "  "})

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), []string{ChallengeTokenInterceptor}, result)
}

func (s *InterceptorRegistryTestSuite) TestSanitizeAndValidate_EmptySlice_ReturnsEmpty() {
	result, err := sanitizeAndValidate([]string{})

	assert.NoError(s.T(), err)
	assert.Empty(s.T(), result)
}

// --- registerChallengeTokenInterceptor ---

func (s *InterceptorRegistryTestSuite) TestRegisterChallengeToken_NilFlowFactory_ReturnsError() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: nil}

	err := registerChallengeTokenInterceptor(deps, registry)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "FlowFactory dependency is required")
}

func (s *InterceptorRegistryTestSuite) TestRegisterChallengeToken_ValidDeps_Registers() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newMockFlowFactory(s.T())}

	err := registerChallengeTokenInterceptor(deps, registry)

	assert.NoError(s.T(), err)
	assert.True(s.T(), registry.IsRegistered(ChallengeTokenInterceptor))
}

// --- registerCaptchaInterceptor ---

func (s *InterceptorRegistryTestSuite) TestRegisterCaptcha_NilFlowFactory_Skips() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: nil}

	err := registerCaptchaInterceptor(deps, registry)

	assert.NoError(s.T(), err)
	assert.False(s.T(), registry.IsRegistered(CaptchaInterceptor))
}

func (s *InterceptorRegistryTestSuite) TestRegisterCaptcha_NilCaptchaService_Skips() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{FlowFactory: newCaptchaMockFlowFactory(s.T())}

	err := registerCaptchaInterceptor(deps, registry)

	assert.NoError(s.T(), err)
	assert.False(s.T(), registry.IsRegistered(CaptchaInterceptor))
}

func (s *InterceptorRegistryTestSuite) TestRegisterCaptcha_ValidDeps_Registers() {
	registry := newInterceptorRegistry()
	deps := InterceptorDependencies{
		FlowFactory:    newCaptchaMockFlowFactory(s.T()),
		CaptchaService: captchamock.NewCaptchaValidationProviderMock(s.T()),
	}

	err := registerCaptchaInterceptor(deps, registry)

	assert.NoError(s.T(), err)
	assert.True(s.T(), registry.IsRegistered(CaptchaInterceptor))
}
