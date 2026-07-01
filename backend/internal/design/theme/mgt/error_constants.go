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

package thememgt

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrorInvalidThemeData is returned when invalid theme data is provided.
	ErrorInvalidThemeData = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1001",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_data",
			DefaultValue: "Invalid theme data",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_data_description",
			DefaultValue: "The provided theme data is invalid",
		},
	}

	// ErrorInvalidThemeID is returned when an invalid theme ID is provided.
	ErrorInvalidThemeID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1002",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_id",
			DefaultValue: "Invalid theme ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_id_description",
			DefaultValue: "The provided theme ID is invalid",
		},
	}

	// ErrorThemeNotFound is returned when a theme is not found.
	ErrorThemeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1003",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.not_found",
			DefaultValue: "Theme not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.not_found_description",
			DefaultValue: "The requested theme configuration was not found",
		},
	}

	// ErrorThemeInUse is returned when trying to delete a theme that is being used by applications.
	ErrorThemeInUse = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1004",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.in_use",
			DefaultValue: "Theme in use",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.in_use_description",
			DefaultValue: "Cannot delete theme that is currently associated with one or more applications",
		},
	}

	// ErrorMissingDisplayName is returned when display name is not provided.
	ErrorMissingDisplayName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1005",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.missing_display_name",
			DefaultValue: "Missing display name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.missing_display_name_description",
			DefaultValue: "Display name is required",
		},
	}

	// ErrorMissingTheme is returned when theme field is not provided.
	ErrorMissingTheme = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1006",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.missing_theme",
			DefaultValue: "Missing theme",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.missing_theme_description",
			DefaultValue: "Theme field is required",
		},
	}

	// ErrorInvalidThemeFormat is returned when theme JSON is invalid.
	ErrorInvalidThemeFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1007",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_format",
			DefaultValue: "Invalid theme format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_format_description",
			DefaultValue: "Theme must be a valid JSON object",
		},
	}

	// ErrorInvalidLimitValue is returned when limit validation fails in service layer.
	ErrorInvalidLimitValue = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1008",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_limit",
			DefaultValue: "Invalid limit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_limit_description",
			DefaultValue: "Limit value is out of valid range",
		},
	}

	// ErrorInvalidOffsetValue is returned when offset validation fails in service layer.
	ErrorInvalidOffsetValue = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1009",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_offset",
			DefaultValue: "Invalid offset",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_offset_description",
			DefaultValue: "Offset must be non-negative",
		},
	}

	// ErrorInvalidLimitParam is returned when limit parameter cannot be parsed.
	ErrorInvalidLimitParam = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1010",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_limit_param",
			DefaultValue: "Invalid limit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_limit_param_description",
			DefaultValue: "Limit must be a valid integer",
		},
	}

	// ErrorInvalidOffsetParam is returned when offset parameter cannot be parsed.
	ErrorInvalidOffsetParam = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1011",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_offset_param",
			DefaultValue: "Invalid offset",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.invalid_offset_param_description",
			DefaultValue: "Offset must be a valid integer",
		},
	}

	// ErrorCannotUpdateDeclarativeTheme is returned when attempting to update a declarative theme.
	ErrorCannotUpdateDeclarativeTheme = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1012",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.cannot_update_declarative",
			DefaultValue: "Cannot update declarative theme",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.cannot_update_declarative_description",
			DefaultValue: "Theme is defined in declarative resources and cannot be modified",
		},
	}

	// ErrorCannotDeleteDeclarativeTheme is returned when attempting to delete a declarative theme.
	ErrorCannotDeleteDeclarativeTheme = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1013",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.cannot_delete_declarative",
			DefaultValue: "Cannot delete declarative theme",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.cannot_delete_declarative_description",
			DefaultValue: "Theme is defined in declarative resources and cannot be deleted",
		},
	}

	// ErrorResultLimitExceededInCompositeMode is returned when composite store result count exceeds max limit.
	ErrorResultLimitExceededInCompositeMode = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "THM-5001",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.result_limit_exceeded_description",
			DefaultValue: "Total count of themes exceeds maximum allowed limit in composite mode",
		},
	}

	// ErrorCannotModifyDeclarativeResource is returned when attempting to modify a declarative theme.
	ErrorCannotModifyDeclarativeResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1014",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.cannot_modify_declarative",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.cannot_modify_declarative_description",
			DefaultValue: "The theme is declarative and cannot be modified or deleted",
		},
	}

	// ErrorDuplicateThemeHandle is returned when a theme with the same handle already exists.
	ErrorDuplicateThemeHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1015",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.duplicate_handle",
			DefaultValue: "Duplicate theme handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.duplicate_handle_description",
			DefaultValue: "A theme with the same handle already exists",
		},
	}

	// ErrorMissingThemeHandle is returned when handle is not provided.
	ErrorMissingThemeHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1016",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.missing_handle",
			DefaultValue: "Missing theme handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.missing_handle_description",
			DefaultValue: "Theme handle is required",
		},
	}

	// ErrorThemeHandleImmutable is returned when attempting to change the handle of an existing theme.
	ErrorThemeHandleImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "THM-1017",
		Error: tidcommon.I18nMessage{
			Key:          "theme.error.handle_immutable",
			DefaultValue: "Theme handle is immutable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "theme.error.handle_immutable_description",
			DefaultValue: "The theme handle cannot be changed after creation",
		},
	}
)

// errCannotUpdateDeclarativeTheme is an internal error for composite store operations.
var errCannotUpdateDeclarativeTheme = errors.New("cannot update declarative theme")

// errCannotDeleteDeclarativeTheme is an internal error for composite store operations.
var errCannotDeleteDeclarativeTheme = errors.New("cannot delete declarative theme")

// errResultLimitExceededInCompositeMode is returned when composite store result count exceeds max limit.
var errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
