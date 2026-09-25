// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// errTemplateNotFound is the internal sentinel returned by the store when a template row is absent.
var errTemplateNotFound = errors.New("notification template not found")

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

	// ErrorMissingName is returned when the name is not provided.
	ErrorMissingName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_name",
			DefaultValue: "Missing name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_name_description",
			DefaultValue: "Name is required",
		},
	}

	// ErrorMissingBodyKey is returned when the content body key is not provided.
	ErrorMissingBodyKey = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_body_key",
			DefaultValue: "Missing body key",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.missing_body_key_description",
			DefaultValue: "The content body key (translation key) is required",
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

	// ErrorInvalidColorScheme is returned when the design color scheme is not light or dark.
	ErrorInvalidColorScheme = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_color_scheme",
			DefaultValue: "Invalid color scheme",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.invalid_color_scheme_description",
			DefaultValue: "Color scheme must be light or dark",
		},
	}

	// ErrorTemplateInUse is returned when a template cannot be deleted because a flow references it.
	ErrorTemplateInUse = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.conflict",
			DefaultValue: "Template conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.conflict_description",
			DefaultValue: "The template is referenced by a flow and cannot be deleted",
		},
	}

	// ErrorTemplateNameConflict is returned when a template name already exists in the channel.
	ErrorTemplateNameConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.name_conflict",
			DefaultValue: "Template name already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.name_conflict_description",
			DefaultValue: "A template with the same name already exists in this channel",
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

	// ErrorNameTooLong is returned when the template name exceeds the maximum length.
	ErrorNameTooLong = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "NTM-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.name_too_long",
			DefaultValue: "Name too long",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationtemplateservice.name_too_long_description",
			DefaultValue: "The template name exceeds the maximum allowed length",
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
)
