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

package flowmgt

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for flow management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingFlowID is the error returned when flow ID is missing.
	ErrorMissingFlowID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_id",
			DefaultValue: "Invalid flow ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_id_description",
			DefaultValue: "The flow ID must be provided",
		},
	}
	// ErrorFlowNotFound is the error returned when a flow is not found.
	ErrorFlowNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_not_found",
			DefaultValue: "Flow not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_not_found_description",
			DefaultValue: "The flow with the specified id does not exist",
		},
	}
	// ErrorInvalidFlowType is the error returned when flow type is invalid.
	ErrorInvalidFlowType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_type",
			DefaultValue: "Invalid flow type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_type_description",
			DefaultValue: "The specified flow type is invalid",
		},
	}
	// ErrorInvalidFlowData is the error returned when flow data is invalid.
	ErrorInvalidFlowData = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_data",
			DefaultValue: "Invalid flow data",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_data_description",
			DefaultValue: "The flow definition contains invalid data",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_limit_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_offset_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorVersionNotFound is the error returned when a flow version is not found.
	ErrorVersionNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_version_not_found",
			DefaultValue: "Flow version not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_version_not_found_description",
			DefaultValue: "The requested flow version does not exist",
		},
	}
	// ErrorInvalidVersion is the error returned when a flow version is invalid.
	ErrorInvalidVersion = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_version",
			DefaultValue: "Invalid flow version",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_version_description",
			DefaultValue: "The specified flow version is invalid",
		},
	}
	// ErrorMissingFlowHandle is the error returned when flow handle is missing.
	ErrorMissingFlowHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_handle",
			DefaultValue: "Invalid flow handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_handle_description",
			DefaultValue: "The flow handle must be provided",
		},
	}
	// ErrorMissingFlowName is the error returned when flow name is missing.
	ErrorMissingFlowName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_name",
			DefaultValue: "Invalid flow name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_name_description",
			DefaultValue: "The flow name must be provided",
		},
	}
	// ErrorCannotUpdateFlowType is the error returned when trying to update flow type.
	ErrorCannotUpdateFlowType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.cannot_update_flow_type",
			DefaultValue: "Invalid update request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.cannot_update_flow_type_description",
			DefaultValue: "The flow type cannot be changed once created",
		},
	}
	// ErrorDuplicateFlowHandle is the error returned when a flow with the same handle and type already exists.
	ErrorDuplicateFlowHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_flow_handle",
			DefaultValue: "Duplicate flow handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_flow_handle_description",
			DefaultValue: "A flow with this handle already exists for the given flow type",
		},
	}
	// ErrorHandleUpdateNotAllowed is the error returned when attempting to update an immutable handle.
	ErrorHandleUpdateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.handle_update_not_allowed",
			DefaultValue: "Invalid update request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.handle_update_not_allowed_description",
			DefaultValue: "The flow handle cannot be modified after creation",
		},
	}
	// ErrorInvalidFlowHandleFormat is the error returned when handle format is invalid.
	ErrorInvalidFlowHandleFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_handle_format",
			DefaultValue: "Invalid flow handle format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_handle_format_description",
			DefaultValue: "The flow handle must be lowercase, alphanumeric, and can only contain underscores or dashes",
		},
	}
	// ErrorFlowDeclarativeReadOnly is the error returned when trying to modify a declarative (immutable) flow.
	ErrorFlowDeclarativeReadOnly = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_is_immutable",
			DefaultValue: "Flow is immutable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_is_immutable_description",
			DefaultValue: "Declarative flows cannot be modified or deleted",
		},
	}

	// ErrorInvalidFlowIDFormat is the error returned when a caller-provided flow ID is not a valid UUID.
	ErrorInvalidFlowIDFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_id_format",
			DefaultValue: "Invalid flow ID format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_id_format_description",
			DefaultValue: "The flow ID must be a valid UUID",
		},
	}

	// ErrorDuplicateFlowID is the error returned when a flow with the same ID already exists.
	ErrorDuplicateFlowID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_flow_id",
			DefaultValue: "Duplicate flow ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_flow_id_description",
			DefaultValue: "Flow ID already exists",
		},
	}
	// ErrorMissingStartNode is the error returned when the flow has no START node.
	ErrorMissingStartNode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_start_node",
			DefaultValue: "Missing start node",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_start_node_description",
			DefaultValue: "Flow definition must have exactly one START node",
		},
	}
	// ErrorMissingEndNode is the error returned when the flow has no END node.
	ErrorMissingEndNode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_end_node",
			DefaultValue: "Missing end node",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_end_node_description",
			DefaultValue: "Flow definition must have exactly one END node",
		},
	}
	// ErrorDuplicateStartNode is the error returned when the flow has multiple START nodes.
	ErrorDuplicateStartNode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_start_node",
			DefaultValue: "Duplicate start node",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_start_node_description",
			DefaultValue: "Flow definition must have exactly one START node, found multiple",
		},
	}
	// ErrorDuplicateEndNode is the error returned when the flow has multiple END nodes.
	ErrorDuplicateEndNode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1023",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_end_node",
			DefaultValue: "Duplicate end node",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_end_node_description",
			DefaultValue: "Flow definition must have exactly one END node, found multiple",
		},
	}
	// ErrorDuplicateNodeID is the error returned when duplicate node IDs are found.
	ErrorDuplicateNodeID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1024",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_node_id",
			DefaultValue: "Duplicate node ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_node_id_description",
			DefaultValue: "Flow definition contains duplicate node IDs",
		},
	}
	// ErrorInvalidNodeType is the error returned when a node has an invalid type.
	ErrorInvalidNodeType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_node_type",
			DefaultValue: "Invalid node type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_node_type_description",
			DefaultValue: "Node has an invalid type",
		},
	}
	// ErrorInvalidNodeReference is the error returned when a node references a non-existent node.
	ErrorInvalidNodeReference = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_node_reference",
			DefaultValue: "Invalid node reference",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_node_reference_description",
			DefaultValue: "Node references a non-existent node",
		},
	}
	// ErrorOrphanedNode is the error returned when a node is not reachable from the START node.
	ErrorOrphanedNode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1027",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.orphaned_node",
			DefaultValue: "Orphaned node",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.orphaned_node_description",
			DefaultValue: "Node is not reachable from the START node",
		},
	}
	// ErrorNoTermination is the error returned when the flow has cycles that prevent reaching the END node.
	ErrorNoTermination = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1028",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.no_termination",
			DefaultValue: "No termination path",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.no_termination_description",
			DefaultValue: "Flow has cycles that prevent reaching the END node",
		},
	}
	// ErrorTaskNodeMissingExecutor is the error returned when a TASK_EXECUTION node has no executor.
	ErrorTaskNodeMissingExecutor = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1029",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_missing_executor",
			DefaultValue: "Missing executor",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_missing_executor_description",
			DefaultValue: "TASK_EXECUTION node must have an executor",
		},
	}
	// ErrorTaskNodeMissingOnSuccess is the error returned when a TASK_EXECUTION node has no onSuccess.
	ErrorTaskNodeMissingOnSuccess = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1030",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_missing_on_success",
			DefaultValue: "Missing onSuccess",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_missing_on_success_description",
			DefaultValue: "TASK_EXECUTION node must have onSuccess",
		},
	}
	// ErrorTaskNodeInvalidFailureTarget is the error returned when onFailure does not point to a PROMPT node.
	ErrorTaskNodeInvalidFailureTarget = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1031",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_invalid_failure_target",
			DefaultValue: "Invalid onFailure target",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_invalid_failure_target_description",
			DefaultValue: "onFailure must point to a PROMPT node",
		},
	}
	// ErrorTaskNodeInvalidIncompleteTarget is the error returned when onIncomplete does not point to a PROMPT node.
	ErrorTaskNodeInvalidIncompleteTarget = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1032",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_invalid_incomplete_target",
			DefaultValue: "Invalid onIncomplete target",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_invalid_incomplete_target_description",
			DefaultValue: "onIncomplete must point to a PROMPT node",
		},
	}
	// ErrorPromptNodeInvalidConfig is the error returned when a PROMPT node has invalid configuration.
	ErrorPromptNodeInvalidConfig = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1033",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.prompt_node_invalid_config",
			DefaultValue: "Invalid prompt node configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.prompt_node_invalid_config_description",
			DefaultValue: "PROMPT node must have either prompts or next, not both or neither",
		},
	}
	// ErrorPromptMissingAction is the error returned when a prompt is missing an action.
	ErrorPromptMissingAction = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1034",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.prompt_missing_action",
			DefaultValue: "Missing prompt action",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.prompt_missing_action_description",
			DefaultValue: "Prompt must have an action with nextNode",
		},
	}
	// ErrorInvalidInputType is the error returned when an input has an invalid type.
	ErrorInvalidInputType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1035",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_input_type",
			DefaultValue: "Invalid input type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_input_type_description",
			DefaultValue: "Input has an invalid type",
		},
	}
	// ErrorInvalidValidationRule is the error returned when a validation rule is invalid.
	ErrorInvalidValidationRule = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1036",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_validation_rule",
			DefaultValue: "Invalid validation rule",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_validation_rule_description",
			DefaultValue: "Input has an invalid validation rule",
		},
	}
	// ErrorExecutorNotRegistered is the error returned when an executor is not registered.
	ErrorExecutorNotRegistered = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1037",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.executor_not_registered",
			DefaultValue: "Executor not registered",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.executor_not_registered_description",
			DefaultValue: "Executor is not registered",
		},
	}
	// ErrorInterceptorInvalidApplyTo is the error returned when interceptor applyTo references a non-existent node.
	ErrorInterceptorInvalidApplyTo = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1038",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.interceptor_invalid_apply_to",
			DefaultValue: "Invalid interceptor applyTo",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.interceptor_invalid_apply_to_description",
			DefaultValue: "Interceptor applyTo references a non-existent node",
		},
	}
	// ErrorExecutorForbiddenForFlowType is the error returned when a flow definition includes an executor
	// that is not permitted in the given flow type.
	ErrorExecutorForbiddenForFlowType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1039",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.executor_forbidden_for_flow_type",
			DefaultValue: "Executor not allowed in this flow type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.executor_forbidden_for_flow_type_description",
			DefaultValue: "The executor is not permitted for the current flow type",
		},
	}
	// ErrorRequiredExecutorMissing is the error returned when a flow definition is missing an executor
	// that is mandatory for the flow to fulfill its purpose.
	ErrorRequiredExecutorMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1040",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.required_executor_missing",
			DefaultValue: "Required executor missing",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.required_executor_missing_description",
			DefaultValue: "A required executor for this flow type is not present in the flow definition",
		},
	}
	// ErrorUnsupportedExecutorMode is the error returned when a node configures an executor mode
	// that the executor does not support.
	ErrorUnsupportedExecutorMode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1041",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.unsupported_executor_mode",
			DefaultValue: "Unsupported executor mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.unsupported_executor_mode_description",
			DefaultValue: "The executor mode is not supported by this executor",
		},
	}
	// ErrorUnsupportedExecutorFlowType is the error returned when an executor's declared SupportedFlowTypes
	// does not include the flow's type.
	ErrorUnsupportedExecutorFlowType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1042",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.unsupported_executor_flow_type",
			DefaultValue: "Executor not compatible with flow type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.unsupported_executor_flow_type_description",
			DefaultValue: "The executor does not support the current flow type",
		},
	}
	// ErrorMissingRequiredExecutorProperty is the error returned when a node is missing a property
	// that its executor requires to function correctly.
	ErrorMissingRequiredExecutorProperty = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1043",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_required_executor_property",
			DefaultValue: "Missing required executor property",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_required_executor_property_description",
			DefaultValue: "A required node property for this executor is missing or empty",
		},
	}
)

// Internal errors
var (
	errFlowNotFound    = errors.New("flow not found")
	errVersionNotFound = errors.New("version not found")
)
