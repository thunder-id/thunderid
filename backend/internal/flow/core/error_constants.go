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

package core

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Define core flow errors

// ErrExecutorPrerequisiteNotMet is returned when an executor prerequisite is not met.
var ErrExecutorPrerequisiteNotMet = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "FLC-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.flow.core.executor_prerequisite_not_met",
		DefaultValue: "A prerequisite for the executor was not met",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key: "error.flow.core.executor_prerequisite_not_met_description",
		DefaultValue: "One or more prerequisites required for the executor were not satisfied. " +
			"Please check the inputs and try again.",
	},
}

// ErrInvalidActionProvided is returned when an invalid action is provided in a prompt node.
var ErrInvalidActionProvided = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "FLC-1002",
	Error: tidcommon.I18nMessage{
		Key:          "error.flow.core.prompt_invalid_action",
		DefaultValue: "Invalid action provided",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flow.core.prompt_invalid_action_description",
		DefaultValue: "The action provided is not valid for the current flow step",
	},
}
