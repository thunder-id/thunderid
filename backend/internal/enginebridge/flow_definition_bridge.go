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

package enginebridge

import (
	"context"

	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type flowDefinitionProviderBridge struct {
	provider thunderidengine.FlowDefinitionProvider
}

func (b *flowDefinitionProviderBridge) GetFlowByID(
	ctx context.Context, id string,
) (*flowmgt.HostFlowDefinition, error) {
	def, err := b.provider.GetFlowByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toHostFlowDefinition(def), nil
}

func (b *flowDefinitionProviderBridge) GetFlowByHandle(
	ctx context.Context, appID, handle string,
) (*flowmgt.HostFlowDefinition, error) {
	def, err := b.provider.GetFlowByHandle(ctx, appID, handle)
	if err != nil {
		return nil, err
	}
	return toHostFlowDefinition(def), nil
}

func toHostFlowDefinition(def *thunderidengine.FlowDefinition) *flowmgt.HostFlowDefinition {
	if def == nil {
		return nil
	}
	return &flowmgt.HostFlowDefinition{
		ID:       def.ID,
		Handle:   def.Handle,
		Name:     def.Name,
		FlowType: string(def.FlowType),
		Nodes:    def.Nodes,
	}
}

// NewRuntimeFlowDefinitionService adapts a public FlowDefinitionProvider for flow execution.
func NewRuntimeFlowDefinitionService(
	provider thunderidengine.FlowDefinitionProvider,
	graphBuilder flowmgt.GraphBuilder,
) flowmgt.FlowMgtServiceInterface {
	if provider == nil {
		return nil
	}
	return flowmgt.NewRuntimeFlowDefinitionService(&flowDefinitionProviderBridge{provider: provider}, graphBuilder)
}
