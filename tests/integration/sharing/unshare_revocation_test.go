// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	unshareUserType     = "unshare-revocation-person"
	unsharePassword     = "UnshareRevocation@123" // #nosec G101 -- test fixture
	unshareObserverID   = "unshare_revocation_observer"
	unshareObserverPass = "unshare_revocation_observer_secret" // #nosec G101 -- test fixture
	unshareResourceID   = "https://unshare-revocation.test"
	unsharePermission   = "unshare-permit"
)

// UnshareRevocationTestSuite takes a shared resource away from an organization unit and checks that what was
// issued for it there stops working, while what was issued in the units that keep it does not. Introspection
// reads the deny list directly, so each answer is immediate.
type UnshareRevocationTestSuite struct {
	suite.Suite
	parentOUID string
	childAOUID string
	childBOUID string
	// grandchildOUID sits beneath child A, which may reshare into it.
	grandchildOUID string
	typeID         string
	observerID     string
	signIn         testutils.OUSignInApps
	cleanups       []func()
}

func TestUnshareRevocationTestSuite(t *testing.T) {
	suite.Run(t, new(UnshareRevocationTestSuite))
}

func (ts *UnshareRevocationTestSuite) SetupSuite() {
	ts.parentOUID = ts.createOU("unshare-revocation-parent", nil)
	ts.childAOUID = ts.createOU("unshare-revocation-child-a", &ts.parentOUID)
	ts.childBOUID = ts.createOU("unshare-revocation-child-b", &ts.parentOUID)
	ts.grandchildOUID = ts.createOU("unshare-revocation-grandchild", &ts.childAOUID)

	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      unshareUserType,
		DisplayName: "Unshare Revocation Person",
		OUID:        ts.parentOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create user type")
	ts.typeID = typeID

	ts.observerID = ts.createApp(ts.parentOUID, unshareObserverID, unshareObserverPass, false)
}

func (ts *UnshareRevocationTestSuite) TearDownSuite() {
	for i := len(ts.cleanups) - 1; i >= 0; i-- {
		ts.cleanups[i]()
	}
	ts.NoError(ts.signIn.DeleteAll())
	if ts.observerID != "" {
		ts.NoError(testutils.DeleteApplication(ts.observerID))
	}
	if ts.typeID != "" {
		ts.NoError(testutils.DeleteUserType(ts.typeID))
	}
	for _, ouID := range []string{ts.grandchildOUID, ts.childAOUID, ts.childBOUID, ts.parentOUID} {
		if ouID != "" {
			ts.NoError(testutils.DeleteOrganizationUnit(ouID))
		}
	}
}

func (ts *UnshareRevocationTestSuite) TestUnsharingAUserRevokesItsTokensOnlyInTheUnitsThatLoseIt() {
	const username = "unshare.revocation.user"
	userID := ts.createUser(username)
	policyID := ts.share("/users/"+userID, ts.childAOUID, ts.childBOUID)

	tokenInA := ts.signInAs(ts.childAOUID, username)
	tokenInB := ts.signInAs(ts.childBOUID, username)
	ts.Require().True(ts.active(tokenInA), "the token issued in A is active before the unshare")
	ts.Require().True(ts.active(tokenInB), "the token issued in B is active before the unshare")

	ts.setUnits("/users/"+userID, policyID, ts.childBOUID)

	ts.False(ts.active(tokenInA), "the token issued in the unit that lost the user still works")
	ts.True(ts.active(tokenInB), "the token issued in the unit that keeps the user was revoked")

	ts.deletePolicy("/users/"+userID, policyID)

	ts.False(ts.active(tokenInB), "the token issued in the last unit still works after the share is deleted")
}

func (ts *UnshareRevocationTestSuite) TestUnsharingAUserRevokesItsTokensInTheUnitsAReshareLosesWithIt() {
	const username = "unshare.revocation.reshared"
	userID := ts.createUser(username)
	policyID := ts.share("/users/"+userID, ts.childAOUID)
	reshare := ts.namedUnits(0, ts.grandchildOUID)
	reshare["initiatingOuId"] = ts.childAOUID
	ts.send(http.MethodPost, "/users/"+userID+"/sharing-policies", http.StatusCreated, reshare)

	tokenInGrandchild := ts.signInAs(ts.grandchildOUID, username)

	ts.deletePolicy("/users/"+userID, policyID)

	ts.False(ts.active(tokenInGrandchild), "the token issued in the unit the reshare reached still works")
}

func (ts *UnshareRevocationTestSuite) TestUnsharingAnApplicationRevokesItsTokensOnlyInTheUnitsThatLoseIt() {
	const clientID, secret = "unshare_revocation_m2m", "unshare_revocation_m2m_secret" // #nosec G101 -- test fixture
	appID := ts.createApp(ts.parentOUID, clientID, secret, true)
	ts.cleanups = append(ts.cleanups, func() { ts.NoError(testutils.DeleteApplication(appID)) })
	policyID := ts.share("/applications/"+appID, ts.childAOUID, ts.childBOUID)

	tokenInA := ts.clientToken(ts.childAOUID, clientID, secret)
	tokenInB := ts.clientToken(ts.childBOUID, clientID, secret)
	tokenInOwner := ts.clientToken(ts.parentOUID, clientID, secret)

	ts.setUnits("/applications/"+appID, policyID, ts.childBOUID)

	ts.False(ts.active(tokenInA), "the token issued in the unit that lost the application still works")
	ts.True(ts.active(tokenInB), "the token issued in the unit that keeps the application was revoked")
	ts.True(ts.active(tokenInOwner), "the token issued in the owning unit was revoked")
}

func (ts *UnshareRevocationTestSuite) TestUnsharingARoleRevokesItsScopesInTheUnitThatLosesIt() {
	const clientID, secret = "unshare_revocation_role_holder", "unshare_revocation_role_holder_secret" // #nosec G101
	role := ts.createScopedRole("Unshare Revocation", unshareResourceID, unsharePermission)
	policyID := ts.share("/roles/"+role.id, ts.childAOUID)
	token := ts.roleHolderToken(ts.childAOUID, clientID, secret, role)
	unscoped := ts.clientToken("", clientID, secret)

	ts.deletePolicy("/roles/"+role.id, policyID)

	ts.False(ts.active(token), "the token carrying the scope the unit lost still works")
	ts.True(ts.active(unscoped), "a token carrying none of the role's scopes was revoked")
}

func (ts *UnshareRevocationTestSuite) TestDroppingAMandatedRoleRevokesItsScopesInThatUnit() {
	const username = "unshare.revocation.mandated"
	role := ts.createScopedRole("Unshare Mandated", "https://unshare-mandate.test", "mandate-permit")
	ts.share("/roles/"+role.id, ts.childAOUID)
	userID := ts.createUser(username)
	policyID, version := ts.shareWithMandatedRole(userID, role.id)
	scoped := ts.scopedSignIn(username, role)
	unscoped := ts.signInAs(ts.childAOUID, username)

	ts.send(http.MethodPut, "/users/"+userID+"/sharing-policies/"+policyID, http.StatusOK,
		ts.namedUnits(version, ts.childAOUID))

	ts.False(ts.active(scoped), "the token carrying the dropped role's permission still works")
	ts.True(ts.active(unscoped), "a token in the unit carrying none of the role's permissions was revoked")
}

func (ts *UnshareRevocationTestSuite) TestAnImportThatUnsharesAUserRevokesItsTokensInTheUnitsItLoses() {
	const username = "unshare.revocation.imported"
	userID := ts.createUser(username)
	ts.share("/users/"+userID, ts.childAOUID, ts.childBOUID)
	tokenInA := ts.signInAs(ts.childAOUID, username)
	tokenInB := ts.signInAs(ts.childBOUID, username)
	document := fmt.Sprintf("resource_type: user\nid: %s\ntype: %s\nouId: %s\nattributes:\n  username: %s\n",
		userID, unshareUserType, ts.parentOUID, username)

	ts.importDocument(document+fmt.Sprintf("sharingPolicies:\n  - childUnits:\n      mode: selected\n"+
		"      units:\n        - ouId: %s\n          reach: unit\n", ts.childBOUID), nil)

	ts.False(ts.active(tokenInA), "the token issued in the unit the import took the user from still works")
	ts.True(ts.active(tokenInB), "the token issued in the unit the import kept was revoked")

	ts.importDocument(document, []map[string]string{{"resourceType": "user", "id": userID}})

	ts.False(ts.active(tokenInB), "the token issued in the unit of the share the import removed still works")
}

func (ts *UnshareRevocationTestSuite) TestARefusedUnshareRevokesNothing() {
	const username = "unshare.revocation.refused"
	userID := ts.createUser(username)
	policyID := ts.share("/users/"+userID, ts.childAOUID, ts.childBOUID)
	tokenInA := ts.signInAs(ts.childAOUID, username)

	ts.send(http.MethodPut, "/users/"+userID+"/sharing-policies/"+policyID, http.StatusConflict,
		ts.namedUnits(99, ts.childBOUID))

	ts.True(ts.active(tokenInA), "a refused unshare revoked the token")
}

func (ts *UnshareRevocationTestSuite) TestUnsharingAUserKeepsItsTokensInTheOwningUnitAndOtherUsersTokens() {
	const username, neighbour = "unshare.revocation.kept", "unshare.revocation.neighbour"
	userID := ts.createUser(username)
	neighbourID := ts.createUser(neighbour)
	policyID := ts.share("/users/"+userID, ts.childAOUID)
	ts.share("/users/"+neighbourID, ts.childAOUID)
	tokenInA := ts.signInAs(ts.childAOUID, username)
	tokenInOwner := ts.signInAs(ts.parentOUID, username)
	neighbourInA := ts.signInAs(ts.childAOUID, neighbour)

	ts.deletePolicy("/users/"+userID, policyID)

	ts.Require().False(ts.active(tokenInA), "the token issued in the unit that lost the user still works")
	ts.True(ts.active(tokenInOwner), "the user's token in its owning unit was revoked")
	ts.True(ts.active(neighbourInA), "another user's token in the unit that lost the user was revoked")
}

func (ts *UnshareRevocationTestSuite) TestATokenIssuedAfterTheUserIsSharedBackWorks() {
	const username = "unshare.revocation.returning"
	userID := ts.createUser(username)
	policyID := ts.share("/users/"+userID, ts.childAOUID)
	before := ts.signInAs(ts.childAOUID, username)
	ts.deletePolicy("/users/"+userID, policyID)
	ts.Require().False(ts.active(before), "the token issued before the unshare still works")

	waitForNextSecond()
	ts.share("/users/"+userID, ts.childAOUID)
	after := ts.signInAs(ts.childAOUID, username)

	ts.True(ts.active(after), "a token issued after the user was shared back was revoked")
	ts.False(ts.active(before), "sharing the user back restored the token the unshare revoked")
}

func (ts *UnshareRevocationTestSuite) TestAnEditThatOnlyWidensAShareRevokesNothing() {
	const username = "unshare.revocation.widened"
	userID := ts.createUser(username)
	policyID := ts.share("/users/"+userID, ts.childAOUID)
	tokenInA := ts.signInAs(ts.childAOUID, username)

	ts.setUnits("/users/"+userID, policyID, ts.childAOUID, ts.childBOUID)

	ts.True(ts.active(tokenInA), "widening the share revoked the token in the unit it already reached")
}

func (ts *UnshareRevocationTestSuite) TestUnsharingAnApplicationKeepsOtherClientsTokensInThatUnit() {
	const clientID, secret = "unshare_revocation_lone_m2m", "unshare_revocation_lone_m2m_secret" // #nosec G101
	const username = "unshare.revocation.app.neighbour"
	appID := ts.createApp(ts.parentOUID, clientID, secret, true)
	ts.cleanups = append(ts.cleanups, func() { ts.NoError(testutils.DeleteApplication(appID)) })
	policyID := ts.share("/applications/"+appID, ts.childAOUID)
	userID := ts.createUser(username)
	ts.share("/users/"+userID, ts.childAOUID)
	appToken := ts.clientToken(ts.childAOUID, clientID, secret)
	userToken := ts.signInAs(ts.childAOUID, username)

	ts.deletePolicy("/applications/"+appID, policyID)

	ts.Require().False(ts.active(appToken), "the application's token in the unit that lost it still works")
	ts.True(ts.active(userToken), "a token another client issued in the unit that lost the application was revoked")
}

func (ts *UnshareRevocationTestSuite) TestUnsharingARoleKeepsItsScopesInTheUnitsThatKeepIt() {
	const clientA, secretA = "unshare_revocation_kept_a", "unshare_revocation_kept_a_secret" // #nosec G101
	const clientB, secretB = "unshare_revocation_kept_b", "unshare_revocation_kept_b_secret" // #nosec G101
	role := ts.createScopedRole("Unshare Kept", "https://unshare-role-kept.test", "kept-permit")
	policyID := ts.share("/roles/"+role.id, ts.childAOUID, ts.childBOUID)
	tokenInA := ts.roleHolderToken(ts.childAOUID, clientA, secretA, role)
	tokenInB := ts.roleHolderToken(ts.childBOUID, clientB, secretB, role)

	ts.setUnits("/roles/"+role.id, policyID, ts.childBOUID)

	ts.Require().False(ts.active(tokenInA), "the holder's token in the unit that lost the role still works")
	ts.True(ts.active(tokenInB), "the holder's token in the unit that keeps the role was revoked")
}

func (ts *UnshareRevocationTestSuite) TestDroppingAMandatedRoleKeepsItsScopesForADirectHolder() {
	const username = "unshare.revocation.direct"
	role := ts.createScopedRole("Unshare Direct", "https://unshare-mandate-direct.test", "direct-permit")
	ts.share("/roles/"+role.id, ts.childAOUID)
	userID := ts.createUser(username)
	policyID, version := ts.shareWithMandatedRole(userID, role.id)
	ts.send(http.MethodPost, "/roles/"+role.id+"/assignments/add", http.StatusNoContent, map[string]interface{}{
		"ouId": ts.childAOUID, "assignments": []map[string]string{{"id": userID, "type": "user"}},
	})
	scoped := ts.scopedSignIn(username, role)

	ts.send(http.MethodPut, "/users/"+userID+"/sharing-policies/"+policyID, http.StatusOK,
		ts.namedUnits(version, ts.childAOUID))

	ts.True(ts.active(scoped), "dropping the mandate revoked a permission the user still holds directly")
}

// scopedRole is a role owned by the parent unit granting one permission on a resource server of its own.
type scopedRole struct {
	id         string
	resourceID string
	permission string
}

// createScopedRole creates a role granting permission on a new resource server identified by resourceID.
func (ts *UnshareRevocationTestSuite) createScopedRole(name, resourceID, permission string) scopedRole {
	ts.T().Helper()
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: name + " API", Identifier: resourceID, OUID: ts.parentOUID,
	}, nil)
	ts.Require().NoError(err, "create resource server")
	ts.cleanups = append(ts.cleanups, func() { ts.NoError(testutils.DeleteResourceServer(rsID)) })
	ts.Require().NoError(testutils.ShareResourceServerWithChildUnits(rsID), "share resource server")
	permissionID, err := testutils.CreateResource(rsID, permission, permission, "")
	ts.Require().NoError(err, "create resource")
	ts.cleanups = append(ts.cleanups, func() { ts.NoError(testutils.DeleteResource(rsID, permissionID)) })
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: name + " Role", OUID: ts.parentOUID,
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: rsID, Permissions: []string{permission}}},
	})
	ts.Require().NoError(err, "create role")
	ts.cleanups = append(ts.cleanups, func() { ts.NoError(testutils.DeleteRole(roleID)) })
	return scopedRole{id: roleID, resourceID: resourceID, permission: permission}
}

// roleHolderToken creates an application in ouID holding the role there and returns its token carrying the
// role's permission.
func (ts *UnshareRevocationTestSuite) roleHolderToken(ouID, clientID, secret string, role scopedRole) string {
	ts.T().Helper()
	holderID := ts.createApp(ouID, clientID, secret, true)
	ts.cleanups = append(ts.cleanups, func() { ts.NoError(testutils.DeleteApplication(holderID)) })
	ts.send(http.MethodPost, "/roles/"+role.id+"/assignments/add", http.StatusNoContent, map[string]interface{}{
		"ouId": ouID, "assignments": []map[string]string{{"id": holderID, "type": "app"}},
	})
	result, err := testutils.RequestClientCredentialsToken(clientID, secret, role.permission, role.resourceID)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "token response: %s", string(result.Body))
	ts.Require().Contains(strings.Fields(result.Token.Scope), role.permission)
	ts.Require().True(ts.active(result.Token.AccessToken), "the token carrying the role's scope is active")
	return result.Token.AccessToken
}

// shareWithMandatedRole shares the user into child A mandating the role there, and returns the policy's id and
// version.
func (ts *UnshareRevocationTestSuite) shareWithMandatedRole(userID, roleID string) (string, int) {
	ts.T().Helper()
	raw := ts.send(http.MethodPost, "/users/"+userID+"/sharing-policies", http.StatusCreated, map[string]interface{}{
		"childUnits": map[string]interface{}{"mode": "selected", "units": []map[string]interface{}{{
			"ouId": ts.childAOUID, "reach": "unit",
			"overlayRules": map[string]interface{}{
				"roles": map[string]interface{}{"editable": false, "value": []string{roleID}},
			},
		}}},
	})
	var created struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &created), string(raw))
	return created.ID, created.Version
}

// scopedSignIn signs the user in to child A asking for the role's permission, and requires it granted.
func (ts *UnshareRevocationTestSuite) scopedSignIn(username string, role scopedRole) string {
	ts.T().Helper()
	token, err := ts.signIn.ObtainAccessToken(ts.childAOUID, "openid "+role.permission, username, unsharePassword,
		role.resourceID)
	ts.Require().NoError(err, "sign in with the role's permission")
	ts.Require().Contains(strings.Fields(fmt.Sprint(ts.claim(token.AccessToken, "scope"))), role.permission,
		"the role grants its permission in the unit")
	ts.Require().True(ts.active(token.AccessToken))
	return token.AccessToken
}

// waitForNextSecond returns once the current second has passed: a revocation covers what was issued up to and
// including the second it was recorded in.
func waitForNextSecond() {
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
}

func (ts *UnshareRevocationTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: handle, Name: handle, Parent: parent})
	ts.Require().NoError(err, "create organization unit %s", handle)
	return id
}

func (ts *UnshareRevocationTestSuite) createUser(username string) string {
	ts.T().Helper()
	id, err := testutils.CreateUser(testutils.User{
		Type: unshareUserType, OUID: ts.parentOUID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`, username, unsharePassword)),
	})
	ts.Require().NoError(err, "create user %s", username)
	ts.cleanups = append(ts.cleanups, func() { ts.NoError(testutils.DeleteUser(id)) })
	return id
}

// createApp creates a client credentials application owned by ouID, stating the unit in its tokens when asked.
func (ts *UnshareRevocationTestSuite) createApp(ouID, clientID, secret string, statesUnit bool) string {
	ts.T().Helper()
	config := map[string]interface{}{
		"clientId":                clientID,
		"clientSecret":            secret,
		"grantTypes":              []string{"client_credentials"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}
	if statesUnit {
		config["token"] = map[string]interface{}{
			"accessToken": map[string]interface{}{"clientConfig": map[string]interface{}{"attributes": []string{"ouId"}}},
		}
	}
	id, err := testutils.CreateApplication(testutils.Application{
		Name: clientID, OUID: ouID, InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": config}},
	})
	ts.Require().NoError(err, "create application %s", clientID)
	return id
}

func (ts *UnshareRevocationTestSuite) namedUnits(version int, ouIDs ...string) map[string]interface{} {
	units := make([]map[string]string, 0, len(ouIDs))
	for _, ouID := range ouIDs {
		units = append(units, map[string]string{"ouId": ouID, "reach": "unit"})
	}
	body := map[string]interface{}{"childUnits": map[string]interface{}{"mode": "selected", "units": units}}
	if version > 0 {
		body["version"] = version
	}
	return body
}

// share shares the resource at path into the named child units and returns the policy's id.
func (ts *UnshareRevocationTestSuite) share(path string, ouIDs ...string) string {
	ts.T().Helper()
	raw := ts.send(http.MethodPost, path+"/sharing-policies", http.StatusCreated, ts.namedUnits(0, ouIDs...))
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &created), string(raw))
	return created.ID
}

// setUnits edits the policy to name only the given child units.
func (ts *UnshareRevocationTestSuite) setUnits(path, policyID string, ouIDs ...string) {
	ts.T().Helper()
	raw := ts.send(http.MethodGet, path+"/sharing-policies/"+policyID, http.StatusOK, nil)
	var current struct {
		Version int `json:"version"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &current), string(raw))
	ts.send(http.MethodPut, path+"/sharing-policies/"+policyID, http.StatusOK, ts.namedUnits(current.Version, ouIDs...))
}

func (ts *UnshareRevocationTestSuite) deletePolicy(path, policyID string) {
	ts.T().Helper()
	ts.send(http.MethodDelete, path+"/sharing-policies/"+policyID, http.StatusNoContent, nil)
}

// send makes an administrator request and requires the given status.
func (ts *UnshareRevocationTestSuite) send(method, path string, status int, body interface{}) []byte {
	ts.T().Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(status, resp.StatusCode, "%s %s: %s", method, path, string(raw))
	return raw
}

// importDocument upserts the document through the import API, removing the named sharing policies, and requires
// every item to succeed.
func (ts *UnshareRevocationTestSuite) importDocument(content string, sharingRemovals []map[string]string) {
	ts.T().Helper()
	body := map[string]interface{}{
		"content": content, "options": map[string]interface{}{"upsert": true, "continueOnError": false},
	}
	if sharingRemovals != nil {
		body["sharingRemovals"] = sharingRemovals
	}
	raw := ts.send(http.MethodPost, "/import", http.StatusOK, body)
	var result struct {
		Summary struct {
			Failed int `json:"failed"`
		} `json:"summary"`
		Results []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &result), string(raw))
	ts.Require().Zero(result.Summary.Failed, string(raw))
	ts.Require().NotEmpty(result.Results, string(raw))
	for _, item := range result.Results {
		ts.Require().Equal("success", item.Status, string(raw))
	}
}

// signInAs signs the user in through ouID's sign-in application, whose tokens state the unit.
func (ts *UnshareRevocationTestSuite) signInAs(ouID, username string) string {
	ts.T().Helper()
	token, err := ts.signIn.ObtainAccessToken(ouID, "openid", username, unsharePassword, "")
	ts.Require().NoError(err, "sign %s in to %s", username, ouID)
	ts.Require().Equal(ouID, ts.claim(token.AccessToken, "ouId"), "the token states the unit it was issued in")
	return token.AccessToken
}

// clientToken requests a client credentials token, for ouID when it is set.
func (ts *UnshareRevocationTestSuite) clientToken(ouID, clientID, secret string) string {
	ts.T().Helper()
	result, err := testutils.RequestOUClientCredentialsToken(ouID, clientID, secret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "token for %s in %s: %s", clientID, ouID,
		string(result.Body))
	ts.Require().True(ts.active(result.Token.AccessToken), "the token is active when issued")
	return result.Token.AccessToken
}

func (ts *UnshareRevocationTestSuite) active(token string) bool {
	ts.T().Helper()
	active, err := testutils.IntrospectToken(token, unshareObserverID, unshareObserverPass)
	ts.Require().NoError(err)
	return active
}

// claim reads one claim from a JWT's payload.
func (ts *UnshareRevocationTestSuite) claim(token, name string) interface{} {
	ts.T().Helper()
	claims, err := testutils.DecodeJWTPayloadMap(token)
	ts.Require().NoError(err)
	return claims[name]
}
