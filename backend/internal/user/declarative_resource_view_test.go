// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func viewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

// An exported user is shown as a read returns it, with its references as they are and without the
// credentials the export carries.
func TestViewResourceShowsAnExportedUserAsItsRead(t *testing.T) {
	exported, err := yaml.Marshal(&userDeclarativeResource{
		ID: "user-1", Type: "employee", OUID: "ou-1",
		Attributes:  map[string]interface{}{"username": "alice", "email": "var:USER_ALICE_EMAIL"},
		Credentials: map[string]interface{}{"password": "sec:USER_ALICE_PASSWORD"},
	})
	require.NoError(t, err)

	view, err := newUserExporter(nil, nil, nil).ViewResource(context.Background(),
		viewDocument(t, "resource_type: user\n"+string(exported)))

	require.NoError(t, err)
	u, ok := view.(*providers.User)
	require.True(t, ok)
	assert.Equal(t, "user-1", u.ID)
	assert.Equal(t, "employee", u.Type)
	assert.Equal(t, "ou-1", u.OUID)
	assert.JSONEq(t, `{"username":"alice","email":"var:USER_ALICE_EMAIL"}`, string(u.Attributes))
	assert.NotContains(t, string(u.Attributes), "USER_ALICE_PASSWORD")
}

// A document that is not a user is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newUserExporter(nil, nil, nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "id: [not, an, id]"))
	assert.Error(t, err)
}
