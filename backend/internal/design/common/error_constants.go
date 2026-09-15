// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package common defines shared error constants for the design module.
package common

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for design resolve operations.
var (
	// ErrorInvalidResolveType is the error returned when resolve type parameter is missing or invalid.
	ErrorInvalidResolveType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "DSR-1001",
		Error: tidcommon.I18nMessage{
			Key:          "design.resolve.error.invalid_type",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "design.resolve.error.invalid_type_description",
			DefaultValue: "The 'type' query parameter is required and must be either 'APP' or 'OU'",
		},
	}
	// ErrorMissingResolveID is the error returned when resolve id parameter is missing.
	ErrorMissingResolveID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "DSR-1002",
		Error: tidcommon.I18nMessage{
			Key:          "design.resolve.error.missing_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "design.resolve.error.missing_id_description",
			DefaultValue: "The 'id' query parameter is required",
		},
	}
	// ErrorUnsupportedResolveType is the error returned when resolve type is not yet supported.
	ErrorUnsupportedResolveType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "DSR-1003",
		Error: tidcommon.I18nMessage{
			Key:          "design.resolve.error.unsupported_type",
			DefaultValue: "Unsupported resolve type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "design.resolve.error.unsupported_type_description",
			DefaultValue: "The specified resolve type is not yet supported. Currently only 'APP' type is supported",
		},
	}
	// DSR-1004 ("Application not found") is retired and must not be reused. It shipped in v1.0.x, so
	// reassigning the code would change the meaning of a response clients may already match on.
	//
	// ErrorApplicationHasNoDesign is the error returned when no design can be resolved for the given
	// application, whether because it has none configured or because it does not exist. The resolve
	// endpoint is unauthenticated, so it deliberately does not distinguish the two: a caller able to
	// tell them apart could confirm which application IDs exist.
	ErrorApplicationHasNoDesign = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "DSR-1005",
		Error: tidcommon.I18nMessage{
			Key:          "design.resolve.error.app_no_design",
			DefaultValue: "Application has no design configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "design.resolve.error.app_no_design_description",
			DefaultValue: "The specified application does not have an associated theme or layout configuration",
		},
	}
)
