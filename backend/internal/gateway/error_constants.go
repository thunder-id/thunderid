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
			DefaultValue: "A gateway with this name is already registered",
		},
	}
	// ErrorGatewayConnectionRequired is returned when a registration is missing what it takes to
	// reach the data plane.
	ErrorGatewayConnectionRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.connection_required",
			DefaultValue: "A base URL, client ID and client secret are required to reach the gateway",
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
	// ErrorInvalidRegistrationBody is returned when the request body is not readable.
	ErrorInvalidRegistrationBody = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.invalid_request_body",
			DefaultValue: "The request body is not valid JSON",
		},
	}
	// ErrorNothingToApply is returned when the control plane holds no configuration to send.
	ErrorNothingToApply = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.nothing_to_apply",
			DefaultValue: "There is no configuration to apply",
		},
	}
	// ErrorGatewayUnreachable is returned when the gateway could not be reached or refused the apply.
	ErrorGatewayUnreachable = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "GTW-5001",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.gateway_unreachable",
			DefaultValue: "The gateway could not be reached, or it refused the configuration",
		},
	}
	// ErrorGatewayCredentialUnreadable is returned when the stored credential cannot be recovered,
	// which usually means the deployment's configuration key changed.
	ErrorGatewayCredentialUnreadable = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "GTW-5002",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.credential_unreadable",
			DefaultValue: "The stored gateway credential could not be read; register the gateway again",
		},
	}
)
