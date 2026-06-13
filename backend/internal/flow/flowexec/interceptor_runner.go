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
	"fmt"
	"slices"
	"sort"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	propertyKeySkipInterceptors = "skipInterceptors"
)

// InterceptorRunnerInterface owns interceptor orchestration for flow instances.
type InterceptorRunnerInterface interface {

	// runInterceptors filters, orders, and executes interceptors for the given mode.
	// Returns an common.InterceptorResponse with accumulated outputs (COMPLETE) or UI-interaction
	// fields (INCOMPLETE). FAIL is returned as a *serviceerror.ServiceError.
	runInterceptors(mode common.InterceptorMode, execCtx *InterceptorRunnerContext) (
		*common.InterceptorResponse, *serviceerror.ServiceError)
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
	mode common.InterceptorMode,
	execCtx *InterceptorRunnerContext,
) (*common.InterceptorResponse, *serviceerror.ServiceError) {
	applicable, svcErr := s.resolveApplicableInterceptors(mode, execCtx)
	if svcErr != nil {
		return nil, svcErr
	}
	if len(applicable) == 0 {
		return &common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil
	}

	return s.executeInterceptors(applicable, mode, execCtx)
}

// resolveApplicableInterceptors validates, filters, resolves, and sorts interceptors for a mode.
func (s *interceptorRunner) resolveApplicableInterceptors(
	mode common.InterceptorMode,
	execCtx *InterceptorRunnerContext,
) ([]core.InterceptorUnitInterface, *serviceerror.ServiceError) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, execCtx.ExecutionID))

	node := execCtx.CurrentNode
	var currentNodeID string
	if node != nil {
		currentNodeID = node.GetID()
	}

	// Validate that configured interceptors are not default ones.
	if err := s.validateConfiguredInterceptors(execCtx, logger); err != nil {
		return nil, err
	}

	// Combine configured interceptors with defaults for this mode.
	allInterceptors := make([]core.InterceptorUnitInterface, 0, len(execCtx.Interceptors))
	allInterceptors = append(allInterceptors, execCtx.Interceptors...)
	allInterceptors = append(allInterceptors, s.addDefaultInterceptorUnits(mode)...)

	// Filter declarations for the requested mode.
	applicable := make([]core.InterceptorUnitInterface, 0, len(allInterceptors))
	for _, b := range allInterceptors {
		if b.GetMode() != mode {
			continue
		}

		// For per-node modes, apply scoping.
		if isPerNodeMode(mode) && node != nil {
			if !shouldApplyToNode(b, currentNodeID, node) {
				continue
			}
		}

		// Resolve the interceptor from the registry if not already set on the unit.
		if b.GetInterceptor() == nil {
			ic, err := s.registry.GetInterceptor(b.GetName())
			if err != nil {
				logger.Error(execCtx.Ctx, "Interceptor not found in registry",
					log.String("interceptorName", b.GetName()), log.Error(err))
				return nil, serviceerror.CustomServiceError(interceptor.ErrorInterceptorExecution, i18ncore.I18nMessage{
					Key:          "error.interceptor.execution_error_description",
					DefaultValue: fmt.Sprintf("Interceptor '%s' not found in registry", b.GetName()),
				})
			}
			b.SetInterceptor(ic)
		}

		applicable = append(applicable, b)
	}

	// Sort by priority ascending.
	sort.Slice(applicable, func(i, j int) bool {
		return applicable[i].GetInterceptor().GetPriority() < applicable[j].GetInterceptor().GetPriority()
	})

	return applicable, nil
}

// executeInterceptors executes interceptors for the given mode. For node-level modes, INCOMPLETE
// results are valid and returned with node-specific fields. For request-level modes, INCOMPLETE
// results are treated as execution errors since request-level interceptors must not request
// user interaction.
func (s *interceptorRunner) executeInterceptors(
	applicable []core.InterceptorUnitInterface,
	mode common.InterceptorMode,
	execCtx *InterceptorRunnerContext,
) (*common.InterceptorResponse, *serviceerror.ServiceError) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, execCtx.ExecutionID))

	resp := &common.InterceptorResponse{
		Status:        common.InterceptorStatusComplete,
		EngineOutputs: make(map[string]string),
	}

	nodeLevel := isPerNodeMode(mode)
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
		case common.InterceptorStatusFail:
			return s.handleInterceptorError(result)
		case common.InterceptorStatusIncomplete:
			if !nodeLevel {
				logger.Error(execCtx.Ctx, "Interceptor returned INCOMPLETE in request-level mode",
					log.String("interceptorName", name),
					log.String("mode", string(mode)))
				return nil, serviceerror.CustomServiceError(interceptor.ErrorInterceptorExecution, i18ncore.I18nMessage{
					Key: "error.interceptor.execution_error_description",
					DefaultValue: fmt.Sprintf("Interceptor '%s' returned INCOMPLETE in request-level mode %s",
						name, mode),
				})
			}
			s.mergeInterceptorResponse(resp, result)
			resp.Status = common.InterceptorStatusIncomplete
			return resp, nil
		}
	}

	return resp, nil
}

// executeInterceptor builds the InterceptorContext and executes a single interceptor.
func (s *interceptorRunner) executeInterceptor(
	ic core.InterceptorInterface,
	name string,
	mode common.InterceptorMode,
	execCtx *InterceptorRunnerContext,
	logger *log.Logger,
) (*common.InterceptorResponse, *serviceerror.ServiceError) {
	node := execCtx.CurrentNode
	var currentNodeID string
	if node != nil {
		currentNodeID = node.GetID()
	}

	logger.Debug(execCtx.Ctx, "Running interceptor",
		log.String("interceptorName", name),
		log.String("mode", string(mode)),
		log.String("nodeID", currentNodeID))

	ctx := &core.InterceptorContext{
		Context:             execCtx.Ctx,
		ExecutionID:         execCtx.ExecutionID,
		AppID:               execCtx.AppID,
		FlowType:            execCtx.FlowType,
		Mode:                mode,
		UserInputs:          execCtx.UserInputs,
		CurrentNode:         node,
		AllowSegmentRestart: execCtx.AllowSegmentRestart,
		ForwardedData:       execCtx.ForwardedData,
		AdditionalData:      execCtx.AdditionalData,
		SharedData:          execCtx.SharedData,
	}
	if node != nil {
		ctx.CurrentNodeID = node.GetID()
		ctx.NodeType = node.GetType()
	}
	if ctx.SharedData == nil {
		ctx.SharedData = make(map[string]string)
	}

	result, err := ic.Execute(ctx)
	if err != nil {
		logger.Error(execCtx.Ctx, "Interceptor execution error",
			log.String("interceptorName", name), log.Error(err))
		return nil, serviceerror.CustomServiceError(interceptor.ErrorInterceptorExecution, i18ncore.I18nMessage{
			Key:          "error.interceptor.execution_error_description",
			DefaultValue: fmt.Sprintf("Interceptor '%s' failed: %s", name, err.Error()),
		})
	}

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
	result *common.InterceptorResponse,
) (*common.InterceptorResponse, *serviceerror.ServiceError) {
	// Handle ErrorChallengeTokenInvalid as a special case.
	if result.Error != nil && result.Error.Code == interceptor.ErrorChallengeTokenInvalid.Code {
		return nil, &ErrorInvalidChallengeToken
	}

	// For other errors, ensure a service error is returned.
	if failErr := result.Error; failErr == nil {
		result.Error = &interceptor.ErrorInterceptorFailed
	}
	return nil, result.Error
}

// shouldApplyToNode determines whether an interceptor should run for the given node.
// Default interceptors always apply and cannot be bypassed by node-level skip lists.
func shouldApplyToNode(
	interceptorUnit core.InterceptorUnitInterface, nodeID string, node core.NodeInterface,
) bool {
	if isDefaultInterceptorUnit(interceptorUnit.GetName()) {
		return true
	}

	switch interceptorUnit.GetScope() {
	case common.InterceptorScopeSelected:
		return slices.Contains(interceptorUnit.GetApplyTo(), nodeID)
	case common.InterceptorScopeAll:
		return !isSkippedByNode(interceptorUnit.GetName(), node)
	default:
		// No scope specified defaults to ALL.
		return !isSkippedByNode(interceptorUnit.GetName(), node)
	}
}

// isDefaultInterceptorUnit checks whether the given interceptor name matches any default interceptor.
func isDefaultInterceptorUnit(name string) bool {
	_, ok := interceptor.DefaultInterceptorNames[name]
	return ok
}

// isSkippedByNode checks whether the node's properties include a skipInterceptors list
// that contains the given interceptor name.
func isSkippedByNode(interceptorName string, node core.NodeInterface) bool {
	props := node.GetProperties()
	if props == nil {
		return false
	}
	val, ok := props[propertyKeySkipInterceptors]
	if !ok {
		return false
	}
	skipList, ok := val.([]interface{})
	if !ok {
		return false
	}
	for _, item := range skipList {
		if name, ok := item.(string); ok && name == interceptorName {
			return true
		}
	}
	return false
}

// isPerNodeMode returns true for PRE_NODE and POST_NODE modes.
func isPerNodeMode(mode common.InterceptorMode) bool {
	return mode == common.InterceptorModePreNode || mode == common.InterceptorModePostNode
}

// addDefaultInterceptorUnits returns baked units for all default interceptors.
// These are added at execution time, in addition to any configurable interceptors defined on the graph.
func (s *interceptorRunner) addDefaultInterceptorUnits(
	mode common.InterceptorMode,
) []core.InterceptorUnitInterface {
	declarations := make([]core.InterceptorUnitInterface, 0, len(interceptor.DefaultInterceptors))
	for _, b := range interceptor.DefaultInterceptors {
		if b.GetMode() != mode {
			continue
		}
		declarations = append(declarations, b)
	}
	return declarations
}

// validateConfiguredInterceptors checks that none of the configured interceptors are default ones.
func (s *interceptorRunner) validateConfiguredInterceptors(
	execCtx *InterceptorRunnerContext, logger *log.Logger,
) *serviceerror.ServiceError {
	for _, b := range execCtx.Interceptors {
		if isDefaultInterceptorUnit(b.GetName()) {
			logger.Error(execCtx.Ctx, "Default interceptor cannot be configured",
				log.String("interceptorName", b.GetName()))
			return serviceerror.CustomServiceError(interceptor.ErrorInterceptorExecution, i18ncore.I18nMessage{
				Key:          "error.interceptor.execution_error_description",
				DefaultValue: fmt.Sprintf("Default interceptor '%s' cannot be configured", b.GetName()),
			})
		}
	}
	return nil
}
