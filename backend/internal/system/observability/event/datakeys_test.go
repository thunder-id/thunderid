// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package event

import "testing"

// The wire names are what operators filter on, so they are part of the event contract and must not
// drift when the surrounding struct is edited.
func TestPrincipalAndCorrelationDataKeys(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{key: DataKey.ActorType, want: "act_type"},
		{key: DataKey.ActorSub, want: "act_sub"},
		{key: DataKey.Subject, want: "sub"},
		{key: DataKey.SubjectType, want: "sub_type"},
		{key: DataKey.IsDelegated, want: "is_delegated"},
		{key: DataKey.CorrelationID, want: "correlation_id"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if tt.key != tt.want {
				t.Errorf("data key = %q, want %q", tt.key, tt.want)
			}
		})
	}
}

// An application is spelled "app" as an entity category and "application" as a principal type, which
// is the spelling the token's sub_type claim uses. Consumers filter on the latter.
func TestPrincipalType(t *testing.T) {
	tests := []struct {
		category string
		want     string
	}{
		{category: "user", want: "user"},
		{category: "agent", want: "agent"},
		{category: "app", want: "application"},
		{category: "", want: ""},
		{category: "something-else", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			if got := PrincipalType(tt.category); got != tt.want {
				t.Errorf("PrincipalType(%q) = %q, want %q", tt.category, got, tt.want)
			}
		})
	}
}
