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
	// ErrorInvalidFlowStructure is the error returned when the flow graph has structural issues
	// (missing/duplicate start or end nodes, duplicate node IDs, orphaned nodes, no termination path).
	ErrorInvalidFlowStructure = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_structure",
			DefaultValue: "Invalid flow structure",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_structure_description",
			DefaultValue: "Flow definition has structural issues",
		},
	}
	// ErrorInvalidNodeConfig is the error returned when a node has invalid configuration
	// (invalid type, missing required fields, invalid targets).
	ErrorInvalidNodeConfig = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_node_config",
			DefaultValue: "Invalid node configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_node_config_description",
			DefaultValue: "Node has invalid configuration",
		},
	}
	// ErrorInvalidNodeReference is the error returned when a node or interceptor references a non-existent node.
	ErrorInvalidNodeReference = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_node_reference",
			DefaultValue: "Invalid node reference",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_node_reference_description",
			DefaultValue: "References a non-existent node",
		},
	}
	// ErrorInvalidExecutorConfig is the error returned when an executor is missing or not registered.
	ErrorInvalidExecutorConfig = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1023",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_executor_config",
			DefaultValue: "Invalid executor configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_executor_config_description",
			DefaultValue: "Executor configuration is invalid",
		},
	}
	// ErrorInvalidInputConfig is the error returned when an input has an invalid type or validation rule.
	ErrorInvalidInputConfig = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1024",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_input_config",
			DefaultValue: "Invalid input configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_input_config_description",
			DefaultValue: "Input configuration is invalid",
		},
	}
	// ErrorCallTargetFlowNotFound is the error returned when a CALL node references a flow that does not exist.
	ErrorCallTargetFlowNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.call_target_flow_not_found",
			DefaultValue: "Call target flow not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.call_target_flow_not_found_description",
			DefaultValue: "A CALL node references a flow that does not exist",
		},
	}
	// ErrorFlowUpdateBlockedByDependent is the error returned when a flow update is rejected because
	// one or more dependent resources (e.g. applications) would end up in a conflicting state.
	ErrorFlowUpdateBlockedByDependent = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_update_blocked_by_dependent",
			DefaultValue: "Flow update conflicts with a resource that references this flow",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.flowmgtservice.flow_update_blocked_by_dependent_description",
			DefaultValue: "The flow update would leave a resource that references this flow in an " +
				"inconsistent state and has been rejected.",
		},
	}
)

// Internal errors
var (
	errFlowNotFound    = errors.New("flow not found")
	errVersionNotFound = errors.New("version not found")
)
