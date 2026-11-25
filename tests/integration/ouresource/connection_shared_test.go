// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ouresource

import (
	"encoding/json"
	"net/http"
)

const sharedConnectionReadOnlyCode = "CON-1041"

// sharedConnectionView is the part of a connection response that says which organization unit it is read as.
type sharedConnectionView struct {
	ID          string `json:"id"`
	OUID        string `json:"ouId"`
	IsOwnedByOU *bool  `json:"isOwnedByOU"`
	Code        string `json:"code"`
}

func (ts *OUResourceTestSuite) TestConnectionSharedFromParent() {
	connection := httpConnectionIn("ou-resource-parent-shared-http", ts.parentID)
	connectionID := ts.createAsRoot("/connections/http", connection)
	status, created := ts.root(http.MethodPost, "/connections/http/"+connectionID+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
	ts.Require().Equalf(http.StatusCreated, status, "share the connection: %s", created)
	ts.cleanup("/connections/http/" + connectionID + "/sharing-policies/" + idOf(created))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/connections/http", nil)
	var list []sharedConnectionView
	ts.Require().NoError(json.Unmarshal(body, &list))
	var listed *sharedConnectionView
	for i := range list {
		if list[i].ID == connectionID {
			listed = &list[i]
		}
	}
	ts.Require().NotNil(listed, "the list must include the connection shared into the administrator's OU")
	ts.Equal(ts.ou1ID, listed.OUID)
	ts.Require().NotNil(listed.IsOwnedByOU)
	ts.False(*listed.IsOwnedByOU)

	body = ts.requireStatus(http.StatusOK, http.MethodGet, "/connections/http/"+connectionID, nil)
	var read sharedConnectionView
	ts.Require().NoError(json.Unmarshal(body, &read))
	ts.Equal(connectionID, read.ID)
	ts.Equal(ts.ou1ID, read.OUID, "a shared connection is read under the unit it is shared into")
	ts.Require().NotNil(read.IsOwnedByOU)
	ts.False(*read.IsOwnedByOU)

	var refused sharedConnectionView
	body = ts.requireStatus(http.StatusForbidden, http.MethodPut, "/connections/http/"+connectionID, connection)
	ts.Require().NoError(json.Unmarshal(body, &refused))
	ts.Equal(sharedConnectionReadOnlyCode, refused.Code)
	body = ts.requireStatus(http.StatusForbidden, http.MethodDelete, "/connections/http/"+connectionID, nil)
	ts.Require().NoError(json.Unmarshal(body, &refused))
	ts.Equal(sharedConnectionReadOnlyCode, refused.Code)

	status, body = ts.root(http.MethodGet, "/connections/http/"+connectionID, nil)
	ts.Require().Equalf(http.StatusOK, status, "the connection must survive the refused delete: %s", body)
	ts.Require().NoError(json.Unmarshal(body, &read))
	ts.Equal(ts.parentID, read.OUID)
	ts.Require().NotNil(read.IsOwnedByOU)
	ts.True(*read.IsOwnedByOU)
}

func (ts *OUResourceTestSuite) TestConnectionNotSharedStaysRefused() {
	other := ts.createAsRoot("/connections/http", httpConnectionIn("ou-resource-unshared-http", ts.ou2ID))

	body := ts.requireStatus(http.StatusForbidden, http.MethodPut, "/connections/http/"+other,
		httpConnectionIn("ou-resource-unshared-http", ts.ou2ID))
	var refused sharedConnectionView
	ts.Require().NoError(json.Unmarshal(body, &refused))
	ts.NotEqual(sharedConnectionReadOnlyCode, refused.Code)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/connections/http/"+other, nil)
}
