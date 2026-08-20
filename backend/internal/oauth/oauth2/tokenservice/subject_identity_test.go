// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package tokenservice

import (
	"testing"

	"github.com/stretchr/testify/assert"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
)

const (
	subjectIdentityClientID = "client-entity-1"
	subjectIdentityUserID   = "user-entity-1"
)

// authorization_code carries both values on the flow assertion, so the login path resolves nothing.
// The builder is given no actor provider here: needing one would mean the fast path is not taken.
func TestResolveSubjectIdentity_CarriedFromTheAssertion(t *testing.T) {
	tb := &tokenBuilder{}

	id, category := tb.resolveSubjectIdentity(&AccessTokenBuildContext{
		Subject:         "alice@example.com",
		SubjectEntityID: subjectIdentityUserID,
		SubjectCategory: string(providers.EntityCategoryUser),
	})

	// The mapped subject attribute is reported as the resource ID, never as the token's own sub.
	assert.Equal(t, subjectIdentityUserID, id)
	assert.Equal(t, string(providers.EntityCategoryUser), category)
}

// An assertion that carried the ID but no category still reports the subject; only the category is
// resolved.
func TestResolveSubjectIdentity_AssertionIDWithoutCategory(t *testing.T) {
	actors := actorprovidermock.NewActorProviderMock(t)
	actors.On("GetActor", subjectIdentityUserID).
		Return(&providers.Entity{ID: subjectIdentityUserID, Category: providers.EntityCategoryUser},
			(*tidcommon.ServiceError)(nil))
	tb := &tokenBuilder{actorProvider: actors}

	id, category := tb.resolveSubjectIdentity(&AccessTokenBuildContext{
		SubjectEntityID: subjectIdentityUserID,
	})

	assert.Equal(t, subjectIdentityUserID, id)
	assert.Equal(t, string(providers.EntityCategoryUser), category)
}

// A client_credentials token is issued about the client itself, so both values are known without
// resolving anything.
func TestResolveSubjectIdentity_SubjectIsTheClient(t *testing.T) {
	tb := &tokenBuilder{}

	id, category := tb.resolveSubjectIdentity(&AccessTokenBuildContext{
		Subject: subjectIdentityClientID,
		OAuthApp: &providers.OAuthClient{
			ID:             subjectIdentityClientID,
			EntityCategory: providers.EntityCategoryAgent,
		},
	})

	assert.Equal(t, subjectIdentityClientID, id)
	assert.Equal(t, string(providers.EntityCategoryAgent), category)
}

// An agent can be a token subject as well as a token requester: agent A exchanging a subject token
// minted for agent B must report an agent subject, not a user.
func TestResolveSubjectIdentity_AgentSubjectOfAnExchange(t *testing.T) {
	actors := actorprovidermock.NewActorProviderMock(t)
	actors.On("GetActor", "agent-b").
		Return(&providers.Entity{ID: "agent-b", Category: providers.EntityCategoryAgent},
			(*tidcommon.ServiceError)(nil))
	tb := &tokenBuilder{actorProvider: actors}

	id, category := tb.resolveSubjectIdentity(&AccessTokenBuildContext{
		Subject:  "agent-b",
		OAuthApp: &providers.OAuthClient{ID: subjectIdentityClientID, EntityCategory: providers.EntityCategoryAgent},
	})

	assert.Equal(t, "agent-b", id)
	assert.Equal(t, string(providers.EntityCategoryAgent), category)
}

// On an exchange the subject arrives as the presented token's sub, which is a mapped attribute when
// the issuing application configured one. It resolves to no entity, and both fields are left empty
// so the attribute — an email address here — is never published.
func TestResolveSubjectIdentity_MappedAttributeIsNotReported(t *testing.T) {
	actors := actorprovidermock.NewActorProviderMock(t)
	actors.On("GetActor", "alice@example.com").
		Return((*providers.Entity)(nil), &tidcommon.ServiceError{Code: "ENTITY-404"})
	tb := &tokenBuilder{actorProvider: actors}

	id, category := tb.resolveSubjectIdentity(&AccessTokenBuildContext{
		Subject:  "alice@example.com",
		OAuthApp: &providers.OAuthClient{ID: subjectIdentityClientID},
	})

	assert.Empty(t, id)
	assert.Empty(t, category)
}

func TestResolveSubjectIdentity_NoProviderOrNoSubject(t *testing.T) {
	tb := &tokenBuilder{}

	// A subject that is not the client cannot be resolved without a provider, so it is not reported.
	id, category := tb.resolveSubjectIdentity(&AccessTokenBuildContext{
		Subject:  subjectIdentityUserID,
		OAuthApp: &providers.OAuthClient{ID: subjectIdentityClientID},
	})
	assert.Empty(t, id)
	assert.Empty(t, category)

	// No subject at all.
	id, category = tb.resolveSubjectIdentity(&AccessTokenBuildContext{})
	assert.Empty(t, id)
	assert.Empty(t, category)
}
