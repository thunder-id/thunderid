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

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

var (
	errorInvalidFlowData = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLG-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.flow.graphbuilder.invalid_flow_data",
			DefaultValue: "Invalid flow data",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flow.graphbuilder.invalid_flow_data_description",
			DefaultValue: "The flow definition contains invalid data",
		},
	}
	errorGraphBuildFailure = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLG-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.flow.graphbuilder.graph_build_failure",
			DefaultValue: "Graph build failure",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flow.graphbuilder.graph_build_failure_description",
			DefaultValue: "Failed to build the flow graph",
		},
	}
)
