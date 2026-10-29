// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for project management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request body is malformed.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}

	// ErrorProjectNotFound is the error returned when a project does not exist.
	ErrorProjectNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.project_not_found",
			DefaultValue: "Project not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.project_not_found_description",
			DefaultValue: "No project exists with the given ID",
		},
	}

	// ErrorInvalidDisplayName is the error returned when the project display name is missing or too long.
	ErrorInvalidDisplayName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_display_name",
			DefaultValue: "Invalid project display name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_display_name_description",
			DefaultValue: "The display name is required and must not exceed 100 characters",
		},
	}

	// ErrorInvalidDescription is the error returned when the project description is too long.
	ErrorInvalidDescription = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_description",
			DefaultValue: "Invalid project description",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_description_description",
			DefaultValue: "The description must not exceed 255 characters",
		},
	}

	// ErrorInvalidProjectID is the error returned when a supplied project ID is not a UUID.
	ErrorInvalidProjectID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_project_id",
			DefaultValue: "Invalid project ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_project_id_description",
			DefaultValue: "A supplied project ID must be a valid UUID",
		},
	}

	// ErrorProjectIDConflict is the error returned when a project with the supplied ID already exists.
	ErrorProjectIDConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.project_id_conflict",
			DefaultValue: "Project ID already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.project_id_conflict_description",
			DefaultValue: "Omit the ID to have one generated, or supply an ID that no other project uses",
		},
	}

	// ErrorHandleChangeNotAllowed is the error returned when an update tries to change the handle.
	ErrorHandleChangeNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.handle_change_not_allowed",
			DefaultValue: "Project handle cannot be changed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.handle_change_not_allowed_description",
			DefaultValue: "Send the current handle, or omit it, when updating a project",
		},
	}

	// ErrorInvalidLimit is the error returned when the limit query parameter is invalid.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_limit",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_limit_description",
			DefaultValue: "The limit parameter must be a positive integer within the maximum page size",
		},
	}

	// ErrorInvalidOffset is the error returned when the offset query parameter is invalid.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_offset",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_offset_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}

	// ErrorRootOUMismatch is the error returned when an imported project names a different root
	// organization unit from the one it already has.
	ErrorRootOUMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.root_ou_mismatch",
			DefaultValue: "Root organization unit mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.root_ou_mismatch_description",
			DefaultValue: "A project's root organization unit cannot be changed",
		},
	}

	// ErrorNotARootOU is the error returned when a project names an organization unit that has a
	// parent as its root.
	ErrorNotARootOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.not_a_root_ou",
			DefaultValue: "Not a root organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.not_a_root_ou_description",
			DefaultValue: "A project's root organization unit must not have a parent",
		},
	}

	// ErrorRootOUInUse is the error returned when a project names a root organization unit that
	// already belongs to another project.
	ErrorRootOUInUse = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.root_ou_in_use",
			DefaultValue: "Root organization unit in use",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.root_ou_in_use_description",
			DefaultValue: "The root organization unit already belongs to another project",
		},
	}
)
