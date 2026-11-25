// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
)

// reservedSystemAttributes are the server-owned system attribute keys that survive a wholesale
// replacement of the blob. Both write paths replace it, and the services that own an entity rebuild
// it from their own model, so a key that is not listed here is dropped by an unrelated update.
var reservedSystemAttributes = []string{
	systemAttrAttributeVerification,
	authnprovidercm.SystemAttrCredentialUpdatedAt,
	authnprovidercm.SystemAttrLinkedIDs,
	authnprovidercm.SystemAttrValidUntil,
}

// maxEntityStateLength is the longest state the runtime table's STATE column holds.
const maxEntityStateLength = 50

// verificationInvalidationReason is why a write cleared a recorded verification.
type verificationInvalidationReason string

const (
	// verificationValueChanged is a write that changes the verified value.
	verificationValueChanged verificationInvalidationReason = "value_changed"
	// verificationValueRemoved is a write that removes the verified value.
	verificationValueRemoved verificationInvalidationReason = "removed"
	// verificationUserTypeChanged is a write that changes the user type.
	verificationUserTypeChanged verificationInvalidationReason = "user_type_changed"
)

// systemAttrAttributeVerification holds server-owned contact verification evidence.
const systemAttrAttributeVerification = "attributeVerification"
