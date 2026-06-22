/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/flowdef"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// FlowExecServiceInterface defines the interface for flow orchestration and acts as the
// entry point for flow execution.
type FlowExecServiceInterface interface {
	Execute(ctx context.Context, appID, executionID, flowType string, verbose bool,
		action string, inputs map[string]string, challengeToken string) (*FlowStep, *serviceerror.ServiceError)
	InitiateFlow(ctx context.Context, initContext *FlowInitContext) (string, *serviceerror.ServiceError)
	InitiateAndExecute(ctx context.Context, initContext *FlowInitContext) (*FlowStep, *serviceerror.ServiceError)
}

// FlowProviderInterface defines the flow management operations required for flow execution.
type FlowProviderInterface interface {
	GetFlowByHandle(ctx context.Context, handle string, flowType common.FlowType) (
		*flowdef.CompleteFlowDefinition, *serviceerror.ServiceError)
	GetGraph(ctx context.Context, flowID string) (core.GraphInterface, *serviceerror.ServiceError)
}

// FlowStoreInterface defines the methods for flow context storage operations.
type FlowStoreInterface interface {
	StoreFlowContext(ctx context.Context, dbModel FlowContextDB, expirySeconds int64) error
	GetFlowContext(ctx context.Context, executionID string) (*FlowContextDB, error)
	UpdateFlowContext(ctx context.Context, dbModel FlowContextDB) error
	DeleteFlowContext(ctx context.Context, executionID string) error
}
