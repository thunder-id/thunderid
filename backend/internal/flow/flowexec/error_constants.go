// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client error structs

// APIErrorFlowRequestJSONDecodeError defines the error response for json decode errors.
var APIErrorFlowRequestJSONDecodeError = apierror.ErrorResponse{
	Code: "FES-1001",
	Message: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_request_payload",
		DefaultValue: "Invalid request payload",
	},
	Description: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_request_payload_description",
		DefaultValue: "Failed to decode request payload",
	},
}

// APIErrorInvalidSessionData defines the error response for a sessionData entry missing its flow
// identifier or session handle.
var APIErrorInvalidSessionData = apierror.ErrorResponse{
	Code: "FES-1021",
	Message: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_session_data",
		DefaultValue: "Invalid session data",
	},
	Description: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_session_data_description",
		DefaultValue: "Each session data entry needs a flow identifier and a session handle",
	},
}

// ErrorNodeResponse defines the error response for errors received from nodes.
var ErrorNodeResponse = tidcommon.ServiceError{
	Code: "FES-1002",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_node_response",
		DefaultValue: "Invalid node response",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_node_response_description",
		DefaultValue: "Error response received from the node",
	},
}

// ErrorInvalidAppID defines the error response for invalid app ID errors.
var ErrorInvalidAppID = tidcommon.ServiceError{
	Code: "FES-1003",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_app_id",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_app_id_description",
		DefaultValue: "Invalid app ID provided in the request",
	},
}

// ErrorInvalidExecutionID defines the error response for invalid execution ID errors.
var ErrorInvalidExecutionID = tidcommon.ServiceError{
	Code: "FES-1004",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_execution_id",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_execution_id_description",
		DefaultValue: "Invalid flow execution ID provided in the request",
	},
}

// ErrorInvalidFlowType defines the error response for invalid flow type errors.
var ErrorInvalidFlowType = tidcommon.ServiceError{
	Code: "FES-1005",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_flow_type",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_flow_type_description",
		DefaultValue: "Invalid flow type provided in the request",
	},
}

// ErrorRegistrationFlowDisabled defines the error response for registration flow disabled errors.
var ErrorRegistrationFlowDisabled = tidcommon.ServiceError{
	Code: "FES-1006",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.registration_not_allowed",
		DefaultValue: "Registration not allowed",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.registration_not_allowed_description",
		DefaultValue: "Registration flow is disabled for the application",
	},
}

// ErrorApplicationRetrievalClientError defines the error response for application retrieval client errors.
var ErrorApplicationRetrievalClientError = tidcommon.ServiceError{
	Code: "FES-1007",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.application_retrieval_error",
		DefaultValue: "Application retrieval error",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.application_retrieval_error_description",
		DefaultValue: "Error while retrieving application details",
	},
}

// ErrorInvalidFlowInitContext defines the error response for invalid flow init context.
var ErrorInvalidFlowInitContext = tidcommon.ServiceError{
	Code: "FES-1008",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_flow_init_context",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.invalid_flow_init_context_description",
		DefaultValue: "Invalid flow initialization context provided",
	},
}

// ErrorRecoveryFlowDisabled defines the error response for recovery flow disabled errors.
var ErrorRecoveryFlowDisabled = tidcommon.ServiceError{
	Code: "FES-1009",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.recovery_not_allowed",
		DefaultValue: "Recovery not allowed",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.recovery_not_allowed_description",
		DefaultValue: "Recovery flow is disabled for the application",
	},
}

// ErrorDirectFlowInitiationNotPermitted defines the error for applications that do not allow
// direct flow initiation via the HTTP endpoint (e.g. authorization_code grant type apps).
var ErrorDirectFlowInitiationNotPermitted = tidcommon.ServiceError{
	Code: "FES-1010",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.direct_flow_initiation_not_permitted",
		DefaultValue: "Direct flow initiation not permitted",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.direct_flow_initiation_not_permitted_description",
		DefaultValue: "Direct flow initiation is not permitted for this application type",
	},
}

// ErrorFlowSecretRequired defines the error when a backend application initiates a new flow
// without providing its Flow Secret.
var ErrorFlowSecretRequired = tidcommon.ServiceError{
	Code: "FES-1011",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.flow_secret_required",
		DefaultValue: "Authentication required",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.flow_secret_required_description",
		DefaultValue: "Backend applications must provide a valid flow secret to initiate a new flow",
	},
}

// ErrorFlowSecretInvalid defines the error when a backend application provides an incorrect
// Flow Secret when initiating a new flow.
var ErrorFlowSecretInvalid = tidcommon.ServiceError{
	Code: "FES-1012",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.flow_secret_invalid",
		DefaultValue: "Authentication failed",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.flow_secret_invalid_description",
		DefaultValue: "The provided flow secret is invalid",
	},
}

// ErrorMaxCallDepthExceeded defines the error when the maximum allowed call depth is exceeded
// during flow execution.
var ErrorMaxCallDepthExceeded = tidcommon.ServiceError{
	Code: "FES-1013",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.max_call_depth_exceeded",
		DefaultValue: "Maximum call depth exceeded",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.max_call_depth_exceeded_description",
		DefaultValue: "The maximum allowed call depth has been exceeded during flow execution",
	},
}

// ErrorAttestationRequired defines the error when a mobile application initiates a new flow without
// presenting a platform attestation token.
var ErrorAttestationRequired = tidcommon.ServiceError{
	Code: "FES-1014",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.attestation_required",
		DefaultValue: "Authentication required",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.attestation_required_description",
		DefaultValue: "Mobile applications must present a valid attestation token to initiate a new flow",
	},
}

// ErrorAttestationInvalid defines the error when a mobile application presents an attestation token
// that fails verification.
var ErrorAttestationInvalid = tidcommon.ServiceError{
	Code: "FES-1015",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.attestation_invalid",
		DefaultValue: "Authentication failed",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.attestation_invalid_description",
		DefaultValue: "The provided attestation token is invalid",
	},
}

// ErrorAttestationNotConfigured defines the error when a mobile application initiates a flow but has
// no platform attestation configured. Mobile apps must configure attestation to access the flow APIs.
var ErrorAttestationNotConfigured = tidcommon.ServiceError{
	Code: "FES-1016",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.attestation_not_configured",
		DefaultValue: "Attestation not configured",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.attestation_not_configured_description",
		DefaultValue: "Mobile applications must configure platform attestation to initiate a flow",
	},
}

// ErrorAdministrationAuthenticationRequired defines the error returned when an administrative
// flow is invoked without an authenticated caller.
var ErrorAdministrationAuthenticationRequired = tidcommon.ServiceError{
	Code: "FES-1017",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.administration_authentication_required",
		DefaultValue: "Authentication required",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.administration_authentication_required_description",
		DefaultValue: "Administrative flows require an authenticated caller",
	},
}

// ErrorFlowIDExecutionNotPermitted is returned when direct execution by ID targets a non-administrative flow.
var ErrorFlowIDExecutionNotPermitted = tidcommon.ServiceError{
	Code: "FES-1018",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.flow_id_execution_not_permitted",
		DefaultValue: "Flow execution not permitted",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.flow_id_execution_not_permitted_description",
		DefaultValue: "The selected flow cannot be executed directly by its ID",
	},
}

// ErrorAdministrationPermissionRequired defines the error returned when an authenticated caller
// invokes an administrative flow without the root system permission.
var ErrorAdministrationPermissionRequired = tidcommon.ServiceError{
	Code: "FES-1019",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.administration_permission_required",
		DefaultValue: "Insufficient permissions",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.administration_permission_required_description",
		DefaultValue: "Administrative flows require the system permission",
	},
}

// ErrorFlowApplicationNotUsable is returned when the application driving a flow is suspended. It
// does not say why.
var ErrorFlowApplicationNotUsable = tidcommon.ServiceError{
	Code: "FES-1020",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.application_not_usable",
		DefaultValue: "Flow execution not permitted",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.application_not_usable_description",
		DefaultValue: "The flow cannot be executed for the requested application",
	},
}

// ErrorFlowConnectionNotSet is returned when a flow needs a connection that was never set for the organization unit of
// the application driving it.
var ErrorFlowConnectionNotSet = tidcommon.ServiceError{
	Code: "FES-1022",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.connection_not_set",
		DefaultValue: "Flow not available",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowexecservice.connection_not_set_description",
		DefaultValue: "The flow is not available for this organization yet. Contact your administrator",
	},
}
