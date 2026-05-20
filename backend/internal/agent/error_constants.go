/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package agent

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for agent operations. Codes follow the AGT-* convention from api/agent.yaml.
var (
	// ErrorInvalidRequestFormat is returned when the request body cannot be decoded.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1001",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}

	// ErrorInvalidRedirectURI is returned when a redirect URI fails validation.
	ErrorInvalidRedirectURI = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1002",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_redirect_uri",
			DefaultValue: "Invalid redirect URI",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_redirect_uri_description",
			DefaultValue: "One or more redirect URIs are not valid",
		},
	}

	// ErrorInvalidGrantType is returned when an unsupported grant type is requested.
	ErrorInvalidGrantType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1003",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_grant_type",
			DefaultValue: "Invalid grant type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_grant_type_description",
			DefaultValue: "One or more grant types are not supported",
		},
	}

	// ErrorAgentNotFound is returned when no agent exists with the given identifier.
	ErrorAgentNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1004",
		Error: core.I18nMessage{
			Key:          "error.agentservice.agent_not_found",
			DefaultValue: "Agent not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.agent_not_found_description",
			DefaultValue: "The agent with the specified id does not exist",
		},
	}

	// ErrorOrganizationUnitNotFound is returned when the target OU cannot be resolved.
	ErrorOrganizationUnitNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1005",
		Error: core.I18nMessage{
			Key:          "error.agentservice.organization_unit_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.organization_unit_not_found_description",
			DefaultValue: "The specified organization unit does not exist",
		},
	}

	// ErrorMissingAgentID is returned when the path id is empty.
	ErrorMissingAgentID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1008",
		Error: core.I18nMessage{
			Key:          "error.agentservice.missing_agent_id",
			DefaultValue: "Missing agent ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.missing_agent_id_description",
			DefaultValue: "The agent ID is required",
		},
	}

	// ErrorInvalidAgentName is returned when name is empty.
	ErrorInvalidAgentName = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1009",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_agent_name",
			DefaultValue: "Invalid agent name",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_agent_name_description",
			DefaultValue: "The agent name must be provided and non-empty",
		},
	}

	// ErrorInvalidAgentType is returned when type is empty.
	ErrorInvalidAgentType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1010",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_agent_type",
			DefaultValue: "Invalid agent type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_agent_type_description",
			DefaultValue: "The agent type must be provided",
		},
	}

	// ErrorInvalidLimit is returned for invalid pagination limit.
	ErrorInvalidLimit = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1011",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_limit",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_limit_description",
			DefaultValue: "The limit parameter must be between 1 and 100",
		},
	}

	// ErrorInvalidOffset is returned for invalid pagination offset.
	ErrorInvalidOffset = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1012",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_offset",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_offset_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}

	// ErrorAgentAlreadyExistsWithName is returned when another agent already has the same name.
	ErrorAgentAlreadyExistsWithName = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1013",
		Error: core.I18nMessage{
			Key:          "error.agentservice.agent_already_exists_with_name",
			DefaultValue: "Agent already exists",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.agent_already_exists_with_name_description",
			DefaultValue: "An agent with the same name already exists",
		},
	}

	// ErrorAttributeConflict is returned when a unique attribute clashes with another entity.
	ErrorAttributeConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1014",
		Error: core.I18nMessage{
			Key:          "error.agentservice.attribute_conflict",
			DefaultValue: "Attribute conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.attribute_conflict_description",
			DefaultValue: "An agent with the same unique attribute value already exists",
		},
	}

	// ErrorSchemaValidationFailed is returned when agent attributes fail schema validation.
	ErrorSchemaValidationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1015",
		Error: core.I18nMessage{
			Key:          "error.agentservice.schema_validation_failed",
			DefaultValue: "Schema validation failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.schema_validation_failed_description",
			DefaultValue: "The provided attributes failed schema validation",
		},
	}

	// ErrorInvalidCredential is returned when a supplied credential is invalid.
	ErrorInvalidCredential = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1016",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_credential",
			DefaultValue: "Invalid credential",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_credential_description",
			DefaultValue: "The provided credential is invalid",
		},
	}

	// ErrorInvalidFilter is returned when the filter query parameter is malformed.
	ErrorInvalidFilter = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1020",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_filter",
			DefaultValue: "Invalid filter parameter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_filter_description",
			DefaultValue: "The filter format is invalid",
		},
	}

	// ErrorInvalidOAuthConfiguration is returned for generic OAuth configuration validation failures.
	ErrorInvalidOAuthConfiguration = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1021",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_oauth_configuration",
			DefaultValue: "Invalid OAuth configuration",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_oauth_configuration_description",
			DefaultValue: "The provided OAuth configuration is invalid",
		},
	}

	// ErrorInvalidTokenEndpointAuthMethod is returned when the token endpoint auth method is not supported.
	ErrorInvalidTokenEndpointAuthMethod = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1022",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_token_endpoint_auth_method",
			DefaultValue: "Invalid token endpoint authentication method",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_token_endpoint_auth_method_description",
			DefaultValue: "The provided token endpoint authentication method is not supported",
		},
	}

	// ErrorInvalidPublicClientConfiguration is returned for public client misconfiguration.
	ErrorInvalidPublicClientConfiguration = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1023",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_public_client_configuration",
			DefaultValue: "Invalid public client configuration",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_public_client_configuration_description",
			DefaultValue: "The public client configuration is invalid",
		},
	}

	// ErrorInvalidCertificateType is returned when the certificate type is not supported.
	ErrorInvalidCertificateType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1024",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_certificate_type",
			DefaultValue: "Invalid certificate type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_certificate_type_description",
			DefaultValue: "The provided certificate type is not supported",
		},
	}

	// ErrorInvalidCertificateValue is returned when the certificate value is malformed or missing.
	ErrorInvalidCertificateValue = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1025",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_certificate_value",
			DefaultValue: "Invalid certificate value",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_certificate_value_description",
			DefaultValue: "The provided certificate value is invalid or missing",
		},
	}

	// ErrorInvalidJWKSURI is returned when the JWKS URI is not a valid SSRF-safe HTTPS URL.
	ErrorInvalidJWKSURI = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1026",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_jwks_uri",
			DefaultValue: "Invalid JWKS URI",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_jwks_uri_description",
			DefaultValue: "The JWKS URI must be a publicly reachable HTTPS URL",
		},
	}

	// ErrorCannotModifyDeclarativeResource is returned when attempting to mutate a declaratively managed agent.
	ErrorCannotModifyDeclarativeResource = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1027",
		Error: core.I18nMessage{
			Key:          "error.agentservice.cannot_modify_declarative_resource",
			DefaultValue: "Cannot modify declarative resource",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.cannot_modify_declarative_resource_description",
			DefaultValue: "Declaratively managed agents cannot be modified via the API",
		},
	}

	// ErrorInvalidAuthFlowID is returned when the referenced auth flow does not exist.
	ErrorInvalidAuthFlowID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1028",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_auth_flow_id",
			DefaultValue: "Invalid auth flow ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_auth_flow_id_description",
			DefaultValue: "The provided authentication flow ID is invalid",
		},
	}

	// ErrorInvalidRegistrationFlowID is returned when the referenced registration flow does not exist.
	ErrorInvalidRegistrationFlowID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1029",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_registration_flow_id",
			DefaultValue: "Invalid registration flow ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_registration_flow_id_description",
			DefaultValue: "The provided registration flow ID is invalid",
		},
	}

	// ErrorWhileRetrievingFlowDefinition is returned when the flow definition cannot be fetched.
	ErrorWhileRetrievingFlowDefinition = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1030",
		Error: core.I18nMessage{
			Key:          "error.agentservice.error_retrieving_flow_definition",
			DefaultValue: "Error retrieving flow definition",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.error_retrieving_flow_definition_description",
			DefaultValue: "An error occurred while retrieving the flow definition",
		},
	}

	// ErrorInvalidUserType is returned when an allowed user type does not exist.
	ErrorInvalidUserType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1031",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_user_type",
			DefaultValue: "Invalid user type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_user_type_description",
			DefaultValue: "One or more specified allowed user types are invalid",
		},
	}

	// ErrorThemeNotFound is returned when the referenced theme does not exist.
	ErrorThemeNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1032",
		Error: core.I18nMessage{
			Key:          "error.agentservice.theme_not_found",
			DefaultValue: "Theme not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.theme_not_found_description",
			DefaultValue: "The specified theme does not exist",
		},
	}

	// ErrorLayoutNotFound is returned when the referenced layout does not exist.
	ErrorLayoutNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1033",
		Error: core.I18nMessage{
			Key:          "error.agentservice.layout_not_found",
			DefaultValue: "Layout not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.layout_not_found_description",
			DefaultValue: "The specified layout does not exist",
		},
	}

	// ErrorInvalidResponseType is returned when an unsupported response type is supplied.
	ErrorInvalidResponseType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1034",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_response_type",
			DefaultValue: "Invalid response type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_response_type_description",
			DefaultValue: "One or more provided response types are invalid",
		},
	}

	// ErrorConsentSyncFailed is returned when attribute changes fail to sync with the consent service.
	ErrorConsentSyncFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1035",
		Error: core.I18nMessage{
			Key:          "error.agentservice.consent_sync_failed",
			DefaultValue: "Consent sync failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.consent_sync_failed_description",
			DefaultValue: "Failed to sync agent attribute changes with the consent service",
		},
	}

	// ErrorCertificateClientError is returned when a certificate operation fails due to a client error.
	ErrorCertificateClientError = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1036",
		Error: core.I18nMessage{
			Key:          "error.agentservice.certificate_operation_failed",
			DefaultValue: "Certificate operation failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.certificate_operation_failed_description",
			DefaultValue: "A certificate operation failed due to invalid input",
		},
	}

	// ErrorAgentAlreadyExistsWithClientID is returned when another entity already uses the supplied client ID.
	ErrorAgentAlreadyExistsWithClientID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1037",
		Error: core.I18nMessage{
			Key:          "error.agentservice.agent_already_exists_with_client_id",
			DefaultValue: "Client ID already in use",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.agent_already_exists_with_client_id_description",
			DefaultValue: "An entity with the same client ID already exists",
		},
	}

	// ErrorMultipleOAuthConfigs is returned when more than one OAuth inbound auth config is supplied.
	ErrorMultipleOAuthConfigs = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1038",
		Error: core.I18nMessage{
			Key:          "error.agentservice.multiple_oauth_configs",
			DefaultValue: "Multiple OAuth inbound auth configs are not allowed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.multiple_oauth_configs_description",
			DefaultValue: "An entity may have at most one inbound auth config per protocol",
		},
	}

	// ErrorOwnerNotFound is returned when the supplied owner identifier does not resolve to a known entity.
	ErrorOwnerNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1039",
		Error: core.I18nMessage{
			Key:          "error.agentservice.owner_not_found",
			DefaultValue: "Owner not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.owner_not_found_description",
			DefaultValue: "The specified owner does not match any known user, application, or agent",
		},
	}

	// ErrorInvalidUserAttribute is returned when a user attribute is not valid for any
	// of the agent's allowed user types.
	ErrorInvalidUserAttribute = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AGT-1040",
		Error: core.I18nMessage{
			Key:          "error.agentservice.invalid_user_attribute",
			DefaultValue: "Invalid user attribute",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.agentservice.invalid_user_attribute_description",
			DefaultValue: "One or more user attributes are not valid for the configured allowed user types",
		},
	}
)
