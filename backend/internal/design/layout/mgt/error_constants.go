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

package layoutmgt

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrorInvalidLayoutData is returned when invalid layout data is provided.
	ErrorInvalidLayoutData = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1001",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_data",
			DefaultValue: "Invalid layout data",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_data_description",
			DefaultValue: "The provided layout data is invalid",
		},
	}

	// ErrorInvalidLayoutID is returned when an invalid layout ID is provided.
	ErrorInvalidLayoutID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1002",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_id",
			DefaultValue: "Invalid layout ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_id_description",
			DefaultValue: "The provided layout ID is invalid",
		},
	}

	// ErrorLayoutNotFound is returned when a layout is not found.
	ErrorLayoutNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1003",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.not_found",
			DefaultValue: "Layout not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.not_found_description",
			DefaultValue: "The requested layout configuration was not found",
		},
	}

	// ErrorLayoutAlreadyExists is returned when trying to create a layout that already exists.
	ErrorLayoutAlreadyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1004",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.already_exists",
			DefaultValue: "Layout already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.already_exists_description",
			DefaultValue: "A layout with the same ID already exists",
		},
	}

	// ErrorMissingDisplayName is returned when display name is not provided.
	ErrorMissingDisplayName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1005",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.missing_display_name",
			DefaultValue: "Missing display name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.missing_display_name_description",
			DefaultValue: "Display name is required",
		},
	}

	// ErrorMissingLayout is returned when layout field is not provided.
	ErrorMissingLayout = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1006",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.missing_layout",
			DefaultValue: "Missing layout",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.missing_layout_description",
			DefaultValue: "Layout field is required",
		},
	}

	// ErrorInvalidLayoutFormat is returned when layout JSON is invalid.
	ErrorInvalidLayoutFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1007",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_format",
			DefaultValue: "Invalid layout format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_format_description",
			DefaultValue: "Layout must be a valid JSON object",
		},
	}

	// ErrorLayoutInUse is returned when trying to delete a layout that is being used by applications.
	ErrorLayoutInUse = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1008",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.in_use",
			DefaultValue: "Layout in use",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.in_use_description",
			DefaultValue: "Cannot delete layout that is currently associated with one or more applications",
		},
	}

	// ErrorInvalidLimitValue is returned when limit validation fails in service layer.
	ErrorInvalidLimitValue = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1009",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_limit",
			DefaultValue: "Invalid limit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_limit_description",
			DefaultValue: "Limit value is out of valid range",
		},
	}

	// ErrorInvalidOffsetValue is returned when offset validation fails in service layer.
	ErrorInvalidOffsetValue = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1010",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_offset",
			DefaultValue: "Invalid offset",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_offset_description",
			DefaultValue: "Offset must be non-negative",
		},
	}

	// ErrorInvalidLimitParam is returned when limit parameter cannot be parsed.
	ErrorInvalidLimitParam = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1011",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_limit_param",
			DefaultValue: "Invalid limit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_limit_param_description",
			DefaultValue: "Limit must be a valid integer",
		},
	}

	// ErrorInvalidOffsetParam is returned when offset parameter cannot be parsed.
	ErrorInvalidOffsetParam = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1012",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_offset_param",
			DefaultValue: "Invalid offset",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.invalid_offset_param_description",
			DefaultValue: "Offset must be a valid integer",
		},
	}

	// ErrorCannotUpdateDeclarativeLayout is returned when attempting to update a declarative layout.
	ErrorCannotUpdateDeclarativeLayout = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1013",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.cannot_update_declarative",
			DefaultValue: "Cannot update declarative layout",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.cannot_update_declarative_description",
			DefaultValue: "Layout is defined in declarative resources and cannot be modified",
		},
	}

	// ErrorCannotDeleteDeclarativeLayout is returned when attempting to delete a declarative layout.
	ErrorCannotDeleteDeclarativeLayout = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1014",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.cannot_delete_declarative",
			DefaultValue: "Cannot delete declarative layout",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.cannot_delete_declarative_description",
			DefaultValue: "Layout is defined in declarative resources and cannot be deleted",
		},
	}

	// ErrorResultLimitExceededInCompositeMode is returned when composite store result count exceeds max limit.
	ErrorResultLimitExceededInCompositeMode = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "LAY-5001",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.result_limit_exceeded_description",
			DefaultValue: "Total count of layouts exceeds maximum allowed limit in composite mode",
		},
	}

	// ErrorCannotModifyDeclarativeResource is returned when attempting to modify a declarative layout.
	ErrorCannotModifyDeclarativeResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1015",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.cannot_modify_declarative",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.cannot_modify_declarative_description",
			DefaultValue: "The layout is declarative and cannot be modified or deleted",
		},
	}

	// ErrorDuplicateLayoutHandle is returned when a layout with the same handle already exists.
	ErrorDuplicateLayoutHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1016",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.duplicate_handle",
			DefaultValue: "Duplicate layout handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.duplicate_handle_description",
			DefaultValue: "A layout with the same handle already exists",
		},
	}

	// ErrorMissingLayoutHandle is returned when handle is not provided.
	ErrorMissingLayoutHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1017",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.missing_handle",
			DefaultValue: "Missing layout handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.missing_handle_description",
			DefaultValue: "Layout handle is required",
		},
	}

	// ErrorLayoutHandleImmutable is returned when attempting to change the handle of an existing layout.
	ErrorLayoutHandleImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "LAY-1018",
		Error: tidcommon.I18nMessage{
			Key:          "layout.error.handle_immutable",
			DefaultValue: "Layout handle is immutable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "layout.error.handle_immutable_description",
			DefaultValue: "The layout handle cannot be changed after creation",
		},
	}
)

// errCannotUpdateDeclarativeLayout is an internal error for composite store operations.
var errCannotUpdateDeclarativeLayout = errors.New("cannot update declarative layout")

// errCannotDeleteDeclarativeLayout is an internal error for composite store operations.
var errCannotDeleteDeclarativeLayout = errors.New("cannot delete declarative layout")

// errResultLimitExceededInCompositeMode is returned when composite store result count exceeds max limit.
var errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
