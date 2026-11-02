// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Internal error sentinels matched with errors.Is for control flow; the service translates them into
// the API ServiceError values below.
var (
	// errTemplateNotFound is the internal sentinel returned by the store when a template row is absent.
	errTemplateNotFound = errors.New("notification template not found")

	// errHandleConflict is an internal sentinel used to roll back a transaction when a handle already
	// exists; the caller surfaces ErrorTemplateHandleConflict.
	errHandleConflict = errors.New("template handle already exists")

	// errDeclarativeTemplate is returned when a write targets a file-declared template; the service
	// surfaces ErrorTemplateReadOnly.
	errDeclarativeTemplate = errors.New("a notification template declared in a file cannot be changed through the API")

	// errResultLimitExceededInCompositeMode is returned when composite store results exceed the configured limit.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
)

var (
	// ErrorInvalidTemplateData is returned when the request body cannot be parsed.
	ErrorInvalidTemplateData = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_request",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_request_description",
			DefaultValue: "The request contains invalid parameters",
		},
	}

	// ErrorInvalidTemplateID is returned when an invalid template id is provided.
	ErrorInvalidTemplateID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_id",
			DefaultValue: "Invalid template id",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_id_description",
			DefaultValue: "The provided template id is invalid",
		},
	}

	// ErrorMissingSubject is returned when an email template has no subject.
	ErrorMissingSubject = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_subject",
			DefaultValue: "Missing subject",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_subject_description",
			DefaultValue: "A subject is required for email templates",
		},
	}

	// ErrorTemplateNotFound is returned when a template does not exist.
	ErrorTemplateNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.not_found",
			DefaultValue: "Template not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.not_found_description",
			DefaultValue: "The requested notification template does not exist",
		},
	}

	// ErrorMissingDisplayName is returned when the displayName is not provided.
	ErrorMissingDisplayName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_display_name",
			DefaultValue: "Missing display name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_display_name_description",
			DefaultValue: "Display name is required",
		},
	}

	// ErrorMissingBody is returned when the content body is not provided.
	ErrorMissingBody = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_body",
			DefaultValue: "Missing body",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_body_description",
			DefaultValue: "The content body is required",
		},
	}

	// ErrorInvalidChannel is returned when the channel path parameter is not email or sms.
	ErrorInvalidChannel = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_channel",
			DefaultValue: "Invalid channel",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_channel_description",
			DefaultValue: "The channel must be either email or sms",
		},
	}

	// ErrorMissingHandle is returned when the handle is not provided on create.
	ErrorMissingHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_handle",
			DefaultValue: "Missing handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_handle_description",
			DefaultValue: "A handle is required",
		},
	}

	// ErrorTemplateHandleConflict is returned when a template handle already exists in the channel.
	ErrorTemplateHandleConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.handle_conflict",
			DefaultValue: "Template handle already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.handle_conflict_description",
			DefaultValue: "A template with the same handle already exists in this channel",
		},
	}

	// ErrorTemplateInUse is returned when a template cannot be deleted because it is in use.
	ErrorTemplateInUse = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.template_in_use",
			DefaultValue: "Template in use",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.template_in_use_description",
			DefaultValue: "The template is in use and cannot be deleted",
		},
	}

	// ErrorInvalidHandle is returned when the handle is not a valid kebab-case identifier.
	ErrorInvalidHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_handle",
			DefaultValue: "Invalid handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_handle_description",
			DefaultValue: "The handle must be a lowercase kebab-case identifier",
		},
	}

	// ErrorSubjectNotAllowed is returned when a subject is supplied for a channel that has no subject.
	ErrorSubjectNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.subject_not_allowed",
			DefaultValue: "Subject not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.subject_not_allowed_description",
			DefaultValue: "A subject cannot be set for this channel",
		},
	}

	// ErrorDesignNotAllowed is returned when a design is supplied for a channel that has no design.
	ErrorDesignNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.design_not_allowed",
			DefaultValue: "Design not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.design_not_allowed_description",
			DefaultValue: "A design cannot be set for this channel",
		},
	}

	// ErrorDisplayNameTooLong is returned when the template displayName exceeds the maximum length.
	ErrorDisplayNameTooLong = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.display_name_too_long",
			DefaultValue: "Display name too long",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.display_name_too_long_description",
			DefaultValue: "The template display name exceeds the maximum allowed length",
		},
	}

	// ErrorDescriptionTooLong is returned when the template description exceeds the maximum length.
	ErrorDescriptionTooLong = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.description_too_long",
			DefaultValue: "Description too long",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.description_too_long_description",
			DefaultValue: "The template description exceeds the maximum allowed length",
		},
	}

	// ErrorHandleTooLong is returned when the template handle exceeds the maximum length.
	ErrorHandleTooLong = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.handle_too_long",
			DefaultValue: "Handle too long",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.handle_too_long_description",
			DefaultValue: "The template handle exceeds the maximum allowed length",
		},
	}

	// ErrorInvalidColorScheme is returned when the design color scheme is not light or dark.
	ErrorInvalidColorScheme = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_color_scheme",
			DefaultValue: "Invalid color scheme",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_color_scheme_description",
			DefaultValue: "Color scheme must be light or dark",
		},
	}

	// ErrorInvalidLimit is returned when the limit query parameter is not a valid integer or is out of
	// range.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_limit",
			DefaultValue: "Invalid limit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_limit_description",
			DefaultValue: "The limit must be a positive integer within the allowed range",
		},
	}

	// ErrorInvalidOffset is returned when the offset query parameter is not a valid non-negative
	// integer.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_offset",
			DefaultValue: "Invalid offset",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_offset_description",
			DefaultValue: "The offset must be a non-negative integer",
		},
	}

	// ErrorInvalidPlaceholder is returned when the subject or body contains a malformed or unsupported
	// placeholder.
	ErrorInvalidPlaceholder = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_placeholder",
			DefaultValue: "Invalid placeholder",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.notificationtemplateservice.invalid_placeholder_description",
			DefaultValue: "The content contains a malformed or unsupported placeholder; use " +
				"{{ctx(key)}}, {{t(key)}}, or {{design(token)}}",
		},
	}

	// ErrorDesignPlaceholderNotAllowed is returned when a {{design(...)}} placeholder is used where a
	// design does not apply (a subject, or a channel without a design such as SMS).
	ErrorDesignPlaceholderNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.design_placeholder_not_allowed",
			DefaultValue: "Design placeholder not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.design_placeholder_not_allowed_description",
			DefaultValue: "A {{design(...)}} placeholder is only allowed in an email template body",
		},
	}

	// ErrorTemplateReadOnly is returned when a write targets a file-declared template.
	ErrorTemplateReadOnly = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.read_only",
			DefaultValue: "Template is read-only",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.read_only_description",
			DefaultValue: "The template is declared in a file and cannot be modified through the API",
		},
	}

	// ErrorResultLimitExceededInCompositeMode is returned when a composite-mode listing would exceed the
	// per-source cap, so the merged page cannot be computed reliably.
	ErrorResultLimitExceededInCompositeMode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1023",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.result_limit_exceeded_in_composite_mode",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.notificationtemplateservice.result_limit_exceeded_in_composite_mode_description",
			DefaultValue: "Too many declared or stored templates to merge in composite mode; reduce the " +
				"number of templates in a channel",
		},
	}

	// ErrorDeclarativeModeCreateNotAllowed is returned when a create is attempted while templates run in
	// declarative-only mode, where the store is read-only. Distinct from ErrorTemplateReadOnly: no file
	// declares the new handle; creation itself is disabled.
	ErrorDeclarativeModeCreateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1024",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.cannot_create_in_declarative_only_mode",
			DefaultValue: "Cannot create template in declarative-only mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.notificationtemplateservice.cannot_create_in_declarative_only_mode_description",
			DefaultValue: "Template creation is not allowed when running in declarative-only mode. " +
				"Templates must be defined in declarative configuration files",
		},
	}

	// ErrorInvalidOU is returned when the ouId is missing or does not reference an existing organization
	// unit.
	ErrorInvalidOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_ou",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_ou_description",
			DefaultValue: "A valid ouId is required",
		},
	}

	// ErrorOUImmutable is returned when an update (for example a declarative re-import) tries to change
	// the ouId of an existing template. The ouId is set at creation and cannot be changed.
	ErrorOUImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.ou_immutable",
			DefaultValue: "Organization unit cannot be changed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.ou_immutable_description",
			DefaultValue: "The ouId is set at creation and cannot be changed for an existing template",
		},
	}
)
