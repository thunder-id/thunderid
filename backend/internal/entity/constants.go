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

// The system flows the entity service runs around a change that revokes what was issued before it, and the
// input naming the entity they act on. Each pre flow revokes up to a cutoff before the change is stored, and
// its post flow moves the cutoff past the write, so an artifact issued while it ran is revoked too.
const (
	// systemFlowEntityDeletePre revokes what was issued to the entity as a subject and ends its sessions.
	systemFlowEntityDeletePre = "system-entity-delete-pre"
	// systemFlowCredentialUpdatePre revokes what was issued to the entity as a subject and ends its sessions,
	// before a credential it holds is replaced.
	systemFlowCredentialUpdatePre = "system-entity-credential-update-pre" //nolint:gosec // flow handle
	// systemFlowCredentialUpdatePost also ends the sessions started while the credential was replaced.
	systemFlowCredentialUpdatePost = "system-entity-credential-update-post" //nolint:gosec // flow handle
	// systemFlowSecretRotatePre revokes what the entity's OAuth client issued before its secret is replaced.
	systemFlowSecretRotatePre = "system-entity-secret-rotate-pre"
	// systemFlowRevocationPost moves the cutoff past the write.
	systemFlowRevocationPost = "system-revocation-post"
	// systemFlowInputEntity names the entity the flows act on.
	systemFlowInputEntity = "targetEntityId"
)
