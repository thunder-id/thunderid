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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type InterceptorRunnerTestSuite struct {
	suite.Suite
	registry interceptor.InterceptorRegistryInterface
	service  InterceptorRunnerInterface
}

func TestInterceptorRunnerSuite(t *testing.T) {
	suite.Run(t, new(InterceptorRunnerTestSuite))
}

func (s *InterceptorRunnerTestSuite) SetupTest() {
	s.registry = newTestInterceptorRegistry()
	s.service = newInterceptorRunner(s.registry)
}

// --- RunInterceptors tests ---

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_NoDeclarations() {
	execCtx := &InterceptorRunnerContext{
		Ctx:        context.Background(),
		SharedData: map[string]string{},
	}

	resp, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), common.InterceptorStatusComplete, resp.Status)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_BindingResolvedAndExecuted() {
	executed := false
	s.registry.RegisterInterceptor("ConfigIC", &stubInterceptor{
		name:      "ConfigIC",
		isDefault: false, priority: 200,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			executed = true
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	})

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{
				name: "ConfigIC", mode: common.InterceptorModePreRequest, scope: common.InterceptorScopeAll,
			},
		},
		SharedData: map[string]string{},
	}

	_, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.True(s.T(), executed)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_SkipsNonMatchingMode() {
	executed := false
	s.registry.RegisterInterceptor("ConfigIC", &stubInterceptor{
		name:      "ConfigIC",
		isDefault: false, priority: 200,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			executed = true
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	})

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{name: "ConfigIC", mode: common.InterceptorModePreNode},
		},
		SharedData: map[string]string{},
	}

	// Request PRE_REQUEST but binding is for PRE_NODE.
	_, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.False(s.T(), executed, "interceptor should not run for non-matching mode")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_FailHaltsExecution() {
	callOrder := []string{}
	s.registry.RegisterInterceptor("First", &stubInterceptor{
		name:      "First",
		isDefault: false, priority: 100,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			callOrder = append(callOrder, "First")
			return &common.InterceptorResponse{
				Status: common.InterceptorStatusFail,
				Error:  &interceptor.ErrorInterceptorFailed,
			}, nil
		},
	})
	s.registry.RegisterInterceptor("Second", &stubInterceptor{
		name:      "Second",
		isDefault: false, priority: 200,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			callOrder = append(callOrder, "Second")
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	})

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{name: "First", mode: common.InterceptorModePreRequest},
			&testInterceptorUnit{name: "Second", mode: common.InterceptorModePreRequest},
		},
		SharedData: map[string]string{},
	}

	resp, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), resp)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), interceptor.ErrorInterceptorFailed.Code, svcErr.Code)
	assert.Equal(s.T(), []string{"First"}, callOrder, "second interceptor should not run after first fails")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_FailWithNilError_UsesDefault() {
	s.registry.RegisterInterceptor("NilErr", &stubInterceptor{
		name:      "NilErr",
		isDefault: false, priority: 100,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			return &common.InterceptorResponse{
				Status: common.InterceptorStatusFail,
				Error:  nil,
			}, nil
		},
	})

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{name: "NilErr", mode: common.InterceptorModePreRequest},
		},
		SharedData: map[string]string{},
	}

	resp, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), resp)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), interceptor.ErrorInterceptorFailed.Code, svcErr.Code)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_ErrorHaltsExecution() {
	s.registry.RegisterInterceptor("ErrorIC", &stubInterceptor{
		name:      "ErrorIC",
		isDefault: false, priority: 100,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			return nil, fmt.Errorf("unexpected error")
		},
	})

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{name: "ErrorIC", mode: common.InterceptorModePreRequest},
		},
		SharedData: map[string]string{},
	}

	_, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.NotNil(s.T(), svcErr)
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_SharedDataDirectWritesPersist() {
	s.registry.RegisterInterceptor("Writer", &stubInterceptor{
		name:      "Writer",
		isDefault: false, priority: 100,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			ctx.SharedData["key1"] = "val1"
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	})

	sharedData := map[string]string{}
	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{name: "Writer", mode: common.InterceptorModePreRequest},
		},
		SharedData: sharedData,
	}

	_, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.Equal(s.T(), "val1", sharedData["key1"])
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_EngineOutputsAreReturned() {
	s.registry.RegisterInterceptor("Enricher", &stubInterceptor{
		name:      "Enricher",
		isDefault: false, priority: 100,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			return &common.InterceptorResponse{
				Status:        common.InterceptorStatusComplete,
				EngineOutputs: map[string]string{"rtKey": "rtVal"},
			}, nil
		},
	})

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{name: "Enricher", mode: common.InterceptorModePreRequest},
		},
		SharedData: map[string]string{},
	}

	resp, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), common.InterceptorStatusComplete, resp.Status)
	assert.Equal(s.T(), "rtVal", resp.EngineOutputs["rtKey"])
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_SharedDataVisibleAcrossInterceptors() {
	s.registry.RegisterInterceptor("First", &stubInterceptor{
		name:      "First",
		isDefault: false, priority: 100,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			ctx.SharedData["fromFirst"] = "hello"
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	})
	s.registry.RegisterInterceptor("Second", &stubInterceptor{
		name:      "Second",
		isDefault: false, priority: 200,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			// Second interceptor should see what the first wrote.
			if ctx.SharedData["fromFirst"] != "hello" {
				return nil, fmt.Errorf("expected SharedData from first interceptor")
			}
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	})

	sharedData := map[string]string{}
	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{name: "First", mode: common.InterceptorModePreRequest},
			&testInterceptorUnit{name: "Second", mode: common.InterceptorModePreRequest},
		},
		SharedData: sharedData,
	}

	_, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.Equal(s.T(), "hello", sharedData["fromFirst"])
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_ScopeSelected() {
	executed := false
	s.registry.RegisterInterceptor("ScopedIC", &stubInterceptor{
		name:      "ScopedIC",
		isDefault: false, priority: 200,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			executed = true
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	})

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{
				name:    "ScopedIC",
				mode:    common.InterceptorModePreNode,
				scope:   common.InterceptorScopeSelected,
				applyTo: []string{"target_node"},
			},
		},
		SharedData: map[string]string{},
	}

	// Node that is NOT in applyTo.
	execCtx.CurrentNode = &stubNode{id: "other_node", nodeType: common.NodeTypeTaskExecution}
	_, svcErr := s.service.runInterceptors(common.InterceptorModePreNode, execCtx)
	assert.Nil(s.T(), svcErr)
	assert.False(s.T(), executed, "should not run for non-matching node")

	// Node that IS in applyTo.
	execCtx.CurrentNode = &stubNode{id: "target_node", nodeType: common.NodeTypeTaskExecution}
	_, svcErr = s.service.runInterceptors(common.InterceptorModePreNode, execCtx)
	assert.Nil(s.T(), svcErr)
	assert.True(s.T(), executed, "should run for matching node")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_SkipInterceptors() {
	executed := false
	s.registry.RegisterInterceptor("SkippableIC", &stubInterceptor{
		name:      "SkippableIC",
		isDefault: false, priority: 200,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			executed = true
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	})

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{
				name: "SkippableIC", mode: common.InterceptorModePreNode, scope: common.InterceptorScopeAll,
			},
		},
		SharedData: map[string]string{},
	}

	execCtx.CurrentNode = &stubNode{
		id:       "skip_node",
		nodeType: common.NodeTypeTaskExecution,
		properties: map[string]interface{}{
			"skipInterceptors": []interface{}{"SkippableIC"},
		},
	}

	_, svcErr := s.service.runInterceptors(common.InterceptorModePreNode, execCtx)
	assert.Nil(s.T(), svcErr)
	assert.False(s.T(), executed, "interceptor should be skipped when listed in skipInterceptors")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_DefaultInterceptorCannotBeSkippedByNode() {
	executed := false
	defaultIC := &stubInterceptor{
		name:      "DefaultIC",
		isDefault: true, priority: 100,
		execFn: func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
			executed = true
			return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
		},
	}

	s.registry.RegisterInterceptor("DefaultIC", defaultIC)

	original := interceptor.DefaultInterceptors
	originalNames := interceptor.DefaultInterceptorNames
	interceptor.DefaultInterceptors = []core.InterceptorUnitInterface{
		&testInterceptorUnit{name: "DefaultIC", mode: common.InterceptorModePreNode},
	}
	interceptor.DefaultInterceptorNames = map[string]struct{}{"DefaultIC": {}}
	defer func() {
		interceptor.DefaultInterceptors = original
		interceptor.DefaultInterceptorNames = originalNames
	}()

	execCtx := &InterceptorRunnerContext{
		Ctx:        context.Background(),
		SharedData: map[string]string{},
		CurrentNode: &stubNode{
			id:       "node1",
			nodeType: common.NodeTypeTaskExecution,
			properties: map[string]interface{}{
				"skipInterceptors": []interface{}{"DefaultIC"},
			},
		},
	}

	_, svcErr := s.service.runInterceptors(common.InterceptorModePreNode, execCtx)

	assert.Nil(s.T(), svcErr)
	assert.True(s.T(), executed, "default interceptor must not be bypassed by node skipInterceptors")
}

func (s *InterceptorRunnerTestSuite) TestRunInterceptors_RejectsConfiguredDefaultInterceptor() {
	// Set up a default interceptor for the duration of this test.
	original := interceptor.DefaultInterceptors
	originalNames := interceptor.DefaultInterceptorNames
	interceptor.DefaultInterceptors = []core.InterceptorUnitInterface{
		&testInterceptorUnit{name: "DefaultIC", mode: common.InterceptorModePreRequest},
	}
	interceptor.DefaultInterceptorNames = map[string]struct{}{"DefaultIC": {}}
	defer func() {
		interceptor.DefaultInterceptors = original
		interceptor.DefaultInterceptorNames = originalNames
	}()

	execCtx := &InterceptorRunnerContext{
		Ctx: context.Background(),
		Interceptors: []core.InterceptorUnitInterface{
			&testInterceptorUnit{name: "DefaultIC", mode: common.InterceptorModePreRequest},
		},
		SharedData: map[string]string{},
	}

	_, svcErr := s.service.runInterceptors(common.InterceptorModePreRequest, execCtx)

	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), interceptor.ErrorInterceptorExecution.Code, svcErr.Code)
}

// --- Test helpers ---

// testInterceptorRegistry is a simple in-test registry for interceptor.InterceptorRegistryInterface.
type testInterceptorRegistry struct {
	interceptors map[string]core.InterceptorInterface
}

func newTestInterceptorRegistry() interceptor.InterceptorRegistryInterface {
	return &testInterceptorRegistry{interceptors: make(map[string]core.InterceptorInterface)}
}

func (r *testInterceptorRegistry) RegisterInterceptor(name string, ic core.InterceptorInterface) {
	r.interceptors[name] = ic
}

func (r *testInterceptorRegistry) GetInterceptor(name string) (core.InterceptorInterface, error) {
	ic, ok := r.interceptors[name]
	if !ok {
		return nil, fmt.Errorf("interceptor '%s' not found", name)
	}
	return ic, nil
}

func (r *testInterceptorRegistry) IsRegistered(name string) bool {
	_, ok := r.interceptors[name]
	return ok
}

// --- Stubs ---

// stubInterceptor implements core.InterceptorInterface for testing.
type stubInterceptor struct {
	name      string
	isDefault bool
	priority  int
	execFn    func(ctx *core.InterceptorContext) (*common.InterceptorResponse, error)
}

var _ core.InterceptorInterface = (*stubInterceptor)(nil)

func (s *stubInterceptor) GetName() string  { return s.name }
func (s *stubInterceptor) IsDefault() bool  { return s.isDefault }
func (s *stubInterceptor) GetPriority() int { return s.priority }
func (s *stubInterceptor) Execute(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
	if s.execFn != nil {
		return s.execFn(ctx)
	}
	return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
}

// stubNode implements core.NodeInterface for testing.
type stubNode struct {
	id         string
	nodeType   common.NodeType
	properties map[string]interface{}
}

var _ core.NodeInterface = (*stubNode)(nil)

func (n *stubNode) GetID() string                         { return n.id }
func (n *stubNode) GetType() common.NodeType              { return n.nodeType }
func (n *stubNode) GetProperties() map[string]interface{} { return n.properties }
func (n *stubNode) Execute(_ *core.NodeContext) (*common.NodeResponse, *serviceerror.ServiceError) {
	return nil, nil
}
func (n *stubNode) ShouldExecute(_ *core.NodeContext) bool    { return true }
func (n *stubNode) SetCondition(_ *core.NodeCondition)        {}
func (n *stubNode) GetCondition() *core.NodeCondition         { return nil }
func (n *stubNode) IsStartNode() bool                         { return false }
func (n *stubNode) SetAsStartNode()                           {}
func (n *stubNode) IsFinalNode() bool                         { return false }
func (n *stubNode) SetAsFinalNode()                           {}
func (n *stubNode) GetNextNodeList() []string                 { return nil }
func (n *stubNode) SetNextNodeList(_ []string)                {}
func (n *stubNode) AddNextNode(_ string)                      {}
func (n *stubNode) RemoveNextNode(_ string)                   {}
func (n *stubNode) GetPreviousNodeList() []string             { return nil }
func (n *stubNode) SetPreviousNodeList(_ []string)            {}
func (n *stubNode) AddPreviousNode(_ string)                  {}
func (n *stubNode) RemovePreviousNode(_ string)               {}
func (n *stubNode) GetExecutionPolicy() *core.ExecutionPolicy { return nil }

// testInterceptorUnit implements core.InterceptorBackedUnitInterface for testing.
type testInterceptorUnit struct {
	name        string
	mode        common.InterceptorMode
	scope       common.InterceptorScope
	applyTo     []string
	properties  map[string]interface{}
	interceptor core.InterceptorInterface
}

var _ core.InterceptorUnitInterface = (*testInterceptorUnit)(nil)

func (u *testInterceptorUnit) GetName() string                             { return u.name }
func (u *testInterceptorUnit) GetMode() common.InterceptorMode             { return u.mode }
func (u *testInterceptorUnit) GetScope() common.InterceptorScope           { return u.scope }
func (u *testInterceptorUnit) GetApplyTo() []string                        { return u.applyTo }
func (u *testInterceptorUnit) GetProperties() map[string]interface{}       { return u.properties }
func (u *testInterceptorUnit) GetInterceptor() core.InterceptorInterface   { return u.interceptor }
func (u *testInterceptorUnit) SetName(name string)                         { u.name = name }
func (u *testInterceptorUnit) SetMode(mode common.InterceptorMode)         { u.mode = mode }
func (u *testInterceptorUnit) SetScope(scope common.InterceptorScope)      { u.scope = scope }
func (u *testInterceptorUnit) SetApplyTo(applyTo []string)                 { u.applyTo = applyTo }
func (u *testInterceptorUnit) SetProperties(p map[string]interface{})      { u.properties = p }
func (u *testInterceptorUnit) SetInterceptor(ic core.InterceptorInterface) { u.interceptor = ic }
