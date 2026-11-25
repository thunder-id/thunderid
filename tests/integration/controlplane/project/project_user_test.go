// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// projectUser is the part of a user response these tests read.
type projectUser struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// projectUserList is the part of a user list response these tests read.
type projectUserList struct {
	TotalResults int           `json:"totalResults"`
	Users        []projectUser `json:"users"`
}

// Projects share the system organization unit, yet each lists only its own users. A user created without a
// project belongs to the organization, and one in a project that names no organization unit goes to the
// system organization unit.
func (ts *ProjectAPITestSuite) TestCreateUserInProject() {
	userType := ts.createProjectUserType("create")
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("user-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("user-other")))

	inShop := ts.createUser(ts.userBody(userType, "shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createUser(ts.userBody(userType, "other", `"projectId":%q,"ouId":%q`, other.ID, defaultOUID))
	inOrganization := ts.createUser(ts.userBody(userType, "org", `"ouId":%q`, defaultOUID))
	ts.Empty(inOrganization.ProjectID)

	ts.Equal([]string{inShop.ID}, ts.userIDs(ts.listUsers("?limit=100&projectId="+url.QueryEscape(shop.ID))))
	ts.Equal([]string{inOther.ID}, ts.userIDs(ts.listUsers("?limit=100&projectId="+url.QueryEscape(other.ID))))

	var fetched projectUser
	ts.Require().Equal(http.StatusOK, ts.getJSON("/users/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// A user in a project is in the system organization unit; one below it is refused.
func (ts *ProjectAPITestSuite) TestUserInProjectOrganizationUnit() {
	userType := ts.createProjectUserType("ou")
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("user-ou-shop")))
	childOUID := ts.createChildOU("user-ou-child", defaultOUID)

	status, body := ts.do(http.MethodPost, "/users", ts.userBody(userType, "ou-child", `"projectId":%q,"ouId":%q`,
		shop.ID, childOUID))

	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("USR-1043", codeOf(ts.T(), body))
}

// An update keeps a user in its project unless it names another, and moves it between projects.
func (ts *ProjectAPITestSuite) TestUpdateUserWithProject() {
	userType := ts.createProjectUserType("update")
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("user-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("user-upd-second")))
	user := ts.createUser(ts.userBody(userType, "updated", `"projectId":%q`, first.ID))

	updated := ts.updateUser(user.ID, ts.userBody(userType, "updated", `"ouId":%q`, defaultOUID))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateUser(user.ID, ts.userBody(userType, "updated", `"ouId":%q,"projectId":%q`,
		defaultOUID, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.userIDs(ts.listUsers("?limit=100&projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{user.ID}, ts.userIDs(ts.listUsers("?limit=100&projectId="+url.QueryEscape(second.ID))))
}

// A shared user stays in its project: its sharing policies were issued while it was there.
func (ts *ProjectAPITestSuite) TestSharedUserCannotChangeProject() {
	userType := ts.createProjectUserType("shared")
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("user-shared-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("user-shared-second")))
	ts.createChildOU("user-shared-child", defaultOUID)
	user := ts.createUser(ts.userBody(userType, "shared", `"projectId":%q`, first.ID))
	status, body := ts.do(http.MethodPost, "/users/"+user.ID+"/sharing-policies", `{"childUnits":{"mode":"all"}}`)
	ts.Require().Equal(http.StatusCreated, status, body)

	status, body = ts.do(http.MethodPut, "/users/"+user.ID, ts.userBody(userType, "shared",
		`"ouId":%q,"projectId":%q`, defaultOUID, second.ID))

	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("USR-1038", codeOf(ts.T(), body))
}

func (ts *ProjectAPITestSuite) TestUserWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"
	userType := ts.createProjectUserType("unknown")

	status, body := ts.do(http.MethodPost, "/users", ts.userBody(userType, "unknown", `"projectId":%q`,
		unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("USR-1042", codeOf(ts.T(), body))

	user := ts.createUser(ts.userBody(userType, "unknown-upd", `"ouId":%q`, defaultOUID))
	status, body = ts.do(http.MethodPut, "/users/"+user.ID, ts.userBody(userType, "unknown-upd",
		`"ouId":%q,"projectId":%q`, defaultOUID, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("USR-1042", codeOf(ts.T(), body))

	list := ts.listUsers("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.Users)
}

// An exported user carries its project, so importing the export brings it back in the same project.
func (ts *ProjectAPITestSuite) TestExportedUserKeepsItsProject() {
	userType := ts.createProjectUserType("export")
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Export"}`, ts.handle("user-export")))
	user := ts.createUser(ts.userBody(userType, "exported", `"projectId":%q`, created.ID))

	body, err := json.Marshal(map[string]any{"users": []string{user.ID}})
	ts.Require().NoError(err)
	status, resp := ts.do(http.MethodPost, "/export", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Equal(1, strings.Count(exported.Resources, "projectId: "+created.ID), exported.Resources)

	status, resp = ts.do(http.MethodDelete, "/users/"+user.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, resp)

	ts.importContent(exported.Resources)
	var imported projectUser
	ts.Require().Equal(http.StatusOK, ts.getJSON("/users/"+user.ID, &imported))
	ts.Equal(created.ID, imported.ProjectID)
}

// A user in no project must name its organization unit, so a create with neither ouId nor projectId is
// refused as the required field was before projects.
func (ts *ProjectAPITestSuite) TestCreateUserWithoutOUOrProject() {
	userType := ts.createProjectUserType("no-placement")

	status, body := ts.do(http.MethodPost, "/users", fmt.Sprintf(`{"type":%q,"attributes":{"username":%q}}`,
		userType, ts.handle("user-no-placement")))

	ts.Equal(http.StatusBadRequest, status, body)
	var resp struct {
		Message string            `json:"message"`
		Errors  map[string]string `json:"errors"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &resp))
	ts.Equal("Validation Failed", resp.Message)
	ts.Contains(resp.Errors, "ouId")
}

// A project's users can be narrowed to an organization unit and listed with their display names; no other
// filter applies to a project listing, and an empty project lists none.
func (ts *ProjectAPITestSuite) TestListUsersInProject() {
	userType := ts.createProjectUserType("list")
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("user-list-shop")))
	childOUID := ts.createChildOU("user-list-child", defaultOUID)
	user := ts.createUser(ts.userBody(userType, "listed", `"projectId":%q`, shop.ID))
	inShop := "?limit=100&projectId=" + url.QueryEscape(shop.ID)

	ts.Equal([]string{user.ID}, ts.userIDs(ts.listUsers(inShop+userOUFilter(defaultOUID))))
	ts.Empty(ts.listUsers(inShop + userOUFilter(childOUID)).Users)

	var displayed struct {
		Users []struct {
			ID       string `json:"id"`
			OUHandle string `json:"ouHandle"`
		} `json:"users"`
	}
	ts.Require().Equal(http.StatusOK, ts.getJSON("/users"+inShop+"&include=display", &displayed))
	ts.Require().Len(displayed.Users, 1)
	ts.Equal(user.ID, displayed.Users[0].ID)
	ts.NotEmpty(displayed.Users[0].OUHandle)

	status, body := ts.do(http.MethodGet, "/users"+inShop+"&filter="+
		url.QueryEscape(fmt.Sprintf("username eq %q", ts.handle("user-listed"))), "")
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("USR-1020", codeOf(ts.T(), body))

	empty := ts.listUsers("?projectId=")
	ts.Zero(empty.TotalResults)
	ts.Empty(empty.Users)
}

// list_users lists a project's users, and refuses a project combined with an attribute.
func (ts *ProjectAPITestSuite) TestUserMCPToolsWithProject() {
	userType := ts.createProjectUserType("mcp")
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("user-mcp-shop")))
	user := ts.createUser(ts.userBody(userType, "mcp", `"projectId":%q`, shop.ID))

	listed, raw := ts.callMCPTool("list_users", map[string]any{"projectId": shop.ID})
	ts.Require().Falsef(listed.Result.IsError, "list failed: %s", raw)
	ts.EqualValues(1, listed.Result.StructuredContent["totalResults"])
	ts.Contains(raw, user.ID)
	ts.Contains(raw, `"projectId":"`+shop.ID+`"`)

	refused, raw := ts.callMCPTool("list_users", map[string]any{"projectId": shop.ID, "attribute": "username",
		"value": ts.handle("user-mcp")})
	ts.Truef(refused.Result.IsError, "a project combined with an attribute must be refused: %s", raw)
}

// createProjectUserType creates a user type in the system organization unit with only a username, and
// records it for cleanup.
func (ts *ProjectAPITestSuite) createProjectUserType(name string) string {
	handle := ts.handle("user-type-" + name)
	id, err := testutils.CreateUserType(testutils.UserType{
		Handle: handle, DisplayName: "Project user", OUID: defaultOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err)
	ts.userTypes = append(ts.userTypes, id)
	return handle
}

// userBody builds a user request of the given type with a unique username, with the given placement fields
// spliced in.
func (ts *ProjectAPITestSuite) userBody(userType, name, placement string, args ...interface{}) string {
	return fmt.Sprintf(`{"type":%q,"attributes":{"username":%q},%s}`, userType, ts.handle("user-"+name),
		fmt.Sprintf(placement, args...))
}

// createUser posts a user, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createUser(body string) projectUser {
	status, resp := ts.do(http.MethodPost, "/users", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectUser
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.users = append(ts.users, created.ID)
	return created
}

// updateUser puts a user and requires a 200.
func (ts *ProjectAPITestSuite) updateUser(id, body string) projectUser {
	status, resp := ts.do(http.MethodPut, "/users/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectUser
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listUsers lists users with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listUsers(query string) projectUserList {
	var list projectUserList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/users"+query, &list))
	return list
}

// userIDs returns the IDs of the listed users.
func (ts *ProjectAPITestSuite) userIDs(list projectUserList) []string {
	ids := []string{}
	for _, user := range list.Users {
		ids = append(ids, user.ID)
	}
	return ids
}

// userOUFilter builds the query parameter that narrows a user listing to an organization unit.
func userOUFilter(ouID string) string {
	return "&filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}
