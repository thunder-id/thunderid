// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// errUnsupportedAuthenticationType reports a connection payload naming an outbound
// authentication method this deployment does not implement. It is the caller's mistake, so it
// maps onto ErrorInvalidAuthenticationType rather than a server fault.
var errUnsupportedAuthenticationType = errors.New("unsupported authentication type")

var errInvalidSMSGatewayAuthentication = errors.New("invalid SMS gateway authentication")

// errInvalidHTTPAuthentication reports an HTTP connection payload whose authentication the request
// mapping refuses, such as a password sent as the mask.
var errInvalidHTTPAuthentication = errors.New("invalid HTTP connection authentication")

// Client errors for connection operations.
var (
	// ErrorInvalidConnectionCategory is the error returned when the category query parameter
	// on GET /connections is not a recognized category.
	ErrorInvalidConnectionCategory = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_category",
			DefaultValue: "Invalid connection category",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.invalid_category_description",
			DefaultValue: "The category must be one of: identity-provider, sms-provider, " +
				"authorization-pdp, email-provider, http",
		},
	}
	// ErrorInvalidLimit is the error returned when an invalid limit query parameter is provided.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when an invalid offset query parameter is provided.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorInvalidRequestFormat is returned when an AuthZEN PDP request body is malformed.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorInvalidConnectionVendor is the error returned when the vendor query parameter on
	// GET /connections/meta does not name a registered connection vendor.
	ErrorInvalidConnectionVendor = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_vendor",
			DefaultValue: "Invalid connection vendor",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_vendor_description",
			DefaultValue: "The vendor parameter must name a supported connection vendor",
		},
	}
	// ErrorInvalidAuthenticationType is the error returned when a connection payload names an
	// outbound authentication method this deployment does not implement.
	ErrorInvalidAuthenticationType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_authentication_type",
			DefaultValue: "Invalid authentication type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_authentication_type_description",
			DefaultValue: "The authentication type must name a method this deployment supports",
		},
	}
	// ErrorInvalidSharingPolicy is returned when a sharing policy is not one the issuing
	// organization unit may write. It carries the sharing framework's own description through.
	ErrorInvalidSharingPolicy = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1031",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_sharing_policy",
			DefaultValue: "Invalid sharing policy",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_sharing_policy_description",
			DefaultValue: "The sharing policy is not one the issuing organization unit may write",
		},
	}
	// ErrorSharingPolicyNotFound is returned when the connection holds no policy under the given id.
	ErrorSharingPolicyNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1032",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.sharing_policy_not_found",
			DefaultValue: "Sharing policy not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.sharing_policy_not_found_description",
			DefaultValue: "The connection has no sharing policy with the specified id",
		},
	}
	// ErrorSharingPolicyExists is returned when the organization unit already holds a policy for
	// the connection.
	ErrorSharingPolicyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1033",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.sharing_policy_exists",
			DefaultValue: "Sharing policy already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.sharing_policy_exists_description",
			DefaultValue: "This organization unit already has a sharing policy for the " +
				"connection; edit it instead",
		},
	}
	// ErrorSharingPolicyVersionMismatch is returned when an edit names a version the policy has
	// moved past.
	ErrorSharingPolicyVersionMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1034",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.sharing_policy_version_mismatch",
			DefaultValue: "Sharing policy version mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.sharing_policy_version_mismatch_description",
			DefaultValue: "The sharing policy changed since it was read; read it again and " +
				"retry the edit",
		},
	}
	// ErrorSharingPolicyDeclared is returned when an edit or a delete is aimed at a policy a
	// resource file declares.
	ErrorSharingPolicyDeclared = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1035",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.sharing_policy_declared",
			DefaultValue: "Sharing policy is declared",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.sharing_policy_declared_description",
			DefaultValue: "The sharing policy is declared in a resource file and cannot be " +
				"changed through the API",
		},
	}
	// ErrorConnectionNotSharedToOU is returned when an organization unit cannot see the
	// connection, so there are no terms to resolve for it.
	ErrorConnectionNotSharedToOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1036",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.connection_not_shared_to_ou",
			DefaultValue: "Connection is not shared to the organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.connection_not_shared_to_ou_description",
			DefaultValue: "No sharing policy reaches the specified organization unit, so the " +
				"connection is not available there",
		},
	}
	// ErrorMissingOUIDParameter is returned when an overlay-rules request does not say which
	// organization unit to resolve for.
	ErrorMissingOUIDParameter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1037",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.missing_ou_id_parameter",
			DefaultValue: "Missing required parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.missing_ou_id_parameter_description",
			DefaultValue: "The 'ouId' query parameter is required",
		},
	}
	// ErrorDeploymentWideSharingNotAllowed is returned when a create through the API states a
	// section whose reach is not bounded by where the issuing organization unit sits in the tree.
	ErrorDeploymentWideSharingNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1038",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.deployment_wide_sharing_not_allowed",
			DefaultValue: "Deployment-wide sharing is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.deployment_wide_sharing_not_allowed_description",
			DefaultValue: "A policy reaching all organization units or the roots of all other trees " +
				"can only be created from a declarative resource file or an import",
		},
	}
	// ErrorInvalidFilter is returned when a connection listing carries a filter it cannot apply.
	ErrorInvalidFilter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1039",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_filter_parameter",
			DefaultValue: "Invalid filter parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_filter_parameter_description",
			DefaultValue: "The filter must be a single ouId eq expression with a quoted organization unit ID",
		},
	}
)
