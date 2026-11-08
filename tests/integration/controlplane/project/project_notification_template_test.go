// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// templatesPath is the collection of email templates these tests create.
const templatesPath = "/notification-templates/email/templates"

// projectTemplate is the part of a notification template response these tests read.
type projectTemplate struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// projectTemplateList is the part of a notification template list response these tests read.
type projectTemplateList struct {
	TotalResults int `json:"totalResults"`
	Templates    []struct {
		ID        string `json:"id"`
		Handle    string `json:"handle"`
		ProjectID string `json:"projectId"`
	} `json:"templates"`
}

// On the Control Plane, a notification template belongs to a project.
func (ts *ProjectAPITestSuite) TestCreateTemplateRequiresProject() {
	status, body := ts.do(http.MethodPost, templatesPath, ts.templateBody("unplaced", `"ouId":%q`, defaultOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("NTM-1031", codeOf(ts.T(), body))
}

// A template in a project is in the system organization unit, and each project lists only its own templates.
func (ts *ProjectAPITestSuite) TestCreateTemplateInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("tpl-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("tpl-other")))

	inShop := ts.createTemplate(ts.templateBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createTemplate(ts.templateBody("other", `"projectId":%q`, other.ID))

	shopList := ts.listTemplates("?limit=100&projectId=" + url.QueryEscape(shop.ID))
	ts.Equal([]string{inShop.ID}, ts.templateIDs(shopList))
	ts.Equal(shop.ID, shopList.Templates[0].ProjectID)
	ts.Equal([]string{inOther.ID},
		ts.templateIDs(ts.listTemplates("?limit=100&projectId="+url.QueryEscape(other.ID))))

	var fetched projectTemplate
	ts.Require().Equal(http.StatusOK, ts.getJSON(templatesPath+"/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// A template in a project is in the system organization unit, and no other.
func (ts *ProjectAPITestSuite) TestTemplateInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("tpl-ou-shop")))
	childOUID := ts.createChildOU("tpl-ou-child", defaultOUID)

	inDefault := ts.createTemplate(ts.templateBody("ou-default", `"projectId":%q,"ouId":%q`, shop.ID, defaultOUID))
	ts.Equal(defaultOUID, inDefault.OUID)

	status, body := ts.do(http.MethodPost, templatesPath,
		ts.templateBody("ou-child", `"projectId":%q,"ouId":%q`, shop.ID, childOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("NTM-1030", codeOf(ts.T(), body))
}

// An update keeps a template in its project unless it names another one, and moves it between projects.
func (ts *ProjectAPITestSuite) TestUpdateTemplateWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("tpl-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("tpl-upd-second")))
	created := ts.createTemplate(ts.templateBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateTemplate(created.ID, templateUpdateBody(""))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateTemplate(created.ID, templateUpdateBody(fmt.Sprintf(`,"projectId":%q`, second.ID)))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.templateIDs(ts.listTemplates("?limit=100&projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{created.ID},
		ts.templateIDs(ts.listTemplates("?limit=100&projectId="+url.QueryEscape(second.ID))))
}

func (ts *ProjectAPITestSuite) TestTemplateWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	status, body := ts.do(http.MethodPost, templatesPath,
		ts.templateBody("unknown", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("NTM-1029", codeOf(ts.T(), body))

	list := ts.listTemplates("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.Templates)

	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("tpl-unknown-shop")))
	created := ts.createTemplate(ts.templateBody("unknown-upd", `"projectId":%q`, shop.ID))
	status, body = ts.do(http.MethodPut, templatesPath+"/"+created.ID,
		templateUpdateBody(fmt.Sprintf(`,"projectId":%q`, unknownProjectID)))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("NTM-1029", codeOf(ts.T(), body))
}

// A project that still has a template is not deleted until the template is gone.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithTemplateIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("tpl-busy")))
	template := ts.createTemplate(ts.templateBody("busy", `"projectId":%q`, created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodDelete, templatesPath+"/"+template.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

// The bootstrap places the built-in templates in the Default project.
func (ts *ProjectAPITestSuite) TestBuiltInTemplatesAreInTheDefaultProject() {
	handles := []string{}
	for _, template := range ts.listTemplates("?limit=100&projectId=" + defaultProjectID).Templates {
		handles = append(handles, template.Handle)
	}
	ts.Contains(handles, "user-invite")

	var smsList projectTemplateList
	ts.Require().Equal(http.StatusOK,
		ts.getJSON("/notification-templates/sms/templates?limit=100&projectId="+defaultProjectID, &smsList))
	smsHandles := []string{}
	for _, template := range smsList.Templates {
		smsHandles = append(smsHandles, template.Handle)
	}
	ts.Contains(smsHandles, "otp")
}

// An exported template keeps its project, so a deploy of the project carries it.
func (ts *ProjectAPITestSuite) TestExportedTemplateKeepsItsProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("tpl-export-shop")))
	created := ts.createTemplate(ts.templateBody("export", `"projectId":%q`, shop.ID))

	status, resp := ts.do(http.MethodPost, "/export", fmt.Sprintf(`{"notificationTemplates":[%q]}`, created.ID))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "projectId: "+shop.ID)
}

// An import that updates an existing template without naming a project, such as a file exported before
// templates had projects, keeps the template in its project. A new template without one is refused.
func (ts *ProjectAPITestSuite) TestImportedTemplateWithoutProjectKeepsItsProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("tpl-import-shop")))
	created := ts.createTemplate(ts.templateBody("import", `"projectId":%q`, shop.ID))
	doc := func(handle string) string {
		return fmt.Sprintf("resource_type: notification_template\nchannel: email\nhandle: %s\n"+
			"displayName: Imported\ncontent:\n  subject: Hello\n  body: \"<p>Hello</p>\"\n", handle)
	}

	dryRun := ts.importWith(doc(ts.handle("tpl-import")), true, true)
	ts.Require().Len(dryRun.Results, 1)
	ts.Equal("success", dryRun.Results[0].Status, dryRun.Results[0].Message)
	ts.Equal("update", dryRun.Results[0].Operation)

	ts.importContent(doc(ts.handle("tpl-import")))
	var fetched struct {
		projectTemplate
		DisplayName string `json:"displayName"`
	}
	ts.Require().Equal(http.StatusOK, ts.getJSON(templatesPath+"/"+created.ID, &fetched))
	ts.Equal("Imported", fetched.DisplayName)
	ts.Equal(shop.ID, fetched.ProjectID)

	ts.Equal("NTM-1031", ts.importFailure(doc(ts.handle("tpl-import-new")), true))
}

// templateBody builds an email template request, with the given placement fields spliced in.
func (ts *ProjectAPITestSuite) templateBody(name, placement string, args ...interface{}) string {
	return fmt.Sprintf(`{"handle":%q,"displayName":%q,"content":{"subject":"Hello","body":"<p>Hello</p>"},%s}`,
		ts.handle("tpl-"+name), "Template "+name, fmt.Sprintf(placement, args...))
}

// templateUpdateBody builds an email template update request, with the given fields appended.
func templateUpdateBody(extra string) string {
	return `{"displayName":"Updated","content":{"subject":"Hello","body":"<p>Hello</p>"}` + extra + `}`
}

// createTemplate posts an email template, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createTemplate(body string) projectTemplate {
	status, resp := ts.do(http.MethodPost, templatesPath, body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectTemplate
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.templates = append(ts.templates, created.ID)
	return created
}

// updateTemplate puts an email template and requires a 200.
func (ts *ProjectAPITestSuite) updateTemplate(id, body string) projectTemplate {
	status, resp := ts.do(http.MethodPut, templatesPath+"/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectTemplate
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listTemplates lists email templates with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listTemplates(query string) projectTemplateList {
	var list projectTemplateList
	ts.Require().Equal(http.StatusOK, ts.getJSON(templatesPath+query, &list))
	return list
}

// templateIDs returns the IDs of the listed templates.
func (ts *ProjectAPITestSuite) templateIDs(list projectTemplateList) []string {
	ids := []string{}
	for _, template := range list.Templates {
		ids = append(ids, template.ID)
	}
	return ids
}
