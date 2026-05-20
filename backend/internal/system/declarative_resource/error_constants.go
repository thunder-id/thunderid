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

package declarativeresource

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

var (
	// ErrorDeclarativeResourceCreateOperation is the error returned when
	// a declarative resource create operation is attempted.
	ErrorDeclarativeResourceCreateOperation = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "DCR-1001",
		Error: core.I18nMessage{
			Key:          "error.declarative_resource.create_operation_not_allowed",
			DefaultValue: "Declarative resource create operation is not allowed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.declarative_resource.create_operation_not_allowed_description",
			DefaultValue: "Creating declarative resources is not permitted",
		},
	}

	// ErrorDeclarativeResourceUpdateOperation is the error returned when
	// a declarative resource update operation is attempted.
	ErrorDeclarativeResourceUpdateOperation = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "DCR-1002",
		Error: core.I18nMessage{
			Key:          "error.declarative_resource.update_operation_not_allowed",
			DefaultValue: "Declarative resource update operation is not allowed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.declarative_resource.update_operation_not_allowed_description",
			DefaultValue: "Updating declarative resources is not permitted",
		},
	}

	// ErrorDeclarativeResourceDeleteOperation is the error returned when
	// a declarative resource delete operation is attempted.
	ErrorDeclarativeResourceDeleteOperation = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "DCR-1003",
		Error: core.I18nMessage{
			Key:          "error.declarative_resource.delete_operation_not_allowed",
			DefaultValue: "Declarative resource delete operation is not allowed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.declarative_resource.delete_operation_not_allowed_description",
			DefaultValue: "Deleting declarative resources is not permitted",
		},
	}
)
