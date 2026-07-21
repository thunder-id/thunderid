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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// FlowExecServiceInterface defines the interface for flow orchestration and acts as the
// entry point for flow execution.
type FlowExecServiceInterface interface {
	Execute(ctx context.Context, appID, executionID, flowType string, verbose bool,
		action string, inputs map[string]string, challengeToken, flowSecret, attestationToken string) (
		*FlowStep, *tidcommon.ServiceError)
	InitiateFlow(ctx context.Context, initContext *FlowInitContext) (string, *tidcommon.ServiceError)
	InitiateAndExecute(ctx context.Context, initContext *FlowInitContext) (*FlowStep, *tidcommon.ServiceError)
}
