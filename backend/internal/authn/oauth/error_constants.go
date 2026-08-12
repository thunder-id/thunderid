// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for OAuth authentication.
var (
	// ErrorEmptyIdpID is the error when the IDP identifier is empty.
	ErrorEmptyIdpID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.empty_idp_id",
			DefaultValue: "IDP id is empty",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.empty_idp_id_description",
			DefaultValue: "The identity provider id cannot be empty",
		},
	}
	// ErrorInvalidIDP is the error when the retrieved IDP is invalid.
	ErrorInvalidIDP = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.invalid_idp",
			DefaultValue: "Invalid identity provider",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.invalid_idp_description",
			DefaultValue: "The retrieved identity provider is invalid or empty",
		},
	}
	// ErrorEmptyAuthorizationCode is the error when the authorization code is empty.
	ErrorEmptyAuthorizationCode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.empty_authorization_code",
			DefaultValue: "Empty authorization code",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.empty_authorization_code_description",
			DefaultValue: "The authorization code cannot be empty",
		},
	}
	// ErrorEmptyAccessToken is the error when the access token is empty.
	ErrorEmptyAccessToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.empty_access_token",
			DefaultValue: "Empty access token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.empty_access_token_description",
			DefaultValue: "The access token cannot be empty",
		},
	}
	// ErrorClientErrorWhileRetrievingIDP is the error when there is a client error while retrieving the IDP.
	ErrorClientErrorWhileRetrievingIDP = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.failed_to_retrieve_idp",
			DefaultValue: "Failed to retrieve identity provider",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.failed_to_retrieve_idp_description",
			DefaultValue: "A client error occurred while retrieving the identity provider configuration",
		},
	}
	// ErrorEmptySubClaim is the error when the sub claim is empty.
	ErrorEmptySubClaim = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.empty_sub_claim",
			DefaultValue: "Empty sub claim",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.empty_sub_claim_description",
			DefaultValue: "The sub claim cannot be empty",
		},
	}
	// ErrorClientErrorWhileRetrievingUser is the error when there is a client error while retrieving the user.
	ErrorClientErrorWhileRetrievingUser = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.failed_to_retrieve_user",
			DefaultValue: "Failed to retrieve user",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.failed_to_retrieve_user_description",
			DefaultValue: "A client error occurred while retrieving the internal user",
		},
	}
	// ErrorInvalidTokenResponse is the error when the token response is invalid.
	ErrorInvalidTokenResponse = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "AUTH-OAUTH-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.invalid_token_response",
			DefaultValue: "Invalid token response",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.invalid_token_response_description",
			DefaultValue: "The token response received from the identity provider is invalid",
		},
	}
	// ErrorUserProfileRetrievalFailed is the error when the provider's profile endpoint rejects the request.
	ErrorUserProfileRetrievalFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.user_profile_retrieval_failed",
			DefaultValue: "Failed to retrieve the user profile",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authoauthservice.user_profile_retrieval_failed_description",
			DefaultValue: "The identity provider rejected the request to the configured user profile " +
				"endpoint. Verify the endpoint URL and that the configured scopes grant access to it",
		},
	}
	// ErrorNoUserProfileSource is the error when no profile endpoint is configured and the access
	// token carries no subject.
	ErrorNoUserProfileSource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-OAUTH-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.authoauthservice.no_user_profile_source",
			DefaultValue: "No user profile source available",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authoauthservice.no_user_profile_source_description",
			DefaultValue: "The identity provider has no user profile endpoint configured and the access " +
				"token carries no subject identifier. Configure a user profile endpoint on the connection",
		},
	}
)
