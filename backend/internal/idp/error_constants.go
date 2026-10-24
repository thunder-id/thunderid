// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrIDPNotFound is returned when the IdP is not found in the system.
var ErrIDPNotFound = errors.New("IdP not found")

// ErrResultLimitExceededInCompositeMode is the internal sentinel error for composite mode limit exceeded.
var ErrResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")

// Client errors for identity provider operations.
var (
	// ErrorIDPNotFound is the error returned when an identity provider is not found.
	ErrorIDPNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_not_found",
			DefaultValue: "Identity provider not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_not_found_description",
			DefaultValue: "The requested identity provider could not be found",
		},
	}
	// ErrorInvalidIDPID is the error returned when an invalid identity provider ID is provided.
	ErrorInvalidIDPID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_id",
			DefaultValue: "Invalid identity provider ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_id_description",
			DefaultValue: "The provided identity provider ID is invalid or empty",
		},
	}
	// ErrorInvalidIDPName is the error returned when an invalid identity provider name is provided.
	ErrorInvalidIDPName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_name",
			DefaultValue: "Invalid identity provider name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_name_description",
			DefaultValue: "The provided identity provider name is invalid or empty",
		},
	}
	// ErrorInvalidIDPType is the error returned when an invalid identity provider type is provided.
	ErrorInvalidIDPType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_type",
			DefaultValue: "Invalid identity provider type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_type_description",
			DefaultValue: "The provided identity provider type is invalid or empty",
		},
	}
	// ErrorIDPAlreadyExists is the error returned when an identity provider with the same name already exists.
	ErrorIDPAlreadyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_already_exists",
			DefaultValue: "Identity provider already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_already_exists_description",
			DefaultValue: "An identity provider with the same name already exists",
		},
	}
	// ErrorInvalidIDPProperty is the error returned when an invalid identity provider property is provided.
	ErrorInvalidIDPProperty = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_property",
			DefaultValue: "Invalid identity provider property",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_property_description",
			DefaultValue: "One or more identity provider properties are invalid or empty",
		},
	}
	// ErrorUnsupportedIDPProperty is the error returned when an unsupported identity provider property is provided.
	ErrorUnsupportedIDPProperty = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.unsupported_idp_property",
			DefaultValue: "Unsupported identity provider property",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.unsupported_idp_property_description",
			DefaultValue: "One or more identity provider properties are not supported",
		},
	}
	// ErrorIDPNil is the error returned when the identity provider object is nil.
	ErrorIDPNil = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_nil",
			DefaultValue: "Identity provider cannot be null",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_nil_description",
			DefaultValue: "The identity provider object cannot be null or empty",
		},
	}
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorIDPDeclarativeReadOnly is the error returned when attempting to modify a declarative (immutable) IDP.
	ErrorIDPDeclarativeReadOnly = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_declarative_read_only",
			DefaultValue: "Identity provider is immutable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_declarative_read_only_description",
			DefaultValue: "The requested identity provider is declarative and cannot be modified or deleted",
		},
	}
	// ErrorResultLimitExceededInCompositeMode is the error returned when the total number of records exceeds
	ErrorResultLimitExceededInCompositeMode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.result_limit_exceeded",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.result_limit_exceeded_description",
			DefaultValue: "The total number of records exceeds the maximum limit in composite mode",
		},
	}
	// ErrorInvalidAttributeConfiguration is the error returned when the identity provider's
	// attribute configuration is invalid.
	ErrorInvalidAttributeConfiguration = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_attribute_configuration",
			DefaultValue: "Invalid attribute configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_attribute_configuration_description",
			DefaultValue: "The identity provider's attribute configuration is invalid",
		},
	}
	// ErrorIDPHasBlockingDependencies is returned when an identity provider cannot be deleted because
	// other resources depend on it in a way that forbids deletion (e.g. flows that reference it).
	ErrorIDPHasBlockingDependencies = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_has_blocking_dependencies",
			DefaultValue: "Identity provider cannot be deleted",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.idpservice.idp_has_blocking_dependencies_description",
			DefaultValue: "The identity provider cannot be deleted because other resources depend on it. " +
				"Remove or reassign them first.",
		},
	}
	// ErrorInvalidIDPOU is the error returned when the organization unit given for an identity provider does not
	// exist.
	ErrorInvalidIDPOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_ou",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.invalid_idp_ou_description",
			DefaultValue: "The organization unit given for the identity provider does not exist",
		},
	}
	// ErrorIDPOURequired is the error returned when an identity provider is created without an organization unit by
	// a caller that has none to default to.
	ErrorIDPOURequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_ou_required",
			DefaultValue: "Organization unit required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.idpservice.idp_ou_required_description",
			DefaultValue: "An identity provider needs an organization unit, and the caller has none to default to. " +
				"Give one in the request",
		},
	}
	// ErrorIDPOUUpdateNotAllowed is the error returned when attempting to change the organization unit of an
	// identity provider.
	ErrorIDPOUUpdateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IDP-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_ou_update_not_allowed",
			DefaultValue: "Organization unit update not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.idpservice.idp_ou_update_not_allowed_description",
			DefaultValue: "The organization unit of an identity provider cannot be changed",
		},
	}
)
