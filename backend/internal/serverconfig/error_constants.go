// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"errors"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for server-config operations.
var (
	// ErrorUnsupportedConfigName is the error returned when the config name is not supported.
	ErrorUnsupportedConfigName = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "SCF-1001",
		Error: common.I18nMessage{
			Key:          "error.serverconfigservice.unsupported_config_name",
			DefaultValue: "Unsupported configuration name",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.serverconfigservice.unsupported_config_name_description",
			DefaultValue: "The requested server configuration name is not supported",
		},
	}

	// ErrorInvalidConfigValue is the error returned when the config value is invalid.
	ErrorInvalidConfigValue = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "SCF-1003",
		Error: common.I18nMessage{
			Key:          "error.serverconfigservice.invalid_config_value",
			DefaultValue: "Invalid server configuration value",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.serverconfigservice.invalid_config_value_description",
			DefaultValue: "The provided server configuration value is invalid",
		},
	}

	// ErrorConfigWriteConflict is the error returned when a section kept changing under an update, so
	// it could not be written without losing another write.
	ErrorConfigWriteConflict = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "SCF-1005",
		Error: common.I18nMessage{
			Key:          "error.serverconfigservice.config_write_conflict",
			DefaultValue: "The server configuration changed while it was updated",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.serverconfigservice.config_write_conflict_description",
			DefaultValue: "Other writes kept changing the section. Try again.",
		},
	}
	// ErrorManagedConfig is the error returned when the API attempts to change a configuration section a
	// control plane deployed. Only a deploy from the control plane changes it.
	ErrorManagedConfig = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "SCF-1006",
		Error: common.I18nMessage{
			Key:          "error.serverconfigservice.managed_config",
			DefaultValue: "Cannot modify a configuration section deployed by the control plane",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.serverconfigservice.managed_config_description",
			DefaultValue: "The section was deployed by the control plane and can only be changed there",
		},
	}

	// ErrorInvalidRequestFormat is the error returned when the request body is malformed.
	ErrorInvalidRequestFormat = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "SCF-1004",
		Error: common.I18nMessage{
			Key:          "error.serverconfigservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.serverconfigservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
)

// errNilEventBus is returned when the service is initialized without the event bus it audits on.
var errNilEventBus = errors.New("the server config service needs an event bus")
