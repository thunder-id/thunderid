/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// FlowProviderInterface defines the flow-definition methods flowexec requires at runtime.
// This is a consumer-side interface: flowexec declares what it needs from a flow definition
// source; flowmgt satisfies it via structural typing without importing flowexec.
type FlowProviderInterface interface {
	GetFlow(ctx context.Context, flowID string) (*common.CompleteFlowDefinition, *serviceerror.ServiceError)
	GetFlowByHandle(ctx context.Context, handle string, flowType common.FlowType) (
		*common.CompleteFlowDefinition, *serviceerror.ServiceError)
}
