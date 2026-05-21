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

package flowmgt

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// GraphBuilder builds executable graphs from flow definitions.
type GraphBuilder interface {
	GetGraph(ctx context.Context, flow *CompleteFlowDefinition) (core.GraphInterface, *serviceerror.ServiceError)
	InvalidateCache(ctx context.Context, flowID string)
}

// NewGraphBuilder creates a graph builder for runtime flow execution.
func NewGraphBuilder(
	flowFactory core.FlowFactoryInterface,
	executorRegistry executor.ExecutorRegistryInterface,
	graphCache core.GraphCacheInterface,
) GraphBuilder {
	return newGraphBuilder(flowFactory, executorRegistry, graphCache)
}

// RuntimeFlowDefinitionService adapts a host FlowDefinitionProvider for flow execution.
type RuntimeFlowDefinitionService struct {
	provider     HostFlowDefinitionProvider
	graphBuilder GraphBuilder
}

// NewRuntimeFlowDefinitionService creates a flow management adapter backed by a host provider.
func NewRuntimeFlowDefinitionService(
	provider HostFlowDefinitionProvider,
	graphBuilder GraphBuilder,
) FlowMgtServiceInterface {
	return &RuntimeFlowDefinitionService{
		provider:     provider,
		graphBuilder: graphBuilder,
	}
}

// ListFlows is unsupported on the runtime host adapter.
func (s *RuntimeFlowDefinitionService) ListFlows(
	_ context.Context, _, _ int, _ common.FlowType,
) (*FlowListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

// CreateFlow is unsupported on the runtime host adapter.
func (s *RuntimeFlowDefinitionService) CreateFlow(
	_ context.Context, _ *FlowDefinition,
) (*CompleteFlowDefinition, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

// GetFlow loads a flow definition from the host provider by ID.
func (s *RuntimeFlowDefinitionService) GetFlow(ctx context.Context, flowID string) (
	*CompleteFlowDefinition, *serviceerror.ServiceError,
) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}
	def, err := s.provider.GetFlowByID(ctx, flowID)
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	return hostFlowToComplete(def)
}

// GetFlowByHandle loads a flow definition from the host provider by handle.
func (s *RuntimeFlowDefinitionService) GetFlowByHandle(
	ctx context.Context, handle string, flowType common.FlowType,
) (*CompleteFlowDefinition, *serviceerror.ServiceError) {
	if handle == "" {
		return nil, &ErrorMissingFlowHandle
	}
	if !isValidFlowType(flowType) {
		return nil, &ErrorInvalidFlowType
	}
	def, err := s.provider.GetFlowByHandle(ctx, "", handle)
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	complete, svcErr := hostFlowToComplete(def)
	if svcErr != nil {
		return nil, svcErr
	}
	if complete.FlowType != flowType {
		return nil, &ErrorFlowNotFound
	}
	return complete, nil
}

// UpdateFlow is unsupported on the runtime host adapter.
func (s *RuntimeFlowDefinitionService) UpdateFlow(
	_ context.Context, _ string, _ *FlowDefinition,
) (*CompleteFlowDefinition, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

// DeleteFlow is unsupported on the runtime host adapter.
func (s *RuntimeFlowDefinitionService) DeleteFlow(_ context.Context, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

// ListFlowVersions is unsupported on the runtime host adapter.
func (s *RuntimeFlowDefinitionService) ListFlowVersions(
	_ context.Context, _ string,
) (*FlowVersionListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

// GetFlowVersion is unsupported on the runtime host adapter.
func (s *RuntimeFlowDefinitionService) GetFlowVersion(
	_ context.Context, _ string, _ int,
) (*FlowVersion, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

// RestoreFlowVersion is unsupported on the runtime host adapter.
func (s *RuntimeFlowDefinitionService) RestoreFlowVersion(
	_ context.Context, _ string, _ int,
) (*CompleteFlowDefinition, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

// GetGraph builds an executable graph for the given flow ID.
func (s *RuntimeFlowDefinitionService) GetGraph(ctx context.Context, flowID string) (
	core.GraphInterface, *serviceerror.ServiceError,
) {
	flow, svcErr := s.GetFlow(ctx, flowID)
	if svcErr != nil {
		return nil, svcErr
	}
	return s.graphBuilder.GetGraph(ctx, flow)
}

// IsValidFlow reports whether the flow exists and matches the requested type.
func (s *RuntimeFlowDefinitionService) IsValidFlow(
	ctx context.Context, flowID string, flowType common.FlowType,
) (bool, *serviceerror.ServiceError) {
	flow, svcErr := s.GetFlow(ctx, flowID)
	if svcErr != nil {
		return false, svcErr
	}
	return flow != nil && flow.FlowType == flowType, nil
}

func hostFlowToComplete(def *HostFlowDefinition) (*CompleteFlowDefinition, *serviceerror.ServiceError) {
	if def == nil {
		return nil, &ErrorFlowNotFound
	}
	var nodes []NodeDefinition
	if len(def.Nodes) > 0 {
		if err := json.Unmarshal(def.Nodes, &nodes); err != nil {
			return nil, &serviceerror.InternalServerError
		}
	}
	return &CompleteFlowDefinition{
		ID:       def.ID,
		Handle:   def.Handle,
		Name:     def.Name,
		FlowType: common.FlowType(def.FlowType),
		Nodes:    nodes,
	}, nil
}
