// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	// ErrorInvalidAgentType is the error returned when an invalid agent type is provided in
	// allowedAgentTypes.
	ErrorInvalidAgentType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1046",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_agent_type",
			DefaultValue: "Invalid agent type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_agent_type_description",
			DefaultValue: "One or more agent types in allowedAgentTypes do not exist in the system",
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
	// ErrorInvalidApplicationType is returned when an application is created or updated with an
	// unrecognized type value.
	ErrorInvalidApplicationType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1040",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_application_type",
			DefaultValue: "Invalid application type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.invalid_application_type_description",
			DefaultValue: "The provided application type is not supported. It must be one of: " +
				"browser, fullstack, mobile, m2m, mcp, custom.",
		},
	}
	// ErrorApplicationTypeImmutable is returned when an update attempts to change the application type.
	ErrorApplicationTypeImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1041",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_type_immutable",
			DefaultValue: "Application type cannot be changed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_type_immutable_description",
			DefaultValue: "The application type is set at creation and cannot be modified.",
		},
	}
	// ErrorApplicationTypeRequired is returned when an application is created without a type.
	ErrorApplicationTypeRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1042",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_type_required",
			DefaultValue: "Application type is required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.application_type_required_description",
			DefaultValue: "An application type must be provided. It must be one of: " +
				"browser, fullstack, mobile, m2m, mcp, custom.",
		},
	}
	// ErrorInvalidTosURI is the error returned when an invalid Terms of Service URI is provided.
	ErrorInvalidTosURI = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1043",
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
		Code: "APP-1044",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_policy_uri",
			DefaultValue: "Invalid Privacy Policy URI",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_policy_uri_description",
			DefaultValue: "The provided Privacy Policy URI is not a valid URI",
		},
	}
	// ErrorInvalidSubjectAttributeMapping is the error returned when the subject attribute mapping
	// references an attribute that is not unique, required, and string-typed in an allowed user type.
	ErrorInvalidSubjectAttributeMapping = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1045",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_subject_attribute_mapping",
			DefaultValue: "Invalid subject attribute mapping",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.invalid_subject_attribute_mapping_description",
			DefaultValue: "The subject attribute mapping must reference an attribute that is unique, required, " +
				"and string-typed in an allowed user type",
		},
	}
	// ErrorInvalidCredential is returned when a supplied credential is invalid.
	ErrorInvalidCredential = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1046",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_credential",
			DefaultValue: "Invalid credential",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_credential_description",
			DefaultValue: "The provided credential is invalid",
		},
	}
	// ErrorApplicationHasNoClientSecret is the error returned when a client secret regeneration targets an
	// application that authenticates without one: a public client, or one using private_key_jwt.
	ErrorApplicationHasNoClientSecret = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1047",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_has_no_client_secret",
			DefaultValue: "Application has no client secret",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_has_no_client_secret_description",
			DefaultValue: "A client secret is not applicable to this application: it authenticates without one",
		},
	}
	// ErrorApplicationHasBlockingDependencies is the error returned when the application cannot be deleted
	// because another resource holds a reference that forbids it.
	ErrorApplicationHasBlockingDependencies = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1048",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_has_blocking_dependencies",
			DefaultValue: "Application has blocking dependencies",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.application_has_blocking_dependencies_description",
			DefaultValue: "The application cannot be deleted because other resources depend on it. " +
				"Remove or reassign them first",
		},
	}
	// ErrorUnsupportedCredentialAction is the error returned when a credential action the service does
	// not implement is requested.
	ErrorUnsupportedCredentialAction = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1049",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.unsupported_credential_action",
			DefaultValue: "Unsupported credential action",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.unsupported_credential_action_description",
			DefaultValue: "The requested action is not supported for this application's credential",
		},
	}
	// ErrorInvalidCIMDClient is the error returned when a client registered from a Client ID Metadata
	// Document breaks a document rule. The description names the rule.
	ErrorInvalidCIMDClient = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1050",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_cimd_client",
			DefaultValue: "Invalid Client ID Metadata Document client",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_cimd_client_description",
			DefaultValue: "The client does not satisfy the Client ID Metadata Document rules",
		},
	}

	// Sharing policy errors. Each carries the sharing framework's own description through, so a
	// refusal keeps the half that says what is wrong.

	// ErrorInvalidSharingPolicy is returned when a sharing policy is not one the issuing
	// organization unit may write.
	ErrorInvalidSharingPolicy = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1051",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_sharing_policy",
			DefaultValue: "Invalid sharing policy",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_sharing_policy_description",
			DefaultValue: "The sharing policy is not one the issuing organization unit may write",
		},
	}
	// ErrorSharingPolicyNotFound is returned when the application holds no policy under the
	// requested id. A policy belonging to another resource answers the same way.
	ErrorSharingPolicyNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1052",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.sharing_policy_not_found",
			DefaultValue: "Sharing policy not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.sharing_policy_not_found_description",
			DefaultValue: "The application has no sharing policy with the specified id",
		},
	}
	// ErrorSharingPolicyExists is returned when the organization unit already holds a policy for
	// this application.
	ErrorSharingPolicyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1053",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.sharing_policy_exists",
			DefaultValue: "Sharing policy already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.sharing_policy_exists_description",
			DefaultValue: "This organization unit already has a sharing policy for the " +
				"application; edit it instead",
		},
	}
	// ErrorSharingPolicyVersionMismatch is returned when an edit names a version the policy has
	// moved past.
	ErrorSharingPolicyVersionMismatch = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1054",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.sharing_policy_version_mismatch",
			DefaultValue: "Sharing policy version mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.sharing_policy_version_mismatch_description",
			DefaultValue: "The sharing policy changed since it was read; read it again and " +
				"retry the edit",
		},
	}
	// ErrorSharingPolicyDeclared is returned when an edit or a delete is aimed at a policy a
	// resource file declares.
	ErrorSharingPolicyDeclared = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1055",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.sharing_policy_declared",
			DefaultValue: "Sharing policy is declared",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.sharing_policy_declared_description",
			DefaultValue: "The sharing policy is declared in a resource file and cannot be " +
				"changed through the API",
		},
	}
	// ErrorApplicationNotSharedToOU is returned when an organization unit cannot see the
	// application, so there are no terms to resolve for it.
	ErrorApplicationNotSharedToOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1056",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.application_not_shared_to_ou",
			DefaultValue: "Application is not shared to the organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.applicationservice.application_not_shared_to_ou_description",
			DefaultValue: "No sharing policy reaches the specified organization unit, so the " +
				"application is not available there",
		},
	}
	// ErrorInvalidLimit is returned when a sharing policy listing asks for a page size that is not
	// a positive integer within the maximum. A value the handler cannot parse and one the framework
	// refuses are the same mistake, so they answer alike.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1057",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer within the maximum page size",
		},
	}
	// ErrorInvalidOffset is returned when a sharing policy listing starts before the first result.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "APP-1058",
		Error: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.applicationservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must not be negative",
		},
	}
)
