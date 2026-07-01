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
	"fmt"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/interceptormock"
)

const testSharedDataValue = "hello"

type InterceptorRunnerTestSuite struct {
	suite.Suite
	registry *interceptormock.InterceptorRegistryInterfaceMock
	service  InterceptorRunnerInterface
}

func TestInterceptorRunnerSuite(t *testing.T) {
	suite.Run(t, new(InterceptorRunnerTestSuite))
}

func (s *InterceptorRunnerTestSuite) SetupTest() {
	s.registry = interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	s.service = newInterceptorRunner(s.registry)
}

// --- RunInterceptors tests ---

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_NoDeclarations() {
	execCtx := &InterceptorRunnerContext{
		Ctx:        context.Background(),
		SharedData: map[string]string{},
	}

	resp, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), common.InterceptorStatusComplete, resp.Status)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_BindingResolvedAndExecuted() {
	executed := false
	icMock := newTestInterceptorMock(s.T(), "ConfigIC", false, 200)
	icMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) { executed = true }).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	s.registry.On("GetInterceptor", "ConfigIC").Return(icMock, nil)

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "ConfigIC", providers.InterceptorModePreRequest,
				providers.InterceptorScopeAll, nil),
		},
		SharedData: map[string]string{},
	}

	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.True(s.T(), executed)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_FailHaltsExecution() {
	callOrder := []string{}

	firstMock := newTestInterceptorMock(s.T(), "First", false, 100)
	firstMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) { callOrder = append(callOrder, "First") }).
		Return(&common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
			Error:  &interceptor.ErrorInterceptorFailed,
		}, nil)

	secondMock := newTestInterceptorMock(s.T(), "Second", false, 200)
	secondMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) { callOrder = append(callOrder, "Second") }).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil).Maybe()

	s.registry.On("GetInterceptor", "First").Return(firstMock, nil)
	s.registry.On("GetInterceptor", "Second").Return(secondMock, nil)

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "First", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
			newTestInterceptorUnitMock(s.T(), "Second", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
		},
		SharedData: map[string]string{},
	}

	resp, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), common.InterceptorStatusFailure, resp.Status)
	assert.NotNil(s.T(), resp.Error)
	assert.Equal(s.T(), interceptor.ErrorInterceptorFailed.Code, resp.Error.Code)
	assert.Equal(s.T(), []string{"First"}, callOrder, "second interceptor should not run after first fails")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_FailWithNilError_UsesDefault() {
	icMock := newTestInterceptorMock(s.T(), "NilErr", false, 100)
	icMock.On("Execute", mock.Anything).
		Return(&common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
			Error:  nil,
		}, nil)

	s.registry.On("GetInterceptor", "NilErr").Return(icMock, nil)

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "NilErr", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
		},
		SharedData: map[string]string{},
	}

	resp, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), common.InterceptorStatusFailure, resp.Status)
	assert.Nil(s.T(), resp.Error)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_ErrorHaltsExecution() {
	icMock := newTestInterceptorMock(s.T(), "ErrorIC", false, 100)
	icMock.On("Execute", mock.Anything).
		Return(nil, fmt.Errorf("unexpected error"))

	s.registry.On("GetInterceptor", "ErrorIC").Return(icMock, nil)

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "ErrorIC", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
		},
		SharedData: map[string]string{},
	}

	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.NotNil(s.T(), svcErr)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_SharedDataDirectWritesPersist() {
	icMock := newTestInterceptorMock(s.T(), "Writer", false, 100)
	icMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(*core.InterceptorContext)
			ctx.SharedData["key1"] = "val1"
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	s.registry.On("GetInterceptor", "Writer").Return(icMock, nil)

	sharedData := map[string]string{}
	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "Writer", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
		},
		SharedData: sharedData,
	}

	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.Equal(s.T(), "val1", sharedData["key1"])
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_EngineOutputsAreReturned() {
	icMock := newTestInterceptorMock(s.T(), "Enricher", false, 100)
	icMock.On("Execute", mock.Anything).
		Return(&common.InterceptorResponse{
			Status:        common.InterceptorStatusComplete,
			EngineOutputs: map[string]string{"rtKey": "rtVal"},
		}, nil)

	s.registry.On("GetInterceptor", "Enricher").Return(icMock, nil)

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "Enricher", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
		},
		SharedData: map[string]string{},
	}

	resp, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), common.InterceptorStatusComplete, resp.Status)
	assert.Equal(s.T(), "rtVal", resp.EngineOutputs["rtKey"])
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_SharedDataVisibleAcrossInterceptors() {
	firstMock := newTestInterceptorMock(s.T(), "First", false, 100)
	firstMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(*core.InterceptorContext)
			ctx.SharedData["fromFirst"] = testSharedDataValue
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	secondMock := newTestInterceptorMock(s.T(), "Second", false, 200)
	secondMock.On("Execute", mock.Anything).Return(
		func(ctx *core.InterceptorContext) *common.InterceptorResponse {
			if ctx.SharedData["fromFirst"] != testSharedDataValue {
				return nil
			}
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}
		},
		func(ctx *core.InterceptorContext) error {
			if ctx.SharedData["fromFirst"] != testSharedDataValue {
				return fmt.Errorf("expected SharedData from first interceptor")
			}
			return nil
		},
	)

	s.registry.On("GetInterceptor", "First").Return(firstMock, nil)
	s.registry.On("GetInterceptor", "Second").Return(secondMock, nil)

	sharedData := map[string]string{}
	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "First", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
			newTestInterceptorUnitMock(s.T(), "Second", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
		},
		SharedData: sharedData,
	}

	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.Equal(s.T(), testSharedDataValue, sharedData["fromFirst"])
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_ScopeSelected() {
	executed := false
	icMock := newTestInterceptorMock(s.T(), "ScopedIC", false, 200)
	icMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) { executed = true }).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil).Maybe()

	s.registry.On("GetInterceptor", "ScopedIC").Return(icMock, nil).Maybe()

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "ScopedIC", providers.InterceptorModePreNode,
				providers.InterceptorScopeSelected, []string{"target_node"}),
		},
		SharedData: map[string]string{},
	}

	// Node that is NOT in applyTo.
	execCtx.CurrentNodeID = "other_node"
	execCtx.NodeType = common.NodeTypeTaskExecution
	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreNode, execCtx)
	assert.Nil(s.T(), svcErr)
	assert.False(s.T(), executed, "should not run for non-matching node")

	// Node that IS in applyTo.
	execCtx.CurrentNodeID = "target_node"
	execCtx.NodeType = common.NodeTypeTaskExecution
	_, svcErr = s.service.runInterceptors(providers.InterceptorModePreNode, execCtx)
	assert.Nil(s.T(), svcErr)
	assert.True(s.T(), executed, "should run for matching node")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_SkipInterceptors() {
	executed := false
	icMock := newTestInterceptorMock(s.T(), "SkippableIC", false, 200)
	icMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) { executed = true }).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil).Maybe()

	s.registry.On("GetInterceptor", "SkippableIC").Return(icMock, nil).Maybe()

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "SkippableIC", providers.InterceptorModePreNode,
				providers.InterceptorScopeAll, nil),
		},
		SharedData: map[string]string{},
	}

	execCtx.CurrentNodeID = "skip_node"
	execCtx.NodeType = common.NodeTypeTaskExecution
	execCtx.SkipInterceptors = []string{"SkippableIC"}

	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreNode, execCtx)
	assert.Nil(s.T(), svcErr)
	assert.False(s.T(), executed, "interceptor should be skipped when listed in skipInterceptors")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_DefaultInterceptorCannotBeSkippedByNode() {
	executed := false
	defaultICMock := newTestInterceptorMock(s.T(), "DefaultIC", true, 100)
	defaultICMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) { executed = true }).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	s.registry.On("GetInterceptor", "DefaultIC").Return(defaultICMock, nil)

	originalNames := interceptor.DefaultInterceptorNames
	interceptor.DefaultInterceptorNames = map[string]struct{}{"DefaultIC": {}}
	defer func() {
		interceptor.DefaultInterceptorNames = originalNames
	}()

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "DefaultIC", providers.InterceptorModePreNode,
				providers.InterceptorScope(""), nil),
		},
		SharedData:       map[string]string{},
		CurrentNodeID:    "node1",
		NodeType:         common.NodeTypeTaskExecution,
		SkipInterceptors: []string{"DefaultIC"},
	}

	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreNode, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.True(s.T(), executed, "default interceptor must not be bypassed by node skipInterceptors")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_CurrentNodeInputsPassedToInterceptor() {
	expectedInputs := []providers.Input{
		{Identifier: "email", Type: "string", Required: true},
		{Identifier: "password", Type: providers.InputTypePassword, Required: true},
	}

	var receivedInputs []providers.Input
	icMock := newTestInterceptorMock(s.T(), "InputIC", false, 100)
	icMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(*core.InterceptorContext)
			receivedInputs = ctx.CurrentNodeInputs
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	s.registry.On("GetInterceptor", "InputIC").Return(icMock, nil)

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "InputIC", providers.InterceptorModePreNode,
				providers.InterceptorScopeAll, nil),
		},
		CurrentNodeInputs: expectedInputs,
		SharedData:        map[string]string{},
	}

	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.Equal(s.T(), expectedInputs, receivedInputs)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_NilCurrentNodeInputsPassedAsNil() {
	var receivedInputs []providers.Input
	called := false
	icMock := newTestInterceptorMock(s.T(), "NilInputIC", false, 100)
	icMock.On("Execute", mock.Anything).
		Run(func(args mock.Arguments) {
			called = true
			ctx := args.Get(0).(*core.InterceptorContext)
			receivedInputs = ctx.CurrentNodeInputs
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	s.registry.On("GetInterceptor", "NilInputIC").Return(icMock, nil)

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		ResolvedInterceptors: []core.InterceptorUnitInterface{
			newTestInterceptorUnitMock(s.T(), "NilInputIC", providers.InterceptorModePreRequest,
				providers.InterceptorScope(""), nil),
		},
		SharedData: map[string]string{},
	}

	_, svcErr := s.service.runInterceptors(providers.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.True(s.T(), called)
	assert.Nil(s.T(), receivedInputs)
}

// --- Test helpers ---

func newTestInterceptorMock(t interface {
	mock.TestingT
	Cleanup(func())
}, name string, isDefault bool, priority int) *coremock.InterceptorInterfaceMock {
	m := coremock.NewInterceptorInterfaceMock(t)
	m.On("GetName").Return(name).Maybe()
	m.On("IsDefault").Return(isDefault).Maybe()
	m.On("GetPriority").Return(priority).Maybe()
	return m
}

func newTestInterceptorUnitMock(t interface {
	mock.TestingT
	Cleanup(func())
}, name string, mode providers.InterceptorMode, scope providers.InterceptorScope,
	applyTo []string,
) *coremock.InterceptorUnitInterfaceMock {
	m := coremock.NewInterceptorUnitInterfaceMock(t)
	m.On("GetName").Return(name).Maybe()
	m.On("GetMode").Return(mode).Maybe()
	m.On("GetScope").Return(scope).Maybe()
	m.On("GetApplyTo").Return(applyTo).Maybe()

	var ic core.InterceptorInterface
	m.On("GetInterceptor").Return(func() core.InterceptorInterface { return ic }).Maybe()
	m.On("SetInterceptor", mock.Anything).Run(func(args mock.Arguments) {
		if args.Get(0) != nil {
			ic = args.Get(0).(core.InterceptorInterface)
		}
	}).Return().Maybe()
	return m
}
