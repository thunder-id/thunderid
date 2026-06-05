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

// Package flowbuilder provides conversion and graph-building components for flow definitions.
package flowbuilder

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
)

var (
	// ErrorInvalidFlowData is returned when a flow definition is nil/invalid for graph generation.
	ErrorInvalidFlowData = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLG-1001",
		Error: i18ncore.I18nMessage{
			Key:          "error.flowgraph.invalid_flow_data",
			DefaultValue: "Invalid flow data",
		},
		ErrorDescription: i18ncore.I18nMessage{
			Key:          "error.flowgraph.invalid_flow_data_description",
			DefaultValue: "The flow definition contains invalid data",
		},
	}

	// ErrorGraphBuildFailure is returned when graph construction fails.
	ErrorGraphBuildFailure = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLG-1002",
		Error: i18ncore.I18nMessage{
			Key:          "error.flowgraph.graph_build_failure",
			DefaultValue: "Graph build failure",
		},
		ErrorDescription: i18ncore.I18nMessage{
			Key:          "error.flowgraph.graph_build_failure_description",
			DefaultValue: "Failed to build executable graph from flow definition",
		},
	}
)
