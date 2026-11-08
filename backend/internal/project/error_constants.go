// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/projectscope"
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
		Code: projectscope.ProjectNotFoundErrorCode,
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

	// ErrorOUNotInProject is the error returned when an organization unit is outside the organization
	// units a project includes.
	ErrorOUNotInProject = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: projectscope.OUNotInProjectErrorCode,
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.ou_not_in_project",
			DefaultValue: "Organization unit not in project",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.ou_not_in_project_description",
			DefaultValue: "The organization unit must be the system organization unit",
		},
	}

	// ErrorInvalidHandle is the error returned when the handle is missing or malformed.
	ErrorInvalidHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.invalid_handle",
			DefaultValue: "Invalid project handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.projectservice.invalid_handle_description",
			DefaultValue: "The handle is required, must not exceed 100 characters, and may contain only " +
				"lowercase letters, numbers, hyphens and underscores",
		},
	}

	// ErrorHandleConflict is the error returned when another project already uses the handle.
	ErrorHandleConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.handle_conflict",
			DefaultValue: "Project handle already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.projectservice.handle_conflict_description",
			DefaultValue: "Supply a handle that no other project uses",
		},
	}

	// ErrorProjectNotEmpty is the error returned when a project to delete still has resources.
	ErrorProjectNotEmpty = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "PRJ-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.projectservice.project_not_empty",
			DefaultValue: "Project is not empty",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.projectservice.project_not_empty_description",
			DefaultValue: "Delete or move the project's applications, agents, flows and layouts before deleting " +
				"the project",
		},
	}
)
