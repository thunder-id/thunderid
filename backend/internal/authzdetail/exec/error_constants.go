// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailexec

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Errors of the processing of requested authorization details.
var (
	// ErrorAuthorizationDetailRefused is returned when a processor refuses a detail.
	ErrorAuthorizationDetailRefused = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.authorization_detail_refused",
			DefaultValue: "Authorization details refused",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.authorization_detail_refused_description",
			DefaultValue: "The authorization details were refused by the resource owner's domain system",
		},
	}
	// errorAuthorizationDetailRefusedReason is the description of a refusal with a reason.
	errorAuthorizationDetailRefusedReason = tidcommon.I18nMessage{
		Key:          "error.authorizationdetailservice.authorization_detail_refused_reason_description",
		DefaultValue: "Refused by the resource owner's domain system: {{param(reason)}}",
	}
	// ErrorProcessorUnavailable is returned when a processor cannot be reached or gives no usable answer.
	ErrorProcessorUnavailable = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "RAR-5001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.processor_unavailable",
			DefaultValue: "Authorization details processor unavailable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.processor_unavailable_description",
			DefaultValue: "The authorization details processor could not be reached or returned an invalid response",
		},
	}
)

// Errors of the validation and modification of requested authorization details.
var (
	// ErrorMalformedAuthorizationDetails is returned when authorization_details cannot be parsed.
	ErrorMalformedAuthorizationDetails = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.malformed_authorization_details",
			DefaultValue: "Malformed authorization details",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.malformed_authorization_details_description",
			DefaultValue: "{{param(reason)}}",
		},
	}
	// ErrorUnknownAuthorizationDetailType is returned when a requested detail names no registered type.
	ErrorUnknownAuthorizationDetailType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.unknown_authorization_detail_type",
			DefaultValue: "Unknown authorization details type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.unknown_authorization_detail_type_description",
			DefaultValue: "authorization_details entry {{param(index)}} names the unknown type {{param(type)}}",
		},
	}
	// ErrorAuthorizationDetailNotConformant is returned when a detail does not conform to its type.
	ErrorAuthorizationDetailNotConformant = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.authorization_detail_not_conformant",
			DefaultValue: "Authorization detail does not conform to its type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authorizationdetailservice.authorization_detail_not_conformant_description",
			DefaultValue: "authorization_details entry {{param(index)}} does not conform to its type: " +
				"{{param(reason)}}",
		},
	}
	// ErrorAuthorizationDetailNotAllowed is returned when a detail has a disallowed field or exceeds limits.
	ErrorAuthorizationDetailNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1023",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.authorization_detail_not_allowed",
			DefaultValue: "Authorization detail not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.authorization_detail_not_allowed_description",
			DefaultValue: "authorization_details entry {{param(index)}} is not allowed: {{param(reason)}}",
		},
	}
	// ErrorAuthorizationDetailNeedsUser is returned when a type without a processor is requested without a user.
	ErrorAuthorizationDetailNeedsUser = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1024",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.authorization_detail_needs_user",
			DefaultValue: "Authorization details need a user",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.authorization_detail_needs_user_description",
			DefaultValue: "authorization details of type {{param(type)}} cannot be requested without a user",
		},
	}
)

// Internal errors.
var (
	// errProcessorUnavailable is returned when a processor cannot be reached or answers invalidly.
	errProcessorUnavailable = errors.New("authorization details processor unavailable")
)
