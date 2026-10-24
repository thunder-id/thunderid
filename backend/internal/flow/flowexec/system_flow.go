// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/systemflow"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// systemFlowIDPrefix starts every system flow's id. The engine caches graphs by flow id, and no stored
// flow carries an id of this shape, so a system flow's graph is never confused with an administrator's.
const systemFlowIDPrefix = "system-flow:"

// systemFlowFiles holds the system flows, compiled into the binary: the flows the engine owns and a
// service runs in process before it writes a change, whichever entry point asked for it. One JSON
// file per operation, each `{"operation": ..., "exposure": ..., "flow": ...}` with the flow in the
// same shape as a stored flow definition and the exposure saying who may start it (see
// systemflow.Exposure; absent means internal). They are part of the build, so nothing deployed
// beside the server, and nothing written through the APIs, can add, remove or change one; they are
// never imported into the store and never listed by the flow APIs.
//
// A system flow holds only what revokes. It plans what a change invalidates, revokes the tokens, and
// ends or detaches the sessions. The permission check stays with the entry point, which knows who
// asked, and the write stays with the service, which runs the flow and then writes. A step that asked
// for input would stop the flow with no one to answer, so a system flow has no PROMPT nodes.
//
//go:embed systemflows/*.json
var systemFlowFiles embed.FS

// systemFlowsDir is the directory of systemFlowFiles the definitions are read from.
const systemFlowsDir = "systemflows"

// systemFlowDocument is one embedded file: the operation, who may start its flow, and the flow.
type systemFlowDocument struct {
	Operation systemflow.Operation             `json:"operation"`
	Exposure  systemflow.Exposure              `json:"exposure,omitempty"`
	Flow      providers.CompleteFlowDefinition `json:"flow"`
}

// systemFlow is one loaded definition: the flow and who may start it.
type systemFlow struct {
	Flow     *providers.CompleteFlowDefinition
	Exposure systemflow.Exposure
}

// systemFlowExposures is every exposure a definition may name.
var systemFlowExposures = []systemflow.Exposure{systemflow.ExposureInternal, systemflow.ExposureAPI}

// systemFlowOperations is every operation a service may ask for. The build defines each exactly once
// and nothing else.
var systemFlowOperations = []systemflow.Operation{
	systemflow.ApplicationDelete,
	systemflow.ApplicationSecretRotate,
}

// loadSystemFlows reads the system flows from the files compiled into the binary. The files are part
// of the build, so anything wrong with them, a file that does not parse, an operation no service asks
// for, one defined twice or one left out, an exposure that is not one, is a fault in the build and an
// error. A fresh definition is decoded on every call, so a caller writing through one changes nothing
// the next call sees.
func loadSystemFlows(files fs.FS) (map[systemflow.Operation]systemFlow, error) {
	names, err := fs.Glob(files, systemFlowsDir+"/*.json")
	if err != nil {
		return nil, fmt.Errorf("system flows: %w", err)
	}
	flows := map[systemflow.Operation]systemFlow{}
	for _, name := range names {
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("system flow %s: %w", name, err)
		}
		var doc systemFlowDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("system flow %s: %w", name, err)
		}
		if !slices.Contains(systemFlowOperations, doc.Operation) {
			return nil, fmt.Errorf("system flow %s names an operation no service asks for: %q", name, doc.Operation)
		}
		if _, dup := flows[doc.Operation]; dup {
			return nil, fmt.Errorf("system flow %s defines %q again", name, doc.Operation)
		}
		if doc.Flow.ID != systemFlowIDPrefix+string(doc.Operation) {
			return nil, fmt.Errorf("system flow %s has id %q, want %q", name, doc.Flow.ID,
				systemFlowIDPrefix+string(doc.Operation))
		}
		if doc.Exposure == "" {
			doc.Exposure = systemflow.ExposureInternal
		}
		if !slices.Contains(systemFlowExposures, doc.Exposure) {
			return nil, fmt.Errorf("system flow %s has exposure %q, want one of %v", name, doc.Exposure,
				systemFlowExposures)
		}
		flow := doc.Flow
		flows[doc.Operation] = systemFlow{Flow: &flow, Exposure: doc.Exposure}
	}
	for _, op := range systemFlowOperations {
		if _, defined := flows[op]; !defined {
			return nil, fmt.Errorf("no system flow is built in for %s", op)
		}
	}
	return flows, nil
}

// systemFlowRunner runs the engine's own flows in process for the services that change what a
// principal may do. It implements systemflow.Runner.
type systemFlowRunner struct {
	engine       flowEngineInterface
	graphBuilder graphbuilder.GraphBuilderInterface
	flows        map[systemflow.Operation]systemFlow
	// unavailable holds, for each operation whose flow this server cannot run, why not: a step the
	// server did not register, one that does not support system flows, or a graph that would not
	// build. Such an operation is refused when asked for, with the reason logged.
	unavailable map[systemflow.Operation]string
	logger      *log.Logger
}

// newSystemFlowRunner loads the built-in system flows and checks every one when the server starts. A
// definition that could never run unattended is a fault in the build and an error. An operation whose
// steps this server did not register, as the embedded engine does, is noted and logged, and refused
// when a service asks for it: the server still starts, and a change that would leave something
// unrevoked is not written.
func newSystemFlowRunner(engine flowEngineInterface, graphBuilder graphbuilder.GraphBuilderInterface,
	registry executor.ExecutorRegistryInterface) (*systemFlowRunner, error) {
	return newSystemFlowRunnerFrom(engine, graphBuilder, registry, systemFlowFiles)
}

// newSystemFlowRunnerFrom is newSystemFlowRunner over the given definitions, so a test can hand it a
// build that is wrong.
func newSystemFlowRunnerFrom(engine flowEngineInterface, graphBuilder graphbuilder.GraphBuilderInterface,
	registry executor.ExecutorRegistryInterface, files fs.FS) (*systemFlowRunner, error) {
	r := &systemFlowRunner{
		engine:       engine,
		graphBuilder: graphBuilder,
		flows:        map[systemflow.Operation]systemFlow{},
		unavailable:  map[systemflow.Operation]string{},
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SystemFlowRunner")),
	}
	definitions, err := loadSystemFlows(files)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	for _, op := range systemFlowOperations {
		definition := definitions[op]
		flow := definition.Flow
		if err := checkSystemFlowShape(flow); err != nil {
			return nil, fmt.Errorf("system flow for %s: %w", op, err)
		}
		reason := ""
		if err := checkSystemFlowSteps(flow, registry); err != nil {
			reason = err.Error()
		} else if _, svcErr := graphBuilder.GetGraph(ctx, flow); svcErr != nil {
			reason = svcErr.Error.DefaultValue
		}
		if reason != "" {
			r.unavailable[op] = reason
			r.logger.Warn(ctx, "A system flow cannot run on this server; the operation will be refused",
				log.String("operation", string(op)), log.String("reason", reason))
			continue
		}
		r.flows[op] = definition
	}
	return r, nil
}

// checkSystemFlowShape refuses a definition that could not run unattended anywhere: a node that waits
// for a person, or a task with no executor.
func checkSystemFlowShape(flow *providers.CompleteFlowDefinition) error {
	if flow.FlowType != providers.FlowTypeSystem {
		return fmt.Errorf("flow type is %q, not %q", flow.FlowType, providers.FlowTypeSystem)
	}
	for _, node := range flow.Nodes {
		switch common.NodeType(node.Type) {
		case common.NodeTypeStart, common.NodeTypeEnd:
		case common.NodeTypeTaskExecution:
			if node.Executor == nil {
				return fmt.Errorf("node %q names no executor", node.ID)
			}
		default:
			return fmt.Errorf("node %q is of type %q, which a system flow cannot run unattended", node.ID, node.Type)
		}
	}
	return nil
}

// checkSystemFlowSteps refuses a definition whose steps this server did not register, or registered
// without support for system flows.
func checkSystemFlowSteps(flow *providers.CompleteFlowDefinition,
	registry executor.ExecutorRegistryInterface) error {
	for _, node := range flow.Nodes {
		if node.Executor == nil {
			continue
		}
		exec, err := registry.GetExecutor(node.Executor.Name)
		if err != nil {
			return fmt.Errorf("node %q: executor %q is not registered: %w", node.ID, node.Executor.Name, err)
		}
		meta := exec.GetMeta()
		if meta == nil || !slices.Contains(meta.SupportedFlowTypes, providers.FlowTypeSystem) {
			return fmt.Errorf("node %q: executor %q does not support system flows", node.ID, node.Executor.Name)
		}
	}
	return nil
}

// Run executes the operation's system flow to completion with the target the change acts on. The
// flow's own refusal, such as an application that is declared in a file, comes back as it is; a flow
// that stopped for input or failed without a reason is a server error. Nothing is stored: a system
// flow never pauses, so there is no context to keep.
func (r *systemFlowRunner) Run(ctx context.Context, op systemflow.Operation,
	target map[string]string) *tidcommon.ServiceError {
	if reason, unavailable := r.unavailable[op]; unavailable {
		r.logger.Error(ctx, "The system flow cannot run on this server, so the change is refused",
			log.String("operation", string(op)), log.String("reason", reason))
		return &ErrorSystemFlowUnavailable
	}
	definition, ok := r.flows[op]
	if !ok {
		r.logger.Error(ctx, "No system flow is defined for the operation", log.String("operation", string(op)))
		return &ErrorUnknownSystemFlow
	}
	flow := definition.Flow
	graph, svcErr := r.graphBuilder.GetGraph(ctx, flow)
	if svcErr != nil {
		r.logger.Error(ctx, "Failed to build the system flow's graph", log.String("operation", string(op)),
			log.String("error", svcErr.Error.DefaultValue))
		return &tidcommon.InternalServerError
	}
	executionID, err := sysutils.GenerateUUIDv7()
	if err != nil {
		r.logger.Error(ctx, "Failed to generate an execution id for the system flow", log.Error(err))
		return &tidcommon.InternalServerError
	}
	engineCtx := &EngineContext{
		Context:               ctx,
		ExecutionID:           executionID,
		FlowType:              providers.FlowTypeSystem,
		Graph:                 graph,
		UserInputs:            maps.Clone(target),
		RuntimeData:           map[string]string{},
		InterceptorSharedData: map[string]string{},
		TraceID:               sysContext.GetTraceID(ctx),
	}
	if engineCtx.UserInputs == nil {
		engineCtx.UserInputs = map[string]string{}
	}
	step, svcErr := r.engine.Execute(engineCtx)
	if svcErr != nil {
		r.logger.Error(ctx, "The system flow could not run", log.String("operation", string(op)),
			log.String(log.LoggerKeyExecutionID, executionID), log.String("code", svcErr.Code))
		return svcErr
	}
	switch step.Status {
	case providers.FlowStatusComplete:
		return nil
	case providers.FlowStatusError:
		if step.Error != nil {
			return step.Error
		}
		r.logger.Error(ctx, "The system flow failed without a reason", log.String("operation", string(op)),
			log.String(log.LoggerKeyExecutionID, executionID))
		return &ErrorSystemFlowFailed
	default:
		r.logger.Error(ctx, "The system flow stopped for input", log.String("operation", string(op)),
			log.String(log.LoggerKeyExecutionID, executionID), log.String("status", string(step.Status)))
		return &ErrorSystemFlowIncomplete
	}
}
