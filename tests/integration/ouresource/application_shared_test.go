// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ouresource

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const sharedApplicationReadOnlyCode = "APP-1070"

// sharedApplicationView is the part of an application response that says which organization unit it is
// read as.
type sharedApplicationView struct {
	ID          string `json:"id"`
	OUID        string `json:"ouId"`
	IsOwnedByOU *bool  `json:"isOwnedByOU"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// applicationErrorCode returns the code of an error response.
func applicationErrorCode(body []byte) string {
	var resp struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &resp)
	return resp.Code
}

func (ts *OUResourceTestSuite) TestApplicationSharedFromParent() {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "ou-resource-parent-shared-app", OUID: ts.parentID,
	})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + appID)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/applications", nil)
	var list struct {
		Applications []sharedApplicationView `json:"applications"`
	}
	ts.Require().NoError(json.Unmarshal(body, &list))
	var listed *sharedApplicationView
	for i := range list.Applications {
		if list.Applications[i].ID == appID {
			listed = &list.Applications[i]
		}
	}
	ts.Require().NotNil(listed, "the list must include the application shared into the administrator's OU")
	ts.Require().NotNil(listed.IsOwnedByOU)
	ts.False(*listed.IsOwnedByOU)

	body = ts.requireStatus(http.StatusOK, http.MethodGet, "/applications/"+appID, nil)
	var app sharedApplicationView
	ts.Require().NoError(json.Unmarshal(body, &app))
	ts.Equal(appID, app.ID)
	ts.Equal(ts.ou1ID, app.OUID, "a shared application is read under the unit it is shared into")
	ts.Require().NotNil(app.IsOwnedByOU)
	ts.False(*app.IsOwnedByOU)
	ts.True(app.IsReadOnly)

	body = ts.requireStatus(http.StatusForbidden, http.MethodPut, "/applications/"+appID,
		map[string]interface{}{"name": "ou-resource-renamed-shared-app"})
	ts.Equal(sharedApplicationReadOnlyCode, applicationErrorCode(body))
	body = ts.requireStatus(http.StatusForbidden, http.MethodDelete, "/applications/"+appID, nil)
	ts.Equal(sharedApplicationReadOnlyCode, applicationErrorCode(body))

	status, body := ts.root(http.MethodGet, "/applications/"+appID, nil)
	ts.Require().Equalf(http.StatusOK, status, "the application must survive the refused delete: %s", body)
	var owned sharedApplicationView
	ts.Require().NoError(json.Unmarshal(body, &owned))
	ts.Equal(ts.parentID, owned.OUID)
	ts.Require().NotNil(owned.IsOwnedByOU)
	ts.True(*owned.IsOwnedByOU)
	ts.False(owned.IsReadOnly)
}

func (ts *OUResourceTestSuite) TestApplicationOwnedReadsAsOwned() {
	own, err := testutils.CreateApplication(testutils.Application{Name: "ou-resource-owned-read-app", OUID: ts.ou1ID})
	ts.Require().NoError(err)
	ts.cleanup("/applications/" + own)

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/applications/"+own, nil)
	var app sharedApplicationView
	ts.Require().NoError(json.Unmarshal(body, &app))
	ts.Equal(ts.ou1ID, app.OUID)
	ts.Require().NotNil(app.IsOwnedByOU)
	ts.True(*app.IsOwnedByOU)
	ts.False(app.IsReadOnly)
}
