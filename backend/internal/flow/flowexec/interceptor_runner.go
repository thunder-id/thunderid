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
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// InterceptorRunnerInterface owns interceptor orchestration for flow instances.
type InterceptorRunnerInterface interface {
	// runInterceptors filters, orders, and executes interceptors for the given mode.
	// Returns an common.InterceptorResponse with accumulated outputs (COMPLETE) or UI-interaction
	// fields (INCOMPLETE). FAIL is returned as a *tidcommon.ServiceError.
	runInterceptors(mode providers.InterceptorMode, execCtx *InterceptorRunnerContext) (
		*common.InterceptorResponse, *tidcommon.ServiceError)
}

// interceptorRunner is the default implementation of InterceptorRunnerInterface.
type interceptorRunner struct {
	registry interceptor.InterceptorRegistryInterface
	logger   *log.Logger
}

// newInterceptorRunner creates a new interceptor runner.
func newInterceptorRunner(registry interceptor.InterceptorRegistryInterface) InterceptorRunnerInterface {
	return &interceptorRunner{
		registry: registry,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InterceptorRunner")),
	}
}

// runInterceptors resolves applicable interceptors for the given mode and delegates execution
// to mode-specific handlers. The runner internally branches: request-level modes (PRE_REQUEST,
// POST_REQUEST) reject INCOMPLETE results, while node-level modes (PRE_NODE, POST_NODE) allow them.
func (s *interceptorRunner) runInterceptors(
	mode providers.InterceptorMode,
	execCtx *InterceptorRunnerContext,
) (*common.InterceptorResponse, *tidcommon.ServiceError) {
	applicable, svcErr := s.resolveApplicableInterceptors(mode, execCtx)
	if svcErr != nil {
		return nil, svcErr
	}
	if len(applicable) == 0 {
		return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
	}

	return s.executeInterceptors(applicable, mode, execCtx)
}

// resolveApplicableInterceptors filters pre-resolved interceptors by node scope and lazily resolves
// interceptor instances from the registry. The input list is already mode-filtered and priority-sorted
// at graph construction time.
func (s *interceptorRunner) resolveApplicableInterceptors(
	mode providers.InterceptorMode,
	execCtx *InterceptorRunnerContext,
) ([]core.InterceptorUnitInterface, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, execCtx.ExecutionID))

	all := execCtx.ResolvedInterceptors
	if len(all) == 0 {
		return nil, nil
	}

	// For per-node modes, apply node scoping.
	var applicable []core.InterceptorUnitInterface
	if isPerNodeMode(mode) && execCtx.CurrentNodeID != "" {
		applicable = make([]core.InterceptorUnitInterface, 0, len(all))
		for _, b := range all {
			if shouldApplyToNode(b, execCtx.CurrentNodeID, execCtx.SkipInterceptors) {
				applicable = append(applicable, b)
			}
		}
	} else {
		applicable = all
	}

	// Resolve interceptor instances from the registry if not already set.
	for _, b := range applicable {
		if b.GetInterceptor() == nil {
			ic, err := s.registry.GetInterceptor(b.GetName())
			if err != nil {
				logger.Error(execCtx.Ctx, "Interceptor not found in registry",
					log.String("interceptorName", b.GetName()), log.Error(err))
				return nil, &tidcommon.InternalServerError
			}
			b.SetInterceptor(ic)
		}
	}

	return applicable, nil
}

// executeInterceptors executes interceptors for the given mode. For node-level modes, INCOMPLETE
// results are valid and returned with node-specific fields. For request-level modes, INCOMPLETE
// results are treated as execution errors since request-level interceptors must not request
// user interaction.
func (s *interceptorRunner) executeInterceptors(
	applicable []core.InterceptorUnitInterface,
	mode providers.InterceptorMode,
	execCtx *InterceptorRunnerContext,
) (*common.InterceptorResponse, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, execCtx.ExecutionID))

	resp := &common.InterceptorResponse{
		Status:        common.InterceptorStatusComplete,
		EngineOutputs: make(map[string]string),
	}
	for _, b := range applicable {
		ic := b.GetInterceptor()
		name := ic.GetName()

		result, svcErr := s.executeInterceptor(ic, name, mode, execCtx, logger)
		if svcErr != nil {
			return nil, svcErr
		}
		if result == nil {
			continue
		}

		switch result.Status {
		case common.InterceptorStatusComplete:
			s.mergeInterceptorResponse(resp, result)
		case common.InterceptorStatusIncomplete:
			s.mergeInterceptorResponse(resp, result)
			resp.Status = common.InterceptorStatusIncomplete
			return resp, nil
		case common.InterceptorStatusFailure:
			s.handleInterceptorError(resp, result)
			resp.Status = common.InterceptorStatusFailure
			return resp, nil
		default:
			logger.Error(execCtx.Ctx, "Interceptor returned invalid status",
				log.String("interceptorName", name), log.String("status", string(result.Status)))
			return nil, &tidcommon.InternalServerError
		}
	}

	return resp, nil
}

// executeInterceptor builds the InterceptorContext and executes a single interceptor.
func (s *interceptorRunner) executeInterceptor(
	ic core.InterceptorInterface,
	name string,
	mode providers.InterceptorMode,
	execCtx *InterceptorRunnerContext,
	logger *log.Logger,
) (*common.InterceptorResponse, *tidcommon.ServiceError) {
	logger.Debug(execCtx.Ctx, "Running interceptor",
		log.String("interceptorName", name),
		log.String("mode", string(mode)),
		log.String("nodeID", execCtx.CurrentNodeID))

	ctx := &core.InterceptorContext{
		Context:             execCtx.Ctx,
		ExecutionID:         execCtx.ExecutionID,
		AppID:               execCtx.AppID,
		FlowType:            execCtx.FlowType,
		FlowStatus:          execCtx.FlowStatus,
		Mode:                mode,
		UserInputs:          execCtx.UserInputs,
		CurrentNodeID:       execCtx.CurrentNodeID,
		NodeType:            execCtx.NodeType,
		ExecutionPolicy:     execCtx.ExecutionPolicy,
		AllowSegmentRestart: execCtx.AllowSegmentRestart,
		CurrentNodeInputs:   execCtx.CurrentNodeInputs,
		ForwardedData:       execCtx.ForwardedData,
		AdditionalData:      execCtx.AdditionalData,
		SharedData:          execCtx.SharedData,
	}
	if ctx.SharedData == nil {
		ctx.SharedData = make(map[string]string)
	}

	result, err := ic.Execute(ctx)
	if err != nil {
		logger.Error(execCtx.Ctx, "Interceptor execution error",
			log.String("interceptorName", name), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	execCtx.AppendConsumedInputs(ctx.GetConsumedInputs())

	return result, nil
}

// mergeInterceptorResponse accumulates fields from src into dst.
// EngineOutputs are merged (src wins on key collisions); other fields are overwritten.
func (s *interceptorRunner) mergeInterceptorResponse(dst, src *common.InterceptorResponse) {
	for k, v := range src.EngineOutputs {
		dst.EngineOutputs[k] = v
	}
	dst.FieldErrors = src.FieldErrors
	dst.ChallengeToken = src.ChallengeToken
	dst.Error = src.Error
}

// handleInterceptorError extracts the failure error from an interceptor result and converts it
// to an engine-level service error.
func (s *interceptorRunner) handleInterceptorError(
	response *common.InterceptorResponse,
	result *common.InterceptorResponse,
) {
	// For errors, ensure a service error is returned.
	if failErr := result.Error; failErr == nil {
		response.Error = &interceptor.ErrorInterceptorFailed
	}
	response.Error = result.Error
}

// shouldApplyToNode determines whether an interceptor should run for the given node.
// Default interceptors always apply and cannot be bypassed by node-level skip lists.
func shouldApplyToNode(
	interceptorUnit core.InterceptorUnitInterface, nodeID string, skipInterceptors []string,
) bool {
	if isDefaultInterceptorUnit(interceptorUnit.GetName()) {
		return true
	}

	switch interceptorUnit.GetScope() {
	case providers.InterceptorScopeSelected:
		return slices.Contains(interceptorUnit.GetApplyTo(), nodeID)
	case providers.InterceptorScopeAll:
		return !slices.Contains(skipInterceptors, interceptorUnit.GetName())
	default:
		// No scope specified defaults to ALL.
		return !slices.Contains(skipInterceptors, interceptorUnit.GetName())
	}
}

// isDefaultInterceptorUnit checks whether the given interceptor name matches any default interceptor.
func isDefaultInterceptorUnit(name string) bool {
	_, ok := interceptor.DefaultInterceptorNames[name]
	return ok
}

// isPerNodeMode returns true for PRE_NODE and POST_NODE modes.
func isPerNodeMode(mode providers.InterceptorMode) bool {
	return mode == providers.InterceptorModePreNode || mode == providers.InterceptorModePostNode
}
