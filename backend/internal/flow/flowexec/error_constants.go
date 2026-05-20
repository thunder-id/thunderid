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

package flowexec

import (
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client error structs

// APIErrorFlowRequestJSONDecodeError defines the error response for json decode errors.
var APIErrorFlowRequestJSONDecodeError = apierror.ErrorResponse{
	Code: "FES-1001",
	Message: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_request_payload",
		DefaultValue: "Invalid request payload",
	},
	Description: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_request_payload_description",
		DefaultValue: "Failed to decode request payload",
	},
}

// ErrorNodeResponse defines the error response for errors received from nodes.
var ErrorNodeResponse = serviceerror.ServiceError{
	Code: "FES-1002",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_node_response",
		DefaultValue: "Invalid node response",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_node_response_description",
		DefaultValue: "Error response received from the node",
	},
}

// ErrorInvalidAppID defines the error response for invalid app ID errors.
var ErrorInvalidAppID = serviceerror.ServiceError{
	Code: "FES-1003",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_app_id",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_app_id_description",
		DefaultValue: "Invalid app ID provided in the request",
	},
}

// ErrorInvalidExecutionID defines the error response for invalid execution ID errors.
var ErrorInvalidExecutionID = serviceerror.ServiceError{
	Code: "FES-1004",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_execution_id",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_execution_id_description",
		DefaultValue: "Invalid flow execution ID provided in the request",
	},
}

// ErrorInvalidFlowType defines the error response for invalid flow type errors.
var ErrorInvalidFlowType = serviceerror.ServiceError{
	Code: "FES-1005",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_flow_type",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_flow_type_description",
		DefaultValue: "Invalid flow type provided in the request",
	},
}

// ErrorRegistrationFlowDisabled defines the error response for registration flow disabled errors.
var ErrorRegistrationFlowDisabled = serviceerror.ServiceError{
	Code: "FES-1006",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.registration_not_allowed",
		DefaultValue: "Registration not allowed",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.registration_not_allowed_description",
		DefaultValue: "Registration flow is disabled for the application",
	},
}

// ErrorRecoveryFlowDisabled defines the error response for recovery flow disabled errors.
var ErrorRecoveryFlowDisabled = serviceerror.ServiceError{
	Code: "FES-1010",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.recovery_not_allowed",
		DefaultValue: "Recovery not allowed",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.recovery_not_allowed_description",
		DefaultValue: "Recovery flow is disabled for the application",
	},
}

// ErrorApplicationRetrievalClientError defines the error response for application retrieval client errors.
var ErrorApplicationRetrievalClientError = serviceerror.ServiceError{
	Code: "FES-1007",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.application_retrieval_error",
		DefaultValue: "Application retrieval error",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.application_retrieval_error_description",
		DefaultValue: "Error while retrieving application details",
	},
}

// ErrorInvalidFlowInitContext defines the error response for invalid flow init context.
var ErrorInvalidFlowInitContext = serviceerror.ServiceError{
	Code: "FES-1008",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_flow_init_context",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_flow_init_context_description",
		DefaultValue: "Invalid flow initialization context provided",
	},
}

// ErrorInvalidChallengeToken defines the error response for invalid or missing challenge tokens.
var ErrorInvalidChallengeToken = serviceerror.ServiceError{
	Code: "FES-1009",
	Type: serviceerror.ClientErrorType,
	Error: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_challenge_token",
		DefaultValue: "Invalid challenge token",
	},
	ErrorDescription: core.I18nMessage{
		Key:          "error.flowexecservice.invalid_challenge_token_description",
		DefaultValue: "The challenge token is missing or invalid",
	},
}
