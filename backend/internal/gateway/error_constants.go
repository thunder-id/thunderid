// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

var (
	// ErrorInvalidGatewayID is returned when no usable gateway id is supplied.
	ErrorInvalidGatewayID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.invalid_gateway_id",
			DefaultValue: "Invalid gateway ID",
		},
	}
	// ErrorGatewayNotFound is returned when no gateway of this deployment has that id.
	ErrorGatewayNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.gateway_not_found",
			DefaultValue: "Gateway not found",
		},
	}
	// ErrorGatewayNameRequired is returned when a registration carries no name.
	ErrorGatewayNameRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.name_required",
			DefaultValue: "A gateway name is required",
		},
	}
	// ErrorGatewayNameTaken is returned when the deployment already has a gateway of that name.
	ErrorGatewayNameTaken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.name_taken",
			DefaultValue: "Another gateway is already registered under that name",
		},
	}

	// ErrorInvalidBaseURL is returned when a base URL is not an absolute http or https URL.
	ErrorInvalidBaseURL = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.invalid_base_url",
			DefaultValue: "Invalid base URL",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.invalid_base_url_description",
			DefaultValue: "A base URL must be an absolute http or https URL, such as https://dp.example.com:8090",
		},
	}

	// ErrorGatewayAlreadyRegistered is returned when a gateway already answers at that address.
	ErrorGatewayAlreadyRegistered = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.gateway.already_registered",
			DefaultValue: "Gateway already registered",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gateway.already_registered_description",
			DefaultValue: "Another gateway is already registered at that address",
		},
	}
	// ErrorGatewayConnectionRequired is returned when a registration is missing what it takes to
	// reach the gateway.
	ErrorGatewayConnectionRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.connection_required",
			DefaultValue: "A base URL is required to reach the gateway",
		},
	}
	// ErrorGatewayLimitReached is returned when the deployment already holds as many gateways as it
	// is configured to administer.
	ErrorGatewayLimitReached = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.limit_reached",
			DefaultValue: "This deployment already administers as many gateways as it is configured to allow",
		},
	}
	// ErrorNothingToUpdate is returned when an update names no field to change.
	ErrorNothingToUpdate = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.nothing_to_update",
			DefaultValue: "The request changes nothing: name, baseUrl, caCertificate or key is required",
		},
	}
	// ErrorInvalidRegistrationBody is returned when the request body is not readable.
	ErrorInvalidRegistrationBody = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.invalid_request_body",
			DefaultValue: "The request body is not valid JSON",
		},
	}
	// ErrorGatewayKeyRequired is returned when an edit names a key with nothing in it. Omitting the
	// field keeps the stored key; sending an empty one asks for a credential that cannot be
	// presented, which is a mistake rather than a way to clear it.
	ErrorGatewayKeyRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.gateway.key_required",
			DefaultValue: "A key cannot be empty",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gateway.key_required.description",
			DefaultValue: "Omit the key to keep the one this gateway already holds, or send the " +
				"value the gateway expects.",
		},
	}
)
