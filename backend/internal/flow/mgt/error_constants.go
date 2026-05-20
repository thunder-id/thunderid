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

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for flow management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1001",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingFlowID is the error returned when flow ID is missing.
	ErrorMissingFlowID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1002",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_id",
			DefaultValue: "Invalid flow ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_id_description",
			DefaultValue: "The flow ID must be provided",
		},
	}
	// ErrorFlowNotFound is the error returned when a flow is not found.
	ErrorFlowNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1003",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.flow_not_found",
			DefaultValue: "Flow not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.flow_not_found_description",
			DefaultValue: "The flow with the specified id does not exist",
		},
	}
	// ErrorInvalidFlowType is the error returned when flow type is invalid.
	ErrorInvalidFlowType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1004",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_type",
			DefaultValue: "Invalid flow type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_type_description",
			DefaultValue: "The specified flow type is invalid",
		},
	}
	// ErrorInvalidFlowData is the error returned when flow data is invalid.
	ErrorInvalidFlowData = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1005",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_data",
			DefaultValue: "Invalid flow data",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_data_description",
			DefaultValue: "The flow definition contains invalid data",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1006",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_limit_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1007",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_offset_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorVersionNotFound is the error returned when a flow version is not found.
	ErrorVersionNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1008",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.flow_version_not_found",
			DefaultValue: "Flow version not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.flow_version_not_found_description",
			DefaultValue: "The requested flow version does not exist",
		},
	}
	// ErrorInvalidVersion is the error returned when a flow version is invalid.
	ErrorInvalidVersion = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1009",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_version",
			DefaultValue: "Invalid flow version",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_version_description",
			DefaultValue: "The specified flow version is invalid",
		},
	}
	// ErrorMissingFlowHandle is the error returned when flow handle is missing.
	ErrorMissingFlowHandle = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1010",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_handle",
			DefaultValue: "Invalid flow handle",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_handle_description",
			DefaultValue: "The flow handle must be provided",
		},
	}
	// ErrorMissingFlowName is the error returned when flow name is missing.
	ErrorMissingFlowName = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1011",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_name",
			DefaultValue: "Invalid flow name",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_name_description",
			DefaultValue: "The flow name must be provided",
		},
	}
	// ErrorCannotUpdateFlowType is the error returned when trying to update flow type.
	ErrorCannotUpdateFlowType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1012",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.cannot_update_flow_type",
			DefaultValue: "Invalid update request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.cannot_update_flow_type_description",
			DefaultValue: "The flow type cannot be changed once created",
		},
	}
	// ErrorDuplicateFlowHandle is the error returned when a flow with the same handle and type already exists.
	ErrorDuplicateFlowHandle = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1013",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_flow_handle",
			DefaultValue: "Duplicate flow handle",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_flow_handle_description",
			DefaultValue: "A flow with this handle already exists for the given flow type",
		},
	}
	// ErrorHandleUpdateNotAllowed is the error returned when attempting to update an immutable handle.
	ErrorHandleUpdateNotAllowed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1014",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.handle_update_not_allowed",
			DefaultValue: "Invalid update request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.handle_update_not_allowed_description",
			DefaultValue: "The flow handle cannot be modified after creation",
		},
	}
	// ErrorInvalidFlowHandleFormat is the error returned when handle format is invalid.
	ErrorInvalidFlowHandleFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1015",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_handle_format",
			DefaultValue: "Invalid flow handle format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_handle_format_description",
			DefaultValue: "The flow handle must be lowercase, alphanumeric, and can only contain underscores or dashes",
		},
	}

	// ErrorGraphBuildFailure is the error returned when graph building fails.
	// TODO: This should be removed and instead should return InternalServerError
	// for graph build failures. Ideally there should be a graph validation step during
	// flow creation/update to catch such errors early.
	ErrorGraphBuildFailure = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1016",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.graph_build_failure",
			DefaultValue: "Graph build failure",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.graph_build_failure_description",
			DefaultValue: "Failed to build executable graph from flow definition",
		},
	}
	// ErrorFlowDeclarativeReadOnly is the error returned when trying to modify a declarative (immutable) flow.
	ErrorFlowDeclarativeReadOnly = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1017",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.flow_is_immutable",
			DefaultValue: "Flow is immutable",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.flow_is_immutable_description",
			DefaultValue: "Declarative flows cannot be modified or deleted",
		},
	}

	// ErrorInvalidFlowIDFormat is the error returned when a caller-provided flow ID is not a valid UUID.
	ErrorInvalidFlowIDFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1018",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_id_format",
			DefaultValue: "Invalid flow ID format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_id_format_description",
			DefaultValue: "The flow ID must be a valid UUID",
		},
	}

	// ErrorDuplicateFlowID is the error returned when a flow with the same ID already exists.
	ErrorDuplicateFlowID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "FLM-1019",
		Error: core.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_flow_id",
			DefaultValue: "Duplicate flow ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_flow_id_description",
			DefaultValue: "Flow ID already exists",
		},
	}
)

// Internal errors
var (
	errFlowNotFound    = errors.New("flow not found")
	errVersionNotFound = errors.New("version not found")
)
