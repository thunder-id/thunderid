// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

// truthNames are the text forms of the truth values, indexed by value.
var truthNames = [...]string{"FALSE", "UNKNOWN", "TRUE"}

// String returns the text form of the truth value.
func (t Truth) String() string {
	if int(t) < len(truthNames) {
		return truthNames[t]
	}
	return truthNames[TruthUnknown]
}

// MarshalText encodes the truth value as its text form.
func (t Truth) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// andTruth is three-valued AND: false wins, otherwise unknown wins.
func andTruth(a, b Truth) Truth {
	return min(a, b)
}

// orTruth is three-valued OR: true wins, otherwise unknown wins.
func orTruth(a, b Truth) Truth {
	return max(a, b)
}

// notTruth is three-valued NOT: true and false swap, unknown stays unknown.
func notTruth(a Truth) Truth {
	return TruthTrue - a
}
