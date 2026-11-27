// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for agent provider operations. Codes follow the AGP-* convention. Failures raised by
// the agent service keep their own AGT-* codes and are not remapped here.
var (
	// ErrorInvalidRequestFormat is returned when no agent is supplied.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AGP-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.agentmgtprovider.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.agentmgtprovider.invalid_request_format_description",
			DefaultValue: "The agent to provision must be provided",
		},
	}

	// ErrorAgentProvisioningDisabled is returned when the agent provider is disabled by configuration.
	ErrorAgentProvisioningDisabled = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AGP-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.agentmgtprovider.agent_provisioning_disabled",
			DefaultValue: "Agent provisioning is disabled",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.agentmgtprovider.agent_provisioning_disabled_description",
			DefaultValue: "The deployment is not configured to provision agents from the runtime",
		},
	}

	// ErrorAgentManagementDisabled is returned when the agent provider is disabled by configuration and an agent's
	// credentials are changed from the runtime.
	ErrorAgentManagementDisabled = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AGP-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.agentmgtprovider.agent_management_disabled",
			DefaultValue: "Agent management is disabled",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.agentmgtprovider.agent_management_disabled_description",
			DefaultValue: "The deployment is not configured to change agents from the runtime",
		},
	}
)
