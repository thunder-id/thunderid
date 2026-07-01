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

package authzen

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrorInvalidRequestFormat is returned when the request JSON is malformed.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingSubject is returned when subject id is not provided.
	ErrorMissingSubject = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_subject",
			DefaultValue: "Missing subject",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_subject_description",
			DefaultValue: "Subject id is required",
		},
	}
	// ErrorMissingResource is returned when resource type is not provided.
	ErrorMissingResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_resource",
			DefaultValue: "Missing resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_resource_description",
			DefaultValue: "Resource type is required",
		},
	}
	// ErrorMissingResourceID is returned when resource id is not provided.
	ErrorMissingResourceID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_resource_id",
			DefaultValue: "Missing resource id",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_resource_id_description",
			DefaultValue: "Resource id is required",
		},
	}
	// ErrorMissingAction is returned when action name is not provided.
	ErrorMissingAction = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_action",
			DefaultValue: "Missing action",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_action_description",
			DefaultValue: "Action name is required",
		},
	}
	// ErrorMissingEvaluations is returned when batch request has no evaluations.
	ErrorMissingEvaluations = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_evaluations",
			DefaultValue: "Missing evaluations",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.missing_evaluations_description",
			DefaultValue: "At least one evaluation is required",
		},
	}
	// ErrorInvalidSubject is returned when subject type does not match subject id.
	ErrorInvalidSubject = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.invalid_subject",
			DefaultValue: "Invalid subject",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.invalid_subject_description",
			DefaultValue: "Subject id does not match subject type",
		},
	}
	// ErrorInvalidAction is returned when action is not registered on the resource server.
	ErrorInvalidAction = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.invalid_action",
			DefaultValue: "Invalid action",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.invalid_action_description",
			DefaultValue: "Action name is not registered on the resource server",
		},
	}
	// ErrorInvalidResource is returned when resource type does not resolve to a resource server.
	ErrorInvalidResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AZN-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.authzen.invalid_resource",
			DefaultValue: "Invalid resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authzen.invalid_resource_description",
			DefaultValue: "Resource type is not registered as a resource server",
		},
	}
)
