// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federatedsync

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

// Client errors for the federated sync service.
var (
	// ErrorFederatedSyncFailed is the error returned when the account's groups or roles refuse the sync, or
	// an Override all mapping can't be fully resolved.
	ErrorFederatedSyncFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-SYNC-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.federatedsyncservice.federated_sync_failed",
			DefaultValue: "Federated sync failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.federatedsyncservice.federated_sync_failed_description",
			DefaultValue: "The account could not be updated with the identity provider's data",
		},
	}
)
