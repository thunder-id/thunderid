// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	// ErrorInvalidFlowOU is the error returned when the organization unit given for a flow does not exist.
	ErrorInvalidFlowOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1027",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_ou",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_flow_ou_description",
			DefaultValue: "The organization unit given for the flow does not exist",
		},
	}
	// ErrorOUUpdateNotAllowed is the error returned when attempting to change the organization unit of a flow.
	ErrorOUUpdateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1028",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.ou_update_not_allowed",
			DefaultValue: "Organization unit update not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.ou_update_not_allowed_description",
			DefaultValue: "The organization unit of a flow cannot be changed",
		},
	}
	// ErrorFlowOURequired is the error returned when a flow is created or updated without an organization unit.
	ErrorFlowOURequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1029",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_ou_required",
			DefaultValue: "Organization unit required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_ou_required_description",
			DefaultValue: "A flow needs an organization unit. Give its ouId in the request",
		},
	}
	// ErrorSharingPolicyNotFound is the error returned when a sharing policy is not found for the flow.
	ErrorSharingPolicyNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1030",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.sharing_policy_not_found",
			DefaultValue: "Sharing policy not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.sharing_policy_not_found_description",
			DefaultValue: "The sharing policy with the specified id does not exist for this flow",
		},
	}
	// ErrorSharingPolicyExists is the error returned when an organization unit already has a policy for the flow.
	ErrorSharingPolicyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1031",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.sharing_policy_exists",
			DefaultValue: "Sharing policy already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.sharing_policy_exists_description",
			DefaultValue: "The organization unit already has a sharing policy for this flow. Edit it instead",
		},
	}
	// ErrorSharingPolicyVersionMismatch is the error returned when a policy was edited since the caller read it.
	ErrorSharingPolicyVersionMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1032",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.sharing_policy_version_mismatch",
			DefaultValue: "Sharing policy version mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.sharing_policy_version_mismatch_description",
			DefaultValue: "The sharing policy was changed since it was read. Read it again and retry",
		},
	}
	// ErrorSharingPolicyDeclared is the error returned when a policy declared in a resource file is modified.
	ErrorSharingPolicyDeclared = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1033",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.sharing_policy_declared",
			DefaultValue: "Sharing policy is declared",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.sharing_policy_declared_description",
			DefaultValue: "The sharing policy is declared in a resource file and cannot be changed through the API",
		},
	}
	// ErrorInvalidSharingPolicy is the error returned when a sharing policy request is refused.
	ErrorInvalidSharingPolicy = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1034",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_sharing_policy",
			DefaultValue: "Invalid sharing policy",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_sharing_policy_description",
			DefaultValue: "The sharing policy request is not valid",
		},
	}
	// ErrorUnknownVariable is the error returned when a value names a variable the flow does not declare.
	ErrorUnknownVariable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1035",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.unknown_variable",
			DefaultValue: "Unknown variable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.flowmgtservice.unknown_variable_description",
			DefaultValue: "The flow does not declare the variable '{{param(variable)}}', or no sharing policy " +
				"names it for the organization unit",
		},
	}
	// ErrorInvalidVariableValue is the error returned when a value is not valid for its variable.
	ErrorInvalidVariableValue = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1036",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_variable_value",
			DefaultValue: "Invalid variable value",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_variable_value_description",
			DefaultValue: "The value of the variable '{{param(variable)}}' is not valid: {{param(reason)}}",
		},
	}
	// ErrorVariableNotEditable is the error returned when an organization unit sets a variable it may not set.
	ErrorVariableNotEditable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1037",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.variable_not_editable",
			DefaultValue: "Variable cannot be set",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.variable_not_editable_description",
			DefaultValue: "The organization unit may not set the variable '{{param(variable)}}'",
		},
	}
	// ErrorMissingOUID is the error returned when a request does not name the organization unit to act for.
	ErrorMissingOUID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1038",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_ou_id",
			DefaultValue: "Organization unit required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_ou_id_description",
			DefaultValue: "The ouId is required, in the query for a read and in the request body for a change",
		},
	}
	// ErrorSharedFlowReadOnly is the error returned when a change is made to a flow through an organization unit it was
	// only shared with.
	ErrorSharedFlowReadOnly = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1039",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.shared_flow_read_only",
			DefaultValue: "Shared flow cannot be modified",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.flowmgtservice.shared_flow_read_only_description",
			DefaultValue: "The flow is owned by another organization unit and can only be configured through " +
				"its variables",
		},
	}
	// ErrorInvalidFilter is returned when a flow listing carries a filter it cannot apply.
	ErrorInvalidFilter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1040",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_filter_parameter",
			DefaultValue: "Invalid filter parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_filter_parameter_description",
			DefaultValue: "The filter must be a single ouId eq expression with a quoted organization unit ID",
		},
	}
	// ErrorFlowConflictsWithSharingPolicy is returned when a change to the variables of a flow would break a sharing
	// policy recorded for it.
	ErrorFlowConflictsWithSharingPolicy = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1041",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_conflicts_with_sharing_policy",
			DefaultValue: "Flow conflicts with a sharing policy",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_conflicts_with_sharing_policy_description",
			DefaultValue: "The change breaks a sharing policy of the flow. Update or delete the policy first",
		},
	}
	// ErrorFlowConflictsWithSavedValues is returned when a change to the variables of a flow would invalidate a
	// value an organization unit has saved for one.
	ErrorFlowConflictsWithSavedValues = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1042",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_conflicts_with_saved_values",
			DefaultValue: "Flow conflicts with saved values",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.flowmgtservice.flow_conflicts_with_saved_values_description",
			DefaultValue: "The change invalidates values that organization units saved for the variables. Ask " +
				"them to change those values, or update or delete the sharing policy that reaches them, first",
		},
	}
	// ErrorDeploymentWideSharingNotAllowed is returned when a create through the API states a section whose reach is
	// not bounded by where the issuing organization unit sits in the tree.
	ErrorDeploymentWideSharingNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FLM-1043",
		Error: tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.deployment_wide_sharing_not_allowed",
			DefaultValue: "Deployment-wide sharing is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.flowmgtservice.deployment_wide_sharing_not_allowed_description",
			DefaultValue: "A policy reaching all organization units or the roots of all other trees " +
				"can only be created from a declarative resource file or an import",
		},
	}
)

// Internal errors
var (
	errFlowNotFound    = errors.New("flow not found")
	errVersionNotFound = errors.New("version not found")
)
