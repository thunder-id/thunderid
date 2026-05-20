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
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for application operations.
var (
	// ErrorApplicationNotFound is the error returned when an application is not found.
	ErrorApplicationNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1001",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.application_not_found",
			DefaultValue: "Application not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.application_not_found_description",
			DefaultValue: "The requested application could not be found",
		},
	}
	// ErrorInvalidApplicationID is the error returned when an invalid application ID is provided.
	ErrorInvalidApplicationID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1002",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_application_id",
			DefaultValue: "Invalid application ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_application_id_description",
			DefaultValue: "The provided application ID is invalid or empty",
		},
	}
	// ErrorInvalidClientID is the error returned when an invalid client ID is provided.
	ErrorInvalidClientID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1003",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_client_id",
			DefaultValue: "Invalid client ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_client_id_description",
			DefaultValue: "The provided client ID is invalid or empty",
		},
	}
	// ErrorInvalidApplicationName is the error returned when an invalid application name is provided.
	ErrorInvalidApplicationName = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1004",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_application_name",
			DefaultValue: "Invalid application name",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_application_name_description",
			DefaultValue: "The provided application name is invalid or empty",
		},
	}
	// ErrorInvalidApplicationURL is the error returned when an invalid application URL is provided.
	ErrorInvalidApplicationURL = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1005",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_application_url",
			DefaultValue: "Invalid application URL",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_application_url_description",
			DefaultValue: "The provided application URL is not a valid URI",
		},
	}
	// ErrorInvalidLogoURL is the error returned when an invalid logo URL is provided.
	ErrorInvalidLogoURL = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1006",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_logo_url",
			DefaultValue: "Invalid logo URL",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_logo_url_description",
			DefaultValue: "The provided logo URL is not a valid URI",
		},
	}
	// ErrorInvalidAuthFlowID is the error returned when an invalid auth flow ID is provided.
	ErrorInvalidAuthFlowID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1007",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_auth_flow_id",
			DefaultValue: "Invalid auth flow ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_auth_flow_id_description",
			DefaultValue: "The provided authentication flow ID is invalid",
		},
	}
	// ErrorInvalidRegistrationFlowID is the error returned when an invalid registration flow ID
	// is provided.
	ErrorInvalidRegistrationFlowID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1008",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_registration_flow_id",
			DefaultValue: "Invalid registration flow ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_registration_flow_id_description",
			DefaultValue: "The provided registration flow ID is invalid",
		},
	}
	// ErrorInvalidInboundAuthConfig is the error returned when invalid inbound auth config is provided.
	ErrorInvalidInboundAuthConfig = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1009",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_inbound_auth_config",
			DefaultValue: "Invalid inbound auth config",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_inbound_auth_config_description",
			DefaultValue: "The provided inbound authentication configuration is invalid",
		},
	}
	// ErrorInvalidGrantType is the error returned when an invalid grant type is provided.
	ErrorInvalidGrantType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1010",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_grant_type",
			DefaultValue: "Invalid grant type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_grant_type_description",
			DefaultValue: "One or more provided grant types are invalid",
		},
	}
	// ErrorInvalidResponseType is the error returned when an invalid response type is provided.
	ErrorInvalidResponseType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1011",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_response_type",
			DefaultValue: "Invalid response type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_response_type_description",
			DefaultValue: "One or more provided response types are invalid",
		},
	}
	// ErrorInvalidRedirectURI is the error returned when an invalid redirect URI is provided.
	ErrorInvalidRedirectURI = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1012",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_redirect_uri",
			DefaultValue: "Invalid redirect URI",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_redirect_uri_description",
			DefaultValue: "One or more provided redirect URIs are not valid URIs",
		},
	}
	// ErrorInvalidTokenEndpointAuthMethod is the error returned when an invalid token endpoint auth method
	// is provided.
	ErrorInvalidTokenEndpointAuthMethod = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1013",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_token_endpoint_auth_method",
			DefaultValue: "Invalid token endpoint authentication method",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_token_endpoint_auth_method_description",
			DefaultValue: "The provided token endpoint authentication method is invalid",
		},
	}
	// ErrorInvalidCertificateType is the error returned when an invalid certificate type is provided.
	ErrorInvalidCertificateType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1014",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_certificate_type",
			DefaultValue: "Invalid certificate type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_certificate_type_description",
			DefaultValue: "The provided certificate type is not supported",
		},
	}
	// ErrorInvalidCertificateValue is the error returned when an invalid certificate value is provided.
	ErrorInvalidCertificateValue = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1015",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_certificate_value",
			DefaultValue: "Invalid certificate value",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_certificate_value_description",
			DefaultValue: "The provided certificate value is invalid",
		},
	}
	// ErrorInvalidJWKSURI is the error returned when an invalid JWKS URI is provided.
	ErrorInvalidJWKSURI = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1016",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_jwks_uri",
			DefaultValue: "Invalid JWKS URI",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_jwks_uri_description",
			DefaultValue: "The provided JWKS URI is not a valid URI",
		},
	}
	// ErrorApplicationNil is the error returned when the application object is nil.
	ErrorApplicationNil = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1017",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.application_is_nil",
			DefaultValue: "Application is nil",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.application_is_nil_description",
			DefaultValue: "The provided application object is nil",
		},
	}
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1018",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorCertificateClientError is the error returned when a certificate operation fails due to client error.
	ErrorCertificateClientError = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1019",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.certificate_operation_failed",
			DefaultValue: "Certificate operation failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.certificate_operation_failed_description",
			DefaultValue: "An error occurred while processing the application certificate",
		},
	}
	// ErrorApplicationAlreadyExistsWithName is the error returned when an application with the same name
	// already exists.
	ErrorApplicationAlreadyExistsWithName = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1020",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.application_already_exists",
			DefaultValue: "Application already exists",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.application_already_exists_description",
			DefaultValue: "An application with the same name already exists",
		},
	}
	// ErrorApplicationAlreadyExistsWithClientID is the error returned when an application with the same client ID
	// already exists.
	ErrorApplicationAlreadyExistsWithClientID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1021",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.application_with_client_id_already_exists",
			DefaultValue: "Application with client ID already exists",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.application_with_client_id_already_exists_description",
			DefaultValue: "An application with the same client ID already exists",
		},
	}
	// ErrorJWKSUriNotHTTPS is the error returned when jwks_uri does not use HTTPS.
	ErrorJWKSUriNotHTTPS = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1022",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_jwks_uri_scheme",
			DefaultValue: "Invalid JWKS URI scheme",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_jwks_uri_scheme_description",
			DefaultValue: "'jwks_uri' must use HTTPS scheme",
		},
	}
	// ErrorInvalidPublicClientConfiguration is the generic error returned for public client configuration issues.
	ErrorInvalidPublicClientConfiguration = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1023",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_public_client_configuration",
			DefaultValue: "Invalid public client configuration",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_public_client_configuration_description",
			DefaultValue: "The public client configuration is invalid",
		},
	}
	// ErrorInvalidOAuthConfiguration is the generic error returned for OAuth configuration issues.
	ErrorInvalidOAuthConfiguration = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1024",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_oauth_configuration",
			DefaultValue: "Invalid OAuth configuration",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_oauth_configuration_description",
			DefaultValue: "The OAuth configuration is invalid",
		},
	}
	// ErrorInvalidUserType is the error returned when an invalid user type is provided in allowed_user_types.
	ErrorInvalidUserType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1025",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_user_type",
			DefaultValue: "Invalid user type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_user_type_description",
			DefaultValue: "One or more user types in allowed_user_types do not exist in the system",
		},
	}
	// ErrorThemeNotFound is the error returned when theme is not found.
	ErrorThemeNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1026",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.theme_not_found",
			DefaultValue: "Theme not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.theme_not_found_description",
			DefaultValue: "The specified theme configuration does not exist",
		},
	}
	// ErrorLayoutNotFound is the error returned when layout is not found.
	ErrorLayoutNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1027",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.layout_not_found",
			DefaultValue: "Layout not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.layout_not_found_description",
			DefaultValue: "The specified layout configuration does not exist",
		},
	}
	// ErrorWhileRetrievingFlowDefinition is the error returned when there is an issue retrieving flow definition.
	ErrorWhileRetrievingFlowDefinition = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1028",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.error_retrieving_flow_definition",
			DefaultValue: "Error retrieving flow definition",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.error_retrieving_flow_definition_description",
			DefaultValue: "An error occurred while retrieving the flow definition",
		},
	}
	// ErrorResultLimitExceeded is the error returned when the result limit is exceeded in composite mode.
	ErrorResultLimitExceeded = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1029",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.result_limit_exceeded_description",
			DefaultValue: serverconst.CompositeStoreLimitWarning,
		},
	}
	// ErrorCannotModifyDeclarativeResource is the error returned when trying to modify a declarative resource.
	ErrorCannotModifyDeclarativeResource = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1030",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.cannot_modify_declarative_resource_description",
			DefaultValue: "The application is declarative and cannot be modified or deleted",
		},
	}
	// ErrorConsentSyncFailed is the error returned when an application's attributes changes failed to sync
	// with the consent service.
	ErrorConsentSyncFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1031",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.consent_synchronization_failed",
			DefaultValue: "Consent synchronization failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.consent_synchronization_failed_description",
			DefaultValue: "Failed to synchronize consent configurations for the application",
		},
	}
	// ErrorConsentServiceNotEnabled is the error returned when enabling consent for application while
	// the consent service is not enabled.
	ErrorConsentServiceNotEnabled = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1032",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.consent_service_not_enabled",
			DefaultValue: "Consent service not enabled",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.consent_service_not_enabled_description",
			DefaultValue: "Cannot enable consent for the application as the consent service is not enabled",
		},
	}
	// ErrorInvalidAcrValues is the error returned when an unrecognized ACR value is provided in acrValues.
	ErrorInvalidAcrValues = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1033",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_acr_values",
			DefaultValue: "Invalid ACR value",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_acr_values_description",
			DefaultValue: "One or more ACR values in acr_values are not recognized by the system",
		},
	}
	// ErrorMultipleOAuthConfigs is returned when more than one OAuth inbound auth config is supplied.
	ErrorMultipleOAuthConfigs = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1034",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.multiple_oauth_configs",
			DefaultValue: "Multiple OAuth inbound auth configs are not allowed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.multiple_oauth_configs_description",
			DefaultValue: "An application may have at most one inbound auth config per protocol",
		},
	}
	// ErrorInvalidUserAttribute is the error returned when a user attribute is not valid for any
	// of the application's allowed user types.
	ErrorInvalidUserAttribute = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1035",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_user_attribute",
			DefaultValue: "Invalid user attribute",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_user_attribute_description",
			DefaultValue: "One or more user attributes are not valid for the configured allowed user types",
		},
	}
	// ErrorInvalidRecoveryFlowID is the error returned when an invalid recovery flow ID is provided.
	ErrorInvalidRecoveryFlowID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "APP-1036",
		Error: core.I18nMessage{
			Key:          "error.applicationservice.invalid_recovery_flow_id",
			DefaultValue: "Invalid recovery flow ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.applicationservice.invalid_recovery_flow_id_description",
			DefaultValue: "The provided recovery flow ID is invalid",
		},
	}
)
