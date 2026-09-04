// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCandidatesForSubAttr_MappedSubAttr_ReturnsItsCandidate tests that a sub-attribute backed
// by its own CoreAttrRule (e.g. name.givenName) returns exactly that candidate.
func TestCandidatesForSubAttr_MappedSubAttr_ReturnsItsCandidate(t *testing.T) {
	require.Equal(t, []string{"given_name"}, CandidatesForSubAttr(fieldName, "givenName"))
	require.Equal(t, []string{"family_name"}, CandidatesForSubAttr(fieldName, "familyName"))
	require.Equal(t, []string{"street_address"}, CandidatesForSubAttr(fieldAddresses, "streetAddress"))
}

// TestCandidatesForSubAttr_ProtocolOnlySubAttr_ReturnsNil tests that sub-attributes with no
// ThunderID-mapped candidate of their own (protocol metadata, or a value key that is really
// the parent rule's ValueKey) return nil rather than an empty non-nil slice or a false match.
func TestCandidatesForSubAttr_ProtocolOnlySubAttr_ReturnsNil(t *testing.T) {
	require.Nil(t, CandidatesForSubAttr(fieldEmails, "value"))
	require.Nil(t, CandidatesForSubAttr(fieldEmails, "type"))
	require.Nil(t, CandidatesForSubAttr(fieldEmails, "primary"))
	require.Nil(t, CandidatesForSubAttr(fieldAddresses, "formatted"))
}
