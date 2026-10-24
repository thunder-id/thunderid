// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/system/systemflow"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// fakeSystemExecutor answers only for its metadata; the runner asks an executor for nothing else.
type fakeSystemExecutor struct {
	providers.Executor
	meta *providers.ExecutorMeta
}

func (f fakeSystemExecutor) GetMeta() *providers.ExecutorMeta { return f.meta }

// fakeRegistry holds the executors a test says the server registered.
type fakeRegistry struct{ executors map[string]providers.Executor }

func (r fakeRegistry) GetExecutor(name string) (providers.Executor, error) {
	ex, ok := r.executors[name]
	if !ok {
		return nil, fmt.Errorf("executor %q is not registered", name)
	}
	return ex, nil
}

func (fakeRegistry) RegisterExecutor(string, providers.Executor) {}

func (r fakeRegistry) IsRegistered(name string) bool {
	_, ok := r.executors[name]
	return ok
}

func (r fakeRegistry) GetExecutorMeta(name string) (*providers.ExecutorMeta, error) {
	ex, err := r.GetExecutor(name)
	if err != nil {
		return nil, err
	}
	return ex.GetMeta(), nil
}

// systemCapableRegistry registers the three revocation steps as the server does, with system flows
// among the flow types each supports.
func systemCapableRegistry() fakeRegistry {
	both := func() *providers.ExecutorMeta {
		return &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration, providers.FlowTypeSystem},
		}
	}
	return fakeRegistry{executors: map[string]providers.Executor{
		executor.ExecutorNameApplicationActionValidator: fakeSystemExecutor{meta: both()},
		executor.ExecutorNameCriteriaRevocation:         fakeSystemExecutor{meta: both()},
		executor.ExecutorNameSessionRevocation:          fakeSystemExecutor{meta: both()},
	}}
}

// systemFlowBuild is a build's set of system flow files, each file's content keyed by its name, for a
// test that hands the runner a build that is wrong.
func systemFlowBuild(files map[string]string) fstest.MapFS {
	build := fstest.MapFS{}
	for name, content := range files {
		build[systemFlowsDir+"/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	return build
}

// minimalFlow is the smallest definition that passes every check, for the given operation.
func minimalFlow(op systemflow.Operation) string {
	return `{"operation": "` + string(op) + `", "flow": {"id": "` + systemFlowIDPrefix + string(op) + `", ` +
		`"flowType": "SYSTEM", "nodes": [{"id": "start", "type": "START", "onSuccess": "end"}, ` +
		`{"id": "end", "type": "END"}]}}`
}

func newRunnerForTest(t *testing.T) (*systemFlowRunner, *flowEngineInterfaceMock) {
	t.Helper()
	engine := newFlowEngineInterfaceMock(t)
	graphs := NewGraphBuilderInterfaceMock(t)
	graphs.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(nil, nil)
	runner, err := newSystemFlowRunner(engine, graphs, systemCapableRegistry())
	require.NoError(t, err)
	return runner, engine
}

var deleteTarget = map[string]string{systemflow.TargetApplication: "app-1"}

// A run executes the operation's flow in process, as a system flow, with the target as its inputs,
// and reports nothing when the flow completes.
func TestSystemFlowRunnerRunsTheOperationsFlowToCompletion(t *testing.T) {
	runner, engine := newRunnerForTest(t)
	engine.EXPECT().Execute(mock.MatchedBy(func(ctx *EngineContext) bool {
		return ctx.FlowType == providers.FlowTypeSystem && ctx.ExecutionID != "" &&
			ctx.UserInputs[systemflow.TargetApplication] == "app-1" && ctx.RuntimeData != nil
	})).Return(FlowStep{Status: providers.FlowStatusComplete}, nil)

	svcErr := runner.Run(context.Background(), systemflow.ApplicationDelete, deleteTarget)

	assert.Nil(t, svcErr)
	assert.Equal(t, "app-1", deleteTarget[systemflow.TargetApplication], "the caller's target was changed")
}

// Every operation the contract names has a flow, and a run asks the engine for that operation's own
// graph.
func TestEveryOperationHasItsOwnSystemFlow(t *testing.T) {
	for _, op := range []systemflow.Operation{systemflow.ApplicationDelete, systemflow.ApplicationSecretRotate} {
		t.Run(string(op), func(t *testing.T) {
			engine := newFlowEngineInterfaceMock(t)
			graphs := NewGraphBuilderInterfaceMock(t)
			// Startup builds every operation's graph once; only the expectation below may serve the run.
			graphs.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(nil, nil).Times(len(systemFlowOperations))
			runner, err := newSystemFlowRunner(engine, graphs, systemCapableRegistry())
			require.NoError(t, err)
			graphs.EXPECT().GetGraph(mock.Anything, mock.MatchedBy(func(flow *providers.CompleteFlowDefinition) bool {
				return flow.ID == systemFlowIDPrefix+string(op) && flow.FlowType == providers.FlowTypeSystem
			})).Return(nil, nil).Once()
			engine.EXPECT().Execute(mock.Anything).Return(FlowStep{Status: providers.FlowStatusComplete}, nil)

			assert.Nil(t, runner.Run(context.Background(), op, deleteTarget))
		})
	}
}

// A refusal by one of the flow's steps comes back as it is, so the caller reports the real reason.
func TestSystemFlowRunnerReturnsTheFlowsOwnRefusal(t *testing.T) {
	runner, engine := newRunnerForTest(t)
	refusal := &tidcommon.ServiceError{Code: "APP-1030", Type: tidcommon.ClientErrorType}
	engine.EXPECT().Execute(mock.Anything).Return(FlowStep{Status: providers.FlowStatusError, Error: refusal}, nil)

	svcErr := runner.Run(context.Background(), systemflow.ApplicationDelete, deleteTarget)

	assert.Same(t, refusal, svcErr)
}

// A system flow that failed without saying why, or that stopped to ask for input, is a server error:
// nothing can answer it, and the change must not go through.
func TestSystemFlowRunnerRefusesAFlowThatDidNotComplete(t *testing.T) {
	for _, tc := range []struct {
		name string
		step FlowStep
		want *tidcommon.ServiceError
	}{
		{"failed without a reason", FlowStep{Status: providers.FlowStatusError}, &ErrorSystemFlowFailed},
		{"stopped for input", FlowStep{Status: providers.FlowStatusIncomplete}, &ErrorSystemFlowIncomplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner, engine := newRunnerForTest(t)
			engine.EXPECT().Execute(mock.Anything).Return(tc.step, nil)

			svcErr := runner.Run(context.Background(), systemflow.ApplicationDelete, deleteTarget)

			assert.Equal(t, tc.want, svcErr)
		})
	}
}

// An engine failure is passed through, and an operation with no flow is refused before the engine
// is asked.
func TestSystemFlowRunnerPassesEngineFailuresThroughAndRefusesUnknownOperations(t *testing.T) {
	runner, engine := newRunnerForTest(t)
	engine.EXPECT().Execute(mock.Anything).Return(FlowStep{}, &tidcommon.InternalServerError).Once()
	assert.Equal(t, &tidcommon.InternalServerError,
		runner.Run(context.Background(), systemflow.ApplicationDelete, deleteTarget))

	assert.Equal(t, &ErrorUnknownSystemFlow,
		runner.Run(context.Background(), systemflow.Operation("role.delete"), nil))
}

// A server that registered only some executors, as the embedded engine does, still starts: the
// operations whose flows it cannot run are noted at startup and refused when a service asks for them,
// before the engine is involved.
func TestAnOperationTheServerCannotRunIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reg   func() fakeRegistry
		graph *tidcommon.ServiceError
	}{
		{"a step is not registered", func() fakeRegistry {
			reg := systemCapableRegistry()
			delete(reg.executors, executor.ExecutorNameSessionRevocation)
			return reg
		}, nil},
		{"a step does not support system flows", func() fakeRegistry {
			reg := systemCapableRegistry()
			reg.executors[executor.ExecutorNameCriteriaRevocation] = fakeSystemExecutor{
				meta: &providers.ExecutorMeta{
					SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
				},
			}
			return reg
		}, nil},
		{"the graph does not build", systemCapableRegistry, &tidcommon.InternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := newFlowEngineInterfaceMock(t)
			graphs := NewGraphBuilderInterfaceMock(t)
			graphs.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(nil, tc.graph).Maybe()
			runner, err := newSystemFlowRunner(engine, graphs, tc.reg())
			require.NoError(t, err)

			svcErr := runner.Run(context.Background(), systemflow.ApplicationDelete, deleteTarget)

			assert.Equal(t, &ErrorSystemFlowUnavailable, svcErr)
			engine.AssertNotCalled(t, "Execute", mock.Anything)
		})
	}
}

// A system flow holds only steps that run unattended: a prompt, which waits for a person, has no
// place in it anywhere, so such a definition is the engine's own fault and fails startup.
func TestASystemFlowMayNotPrompt(t *testing.T) {
	definitions, err := loadSystemFlows(systemFlowFiles)
	require.NoError(t, err)
	flow := definitions[systemflow.ApplicationDelete].Flow
	flow.Nodes = append(flow.Nodes, providers.NodeDefinition{ID: "ask", Type: "PROMPT"})

	err = checkSystemFlowShape(flow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ask")

	graphs := NewGraphBuilderInterfaceMock(t)
	graphs.EXPECT().GetGraph(mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	_, err = newSystemFlowRunner(newFlowEngineInterfaceMock(t), graphs, systemCapableRegistry())
	assert.NoError(t, err, "the built-in definitions must pass the check the test just exercised")

	prompting := systemFlowBuild(map[string]string{
		"application_delete.json": `{"operation": "application.delete", "flow": ` +
			`{"id": "system-flow:application.delete", "flowType": "SYSTEM", ` +
			`"nodes": [{"id": "ask", "type": "PROMPT"}]}}`,
		"application_secret_rotate.json": minimalFlow(systemflow.ApplicationSecretRotate),
	})
	_, err = newSystemFlowRunnerFrom(newFlowEngineInterfaceMock(t), graphs, systemCapableRegistry(), prompting)
	assert.ErrorContains(t, err, "ask", "a build with a prompting flow must fail startup")
}

// The built-in definitions cover exactly the operations the contract names, each as a SYSTEM flow whose
// steps are the revocation steps, and a build that drifts from the contract fails startup: nothing
// outside the build can supply a flow, so a build that lacks one has nothing to fall back on.
func TestTheBuiltInSystemFlowsMatchTheContract(t *testing.T) {
	definitions, err := loadSystemFlows(systemFlowFiles)
	require.NoError(t, err)
	assert.Len(t, definitions, len(systemFlowOperations))
	for _, op := range systemFlowOperations {
		flow := definitions[op].Flow
		require.NotNil(t, flow, "no flow for %s", op)
		assert.Equal(t, providers.FlowTypeSystem, flow.FlowType)
		assert.Equal(t, systemflow.ExposureInternal, definitions[op].Exposure,
			"a built-in flow is started by a service, in process, and by nothing else")
		assert.NoError(t, checkSystemFlowShape(flow))
		assert.NoError(t, checkSystemFlowSteps(flow, systemCapableRegistry()))
	}

	complete := map[string]string{
		"application_delete.json":        minimalFlow(systemflow.ApplicationDelete),
		"application_secret_rotate.json": minimalFlow(systemflow.ApplicationSecretRotate),
	}
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"an operation no service asks for", map[string]string{
			"application_delete.json":        complete["application_delete.json"],
			"application_secret_rotate.json": complete["application_secret_rotate.json"],
			"role_delete.json":               minimalFlow(systemflow.Operation("role.delete")),
		}, "no service asks for"},
		{"an operation defined twice", map[string]string{
			"application_delete.json":        complete["application_delete.json"],
			"application_secret_rotate.json": complete["application_secret_rotate.json"],
			"delete_again.json":              complete["application_delete.json"],
		}, "again"},
		{"an operation left out", map[string]string{
			"application_secret_rotate.json": complete["application_secret_rotate.json"],
		}, "no system flow is built in for application.delete"},
		{"a flow under another operation's id", map[string]string{
			"application_delete.json": `{"operation": "application.delete", "flow": ` +
				`{"id": "system-flow:application.secret_rotate", "flowType": "SYSTEM"}}`,
			"application_secret_rotate.json": complete["application_secret_rotate.json"],
		}, "want \"system-flow:application.delete\""},
		{"a file that is not JSON", map[string]string{
			"application_delete.json":        "resource_type: system_flow",
			"application_secret_rotate.json": complete["application_secret_rotate.json"],
		}, "application_delete.json"},
		{"an exposure that is not one", map[string]string{
			"application_delete.json": `{"operation": "application.delete", "exposure": "public", "flow": ` +
				`{"id": "system-flow:application.delete", "flowType": "SYSTEM"}}`,
			"application_secret_rotate.json": complete["application_secret_rotate.json"],
		}, `exposure "public"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadSystemFlows(systemFlowBuild(tc.files))
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

// The flow execution API cannot start a system flow: its type is not one a request may name.
func TestTheExecutionAPIRefusesSystemFlows(t *testing.T) {
	_, svcErr := validateFlowType(string(providers.FlowTypeSystem))

	assert.Equal(t, &ErrorInvalidFlowType, svcErr)
}

// A definition says who may start its flow; one that says nothing is internal, started only by a
// service, and one may say the flow execution API may start it as well.
func TestASystemFlowNamesItsExposure(t *testing.T) {
	definitions, err := loadSystemFlows(systemFlowBuild(map[string]string{
		"application_delete.json": minimalFlow(systemflow.ApplicationDelete),
		"application_secret_rotate.json": `{"operation": "application.secret_rotate", "exposure": "api", "flow": ` +
			`{"id": "system-flow:application.secret_rotate", "flowType": "SYSTEM"}}`,
	}))
	require.NoError(t, err)

	assert.Equal(t, systemflow.ExposureInternal, definitions[systemflow.ApplicationDelete].Exposure)
	assert.Equal(t, systemflow.ExposureAPI, definitions[systemflow.ApplicationSecretRotate].Exposure)
}
