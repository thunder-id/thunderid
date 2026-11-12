// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import "net/http"

// ouTypeResponse is the part of an organization unit type the tests read.
type ouTypeResponse struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
	OUID   string `json:"ouId"`
}

// ouTypeListResponse is the part of an organization unit type list the tests read.
type ouTypeListResponse struct {
	Types []ouTypeResponse `json:"types"`
}

func (ts *ControlPlaneTestSuite) TestOUTypesAreManagedOnTheControlPlane() {
	handle := unique("cp-ou-type")
	request := map[string]any{
		"handle":      handle,
		"displayName": "Partner",
		"ouId":        ts.ouID,
		"schema": map[string]any{
			"registrationNo": map[string]any{"type": "string", "required": true, "unique": true},
		},
	}
	var created ouTypeResponse
	status, raw := ts.call(http.MethodPost, "/organization-unit-types", request, &created)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.Require().NotEmpty(created.ID)
	deleted := false
	ts.T().Cleanup(func() {
		if !deleted {
			ts.call(http.MethodDelete, "/organization-unit-types/"+created.ID, nil, nil)
		}
	})
	ts.Equal(handle, created.Handle)
	ts.Equal(ts.ouID, created.OUID)

	var fetched ouTypeResponse
	status, raw = ts.call(http.MethodGet, "/organization-unit-types/"+created.ID, nil, &fetched)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Equal(created.ID, fetched.ID)

	var list ouTypeListResponse
	status, raw = ts.call(http.MethodGet, "/organization-unit-types?limit=100", nil, &list)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ids := make([]string, 0, len(list.Types))
	for _, item := range list.Types {
		ids = append(ids, item.ID)
	}
	ts.Contains(ids, created.ID)

	status, raw = ts.call(http.MethodDelete, "/organization-unit-types/"+created.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	deleted = true

	status, _ = ts.call(http.MethodGet, "/organization-unit-types/"+created.ID, nil, nil)
	ts.Equal(http.StatusNotFound, status)
}
