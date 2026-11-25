// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ouresource

import (
	"net/http"
)

const templatesPath = "/notification-templates/email/templates"

func templateIn(handle, ouID string) map[string]interface{} {
	return map[string]interface{}{
		"handle":      handle,
		"displayName": "OU resource " + handle,
		"ouId":        ouID,
		"content":     map[string]string{"subject": "Hello", "body": "<p>Hello</p>"},
	}
}

func templateUpdate() map[string]interface{} {
	return map[string]interface{}{
		"displayName": "OU resource updated",
		"content":     map[string]string{"subject": "Updated", "body": "<p>Updated</p>"},
	}
}

func (ts *OUResourceTestSuite) TestTemplateInOwnOU() {
	id := ts.createAsRoot(templatesPath, templateIn("ou-resource-own-template", ts.ou1ID))

	ts.requireStatus(http.StatusOK, http.MethodGet, templatesPath+"/"+id, nil)
	ts.requireStatus(http.StatusOK, http.MethodPut, templatesPath+"/"+id, templateUpdate())

	created := ts.requireStatus(http.StatusCreated, http.MethodPost, templatesPath,
		templateIn("ou-resource-admin-template", ts.ou1ID))
	createdID := idOf(created)
	ts.cleanup(templatesPath + "/" + createdID)
	ts.requireStatus(http.StatusNoContent, http.MethodDelete, templatesPath+"/"+createdID, nil)
}

func (ts *OUResourceTestSuite) TestTemplateInOtherOU() {
	id := ts.createAsRoot(templatesPath, templateIn("ou-resource-other-template", ts.ou2ID))

	ts.requireStatus(http.StatusForbidden, http.MethodGet, templatesPath+"/"+id, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPut, templatesPath+"/"+id, templateUpdate())
	ts.requireStatus(http.StatusForbidden, http.MethodDelete, templatesPath+"/"+id, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPost, templatesPath,
		templateIn("ou-resource-other-created", ts.ou2ID))

	status, _ := ts.root(http.MethodGet, templatesPath+"/"+id, nil)
	ts.Equal(http.StatusOK, status, "the template must survive the refused delete")
}

func (ts *OUResourceTestSuite) TestTemplateList() {
	own := ts.createAsRoot(templatesPath, templateIn("ou-resource-list-own", ts.ou1ID))
	other := ts.createAsRoot(templatesPath, templateIn("ou-resource-list-other", ts.ou2ID))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, templatesPath+"?limit=100", nil)
	ids := listedIDs(body, "templates")
	ts.True(ids[own], "the list must include the template in the administrator's OU")
	ts.False(ids[other], "the list must leave out the template in another OU")
}

func (ts *OUResourceTestSuite) TestTemplateInParentOU() {
	id := ts.createAsRoot(templatesPath, templateIn("ou-resource-parent-template", ts.parentID))

	ts.requireStatus(http.StatusOK, http.MethodGet, templatesPath+"/"+id, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodPut, templatesPath+"/"+id, templateUpdate())
	ts.requireStatus(http.StatusForbidden, http.MethodDelete, templatesPath+"/"+id, nil)

	body := ts.requireStatus(http.StatusOK, http.MethodGet, templatesPath+"?limit=100", nil)
	ts.True(listedIDs(body, "templates")[id], "the list must include the template of the parent OU")
}
