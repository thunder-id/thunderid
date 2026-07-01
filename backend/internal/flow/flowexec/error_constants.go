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

// ErrorRecoveryFlowDisabled defines the error response for recovery flow disabled errors.
var ErrorRecoveryFlowDisabled = tidcommon.ServiceError{
	Code: "FES-1010",
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

// ErrorDirectFlowInitiationNotPermitted defines the error for applications that do not allow
// direct flow initiation via the HTTP endpoint (e.g. authorization_code grant type apps).
var ErrorDirectFlowInitiationNotPermitted = tidcommon.ServiceError{
	Code: "FES-1011",
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
