// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ouresource

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const sharedRoleReadOnlyCode = "ROL-1045"

// sharedRoleView is the part of a role response that says which organization unit it is read as.
type sharedRoleView struct {
	ID          string `json:"id"`
	OUID        string `json:"ouId"`
	IsOwnedByOU bool   `json:"isOwnedByOU"`
}

// roleErrorCode returns the code of an error response.
func roleErrorCode(body []byte) string {
	var resp struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &resp)
	return resp.Code
}

func (ts *OUResourceTestSuite) TestRoleSharedFromParent() {
	roleID, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-parent-shared-role", OUID: ts.parentID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + roleID)
	status, created := ts.root(http.MethodPost, "/roles/"+roleID+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
	ts.Require().Equalf(http.StatusCreated, status, "share the role: %s", created)
	ts.cleanup("/roles/" + roleID + "/sharing-policies/" + idOf(created))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/roles?limit=100", nil)
	var list struct {
		Roles []sharedRoleView `json:"roles"`
	}
	ts.Require().NoError(json.Unmarshal(body, &list))
	var listed *sharedRoleView
	for i := range list.Roles {
		if list.Roles[i].ID == roleID {
			listed = &list.Roles[i]
		}
	}
	ts.Require().NotNil(listed, "the list must include the role shared into the administrator's OU")
	ts.False(listed.IsOwnedByOU)

	body = ts.requireStatus(http.StatusOK, http.MethodGet, "/roles/"+roleID, nil)
	var role sharedRoleView
	ts.Require().NoError(json.Unmarshal(body, &role))
	ts.Equal(roleID, role.ID)
	ts.Equal(ts.ou1ID, role.OUID, "a shared role is read under the unit it is shared into")
	ts.False(role.IsOwnedByOU)

	body = ts.requireStatus(http.StatusForbidden, http.MethodPut, "/roles/"+roleID,
		map[string]interface{}{"name": "ou-resource-renamed-shared-role"})
	ts.Equal(sharedRoleReadOnlyCode, roleErrorCode(body))
	body = ts.requireStatus(http.StatusForbidden, http.MethodDelete, "/roles/"+roleID, nil)
	ts.Equal(sharedRoleReadOnlyCode, roleErrorCode(body))

	status, body = ts.root(http.MethodGet, "/roles/"+roleID, nil)
	ts.Require().Equalf(http.StatusOK, status, "the role must survive the refused delete: %s", body)
	ts.Require().NoError(json.Unmarshal(body, &role))
	ts.Equal(ts.parentID, role.OUID)
	ts.True(role.IsOwnedByOU)
}

func (ts *OUResourceTestSuite) TestRoleOwnedReadsAsOwned() {
	own, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-owned-read-role", OUID: ts.ou1ID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + own)

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/roles/"+own, nil)
	var role sharedRoleView
	ts.Require().NoError(json.Unmarshal(body, &role))
	ts.Equal(ts.ou1ID, role.OUID)
	ts.True(role.IsOwnedByOU)
}

// A role shared into the administrator's OU stays read only, but the OU assigns it, which the policy allows unless
// its assignments rule says otherwise. The assignments it makes are its own, in its own OU.
func (ts *OUResourceTestSuite) TestSharedRoleIsAssignedInTheUnit() {
	roleID, err := testutils.CreateRole(testutils.Role{Name: "ou-resource-assignable-role", OUID: ts.parentID})
	ts.Require().NoError(err)
	ts.cleanup("/roles/" + roleID)
	status, created := ts.root(http.MethodPost, "/roles/"+roleID+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
	ts.Require().Equalf(http.StatusCreated, status, "share the role: %s", created)
	ts.cleanup("/roles/" + roleID + "/sharing-policies/" + idOf(created))

	userID, err := testutils.CreateUser(testutils.User{
		Type: "ou-resource-admin-type",
		OUID: ts.ou1ID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			"ou-resource-role-assignee", adminPassword)),
	})
	ts.Require().NoError(err)
	ts.cleanup("/users/" + userID)
	assign := func(ouID string) map[string]interface{} {
		return map[string]interface{}{"ouId": ouID, "assignments": []map[string]string{{"id": userID, "type": "user"}}}
	}
	assigned := func() bool {
		body := ts.requireStatus(http.StatusOK, http.MethodGet,
			"/roles/"+roleID+"/assignments?filter="+url.QueryEscape(`ouId eq "`+ts.ou1ID+`"`), nil)
		var list struct {
			Assignments []struct {
				ID string `json:"id"`
			} `json:"assignments"`
		}
		ts.Require().NoError(json.Unmarshal(body, &list), string(body))
		for _, a := range list.Assignments {
			if a.ID == userID {
				return true
			}
		}
		return false
	}

	ts.requireStatus(http.StatusNoContent, http.MethodPost, "/roles/"+roleID+"/assignments/add", assign(ts.ou1ID))
	ts.True(assigned(), "OU1 assigns the role shared into it")
	ts.requireStatus(http.StatusNoContent, http.MethodPost, "/roles/"+roleID+"/assignments/remove", assign(ts.ou1ID))
	ts.False(assigned(), "OU1 removes the assignment it made")

	ts.requireStatus(http.StatusForbidden, http.MethodPost, "/roles/"+roleID+"/assignments/add", assign(ts.parentID))
}
