// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	refreshSubjectEntityID = "user-entity-1"
	refreshCachedSubjectID = "user-entity-2"
)

// The entity resolved while verifying the refresh token is already in hand, so it is preferred over
// the cache entry and costs no further lookup.
func TestSetRefreshSubjectIdentity_PrefersTheResolvedEntity(t *testing.T) {
	tokenCtx := &tokenservice.AccessTokenBuildContext{}

	setRefreshSubjectIdentity(tokenCtx,
		&providers.Entity{ID: refreshSubjectEntityID, Category: providers.EntityCategoryUser},
		&attributecache.AttributeCache{
			SubjectID:       refreshCachedSubjectID,
			SubjectCategory: string(providers.EntityCategoryAgent),
		})

	assert.Equal(t, refreshSubjectEntityID, tokenCtx.SubjectEntityID)
	assert.Equal(t, string(providers.EntityCategoryUser), tokenCtx.SubjectCategory)
}

// A sub mapped to an attribute such as an email address resolves to no entity. The refresh token
// carries no resource ID, so the attribute cache entry written during login is the only server-side
// record of who the token is for.
func TestSetRefreshSubjectIdentity_FallsBackToTheAttributeCache(t *testing.T) {
	tokenCtx := &tokenservice.AccessTokenBuildContext{}

	setRefreshSubjectIdentity(tokenCtx, nil, &attributecache.AttributeCache{
		SubjectID:       refreshCachedSubjectID,
		SubjectCategory: string(providers.EntityCategoryUser),
	})

	assert.Equal(t, refreshCachedSubjectID, tokenCtx.SubjectEntityID)
	assert.Equal(t, string(providers.EntityCategoryUser), tokenCtx.SubjectCategory)
}

// An entry created before the identity was recorded, or one holding attributes for a subject that
// was never resolved, carries no id. Nothing is set, so the builder resolves sub itself.
func TestSetRefreshSubjectIdentity_CacheWithoutAnIdentityLeavesItToTheBuilder(t *testing.T) {
	tokenCtx := &tokenservice.AccessTokenBuildContext{}

	setRefreshSubjectIdentity(tokenCtx, nil, &attributecache.AttributeCache{
		Attributes: map[string]interface{}{"email": "someone@example.com"},
	})

	assert.Empty(t, tokenCtx.SubjectEntityID)
	assert.Empty(t, tokenCtx.SubjectCategory)
}

// An expired or absent cache entry, with no resolvable subject: the subject is genuinely unknown and
// the event omits it rather than reporting the possibly-mapped sub.
func TestSetRefreshSubjectIdentity_NoSourceAtAll(t *testing.T) {
	tokenCtx := &tokenservice.AccessTokenBuildContext{}

	setRefreshSubjectIdentity(tokenCtx, nil, nil)

	assert.Empty(t, tokenCtx.SubjectEntityID)
	assert.Empty(t, tokenCtx.SubjectCategory)
}

// The category is optional: an entry recorded without one still supplies the id, and the builder
// resolves only the category.
func TestSetRefreshSubjectIdentity_CacheIDWithoutCategory(t *testing.T) {
	tokenCtx := &tokenservice.AccessTokenBuildContext{}

	setRefreshSubjectIdentity(tokenCtx, nil,
		&attributecache.AttributeCache{SubjectID: refreshCachedSubjectID})

	assert.Equal(t, refreshCachedSubjectID, tokenCtx.SubjectEntityID)
	assert.Empty(t, tokenCtx.SubjectCategory)
}
