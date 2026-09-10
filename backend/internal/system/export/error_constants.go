// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for export operations.
var (
	// ErrorInvalidRequest is the error returned when an invalid export request is provided.
	ErrorInvalidRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "EXP-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.exportservice.invalid_request",
			DefaultValue: "Invalid export request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.exportservice.invalid_request_description",
			DefaultValue: "The provided export request is invalid or malformed",
		},
	}

	// ErrorNoResourcesFound is the error returned when no valid resources are found for export.
	ErrorNoResourcesFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "EXP-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.exportservice.no_resources_found",
			DefaultValue: "No resources found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.exportservice.no_resources_found_description",
			DefaultValue: "No valid resources found for the provided identifiers",
		},
	}

	// ErrorDuplicateTemplateVariable is returned when two resources in one export claim the same
	// template variable. The export is refused rather than returned, because a bundle where two
	// resources share one variable imports both with the same value.
	ErrorDuplicateTemplateVariable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "EXP-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.exportservice.duplicate_template_variable",
			DefaultValue: "Duplicate template variable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.exportservice.duplicate_template_variable_description",
			DefaultValue: "Two resources derive the same template variable, so both would import one value",
		},
	}
)
