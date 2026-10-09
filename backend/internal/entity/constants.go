// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"

// reservedSystemAttributes are the server-owned system attribute keys that survive a wholesale
// replacement of the blob. Both write paths replace it, and the services that own an entity rebuild
// it from their own model, so a key that is not listed here is dropped by an unrelated update.
var reservedSystemAttributes = []string{
	authnprovidercm.SystemAttrCredentialUpdatedAt,
	authnprovidercm.SystemAttrLinkedIDs,
}
