// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
	"github.com/thunder-id/thunderid/internal/system/valueref"
)

// unitReferring is an organization unit whose description refers to the variable named.
func unitReferring(id, reference string) string {
	return strings.Join([]string{
		"resource_type: organization_unit",
		"id: " + id,
		"handle: " + id,
		"name: " + id,
		"description: " + reference,
		"",
	}, "\n")
}

func importWithReferences(t *testing.T, lookup secretresolver.Lookup, content string) (*ImportResponse,
	*fakeOUService) {
	t.Helper()
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if lookup != nil {
		svc.(*importService).references = secretresolver.New(lookup)
	}
	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})
	require.Nil(t, err)
	return resp, ouSvc
}

var heldVariables = func(_ context.Context, collection valueref.Collection, name string) (string, bool, error) {
	if collection == valueref.CollectionVariable && name == "TEAM" {
		return "Engineering team", true, nil
	}
	return "", false, nil
}

// A reference is replaced with the value this deployment holds before the resource is written.
func TestImportWritesTheValueAReferenceNames(t *testing.T) {
	resp, ouSvc := importWithReferences(t, heldVariables, unitReferring("ou-1", "var:TEAM"))

	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, ouSvc.created, 1)
	assert.Equal(t, "Engineering team", ouSvc.created[0].Description)
}

// A resource referring to a value this deployment does not hold is refused, naming the reference,
// while the others in the same import still go through.
func TestImportRefusesAResourceWhoseReferenceIsNotHeld(t *testing.T) {
	content := unitReferring("ou-1", "sec:MISSING") + "---\n" + unitReferring("ou-2", "var:TEAM")

	resp, ouSvc := importWithReferences(t, heldVariables, content)

	require.Len(t, resp.Results, 2)
	var refused, imported ImportItemOutcome
	for _, result := range resp.Results {
		if result.Status == statusFailed {
			refused = result
		} else {
			imported = result
		}
	}
	assert.Equal(t, ErrorUnresolvedReference.Code, refused.Code)
	assert.Equal(t, "ou-1", refused.ResourceName)
	assert.Contains(t, refused.Message, "sec:MISSING")
	assert.Equal(t, statusSuccess, imported.Status)
	require.Len(t, ouSvc.created, 1, "a resource with an unresolved reference was written")
	assert.Equal(t, "ou-2", ouSvc.created[0].ID)
	assert.Equal(t, 1, resp.Summary.Failed)
}

// A store that cannot be read refuses the resource without describing why.
func TestImportRefusesAResourceWhenTheStoreCannotBeRead(t *testing.T) {
	failing := func(context.Context, valueref.Collection, string) (string, bool, error) {
		return "", false, errors.New("database is down")
	}

	resp, ouSvc := importWithReferences(t, failing, unitReferring("ou-1", "var:TEAM"))

	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.NotContains(t, resp.Results[0].Message, "database")
	assert.Empty(t, ouSvc.created)
}

// Without a resolver, as on a control plane, a reference is imported as it is.
func TestImportKeepsAReferenceWithoutAResolver(t *testing.T) {
	resp, ouSvc := importWithReferences(t, nil, unitReferring("ou-1", "var:TEAM"))

	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, ouSvc.created, 1)
	assert.Equal(t, "var:TEAM", ouSvc.created[0].Description)
}
