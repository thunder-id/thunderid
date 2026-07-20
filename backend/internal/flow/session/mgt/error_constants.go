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

package mgt

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

var (
	// ErrorInvalidPaginationParams is returned when limit or offset is not a valid integer.
	ErrorInvalidPaginationParams = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1001",
		Error: tidcommon.I18nMessage{
			Key:          "session.mgt.error.invalid_pagination",
			DefaultValue: "Invalid pagination parameters",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "session.mgt.error.invalid_pagination_description",
			DefaultValue: "The limit and offset query parameters must be non-negative integers",
		},
	}

	// ErrorInvalidListFilter is returned unless exactly one of userId or appId is provided.
	ErrorInvalidListFilter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1002",
		Error: tidcommon.I18nMessage{
			Key:          "session.mgt.error.invalid_filter",
			DefaultValue: "Invalid session list filter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "session.mgt.error.invalid_filter_description",
			DefaultValue: "Exactly one of the userId or appId query parameters is required",
		},
	}

	// ErrorAuthenticationRequired is returned when the self listing has no authenticated subject.
	ErrorAuthenticationRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1003",
		Error: tidcommon.I18nMessage{
			Key:          "session.mgt.error.authentication_required",
			DefaultValue: "Authentication required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "session.mgt.error.authentication_required_description",
			DefaultValue: "An authenticated subject is required to list own sessions",
		},
	}

	// ErrorInternalServerError is returned for unexpected failures while listing sessions.
	ErrorInternalServerError = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "SSM-5001",
		Error: tidcommon.I18nMessage{
			Key:          "session.mgt.error.internal",
			DefaultValue: "Something went wrong",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "session.mgt.error.internal_description",
			DefaultValue: "An unexpected error occurred while listing sessions",
		},
	}
)
