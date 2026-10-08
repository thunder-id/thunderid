// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client ID Metadata Document service errors.
var (
	// ErrorInvalidClientID is the error returned when a Client Identifier URL is invalid.
	ErrorInvalidClientID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_client_id",
			DefaultValue: "Invalid Client Identifier URL",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.cimdservice.invalid_client_id_description",
			DefaultValue: "The client ID must be an https URL with a DNS host and a path, " +
				"and no user information, fragment or dot segments",
		},
	}
	// ErrorDocumentUnavailable is the error returned when a Client ID Metadata Document cannot be retrieved.
	ErrorDocumentUnavailable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.document_unavailable",
			DefaultValue: "Client ID Metadata Document could not be retrieved",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.cimdservice.document_unavailable_description",
			DefaultValue: "The document could not be retrieved over HTTPS as a JSON document " +
				"within the time and size limits",
		},
	}
	// ErrorClientIDMismatch is the error returned when a document's client_id differs from its URL.
	ErrorClientIDMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.client_id_mismatch",
			DefaultValue: "Client ID does not match the document URL",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.cimdservice.client_id_mismatch_description",
			DefaultValue: "The client_id in the document must equal the URL it was retrieved from",
		},
	}
	// ErrorInvalidRedirectURI is the error returned when a CIMD redirect URI breaks the document rules.
	ErrorInvalidRedirectURI = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_redirect_uri",
			DefaultValue: "Invalid redirect URI for a Client ID Metadata Document client",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.cimdservice.invalid_redirect_uri_description",
			DefaultValue: "At least one redirect URI is required, and each must be https on the client ID's origin " +
				"or http on a loopback address",
		},
	}
	// ErrorInvalidAuthMethod is the error returned when a CIMD client's authentication method is not supported.
	ErrorInvalidAuthMethod = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_auth_method",
			DefaultValue: "Unsupported authentication method for a Client ID Metadata Document client",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.cimdservice.invalid_auth_method_description",
			DefaultValue: "The token endpoint authentication method must be none, " +
				"or private_key_jwt with exactly one of jwks_uri or jwks",
		},
	}
	// ErrorClientSecretNotAllowed is the error returned when a CIMD client declares a client secret.
	ErrorClientSecretNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.client_secret_not_allowed",
			DefaultValue: "Client secret not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.cimdservice.client_secret_not_allowed_description",
			DefaultValue: "A client registered from a Client ID Metadata Document cannot have a client secret",
		},
	}
	// ErrorInvalidGrantTypes is the error returned when a CIMD client's grant types break the document rules.
	ErrorInvalidGrantTypes = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_grant_types",
			DefaultValue: "Invalid grant types for a Client ID Metadata Document client",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_grant_types_description",
			DefaultValue: "The grant types must include authorization_code and may add only refresh_token",
		},
	}
	// ErrorImmutable is the error returned when an update changes the CIMD marker or a CIMD client's client ID.
	ErrorImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.immutable",
			DefaultValue: "Client ID Metadata Document registration cannot be changed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.cimdservice.immutable_description",
			DefaultValue: "The Client ID Metadata Document marker and the client ID are fixed at creation",
		},
	}
	// ErrorInvalidJWKSURI is the error returned when a CIMD client's JWKS URI is not an https URL on a public host.
	ErrorInvalidJWKSURI = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_jwks_uri",
			DefaultValue: "Invalid JWKS URI",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.cimdservice.invalid_jwks_uri_description",
			DefaultValue: "The jwks_uri must be an https URL whose host is not a loopback, link-local, " +
				"or private address",
		},
	}
	// ErrorPKCERequired is the error returned when a CIMD client does not require PKCE.
	ErrorPKCERequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.pkce_required",
			DefaultValue: "PKCE required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.cimdservice.pkce_required_description",
			DefaultValue: "A client registered from a Client ID Metadata Document must require PKCE",
		},
	}
	// ErrorInvalidKeySet is the error returned when a CIMD client's inline JWKS cannot be stored.
	ErrorInvalidKeySet = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_key_set",
			DefaultValue: "Invalid key set",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.cimdservice.invalid_key_set_description",
			DefaultValue: "The inline jwks must be between 10 and 4096 characters. " +
				"Publish a larger key set at a jwks_uri instead",
		},
	}
	// ErrorInvalidRequestFormat is the error returned when a preview request body is malformed.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CIMD-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.cimdservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
)
