// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

// attributedFailure identifies the subject of a failed proof.
type attributedFailure struct {
	// entityID is the entity the proof was compared against.
	entityID string

	// recipient holds the identifying attribute of the subject when entityID is empty.
	recipient map[string]interface{}
}
