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

// Package graphbuilder builds executable flow graphs from flow definitions.
package graphbuilder

import (
	"context"

	"github.com/thunder-id/thunderid/internal/flow/core"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// GraphBuilderInterface builds and caches executable flow graphs from flow definitions.
type GraphBuilderInterface interface {
	GetGraph(ctx context.Context, flow *providers.CompleteFlowDefinition) (core.GraphInterface, *tidcommon.ServiceError)
	InvalidateCache(ctx context.Context, flowID string)
}
