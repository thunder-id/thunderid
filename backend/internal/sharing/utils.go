// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

// nullableString maps an empty string to a SQL null, so an absent parent or target organization
// unit stores as null rather than as an empty string a foreign key would reject.
func nullableString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

// copyStrings returns a copy of a member list, so a stored rule cannot be mutated through a
// pointer the caller still holds.
func copyStrings(in *[]string) *[]string {
	if in == nil {
		return nil
	}
	out := append([]string{}, *in...)
	return &out
}

// copyRule returns a rule whose member lists share nothing with the rules it was folded from.
//
// The algebra composes by taking whichever side won, so its result aliases its inputs: a resolved
// rule would otherwise point at the parent policy's own slices, and at a cached resolution's. The
// copy is taken where a folded rule escapes into a stored row or a cache, not on the way in.
func copyRule(r OverlayRule) OverlayRule {
	r.Value = copyStrings(r.Value)
	r.AllowedValues = copyStrings(r.AllowedValues)
	r.ExcludedValues = copyStrings(r.ExcludedValues)
	return r
}
