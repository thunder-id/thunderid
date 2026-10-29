// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernanceprovider

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// errGovernanceUnavailable is returned for a lifecycle operation when identity governance is disabled.
var errGovernanceUnavailable = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "IGP-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.identitygovernanceprovider.unavailable",
		DefaultValue: "Identity governance is not available",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.identitygovernanceprovider.unavailable_description",
		DefaultValue: "No identity governance authority is configured for this deployment",
	},
}
