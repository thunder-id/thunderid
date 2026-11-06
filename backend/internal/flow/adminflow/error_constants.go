// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package adminflow

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

var (
	// ErrorConfiguredFlowMissing is returned when the server configuration names an administration flow
	// that no longer exists.
	ErrorConfiguredFlowMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AFR-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.administrationflowrunner.configured_flow_missing",
			DefaultValue: "Configured administration flow not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.administrationflowrunner.configured_flow_missing_description",
			DefaultValue: "The server configuration names an administration flow for this action that no longer " +
				"exists, so nothing was changed. Restore the flow or clear it from the configuration",
		},
	}

	// ErrorFlowNeedsInput is returned when a configured administration flow stops to ask for input.
	ErrorFlowNeedsInput = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AFR-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.administrationflowrunner.flow_needs_input",
			DefaultValue: "Administration flow needs more input",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.administrationflowrunner.flow_needs_input_description",
			DefaultValue: "The configured administration flow asks for input this request can't give, so the " +
				"change was not completed",
		},
	}
)
