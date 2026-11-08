// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	flowcommon "github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// UserMandatedRolesTestSuite exercises the roles a user share mandates in the child units it names.
//
// Fixture topology:
//
//	parent (handle: user-mandate-parent)        ← owns the shared users and most roles
//	├── child A (handle: user-mandate-child-a)
//	│   └── grandchild (handle: user-mandate-grandchild)
//	└── child B (handle: user-mandate-child-b)
//
// A mandated assignment is a role assignment row carrying the ID of the user share that wrote it,
// so every scenario reads the rows back through the role's assignment listing.
type UserMandatedRolesTestSuite struct {
	suite.Suite

	parentOUID     string
	childAOUID     string
	childBOUID     string
	grandchildOUID string
	userTypeID     string

	client *http.Client

	// The admin signing in lives in the parent, and a sign-in is bounded to its application's OU.
	signIn testutils.OUSignInApps

	// What the running test created. Users go first, since a role a share mandates cannot be deleted.
	userIDs []string
	roleIDs []string
	rsIDs   []string
}

const mandateUserType = "user-mandate-test-type"

// mandateAssignment is one row of a role's assignment listing.
type mandateAssignment struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	OUID       string `json:"ouId"`
	IsReadOnly bool   `json:"isReadOnly,omitempty"`
}

func TestUserMandatedRolesTestSuite(t *testing.T) {
	suite.Run(t, new(UserMandatedRolesTestSuite))
}

func (ts *UserMandatedRolesTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	var err error
	ts.parentOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-mandate-parent", Name: "User Mandate Parent",
	})
	ts.Require().NoError(err, "create parent OU")
	ts.childAOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-mandate-child-a", Name: "User Mandate Child A", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "create child A")
	ts.childBOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-mandate-child-b", Name: "User Mandate Child B", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "create child B")
	ts.grandchildOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-mandate-grandchild", Name: "User Mandate Grandchild", Parent: &ts.childAOUID,
	})
	ts.Require().NoError(err, "create grandchild")

	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:      mandateUserType,
		DisplayName: "User Mandate Test Type",
		OUID:        ts.parentOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create user type")
}

func (ts *UserMandatedRolesTestSuite) TearDownSuite() {
	if err := ts.signIn.DeleteAll(); err != nil {
		ts.T().Logf("teardown: delete sign-in applications: %v", err)
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	for _, id := range []string{ts.grandchildOUID, ts.childAOUID, ts.childBOUID, ts.parentOUID} {
		if id == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(id); err != nil {
			ts.T().Logf("teardown: delete OU %s: %v", id, err)
		}
	}
}

func (ts *UserMandatedRolesTestSuite) SetupTest() {
	ts.userIDs, ts.roleIDs, ts.rsIDs = nil, nil, nil
}

func (ts *UserMandatedRolesTestSuite) TearDownTest() {
	for _, id := range ts.userIDs {
		ts.deleteQuietly("/users/" + id)
	}
	for _, id := range ts.roleIDs {
		ts.deleteQuietly("/roles/" + id)
	}
	for _, id := range ts.rsIDs {
		if err := testutils.DeleteResourceServerWithChildren(id); err != nil {
			ts.T().Logf("teardown: delete resource server %s: %v", id, err)
		}
	}
}

// deleteQuietly deletes a resource a test may already have deleted.
func (ts *UserMandatedRolesTestSuite) deleteQuietly(path string) {
	status, raw := ts.do(http.MethodDelete, path, nil)
	if status != http.StatusNoContent && status != http.StatusNotFound {
		ts.T().Logf("teardown: delete %s: %d %s", path, status, raw)
	}
}

// do sends a request as the root client and returns its status and body.
func (ts *UserMandatedRolesTestSuite) do(method, path string, body interface{}) (int, []byte) {
	ts.T().Helper()
	return sendRequest(ts.T(), ts.client, method, path, body)
}

// requireCode asserts a status and an error code.
func (ts *UserMandatedRolesTestSuite) requireCode(status int, raw []byte, wantStatus int, wantCode string) {
	ts.T().Helper()
	ts.Require().Equal(wantStatus, status, string(raw))
	var e sharingError
	ts.Require().NoError(json.Unmarshal(raw, &e))
	ts.Require().Equal(wantCode, e.Code, string(raw))
}

// newUserIn creates a user of the suite's type in ouID, deleted when the test ends. Only a user
// that signs in gets a password, since an exported credential would need variables back.
func (ts *UserMandatedRolesTestSuite) newUserIn(ouID, username, password string) string {
	ts.T().Helper()
	attributes := fmt.Sprintf(`{"username": %q}`, username)
	if password != "" {
		attributes = fmt.Sprintf(`{"username": %q, "password": %q}`, username, password)
	}
	id, err := testutils.CreateUser(testutils.User{
		Type: mandateUserType, OUID: ouID, Attributes: json.RawMessage(attributes),
	})
	ts.Require().NoError(err, "create user %s", username)
	ts.userIDs = append(ts.userIDs, id)
	return id
}

// newUser creates a user without a credential in the parent.
func (ts *UserMandatedRolesTestSuite) newUser(username string) string {
	ts.T().Helper()
	return ts.newUserIn(ts.parentOUID, username, "")
}

// newRole creates a role owned by ouID, deleted when the test ends.
func (ts *UserMandatedRolesTestSuite) newRole(
	name, ouID string, permissions ...testutils.ResourcePermissions,
) string {
	ts.T().Helper()
	id, err := testutils.CreateRole(testutils.Role{Name: name, OUID: ouID, Permissions: permissions})
	ts.Require().NoError(err, "create role %s", name)
	ts.roleIDs = append(ts.roleIDs, id)
	return id
}

// unit names a child unit of a policy.
func unit(ouID, reach string) map[string]interface{} {
	return map[string]interface{}{"ouId": ouID, "reach": reach}
}

// fixedRoles is the one roles rule a share accepts: the roles, pinned.
func fixedRoles(roleIDs ...string) map[string]interface{} {
	return map[string]interface{}{"editable": false, "value": roleIDs}
}

// withRolesRule names a child unit whose roles rule is the given one.
func withRolesRule(ouID, reach string, rule map[string]interface{}) map[string]interface{} {
	named := unit(ouID, reach)
	named["overlayRules"] = map[string]interface{}{"roles": rule}
	return named
}

// mandating names a child unit, reaching it alone, in which the share mandates the roles.
func mandating(ouID string, roleIDs ...string) map[string]interface{} {
	return withRolesRule(ouID, "unit", fixedRoles(roleIDs...))
}

// childUnits is a policy body naming the units one by one.
func childUnits(units ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"childUnits": map[string]interface{}{"mode": "selected", "units": units}}
}

// reshareFrom is a policy body issued by ouID rather than by the user's own unit.
func reshareFrom(ouID string, body map[string]interface{}) map[string]interface{} {
	body["initiatingOuId"] = ouID
	return body
}

// atVersion is an edit of a policy last seen at the given version.
func atVersion(body map[string]interface{}, version int) map[string]interface{} {
	body["version"] = version
	return body
}

// shareRole shares a role from its owner and returns the policy.
func (ts *UserMandatedRolesTestSuite) shareRole(roleID string, units ...map[string]interface{}) sharingPolicy {
	ts.T().Helper()
	status, raw := ts.do(http.MethodPost, "/roles/"+roleID+"/sharing-policies", childUnits(units...))
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	var p sharingPolicy
	ts.Require().NoError(json.Unmarshal(raw, &p))
	return p
}

// rolePolicy reads one of a role's sharing policies.
func (ts *UserMandatedRolesTestSuite) rolePolicy(roleID, policyID string) sharingPolicy {
	ts.T().Helper()
	status, raw := ts.do(http.MethodGet, "/roles/"+roleID+"/sharing-policies/"+policyID, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var p sharingPolicy
	ts.Require().NoError(json.Unmarshal(raw, &p))
	return p
}

// shareUser records a user share and returns it.
func (ts *UserMandatedRolesTestSuite) shareUser(userID string, body map[string]interface{}) sharingPolicy {
	ts.T().Helper()
	status, raw := ts.do(http.MethodPost, "/users/"+userID+"/sharing-policies", body)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	var p sharingPolicy
	ts.Require().NoError(json.Unmarshal(raw, &p))
	return p
}

// editShare replaces a user share with body and returns it.
func (ts *UserMandatedRolesTestSuite) editShare(
	userID string, p sharingPolicy, body map[string]interface{},
) sharingPolicy {
	ts.T().Helper()
	status, raw := ts.do(http.MethodPut, policyPath(userID, p.ID), atVersion(body, p.Version))
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var edited sharingPolicy
	ts.Require().NoError(json.Unmarshal(raw, &edited))
	return edited
}

// sharesOf lists a user's shares.
func (ts *UserMandatedRolesTestSuite) sharesOf(userID string) []sharingPolicy {
	ts.T().Helper()
	status, raw := ts.do(http.MethodGet, "/users/"+userID+"/sharing-policies", nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list sharingPolicyList
	ts.Require().NoError(json.Unmarshal(raw, &list))
	return list.Policies
}

// assignments lists a role's assignments in one unit, or in every unit when ouID is empty.
func (ts *UserMandatedRolesTestSuite) assignments(roleID, ouID string) []mandateAssignment {
	ts.T().Helper()
	path := "/roles/" + roleID + "/assignments?limit=100"
	if ouID != "" {
		path += "&filter=" + url.QueryEscape(`ouId eq "`+ouID+`"`)
	}
	status, raw := ts.do(http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list struct {
		Assignments []mandateAssignment `json:"assignments"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list))
	return list.Assignments
}

// mandatedBy returns which of the roles are mandated for the user in the unit.
func (ts *UserMandatedRolesTestSuite) mandatedBy(userID, ouID string, roleIDs ...string) []string {
	ts.T().Helper()
	held := []string{}
	for _, roleID := range roleIDs {
		for _, a := range ts.assignments(roleID, ouID) {
			if a.ID == userID && a.IsReadOnly {
				held = append(held, roleID)
			}
		}
	}
	return held
}

// mandatedRow is the row a share writes for a user in a unit.
func mandatedRow(userID, ouID string) mandateAssignment {
	return mandateAssignment{ID: userID, Type: "user", OUID: ouID, IsReadOnly: true}
}

// directRow is the row a direct assignment writes for a user in a unit.
func directRow(userID, ouID string) mandateAssignment {
	return mandateAssignment{ID: userID, Type: "user", OUID: ouID}
}

// assignInUnit adds or removes a role's assignment of users in ouID, as that unit does.
func (ts *UserMandatedRolesTestSuite) assignInUnit(action, roleID, ouID string, userIDs ...string) (int, []byte) {
	ts.T().Helper()
	members := make([]map[string]string, 0, len(userIDs))
	for _, id := range userIDs {
		members = append(members, map[string]string{"id": id, "type": "user"})
	}
	body := map[string]interface{}{"assignments": members}
	if ouID != "" {
		body["ouId"] = ouID
	}
	return ts.do(http.MethodPost, "/roles/"+roleID+"/assignments/"+action, body)
}

// mandatedInChildA creates a role shared with child A and a user whose share mandates it there.
func (ts *UserMandatedRolesTestSuite) mandatedInChildA(name string) (roleID, userID string, share sharingPolicy) {
	ts.T().Helper()
	roleID = ts.newRole("Mandate Role "+name, ts.parentOUID)
	ts.shareRole(roleID, unit(ts.childAOUID, "unit"))
	userID = ts.newUser(name)
	share = ts.shareUser(userID, childUnits(mandating(ts.childAOUID, roleID)))
	return roleID, userID, share
}

// runAdministrationFlow runs a shipped administration flow as the root client and returns its step.
func (ts *UserMandatedRolesTestSuite) runAdministrationFlow(
	handle string, inputs map[string]string,
) *flowcommon.FlowStep {
	ts.T().Helper()
	flowID, err := testutils.GetFlowIDByHandle(handle, "ADMINISTRATION")
	ts.Require().NoError(err, "resolve the %s flow", handle)
	result, err := flowcommon.ExecuteAdministrationFlow(flowID, inputs, true)
	ts.Require().NoError(err, "run the %s flow", handle)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "%s: %+v", handle, result.Error)
	return result.Step
}

// deletePolicyRow deletes a sharing policy, its units and its rules straight from the configuration database.
func (ts *UserMandatedRolesTestSuite) deletePolicyRow(policyID string) {
	ts.T().Helper()
	_, err := testutils.QueryConfigDB(fmt.Sprintf(
		`PRAGMA foreign_keys = ON; DELETE FROM "RESOURCE_SHARING_POLICY" WHERE ID = '%s';`, policyID))
	ts.Require().NoError(err, "delete sharing policy %s", policyID)
}

// requireNothingWritten asserts a refused share left the user unshared and the roles unassigned.
func (ts *UserMandatedRolesTestSuite) requireNothingWritten(userID string, roleIDs ...string) {
	ts.T().Helper()
	ts.Empty(ts.sharesOf(userID), "no share was recorded")
	for _, roleID := range roleIDs {
		ts.Empty(ts.assignments(roleID, ""), "no assignment was written")
	}
}

// importDocument imports content and returns the response.
func (ts *UserMandatedRolesTestSuite) importDocument(content string, continueOnError bool) testutils.ImportResponse {
	ts.T().Helper()
	status, raw := ts.do(http.MethodPost, "/import", map[string]interface{}{
		"content": content,
		"options": map[string]interface{}{"upsert": true, "continueOnError": continueOnError},
	})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var resp testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(raw, &resp))
	return resp
}

// resultFor returns the import outcome for one resource type, failing when there is not exactly one.
func (ts *UserMandatedRolesTestSuite) resultFor(resp testutils.ImportResponse, resourceType string) testutils.ImportItem {
	ts.T().Helper()
	var found []testutils.ImportItem
	for _, r := range resp.Results {
		if r.ResourceType == resourceType {
			found = append(found, r)
		}
	}
	ts.Require().Len(found, 1, "%s outcomes: %+v", resourceType, resp.Results)
	return found[0]
}

// export exports the named resources and returns the document.
func (ts *UserMandatedRolesTestSuite) export(body map[string]interface{}) string {
	ts.T().Helper()
	status, raw := ts.do(http.MethodPost, "/export", body)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &exported))
	return exported.Resources
}

// newUUID returns a random version 4 UUID for a resource an import creates.
func (ts *UserMandatedRolesTestSuite) newUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	ts.Require().NoError(err)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// roleDocument is a role import document owned by the parent and shared with the named units.
func (ts *UserMandatedRolesTestSuite) roleDocument(roleID, name, reach string, sharedWith ...string) string {
	var doc strings.Builder
	fmt.Fprintf(&doc, "resource_type: role\nid: %s\nname: %q\nouId: %s\npermissions: []\n", roleID, name, ts.parentOUID)
	doc.WriteString("sharingPolicies:\n  - childUnits:\n      mode: selected\n      units:\n")
	for _, ouID := range sharedWith {
		fmt.Fprintf(&doc, "        - ouId: %s\n          reach: %s\n", ouID, reach)
	}
	return doc.String()
}

// userDocument is a user import document in the parent whose share mandates roleID in child A.
func (ts *UserMandatedRolesTestSuite) userDocument(idLine, username, roleID string) string {
	return fmt.Sprintf(`resource_type: user
%stype: %s
ouId: %s
attributes:
  username: %s
sharingPolicies:
  - childUnits:
      mode: selected
      units:
        - ouId: %s
          reach: unit
          overlayRules:
            roles:
              editable: false
              value: [%s]
`, idLine, mandateUserType, ts.parentOUID, username, ts.childAOUID, roleID)
}

// M1: a share naming a child unit writes a mandated assignment there, tagged with the share.
func (ts *UserMandatedRolesTestSuite) TestAShareMandatesItsRolesInTheUnitItNames() {
	roleID, userID, _ := ts.mandatedInChildA("mandate-m1")

	ts.Equal([]mandateAssignment{mandatedRow(userID, ts.childAOUID)}, ts.assignments(roleID, ts.childAOUID))
	ts.Empty(ts.assignments(roleID, ts.parentOUID), "nothing is assigned in the issuing unit")

	status, raw := ts.do(http.MethodGet, overlayPath(userID, ts.childAOUID), nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var overlay struct {
		Rules map[string]struct {
			Editable bool     `json:"editable"`
			Value    []string `json:"value"`
		} `json:"rules"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &overlay))
	ts.Require().Contains(overlay.Rules, "roles", string(raw))
	ts.False(overlay.Rules["roles"].Editable)
	ts.Equal([]string{roleID}, overlay.Rules["roles"].Value)
}

// M2: the child cannot remove a mandated assignment, but may add and remove a direct one beside it.
func (ts *UserMandatedRolesTestSuite) TestTheChildCannotRemoveAMandatedAssignment() {
	roleID, userID, _ := ts.mandatedInChildA("mandate-m2")
	mandated := mandatedRow(userID, ts.childAOUID)

	status, raw := ts.assignInUnit("remove", roleID, ts.childAOUID, userID)
	ts.requireCode(status, raw, http.StatusBadRequest, "ROL-1036")
	ts.Equal([]mandateAssignment{mandated}, ts.assignments(roleID, ts.childAOUID))

	status, raw = ts.assignInUnit("add", roleID, ts.childAOUID, userID)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	ts.ElementsMatch([]mandateAssignment{mandated, directRow(userID, ts.childAOUID)},
		ts.assignments(roleID, ts.childAOUID), "the direct and the mandated assignment coexist")

	status, raw = ts.assignInUnit("remove", roleID, ts.childAOUID, userID)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	ts.Equal([]mandateAssignment{mandated}, ts.assignments(roleID, ts.childAOUID),
		"removing takes the direct assignment and leaves the mandated one")
}

// M3: an edit adds and removes exactly the rows its roles and units changed.
func (ts *UserMandatedRolesTestSuite) TestEditingAShareFollowsItsRoles() {
	r1 := ts.newRole("Mandate M3 Role 1", ts.parentOUID)
	r2 := ts.newRole("Mandate M3 Role 2", ts.parentOUID)
	for _, roleID := range []string{r1, r2} {
		ts.shareRole(roleID, unit(ts.childAOUID, "unit"), unit(ts.childBOUID, "unit"))
	}
	userID := ts.newUser("mandate-m3")

	share := ts.shareUser(userID, childUnits(mandating(ts.childAOUID, r1), mandating(ts.childBOUID, r1, r2)))
	ts.Equal([]string{r1}, ts.mandatedBy(userID, ts.childAOUID, r1, r2))
	ts.Equal([]string{r1, r2}, ts.mandatedBy(userID, ts.childBOUID, r1, r2))

	// Adding a role in A and dropping one in B touches those two rows only.
	share = ts.editShare(userID, share, childUnits(mandating(ts.childAOUID, r1, r2), mandating(ts.childBOUID, r1)))
	ts.Equal([]string{r1, r2}, ts.mandatedBy(userID, ts.childAOUID, r1, r2))
	ts.Equal([]string{r1}, ts.mandatedBy(userID, ts.childBOUID, r1, r2))
	ts.Empty(ts.assignments(r2, ts.childBOUID), "the dropped role's row is gone")

	// Dropping a unit takes that unit's rows and leaves the other unit's.
	share = ts.editShare(userID, share, childUnits(mandating(ts.childAOUID, r1, r2)))
	ts.Equal([]string{r1, r2}, ts.mandatedBy(userID, ts.childAOUID, r1, r2))
	ts.Empty(ts.assignments(r1, ts.childBOUID))
	ts.Empty(ts.assignments(r2, ts.childBOUID))
}

// M4: deleting a share removes its rows and nothing else; the child's direct assignments stay.
func (ts *UserMandatedRolesTestSuite) TestDeletingAShareRemovesOnlyItsRows() {
	roleID, userID, share := ts.mandatedInChildA("mandate-m4")
	memberID := ts.newUserIn(ts.childAOUID, "mandate-m4-child-member", "")
	status, raw := ts.assignInUnit("add", roleID, ts.childAOUID, userID, memberID)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	status, raw = ts.do(http.MethodDelete, policyPath(userID, share.ID), nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	// The shared user's own direct row stays too: removing a policy removes only its rows (D76).
	ts.ElementsMatch([]mandateAssignment{directRow(userID, ts.childAOUID), directRow(memberID, ts.childAOUID)},
		ts.assignments(roleID, ts.childAOUID))
}

// M5: roles are mandated only on a unit named one by one with reach unit, on every write path.
func (ts *UserMandatedRolesTestSuite) TestRolesOnAShareNotNamingUnitsOneByOneAreRefused() {
	roleID := ts.newRole("Mandate M5 Role", ts.parentOUID)
	ts.shareRole(roleID, unit(ts.childAOUID, "subtree"))
	userID := ts.newUser("mandate-m5")

	for name, body := range map[string]map[string]interface{}{
		"EveryChildUnit": {"childUnits": map[string]interface{}{
			"mode": "all", "overlayRules": map[string]interface{}{"roles": fixedRoles(roleID)},
		}},
		"NamedUnitWithItsSubtree": childUnits(withRolesRule(ts.childAOUID, "subtree", fixedRoles(roleID))),
	} {
		ts.Run(name, func() {
			status, raw := ts.do(http.MethodPost, "/users/"+userID+"/sharing-policies", body)
			ts.requireCode(status, raw, http.StatusBadRequest, "USR-1039")
		})
	}
	ts.requireNothingWritten(userID, roleID)

	// Every unit is a reach only a document may state, and a document mandating roles on it is refused.
	resp := ts.importDocument(fmt.Sprintf(`resource_type: user
type: %s
ouId: %s
attributes:
  username: mandate-m5-every-unit
sharingPolicies:
  - allUnits:
      overlayRules:
        roles:
          editable: false
          value: [%s]
`, mandateUserType, ts.parentOUID, roleID), false)
	result := ts.resultFor(resp, "user")
	ts.Equal("failed", result.Status, "%+v", result)
	ts.Equal("USR-1039", result.Code, "%+v", result)
	ts.Require().NotEmpty(result.ResourceID)
	ts.userIDs = append(ts.userIDs, result.ResourceID)
	status, raw := ts.do(http.MethodGet, "/users/"+result.ResourceID, nil)
	ts.Equal(http.StatusNotFound, status, "the user the import created is gone: %s", raw)
	ts.Empty(ts.assignments(roleID, ""))
}

// M6: a role the named unit neither owns nor has shared with it is refused, and nothing is written.
func (ts *UserMandatedRolesTestSuite) TestARoleNotSharedWithTheUnitIsRefused() {
	roleID := ts.newRole("Mandate M6 Role", ts.parentOUID)
	ts.shareRole(roleID, unit(ts.childBOUID, "unit"))
	userID := ts.newUser("mandate-m6")

	status, raw := ts.do(http.MethodPost, "/users/"+userID+"/sharing-policies",
		childUnits(mandating(ts.childAOUID, roleID)))
	ts.requireCode(status, raw, http.StatusBadRequest, "ROL-1035")
	ts.requireNothingWritten(userID, roleID)
}

// M7 and rule 4: a role cannot be unshared from a unit a share mandates it in, whether by an edit, a
// delete or an import, and each refusal leaves the store as it was. Once the mandate goes, it can.
func (ts *UserMandatedRolesTestSuite) TestAMandatedRoleCannotBeUnsharedFromTheUnit() {
	const name = "Mandate M7 Role"
	roleID := ts.newRole(name, ts.parentOUID)
	rolePolicy := ts.shareRole(roleID, unit(ts.childAOUID, "unit"), unit(ts.childBOUID, "unit"))
	userID := ts.newUser("mandate-m7")
	share := ts.shareUser(userID, childUnits(mandating(ts.childAOUID, roleID)))
	overlayInA := "/roles/" + roleID + "/overlay-rules?ouId=" + ts.childAOUID

	requireUnchanged := func(attempt string) {
		ts.T().Helper()
		ts.Equal(rolePolicy.Version, ts.rolePolicy(roleID, rolePolicy.ID).Version, attempt)
		status, raw := ts.do(http.MethodGet, overlayInA, nil)
		ts.Equal(http.StatusOK, status, "%s: the role still reaches A: %s", attempt, raw)
		ts.Equal([]mandateAssignment{mandatedRow(userID, ts.childAOUID)},
			ts.assignments(roleID, ts.childAOUID), attempt)
	}
	onlyB := func() map[string]interface{} {
		return atVersion(childUnits(unit(ts.childBOUID, "unit")), rolePolicy.Version)
	}

	status, raw := ts.do(http.MethodPut, "/roles/"+roleID+"/sharing-policies/"+rolePolicy.ID, onlyB())
	ts.requireCode(status, raw, http.StatusConflict, "ROL-1037")
	requireUnchanged("edit")

	status, raw = ts.do(http.MethodDelete, "/roles/"+roleID+"/sharing-policies/"+rolePolicy.ID, nil)
	ts.requireCode(status, raw, http.StatusConflict, "ROL-1037")
	requireUnchanged("delete")

	resp := ts.importDocument(ts.roleDocument(roleID, name, "unit", ts.childBOUID), false)
	result := ts.resultFor(resp, "role")
	ts.Equal("failed", result.Status, "%+v", result)
	ts.Equal("ROL-1037", result.Code, "%+v", result)
	requireUnchanged("import")

	// The share keeps the user in A but stops mandating the role there.
	ts.editShare(userID, share, childUnits(unit(ts.childAOUID, "unit")))
	ts.Empty(ts.assignments(roleID, ts.childAOUID))

	status, raw = ts.do(http.MethodPut, "/roles/"+roleID+"/sharing-policies/"+rolePolicy.ID, onlyB())
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.do(http.MethodGet, overlayInA, nil)
	ts.Equal(http.StatusNotFound, status, "the role no longer reaches A: %s", raw)
}

// M8: a mandate holds where the role's own rule closes assignments in the unit, while a direct
// assignment there is still refused.
func (ts *UserMandatedRolesTestSuite) TestAMandateBypassesTheRolesAssignmentsRule() {
	roleID := ts.newRole("Mandate M8 Role", ts.parentOUID)
	closed := unit(ts.childAOUID, "unit")
	closed["overlayRules"] = map[string]interface{}{"assignments": map[string]bool{"editable": false}}
	ts.shareRole(roleID, closed)
	userID := ts.newUser("mandate-m8")

	ts.shareUser(userID, childUnits(mandating(ts.childAOUID, roleID)))
	mandated := []mandateAssignment{mandatedRow(userID, ts.childAOUID)}
	ts.Equal(mandated, ts.assignments(roleID, ts.childAOUID))

	status, raw := ts.assignInUnit("add", roleID, ts.childAOUID, userID)
	ts.requireCode(status, raw, http.StatusBadRequest, "ROL-1031")
	ts.Equal(mandated, ts.assignments(roleID, ts.childAOUID))
}

// M9: the roles rule is only ever a fixed list. Each other shape is refused with the code its own
// check gives: the framework refuses what would widen the issuer's hold, the user type the rest.
func (ts *UserMandatedRolesTestSuite) TestOnlyAFixedRolesRuleIsAccepted() {
	r1 := ts.newRole("Mandate M9 Role 1", ts.parentOUID)
	r2 := ts.newRole("Mandate M9 Role 2", ts.parentOUID)
	ts.shareRole(r1, unit(ts.childAOUID, "unit"))
	ts.shareRole(r2, unit(ts.childAOUID, "unit"))
	userID := ts.newUser("mandate-m9")

	for _, tc := range []struct {
		name string
		rule map[string]interface{}
		code string
	}{
		{"Editable", map[string]interface{}{"editable": true, "value": []string{r1}}, "USR-1029"},
		{"AllowedValues", map[string]interface{}{
			"editable": false, "value": []string{r1}, "allowedValues": []string{r1, r2}}, "USR-1029"},
		{"NoValue", map[string]interface{}{"editable": false}, "USR-1040"},
		{"EmptyValue", map[string]interface{}{"editable": false, "value": []string{}}, "USR-1040"},
		{"ExcludedValues", map[string]interface{}{
			"editable": false, "value": []string{r1}, "excludedValues": []string{r2}}, "USR-1040"},
	} {
		ts.Run(tc.name, func() {
			status, raw := ts.do(http.MethodPost, "/users/"+userID+"/sharing-policies",
				childUnits(withRolesRule(ts.childAOUID, "unit", tc.rule)))
			ts.requireCode(status, raw, http.StatusBadRequest, tc.code)
		})
	}
	ts.requireNothingWritten(userID, r1, r2)
}

// M10: a mandated unit reshares the user with roles from its own mandate only, and an ancestor edit
// that clamps or deletes the reshare carries its rows with it.
func (ts *UserMandatedRolesTestSuite) TestAReshareFollowsItsAncestorsMandate() {
	r1 := ts.newRole("Mandate M10 Role 1", ts.parentOUID)
	r2 := ts.newRole("Mandate M10 Role 2", ts.parentOUID)
	r3 := ts.newRole("Mandate M10 Role 3", ts.parentOUID)
	for _, roleID := range []string{r1, r2, r3} {
		ts.shareRole(roleID, unit(ts.childAOUID, "subtree"))
	}
	userID := ts.newUser("mandate-m10")
	share := ts.shareUser(userID, childUnits(mandating(ts.childAOUID, r1, r2)))

	// R3 reaches the grandchild, but A holds no mandate of it to pass on.
	status, raw := ts.do(http.MethodPost, "/users/"+userID+"/sharing-policies",
		reshareFrom(ts.childAOUID, childUnits(mandating(ts.grandchildOUID, r3))))
	ts.requireCode(status, raw, http.StatusBadRequest, "USR-1029")
	ts.Empty(ts.assignments(r3, ""))

	reshare := ts.shareUser(userID, reshareFrom(ts.childAOUID, childUnits(mandating(ts.grandchildOUID, r1, r2))))
	ts.Equal("reshare", reshare.Stage)
	ts.Equal([]string{r1, r2}, ts.mandatedBy(userID, ts.grandchildOUID, r1, r2, r3))

	// Narrowing A's mandate clamps the reshare beneath it, and the reshare's rows follow.
	share = ts.editShare(userID, share, childUnits(mandating(ts.childAOUID, r1)))
	ts.Equal([]string{r1}, ts.mandatedBy(userID, ts.childAOUID, r1, r2, r3))
	ts.Equal([]string{r1}, ts.mandatedBy(userID, ts.grandchildOUID, r1, r2, r3))
	ts.Empty(ts.assignments(r2, ""), "R2 is mandated nowhere any more")

	// Moving the share off A takes the reshare beneath it, rows and all.
	ts.editShare(userID, share, childUnits(unit(ts.childBOUID, "unit")))
	status, raw = ts.do(http.MethodGet, policyPath(userID, reshare.ID), nil)
	ts.requireCode(status, raw, http.StatusNotFound, "USR-1030")
	for _, roleID := range []string{r1, r2, r3} {
		ts.Empty(ts.assignments(roleID, ""))
	}
}

// M10: deleting a share takes the reshare beneath it, and the reshare's rows go with it.
func (ts *UserMandatedRolesTestSuite) TestDeletingAShareTakesItsReshareRows() {
	roleID := ts.newRole("Mandate M10 Delete Role", ts.parentOUID)
	ts.shareRole(roleID, unit(ts.childAOUID, "subtree"))
	userID := ts.newUser("mandate-m10-delete")
	share := ts.shareUser(userID, childUnits(mandating(ts.childAOUID, roleID)))
	reshare := ts.shareUser(userID, reshareFrom(ts.childAOUID, childUnits(mandating(ts.grandchildOUID, roleID))))
	ts.Equal([]string{roleID}, ts.mandatedBy(userID, ts.grandchildOUID, roleID))

	status, raw := ts.do(http.MethodDelete, policyPath(userID, share.ID), nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	status, raw = ts.do(http.MethodGet, policyPath(userID, reshare.ID), nil)
	ts.requireCode(status, raw, http.StatusNotFound, "USR-1030")
	ts.Empty(ts.assignments(roleID, ""))
}

// M11: a role export leaves mandated assignments out and keeps direct ones; the user's share
// carries the mandate instead.
func (ts *UserMandatedRolesTestSuite) TestARoleExportLeavesMandatedAssignmentsOut() {
	roleID, userID, _ := ts.mandatedInChildA("mandate-m11-export")
	memberID := ts.newUserIn(ts.childAOUID, "mandate-m11-member", "")
	status, raw := ts.assignInUnit("add", roleID, ts.childAOUID, memberID)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	roleDoc := ts.export(map[string]interface{}{"roles": []string{roleID}})
	ts.Contains(roleDoc, memberID, "the direct assignment is exported")
	ts.NotContains(roleDoc, userID, "the mandated assignment is not")

	ts.Contains(ts.export(map[string]interface{}{"users": []string{userID}}), roleID,
		"the user's share carries the mandate")
}

// M11, R12: one document creating a role and a user whose share mandates it imports on a fresh
// deployment, and the user comes out holding the mandated row.
func (ts *UserMandatedRolesTestSuite) TestADocumentWithAUserAndTheRoleItsShareMandatesImports() {
	roleID := ts.newUUID()
	ts.roleIDs = append(ts.roleIDs, roleID)

	resp := ts.importDocument(ts.roleDocument(roleID, "Mandate Imported Role", "unit", ts.childAOUID)+"---\n"+
		ts.userDocument("", "mandate-imported-user", roleID), false)
	ts.Require().Zero(resp.Summary.Failed, "%+v", resp.Results)
	ts.Equal(2, resp.Summary.Imported)
	user := ts.resultFor(resp, "user")
	ts.Require().NotEmpty(user.ResourceID)
	ts.userIDs = append(ts.userIDs, user.ResourceID)

	status, raw := ts.do(http.MethodGet, "/users/"+user.ResourceID, nil)
	ts.Require().Equal(http.StatusOK, status, "the imported user survives: %s", raw)
	shares := ts.sharesOf(user.ResourceID)
	ts.Require().Len(shares, 1)
	ts.Equal([]mandateAssignment{mandatedRow(user.ResourceID, ts.childAOUID)},
		ts.assignments(roleID, ts.childAOUID))
}

// R12b: a plain export of a user, the role its share mandates, and a role assigning it directly in
// the same child imports back after all three are deleted, both rows included.
func (ts *UserMandatedRolesTestSuite) TestAUserAndItsRolesSurviveExportAndImport() {
	mandatedRole := ts.newRole("Mandate Round Trip Mandated", ts.parentOUID)
	directRole := ts.newRole("Mandate Round Trip Direct", ts.parentOUID)
	ts.shareRole(mandatedRole, unit(ts.childAOUID, "unit"))
	ts.shareRole(directRole, unit(ts.childAOUID, "unit"))
	userID := ts.newUser("mandate-roundtrip")
	ts.shareUser(userID, childUnits(mandating(ts.childAOUID, mandatedRole)))
	status, raw := ts.assignInUnit("add", directRole, ts.childAOUID, userID)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	content := ts.export(map[string]interface{}{"users": []string{userID}, "roles": []string{mandatedRole, directRole}})

	// Deleting the user takes its share and rows, after which the roles can go too.
	for _, path := range []string{"/users/" + userID, "/roles/" + mandatedRole, "/roles/" + directRole} {
		status, raw = ts.do(http.MethodDelete, path, nil)
		ts.Require().Equal(http.StatusNoContent, status, "delete %s: %s", path, raw)
	}

	resp := ts.importDocument(content, false)
	ts.Require().Zero(resp.Summary.Failed, "%+v", resp.Results)
	ts.Equal(3, resp.Summary.Imported)

	shares := ts.sharesOf(userID)
	ts.Require().Len(shares, 1)
	ts.Equal([]mandateAssignment{mandatedRow(userID, ts.childAOUID)},
		ts.assignments(mandatedRole, ts.childAOUID))
	ts.Equal([]mandateAssignment{directRow(userID, ts.childAOUID)}, ts.assignments(directRole, ts.childAOUID))
}

// Recorded behaviour (Task 4): on an existing user, a share carrying roles is replayed after every
// document. A role in the same document assigning the user directly in a unit only that new share
// reaches is refused and reported against the role, while the user's share still lands.
func (ts *UserMandatedRolesTestSuite) TestImportingANewMandateOnAnExistingUserLandsAfterTheDocumentsRoles() {
	mandatedRole := ts.newRole("Mandate Existing User Mandated", ts.parentOUID)
	ts.shareRole(mandatedRole, unit(ts.childAOUID, "unit"))
	const username = "mandate-import-existing"
	userID := ts.newUser(username)
	directRole := ts.newUUID()
	ts.roleIDs = append(ts.roleIDs, directRole)

	resp := ts.importDocument(ts.userDocument("id: "+userID+"\n", username, mandatedRole)+"---\n"+
		ts.roleDocument(directRole, "Mandate Existing User Direct", "unit", ts.childAOUID)+
		fmt.Sprintf("assignments:\n  - id: %s\n    type: user\n    ouId: %s\n", userID, ts.childAOUID), true)

	roleResult := ts.resultFor(resp, "role")
	ts.Equal("failed", roleResult.Status, "%+v", roleResult)
	ts.Equal("ROL-1030", roleResult.Code, "%+v", roleResult)
	status, raw := ts.do(http.MethodGet, "/roles/"+directRole, nil)
	ts.Equal(http.StatusNotFound, status, "the refused role's create is undone: %s", raw)

	userResult := ts.resultFor(resp, "user")
	ts.Equal("success", userResult.Status, "%+v", userResult)
	ts.Equal("update", userResult.Operation)
	shares := ts.sharesOf(userID)
	ts.Require().Len(shares, 1)
	ts.Equal([]mandateAssignment{mandatedRow(userID, ts.childAOUID)},
		ts.assignments(mandatedRole, ts.childAOUID))
}

// Task 4 carry: an import can leave a new user shared without its mandated roles (a panic, a cancel
// or a failed undo). Importing the user's document again puts the roles back on the same share.
func (ts *UserMandatedRolesTestSuite) TestReimportingHealsAShareLeftWithoutItsRoles() {
	roleID := ts.newRole("Mandate Heal Role", ts.parentOUID)
	ts.shareRole(roleID, unit(ts.childAOUID, "unit"))
	const username = "mandate-import-heal"
	userID := ts.newUser(username)
	left := ts.shareUser(userID, childUnits(unit(ts.childAOUID, "unit")))

	resp := ts.importDocument(ts.userDocument("id: "+userID+"\n", username, roleID), false)
	ts.Require().Zero(resp.Summary.Failed, "%+v", resp.Results)

	shares := ts.sharesOf(userID)
	ts.Require().Len(shares, 1)
	ts.Equal(left.ID, shares[0].ID, "the share is edited in place")
	ts.Equal([]mandateAssignment{mandatedRow(userID, ts.childAOUID)}, ts.assignments(roleID, ts.childAOUID))
}

// Recorded behaviour (Task 4 carry): once one of an existing user's policies carries roles, the
// import defers every policy of the user, the role-free ones too. Here the reshare into the grandchild
// carries no roles, yet a role in the same document assigning the user there is still refused.
func (ts *UserMandatedRolesTestSuite) TestImportingAnExistingUserDefersEveryPolicyOnceOneMandatesRoles() {
	mandatedRole := ts.newRole("Mandate Deferred Mandated", ts.parentOUID)
	ts.shareRole(mandatedRole, unit(ts.childAOUID, "unit"))
	const username = "mandate-import-deferred"
	userID := ts.newUser(username)
	// The share into A is already there, so only the deferral keeps the reshare from replaying at once.
	ts.shareUser(userID, childUnits(unit(ts.childAOUID, "unit")))
	directRole := ts.newUUID()
	ts.roleIDs = append(ts.roleIDs, directRole)

	userDoc := ts.userDocument("id: "+userID+"\n", username, mandatedRole) + fmt.Sprintf(`  - initiatingOuId: %s
    childUnits:
      mode: selected
      units:
        - ouId: %s
          reach: unit
`, ts.childAOUID, ts.grandchildOUID)
	roleDoc := ts.roleDocument(directRole, "Mandate Deferred Direct", "subtree", ts.childAOUID) +
		fmt.Sprintf("assignments:\n  - id: %s\n    type: user\n    ouId: %s\n", userID, ts.grandchildOUID)
	resp := ts.importDocument(userDoc+"---\n"+roleDoc, true)

	roleResult := ts.resultFor(resp, "role")
	ts.Equal("failed", roleResult.Status, "%+v", roleResult)
	ts.Equal("ROL-1030", roleResult.Code, "%+v", roleResult)
	userResult := ts.resultFor(resp, "user")
	ts.Equal("success", userResult.Status, "%+v", userResult)

	stages := map[string]string{}
	for _, p := range ts.sharesOf(userID) {
		stages[p.InitiatingOUID] = p.Stage
	}
	ts.Equal(map[string]string{ts.parentOUID: "share", ts.childAOUID: "reshare"}, stages)
	ts.Equal([]mandateAssignment{mandatedRow(userID, ts.childAOUID)},
		ts.assignments(mandatedRole, ts.childAOUID))
}

// M12: deleting a user removes every assignment it holds, mandated and direct.
func (ts *UserMandatedRolesTestSuite) TestDeletingTheUserRemovesEveryRow() {
	roleID, userID, _ := ts.mandatedInChildA("mandate-m12-user")
	for _, ouID := range []string{ts.childAOUID, ""} {
		status, raw := ts.assignInUnit("add", roleID, ouID, userID)
		ts.Require().Equal(http.StatusNoContent, status, string(raw))
	}
	ts.Len(ts.assignments(roleID, ""), 3, "mandated in A, direct in A, direct in the parent")

	status, raw := ts.do(http.MethodDelete, "/users/"+userID, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	ts.Empty(ts.assignments(roleID, ""))
}

// M12, R7: a role a share mandates cannot be deleted until the mandate goes.
func (ts *UserMandatedRolesTestSuite) TestAMandatedRoleCannotBeDeleted() {
	roleID, userID, share := ts.mandatedInChildA("mandate-m12-role")

	status, raw := ts.do(http.MethodDelete, "/roles/"+roleID, nil)
	ts.requireCode(status, raw, http.StatusConflict, "ROL-1039")
	status, raw = ts.do(http.MethodGet, "/roles/"+roleID, nil)
	ts.Equal(http.StatusOK, status, "the role is still there: %s", raw)
	ts.Equal([]mandateAssignment{mandatedRow(userID, ts.childAOUID)}, ts.assignments(roleID, ts.childAOUID))

	status, raw = ts.do(http.MethodDelete, policyPath(userID, share.ID), nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	status, raw = ts.do(http.MethodDelete, "/roles/"+roleID, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	status, _ = ts.do(http.MethodGet, "/roles/"+roleID, nil)
	ts.Equal(http.StatusNotFound, status)
}

// M13: the issuing unit may name only a role it can see. A role child A owns could be held there,
// but the parent issuing the share cannot name it.
func (ts *UserMandatedRolesTestSuite) TestARoleTheIssuerCannotSeeIsRefused() {
	roleID := ts.newRole("Mandate M13 Role", ts.childAOUID)
	userID := ts.newUser("mandate-m13")

	status, raw := ts.do(http.MethodPost, "/users/"+userID+"/sharing-policies",
		childUnits(mandating(ts.childAOUID, roleID)))
	ts.requireCode(status, raw, http.StatusBadRequest, "USR-1029")
	ts.requireNothingWritten(userID, roleID)
}

// R8: mandating a role is a grant, so a caller may mandate only a role conferring nothing it lacks.
func (ts *UserMandatedRolesTestSuite) TestMandatingARoleTheCallerCannotGrantIsRefused() {
	const adminName = "mandate-parent-admin"
	adminID := ts.newUserIn(ts.parentOUID, adminName, sharingTestPassword)
	admin, adminRSID, adminRoleID := scopedAdminClient(ts.T(), &ts.signIn, ts.parentOUID, adminID, adminName,
		"User Mandate Test", "https://user-mandate-test.example.com/parent")
	ts.rsIDs = append(ts.rsIDs, adminRSID)
	ts.roleIDs = append(ts.roleIDs, adminRoleID)

	privilegedRSID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "User Mandate Privileged RS", Identifier: "https://user-mandate-test.example.com/privileged",
		OUID: ts.parentOUID,
	}, []testutils.Action{{Name: "Operate", Handle: "operate"}})
	ts.Require().NoError(err)
	ts.rsIDs = append(ts.rsIDs, privilegedRSID)
	privileged := ts.newRole("Mandate Privileged Role", ts.parentOUID,
		testutils.ResourcePermissions{ResourceServerID: privilegedRSID, Permissions: []string{"operate"}})
	harmless := ts.newRole("Mandate Harmless Role", ts.parentOUID)
	ts.shareRole(privileged, unit(ts.childAOUID, "unit"))
	ts.shareRole(harmless, unit(ts.childAOUID, "unit"))
	userID := ts.newUser("mandate-grant-target")
	path := "/users/" + userID + "/sharing-policies"

	status, raw := sendRequest(ts.T(), admin, http.MethodPost, path, childUnits(mandating(ts.childAOUID, privileged)))
	ts.requireCode(status, raw, http.StatusForbidden, "SAZ-4030")
	ts.requireNothingWritten(userID, privileged)

	status, raw = sendRequest(ts.T(), admin, http.MethodPost, path, childUnits(mandating(ts.childAOUID, harmless)))
	ts.Require().Equal(http.StatusCreated, status, "a role conferring nothing may be mandated: %s", raw)
	ts.Len(ts.assignments(harmless, ts.childAOUID), 1)
}

// R12: at startup, a mandate whose share is gone, or whose role no longer reaches its unit, is removed,
// and one its share still holds is kept.
func (ts *UserMandatedRolesTestSuite) TestStartupRemovesMandatesNothingHoldsAnyLonger() {
	if testutils.GetDBType() != "sqlite" {
		ts.T().Skip("the stale mandates are made by editing the SQLite configuration database")
	}
	keptRole, keptUser, _ := ts.mandatedInChildA("mandate-r12-kept")
	orphanRole, orphanUser, orphanShare := ts.mandatedInChildA("mandate-r12-orphan")
	unsharedRole := ts.newRole("Mandate Role mandate-r12-unshared", ts.parentOUID)
	rolePolicy := ts.shareRole(unsharedRole, unit(ts.childAOUID, "unit"))
	unsharedUser := ts.newUser("mandate-r12-unshared")
	ts.shareUser(unsharedUser, childUnits(mandating(ts.childAOUID, unsharedRole)))

	// Through the APIs, deleting the share takes its mandate with it and unsharing the role is refused, so the
	// stale rows the startup cleanup is for are left by editing the database.
	ts.deletePolicyRow(orphanShare.ID)
	ts.deletePolicyRow(rolePolicy.ID)
	ts.Require().NoError(testutils.RestartServer(), "restart the server")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "obtain an admin token after the restart")

	ts.Equal([]string{keptRole}, ts.mandatedBy(keptUser, ts.childAOUID, keptRole), "a held mandate is kept")
	ts.Empty(ts.mandatedBy(orphanUser, ts.childAOUID, orphanRole), "a mandate whose share is gone is removed")
	ts.Empty(ts.assignments(unsharedRole, ""), "a mandate whose role no longer reaches the unit is removed")
}
