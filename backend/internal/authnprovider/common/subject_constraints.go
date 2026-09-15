// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// SubjectTypeConstraints are the request-scoped limits on which entities the application/agent driving
// the current authentication accepts as its subject. They are carried on the Go context.Context rather
// than a NodeContext field or the AuthnMetadata contract, so they reach the authentication providers
// below the flow graph: a flow author cannot omit the check by rewriting a flow definition, and no
// executor has to opt in.
type SubjectTypeConstraints struct {
	// AllowedUserTypes are the user type names accepted as a subject
	AllowedUserTypes []string
	// AllowedAgentTypes are the agent type names accepted as a subject. Empty accepts no agent.
	AllowedAgentTypes []string
}

// PermitsSubject reports whether an entity of the given category and type may authenticate under
// these constraints. Only agent types are subject to these constraints for now.
// Constraints on user types can be enforced after ongoing discussions are complete on the topic.
func (c SubjectTypeConstraints) PermitsSubject(category providers.EntityCategory, entityType string) bool {
	if category == providers.EntityCategoryAgent {
		return slices.Contains(c.AllowedAgentTypes, entityType)
	}
	return true
}

type subjectTypeConstraintsContextKey struct{}

// WithSubjectTypeConstraints returns a context carrying the subject type constraints of the
// application/agent driving the current authentication.
func WithSubjectTypeConstraints(ctx context.Context, c SubjectTypeConstraints) context.Context {
	return context.WithValue(ctx, subjectTypeConstraintsContextKey{}, c)
}

// SubjectTypeConstraintsFrom returns the subject type constraints carried on the context. The
// second return value reports whether any were set: entry points that are not scoped to an
// application (the credentials authentication API, admin operations) set none, and callers must
// skip the check rather than fall back to the zero value, which denies every agent.
func SubjectTypeConstraintsFrom(ctx context.Context) (SubjectTypeConstraints, bool) {
	c, ok := ctx.Value(subjectTypeConstraintsContextKey{}).(SubjectTypeConstraints)
	return c, ok
}
