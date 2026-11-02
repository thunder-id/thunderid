// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/valueref"
)

// The lookup reads the collection a reference names: variables for var:, secrets for sec:.
func TestLookupReadsTheCollectionAReferenceNames(t *testing.T) {
	ctx := context.Background()
	resolver := NewReferenceResolverInterfaceMock(t)
	resolver.On("ResolveVariable", ctx, "HOST").Return("db.test", true, nil).Once()
	resolver.On("ResolveSecret", ctx, "PASSWORD").Return("s3cret", true, nil).Once()
	lookup := Lookup(resolver)

	value, held, err := lookup(ctx, valueref.CollectionVariable, "HOST")
	require.NoError(t, err)
	assert.True(t, held)
	assert.Equal(t, "db.test", value)

	value, held, err = lookup(ctx, valueref.CollectionSecret, "PASSWORD")
	require.NoError(t, err)
	assert.True(t, held)
	assert.Equal(t, "s3cret", value)
}

// A store failure reaches the resolver as an error, not as a value that is missing.
func TestLookupReportsAStoreFailure(t *testing.T) {
	ctx := context.Background()
	resolver := NewReferenceResolverInterfaceMock(t)
	resolver.On("ResolveSecret", ctx, "PASSWORD").Return("", false, &ErrorInternalServerError).Once()

	_, _, err := Lookup(resolver)(ctx, valueref.CollectionSecret, "PASSWORD")

	assert.Error(t, err)
}
