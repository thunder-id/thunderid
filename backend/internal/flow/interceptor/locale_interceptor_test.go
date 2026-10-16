// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/i18nprovidermock"
)

type LocaleInterceptorTestSuite struct {
	suite.Suite
	i18nService *i18nprovidermock.I18nProviderMock
	interceptor *localeInterceptor
}

func TestLocaleInterceptorSuite(t *testing.T) {
	suite.Run(t, new(LocaleInterceptorTestSuite))
}

func (s *LocaleInterceptorTestSuite) SetupTest() {
	s.i18nService = i18nprovidermock.NewI18nProviderMock(s.T())
	// Supported languages; .Maybe() so the mode-guard test need not call it.
	s.i18nService.EXPECT().ListLanguages(mock.Anything).Return([]string{"en-US", "es-AR"}, nil).Maybe()
	s.interceptor = newLocaleInterceptor(newLocaleMockFlowFactory(s.T()), s.i18nService)
}

func (s *LocaleInterceptorTestSuite) TestExecute_NonPreRequestMode_NoOutput() {
	ctx := &core.InterceptorContext{
		Mode:       providers.InterceptorModePostRequest,
		UserInputs: map[string]string{common.InputKeyCurrentLocale: "es-AR"},
	}

	result, err := s.interceptor.Execute(ctx)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), common.InterceptorStatusComplete, result.Status)
	assert.Empty(s.T(), result.SharedRuntimeOutputs)
}

func (s *LocaleInterceptorTestSuite) TestExecute_SupportedLocale_UpdatesSharedData() {
	result, err := s.interceptor.Execute(s.preRequestCtx(map[string]string{common.InputKeyCurrentLocale: "es-AR"}))

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "es-AR", result.SharedRuntimeOutputs[common.RuntimeKeyEffectiveLocale])
}

func (s *LocaleInterceptorTestSuite) TestExecute_CanonicalisesLocale() {
	result, err := s.interceptor.Execute(s.preRequestCtx(map[string]string{common.InputKeyCurrentLocale: "en-us"}))

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "en-US", result.SharedRuntimeOutputs[common.RuntimeKeyEffectiveLocale])
}

func (s *LocaleInterceptorTestSuite) TestExecute_UnsupportedLocale_NoOutput() {
	result, err := s.interceptor.Execute(s.preRequestCtx(map[string]string{common.InputKeyCurrentLocale: "fr-FR"}))

	assert.NoError(s.T(), err)
	assert.Empty(s.T(), result.SharedRuntimeOutputs, "an unsupported tag must not update the stored locale")
}

func (s *LocaleInterceptorTestSuite) TestExecute_NoLocale_NoOutput() {
	result, err := s.interceptor.Execute(s.preRequestCtx(map[string]string{"username": "alice"}))

	assert.NoError(s.T(), err)
	assert.Empty(s.T(), result.SharedRuntimeOutputs, "no fresh locale must leave the stored value unchanged")
}

func (s *LocaleInterceptorTestSuite) TestExecute_FormLocaleWinsOverCurrent() {
	result, err := s.interceptor.Execute(s.preRequestCtx(map[string]string{
		common.InputKeyFormLocale:    "es-AR",
		common.InputKeyCurrentLocale: "en-US",
	}))

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "es-AR", result.SharedRuntimeOutputs[common.RuntimeKeyEffectiveLocale])
}

func (s *LocaleInterceptorTestSuite) TestExecute_UnsupportedFormLocale_FallsBackToCurrent() {
	result, err := s.interceptor.Execute(s.preRequestCtx(map[string]string{
		common.InputKeyFormLocale:    "fr-FR", // not supported
		common.InputKeyCurrentLocale: "en-US",
	}))

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "en-US", result.SharedRuntimeOutputs[common.RuntimeKeyEffectiveLocale])
}

func (s *LocaleInterceptorTestSuite) preRequestCtx(inputs map[string]string) *core.InterceptorContext {
	return &core.InterceptorContext{
		Context:     context.Background(),
		Mode:        providers.InterceptorModePreRequest,
		ExecutionID: "exec-1",
		UserInputs:  inputs,
	}
}

func newLocaleMockFlowFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *coremock.FlowFactoryInterfaceMock {
	factoryMock := coremock.NewFlowFactoryInterfaceMock(t)

	baseMock := coremock.NewInterceptorInterfaceMock(t)
	baseMock.On("GetName").Return(LocaleInterceptor).Maybe()
	baseMock.On("IsDefault").Return(true).Maybe()
	baseMock.On("GetPriority").Return(PriorityDefault).Maybe()
	factoryMock.On("CreateInterceptor", LocaleInterceptor, true, PriorityDefault).
		Return(baseMock).Maybe()

	return factoryMock
}
