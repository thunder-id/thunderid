// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

// errNilEventBus is returned when the service is initialized without the event bus it audits on.
var errNilEventBus = errors.New("the layout management service needs an event bus")

// errLayoutNotFoundNoop is returned by deleteLayout for a layout that does not exist. DeleteLayout turns
// it into a success, so it never reaches a caller.
var errLayoutNotFoundNoop = &tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: ErrorLayoutNotFound.Code,
}

// errCannotUpdateDeclarativeLayout is an internal error for composite store operations.
var errCannotUpdateDeclarativeLayout = errors.New("cannot update declarative layout")

// errCannotDeleteDeclarativeLayout is an internal error for composite store operations.
var errCannotDeleteDeclarativeLayout = errors.New("cannot delete declarative layout")

// errResultLimitExceededInCompositeMode is returned when composite store result count exceeds max limit.
var errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
