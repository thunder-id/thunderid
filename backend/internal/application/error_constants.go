/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package application

import (
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for application operations.
var (
	// ErrorApplicationNotFound is the error returned when an application is not found.
	ErrorApplicationNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_not_found",
			DefaultValue: "Application not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_not_found_description",
			DefaultValue: "The requested application could not be found",
		},
	}
	// ErrorInvalidApplicationID is the error returned when an invalid application ID is provided.
	ErrorInvalidApplicationID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_application_id",
			DefaultValue: "Invalid application ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_application_id_description",
			DefaultValue: "The provided application ID is invalid or empty",
		},
	}
	// ErrorInvalidClientID is the error returned when an invalid client ID is provided.
	ErrorInvalidClientID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_client_id",
			DefaultValue: "Invalid client ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_client_id_description",
			DefaultValue: "The provided client ID is invalid or empty",
		},
	}
	// ErrorInvalidApplicationName is the error returned when an invalid application name is provided.
	ErrorInvalidApplicationName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_application_name",
			DefaultValue: "Invalid application name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_application_name_description",
			DefaultValue: "The provided application name is invalid or empty",
		},
	}
	// ErrorInvalidApplicationURL is the error returned when an invalid application URL is provided.
	ErrorInvalidApplicationURL = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_application_url",
			DefaultValue: "Invalid application URL",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_application_url_description",
			DefaultValue: "The provided application URL is not a valid URI",
		},
	}
	// ErrorInvalidLogoURL is the error returned when an invalid logo URL is provided.
	ErrorInvalidLogoURL = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_logo_url",
			DefaultValue: "Invalid logo URL",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_logo_url_description",
			DefaultValue: "The provided logo URL is not a valid URI",
		},
	}
	// ErrorInvalidAuthFlowID is the error returned when an invalid auth flow ID is provided.
	ErrorInvalidAuthFlowID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_auth_flow_id",
			DefaultValue: "Invalid auth flow ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_auth_flow_id_description",
			DefaultValue: "The provided authentication flow ID is invalid",
		},
	}
	// ErrorInvalidRegistrationFlowID is the error returned when an invalid registration flow ID
	// is provided.
	ErrorInvalidRegistrationFlowID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_registration_flow_id",
			DefaultValue: "Invalid registration flow ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_registration_flow_id_description",
			DefaultValue: "The provided registration flow ID is invalid",
		},
	}
	// ErrorInvalidInboundAuthConfig is the error returned when invalid inbound auth config is provided.
	ErrorInvalidInboundAuthConfig = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_inbound_auth_config",
			DefaultValue: "Invalid inbound auth config",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_inbound_auth_config_description",
			DefaultValue: "The provided inbound authentication configuration is invalid",
		},
	}
	// ErrorInvalidGrantType is the error returned when an invalid grant type is provided.
	ErrorInvalidGrantType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_grant_type",
			DefaultValue: "Invalid grant type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_grant_type_description",
			DefaultValue: "One or more provided grant types are invalid",
		},
	}
	// ErrorInvalidResponseType is the error returned when an invalid response type is provided.
	ErrorInvalidResponseType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_response_type",
			DefaultValue: "Invalid response type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_response_type_description",
			DefaultValue: "One or more provided response types are invalid",
		},
	}
	// ErrorInvalidRedirectURI is the error returned when an invalid redirect URI is provided.
	ErrorInvalidRedirectURI = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_redirect_uri",
			DefaultValue: "Invalid redirect URI",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_redirect_uri_description",
			DefaultValue: "One or more provided redirect URIs are not valid URIs",
		},
	}
	// ErrorInvalidTokenEndpointAuthMethod is the error returned when an invalid token endpoint auth method
	// is provided.
	ErrorInvalidTokenEndpointAuthMethod = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_token_endpoint_auth_method",
			DefaultValue: "Invalid token endpoint authentication method",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_token_endpoint_auth_method_description",
			DefaultValue: "The provided token endpoint authentication method is invalid",
		},
	}
	// ErrorInvalidCertificateType is the error returned when an invalid certificate type is provided.
	ErrorInvalidCertificateType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_certificate_type",
			DefaultValue: "Invalid certificate type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_certificate_type_description",
			DefaultValue: "The provided certificate type is not supported",
		},
	}
	// ErrorInvalidCertificateValue is the error returned when an invalid certificate value is provided.
	ErrorInvalidCertificateValue = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_certificate_value",
			DefaultValue: "Invalid certificate value",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_certificate_value_description",
			DefaultValue: "The provided certificate value is invalid",
		},
	}
	// ErrorInvalidJWKSURI is the error returned when an invalid JWKS URI is provided.
	ErrorInvalidJWKSURI = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_jwks_uri",
			DefaultValue: "Invalid JWKS URI",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_jwks_uri_description",
			DefaultValue: "The provided JWKS URI is not a valid URI",
		},
	}
	// ErrorApplicationNil is the error returned when the application object is nil.
	ErrorApplicationNil = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_is_nil",
			DefaultValue: "Application is nil",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_is_nil_description",
			DefaultValue: "The provided application object is nil",
		},
	}
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorCertificateClientError is the error returned when a certificate operation fails due to client error.
	ErrorCertificateClientError = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.certificate_operation_failed",
			DefaultValue: "Certificate operation failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.certificate_operation_failed_description",
			DefaultValue: "An error occurred while processing the application certificate",
		},
	}
	// ErrorApplicationAlreadyExistsWithName is the error returned when an application with the same name
	// already exists.
	ErrorApplicationAlreadyExistsWithName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_already_exists",
			DefaultValue: "Application already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_already_exists_description",
			DefaultValue: "An application with the same name already exists",
		},
	}
	// ErrorApplicationAlreadyExistsWithClientID is the error returned when an application with the same client ID
	// already exists.
	ErrorApplicationAlreadyExistsWithClientID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_with_client_id_already_exists",
			DefaultValue: "Application with client ID already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_with_client_id_already_exists_description",
			DefaultValue: "An application with the same client ID already exists",
		},
	}
	// ErrorJWKSUriNotHTTPS is the error returned when jwks_uri does not use HTTPS.
	ErrorJWKSUriNotHTTPS = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_jwks_uri_scheme",
			DefaultValue: "Invalid JWKS URI scheme",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_jwks_uri_scheme_description",
			DefaultValue: "'jwks_uri' must use HTTPS scheme",
		},
	}
	// ErrorInvalidPublicClientConfiguration is the generic error returned for public client configuration issues.
	ErrorInvalidPublicClientConfiguration = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1023",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_public_client_configuration",
			DefaultValue: "Invalid public client configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_public_client_configuration_description",
			DefaultValue: "The public client configuration is invalid",
		},
	}
	// ErrorInvalidOAuthConfiguration is the generic error returned for OAuth configuration issues.
	ErrorInvalidOAuthConfiguration = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1024",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_oauth_configuration",
			DefaultValue: "Invalid OAuth configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_oauth_configuration_description",
			DefaultValue: "The OAuth configuration is invalid",
		},
	}
	// ErrorInvalidUserType is the error returned when an invalid user type is provided in allowed_user_types.
	ErrorInvalidUserType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_user_type",
			DefaultValue: "Invalid user type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_user_type_description",
			DefaultValue: "One or more user types in allowed_user_types do not exist in the system",
		},
	}
	// ErrorThemeNotFound is the error returned when theme is not found.
	ErrorThemeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.theme_not_found",
			DefaultValue: "Theme not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.theme_not_found_description",
			DefaultValue: "The specified theme configuration does not exist",
		},
	}
	// ErrorLayoutNotFound is the error returned when layout is not found.
	ErrorLayoutNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1027",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.layout_not_found",
			DefaultValue: "Layout not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.layout_not_found_description",
			DefaultValue: "The specified layout configuration does not exist",
		},
	}
	// ErrorWhileRetrievingFlowDefinition is the error returned when there is an issue retrieving flow definition.
	ErrorWhileRetrievingFlowDefinition = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1028",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.error_retrieving_flow_definition",
			DefaultValue: "Error retrieving flow definition",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.error_retrieving_flow_definition_description",
			DefaultValue: "An error occurred while retrieving the flow definition",
		},
	}
	// ErrorResultLimitExceeded is the error returned when the result limit is exceeded in composite mode.
	ErrorResultLimitExceeded = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1029",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.result_limit_exceeded_description",
			DefaultValue: serverconst.CompositeStoreLimitWarning,
		},
	}
	// ErrorCannotModifyDeclarativeResource is the error returned when trying to modify a declarative resource.
	ErrorCannotModifyDeclarativeResource = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1030",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.cannot_modify_declarative_resource_description",
			DefaultValue: "The application is declarative and cannot be modified or deleted",
		},
	}
	// ErrorInvalidAcrValues is the error returned when an unrecognized ACR value is provided in acrValues.
	ErrorInvalidAcrValues = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1033",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_acr_values",
			DefaultValue: "Invalid ACR value",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_acr_values_description",
			DefaultValue: "One or more ACR values in acr_values are not recognized by the system",
		},
	}
	// ErrorMultipleOAuthConfigs is returned when more than one OAuth inbound auth config is supplied.
	ErrorMultipleOAuthConfigs = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1034",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.multiple_oauth_configs",
			DefaultValue: "Multiple OAuth inbound auth configs are not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.multiple_oauth_configs_description",
			DefaultValue: "An application may have at most one inbound auth config per protocol",
		},
	}
	// ErrorInvalidUserAttribute is the error returned when a user attribute is not valid for any
	// of the application's allowed user types.
	ErrorInvalidUserAttribute = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1035",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_user_attribute",
			DefaultValue: "Invalid user attribute",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_user_attribute_description",
			DefaultValue: "One or more user attributes are not valid for the configured allowed user types",
		},
	}
	// ErrorInvalidRecoveryFlowID is the error returned when an invalid recovery flow ID is provided.
	ErrorInvalidRecoveryFlowID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1036",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_recovery_flow_id",
			DefaultValue: "Invalid recovery flow ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_recovery_flow_id_description",
			DefaultValue: "The provided recovery flow ID is invalid",
		},
	}
	// ErrorNativeFlowNotAllowedForSPA is returned when a public client (SPA) is configured for
	// native (embedded) flow execution instead of a redirect-based authorization_code flow.
	ErrorNativeFlowNotAllowedForSPA = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1037",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.native_flow_not_allowed_for_spa",
			DefaultValue: "Native flow execution is not allowed for single-page applications",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.native_flow_not_allowed_for_spa_description",
			DefaultValue: "Single-page applications (public clients) must use the authorization_code grant type " +
				"with PKCE for redirect-based flows. Direct (native) flow execution is not supported for " +
				"browser-based single-page applications.",
		},
	}
	// ErrorAmbiguousAttestationConfig is returned when an application's attestation configuration
	// sets more than one platform, which the flow-initiation verifier cannot unambiguously dispatch.
	ErrorAmbiguousAttestationConfig = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1038",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.ambiguous_attestation_config",
			DefaultValue: "Attestation configuration must set exactly one platform",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.ambiguous_attestation_config_description",
			DefaultValue: "An application's attestation configuration may configure only one platform " +
				"(android or apple) at a time",
		},
	}
	// ErrorApplicationFlowMismatch is returned when a flow reached (via a CALL node) from one of the
	// flows configured on the application conflicts with the application's binding of the same
	// flow type — either the binding points at a different flow, or no binding exists.
	ErrorApplicationFlowMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1039",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_flow_mismatch",
			DefaultValue: "Conflicting flow references",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.application_flow_mismatch_description",
			DefaultValue: "The {{param(sourceFlowType)}} flow references a different " +
				"{{param(flowType)}} flow than the one configured on the application. " +
				"Both must point to the same {{param(flowType)}} flow.",
		},
	}
	// ErrorInvalidTosURI is the error returned when an invalid Terms of Service URI is provided.
	ErrorInvalidTosURI = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1040",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_tos_uri",
			DefaultValue: "Invalid Terms of Service URI",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_tos_uri_description",
			DefaultValue: "The provided Terms of Service URI is not a valid URI",
		},
	}
	// ErrorInvalidPolicyURI is the error returned when an invalid Privacy Policy URI is provided
	ErrorInvalidPolicyURI = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1041",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_policy_uri",
			DefaultValue: "Invalid Privacy Policy URI",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_policy_uri_description",
			DefaultValue: "The provided Privacy Policy URI is not a valid URI",
		},
	}
)
