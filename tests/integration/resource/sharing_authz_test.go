// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

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
	// The console client is the one the fixtures log the sharee administrator in through.
	shareeAuthzClientID    = "CONSOLE"
	shareeAuthzRedirectURI = "https://localhost:8095/console"

	// ErrorResourceServerModificationRestrictedToOwner.
	errCodeModificationRestrictedToOwner = "RES-1024"

	shareeAuthzPassword = "ShareeAdmin@123"
)

// ShareeWriteAccessTestSuite covers what a shared resource server may not be used for.
//
// The caller here is as privileged as a tenant administrator gets: it holds system:resource-server,
// the scope that governs every write to the resource-server API, bounded to its own organization
// unit. Everything refused below is therefore refused because the server belongs to another
// organization unit, not because the token was under-scoped. Being shared a resource server conveys
// the right to use its permissions, not to change what they mean for everyone else holding them.
type ShareeWriteAccessTestSuite struct {
	suite.Suite

	ownerOUID  string // owns the resource server
	shareeOUID string // a second root, shared to

	serverID   string
	serverName string
	identifier string
	bookingsID string
	refundID   string
	actionID   string
	policyID   string

	// The sharee administrator: a user in the sharee organization unit holding
	// system:resource-server through a role owned by that same organization unit.
	scopedRSID   string
	userTypeID   string
	userID       string
	roleID       string
	shareeClient *http.Client
}

func TestShareeWriteAccessTestSuite(t *testing.T) {
	suite.Run(t, new(ShareeWriteAccessTestSuite))
}

func (suite *ShareeWriteAccessTestSuite) SetupSuite() {
	stamp := time.Now().UnixNano()

	var err error
	suite.ownerOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("sharee_authz_owner_%d", stamp),
		Name:   "Sharee Authz Owner",
	})
	suite.Require().NoError(err, "failed to create the owning organization unit")

	suite.shareeOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("sharee_authz_sharee_%d", stamp),
		Name:   "Sharee Authz Sharee",
	})
	suite.Require().NoError(err, "failed to create the sharee organization unit")

	// The server under test, owned by the owner: a small tree, so a resource and an action are
	// both available as write targets.
	suite.serverName = fmt.Sprintf("Sharee Authz Booking System %d", stamp)
	suite.identifier = fmt.Sprintf("https://sharee-authz.example.com/booking/%d", stamp)
	suite.serverID, err = createResourceServer(CreateResourceServerRequest{
		Name:       suite.serverName,
		Identifier: suite.identifier,
		OUID:       suite.ownerOUID,
	})
	suite.Require().NoError(err, "failed to create the resource server")

	suite.bookingsID, err = createResource(suite.serverID, CreateResourceRequest{
		Name: "Bookings", Handle: "bookings",
	})
	suite.Require().NoError(err)
	suite.refundID, err = createResource(suite.serverID, CreateResourceRequest{
		Name: "Refund", Handle: "refund", Parent: &suite.bookingsID,
	})
	suite.Require().NoError(err)
	suite.actionID, err = createActionAtResource(suite.serverID, suite.bookingsID, CreateActionRequest{
		Name: "Cancel", Handle: "cancel",
	})
	suite.Require().NoError(err)

	policy, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.shareeOUID}},
	})
	suite.Require().NoError(err, "failed to share the resource server")
	suite.policyID = policy.ID

	suite.setUpShareeAdministrator(stamp)
}

// setUpShareeAdministrator mints the token the refusals below are made against.
//
// The product ships only the root "system" scope, so the fine-grained one has to be declared: a
// resource server reproducing system:resource-server, a role in the sharee's own organization unit
// carrying it, and a user there to hold the role. The token's organization unit is the user's, which
// is what bounds every answer the resource-server API gives it.
func (suite *ShareeWriteAccessTestSuite) setUpShareeAdministrator(stamp int64) {
	var err error
	suite.scopedRSID, err = testutils.CreateSystemScopedResourceServer(
		suite.shareeOUID,
		fmt.Sprintf("Sharee Authz Scoped RS %d", stamp),
		fmt.Sprintf("https://sharee-authz.example.com/system/%d", stamp),
		"resource-server")
	suite.Require().NoError(err, "failed to create the scoped resource server")

	userTypeName := fmt.Sprintf("sharee_authz_schema_%d", stamp)
	suite.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Name: userTypeName,
		OUID: suite.shareeOUID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	suite.Require().NoError(err, "failed to create the user type")

	username := fmt.Sprintf("sharee_authz_admin_%d", stamp)
	suite.userID, err = testutils.CreateUser(testutils.User{
		Type: userTypeName,
		OUID: suite.shareeOUID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Sharee Administrator"}`,
			username, shareeAuthzPassword)),
	})
	suite.Require().NoError(err, "failed to create the sharee administrator")

	// Owned by the sharee's organization unit, so the assignment is recorded there and the
	// permission resolution at token issuance finds it.
	suite.roleID, err = testutils.CreateRole(testutils.Role{
		Name: fmt.Sprintf("Sharee Resource Server Admin %d", stamp),
		OUID: suite.shareeOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: suite.scopedRSID, Permissions: []string{"system:resource-server"}},
		},
		Assignments: []testutils.Assignment{{ID: suite.userID, Type: "user"}},
	})
	suite.Require().NoError(err, "failed to create the sharee administrator role")

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		shareeAuthzClientID,
		shareeAuthzRedirectURI,
		"system:resource-server",
		username,
		shareeAuthzPassword,
		true,
		"",
		fmt.Sprintf("https://sharee-authz.example.com/system/%d", stamp),
	)
	suite.Require().NoError(err, "failed to obtain the sharee administrator token")
	suite.Require().NotEmpty(tokenResp.AccessToken, "the sharee administrator token must be non-empty")

	// The scope names no organization unit; the ouId claim is what bounds it. Both have to hold for
	// the refusals below to mean anything.
	claims, err := testutils.DecodeJWT(tokenResp.AccessToken)
	suite.Require().NoError(err)
	suite.Require().Equal(suite.shareeOUID, claims.OUID, "the token must be bounded to the sharee")
	scope, _ := claims.Additional["scope"].(string)
	suite.Require().Contains(strings.Split(scope, " "), "system:resource-server",
		"the token must carry the scope the refusals are meant to survive")

	suite.shareeClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

func (suite *ShareeWriteAccessTestSuite) TearDownSuite() {
	if suite.policyID != "" {
		if err := deleteSharingPolicy(suite.serverID, suite.policyID); err != nil {
			suite.T().Logf("failed to delete the sharing policy: %v", err)
		}
	}
	if suite.roleID != "" {
		if err := testutils.DeleteRole(suite.roleID); err != nil {
			suite.T().Logf("failed to delete the sharee administrator role: %v", err)
		}
	}
	if suite.userID != "" {
		if err := testutils.DeleteUser(suite.userID); err != nil {
			suite.T().Logf("failed to delete the sharee administrator: %v", err)
		}
	}
	if suite.userTypeID != "" {
		if err := testutils.DeleteUserType(suite.userTypeID); err != nil {
			suite.T().Logf("failed to delete the user type: %v", err)
		}
	}
	if suite.scopedRSID != "" {
		if err := testutils.DeleteResourceServerWithChildren(suite.scopedRSID); err != nil {
			suite.T().Logf("failed to delete the scoped resource server: %v", err)
		}
	}
	if suite.serverID != "" {
		if err := testutils.DeleteResourceServerWithChildren(suite.serverID); err != nil {
			suite.T().Logf("failed to delete the resource server: %v", err)
		}
	}
	for _, ouID := range []string{suite.shareeOUID, suite.ownerOUID} {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			suite.T().Logf("failed to delete organization unit %s: %v", ouID, err)
		}
	}
}

// ---------------------------------------------------------------------------
// The share is real
// ---------------------------------------------------------------------------

// Reads succeed, which is what makes every refusal below a statement about ownership rather than
// about the token being unable to reach the server at all.
func (suite *ShareeWriteAccessTestSuite) TestShareeReadsTheSharedServer() {
	resp := suite.doAsSharee(http.MethodGet, "/resource-servers/"+suite.serverID, nil)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "the sharee must see the server it was shared")

	var server ResourceServerResponse
	suite.Require().NoError(json.Unmarshal([]byte(resp.Body), &server))
	suite.Equal(suite.ownerOUID, server.OUID, "the server still belongs to its owner")

	resources := suite.doAsSharee(http.MethodGet,
		fmt.Sprintf("/resource-servers/%s/resources", suite.serverID), nil)
	suite.Equal(http.StatusOK, resources.StatusCode, "the sharee must see the tree it was shared")
}

// ---------------------------------------------------------------------------
// Core configuration
// ---------------------------------------------------------------------------

// Name, identifier and organization unit are the owner's own definition of the server, and a sharee
// editing them would rename the server for everyone it is shared with.
func (suite *ShareeWriteAccessTestSuite) TestShareeCannotEditCoreConfiguration() {
	resp := suite.doAsSharee(http.MethodPut, "/resource-servers/"+suite.serverID,
		UpdateResourceServerRequest{
			Name:        suite.serverName + " (renamed by the sharee)",
			Description: "Edited by an organization unit that was only shared the server",
			Identifier:  suite.identifier + "/hijacked",
			OUID:        suite.ownerOUID,
		})
	suite.requireOwnerOnly(resp, "renaming a shared resource server")

	unchanged, err := getResourceServer(suite.serverID)
	suite.Require().NoError(err)
	suite.Equal(suite.serverName, unchanged.Name, "the name the owner chose still stands")
	suite.Equal(suite.identifier, unchanged.Identifier, "the identifier the owner chose still stands")
}

// Moving the server into the sharee's own organization unit is the same edit by another route: it
// would hand the sharee ownership of a definition it was only lent.
func (suite *ShareeWriteAccessTestSuite) TestShareeCannotMoveTheServerIntoItsOwnOU() {
	resp := suite.doAsSharee(http.MethodPut, "/resource-servers/"+suite.serverID,
		UpdateResourceServerRequest{
			Name:       suite.serverName,
			Identifier: suite.identifier,
			OUID:       suite.shareeOUID,
		})
	suite.requireOwnerOnly(resp, "moving a shared resource server into the sharee's organization unit")

	unchanged, err := getResourceServer(suite.serverID)
	suite.Require().NoError(err)
	suite.Equal(suite.ownerOUID, unchanged.OUID, "the server still belongs to its owner")
}

// ---------------------------------------------------------------------------
// Resources and actions
// ---------------------------------------------------------------------------

// The permission tree is one definition shared by every organization unit that can see it, so
// adding to it or editing it is the owner's alone.
func (suite *ShareeWriteAccessTestSuite) TestShareeCannotCreateOrEditResources() {
	suite.Run("creating a resource is refused", func() {
		resp := suite.doAsSharee(http.MethodPost,
			fmt.Sprintf("/resource-servers/%s/resources", suite.serverID),
			CreateResourceRequest{Name: "Sharee Branch", Handle: "sharee-branch"})
		suite.requireOwnerOnly(resp, "adding a resource to a shared resource server")
	})

	suite.Run("editing a resource is refused", func() {
		resp := suite.doAsSharee(http.MethodPut,
			fmt.Sprintf("/resource-servers/%s/resources/%s", suite.serverID, suite.bookingsID),
			UpdateResourceRequest{Name: "Bookings (renamed by the sharee)"})
		suite.requireOwnerOnly(resp, "editing a resource of a shared resource server")
	})

	unchanged, err := listResources(suite.serverID, "", 0, 10)
	suite.Require().NoError(err)
	suite.Len(unchanged.Resources, 1, "the tree still holds only what the owner put in it")
	suite.Equal("Bookings", unchanged.Resources[0].Name)
}

// An action is a permission the owner published; editing one changes what a permission already
// granted elsewhere means.
func (suite *ShareeWriteAccessTestSuite) TestShareeCannotCreateOrEditActions() {
	suite.Run("creating an action is refused", func() {
		resp := suite.doAsSharee(http.MethodPost,
			fmt.Sprintf("/resource-servers/%s/resources/%s/actions", suite.serverID, suite.bookingsID),
			CreateActionRequest{Name: "Approve", Handle: "approve"})
		suite.requireOwnerOnly(resp, "adding an action to a shared resource server")
	})

	suite.Run("editing an action is refused", func() {
		resp := suite.doAsSharee(http.MethodPut,
			fmt.Sprintf("/resource-servers/%s/resources/%s/actions/%s",
				suite.serverID, suite.bookingsID, suite.actionID),
			UpdateActionRequest{Name: "Cancel (renamed by the sharee)"})
		suite.requireOwnerOnly(resp, "editing an action of a shared resource server")
	})
}

// ---------------------------------------------------------------------------
// Deletion
// ---------------------------------------------------------------------------

// A delete is the widest edit there is: it would remove the server, or part of its tree, from every
// organization unit holding it, not only from the one asking.
func (suite *ShareeWriteAccessTestSuite) TestShareeCannotDeleteTheServerOrItsTree() {
	suite.Run("deleting an action is refused", func() {
		resp := suite.doAsSharee(http.MethodDelete,
			fmt.Sprintf("/resource-servers/%s/resources/%s/actions/%s",
				suite.serverID, suite.bookingsID, suite.actionID), nil)
		suite.requireOwnerOnly(resp, "deleting an action of a shared resource server")
	})

	// The leaf of the tree, so the refusal is about ownership and not about dependants.
	suite.Run("deleting a resource is refused", func() {
		resp := suite.doAsSharee(http.MethodDelete,
			fmt.Sprintf("/resource-servers/%s/resources/%s", suite.serverID, suite.refundID), nil)
		suite.requireOwnerOnly(resp, "deleting a resource of a shared resource server")
	})

	suite.Run("deleting the server is refused", func() {
		resp := suite.doAsSharee(http.MethodDelete, "/resource-servers/"+suite.serverID, nil)
		suite.requireOwnerOnly(resp, "deleting a shared resource server")
	})

	survived, err := getResourceServer(suite.serverID)
	suite.Require().NoError(err, "the server outlives every attempt to delete it")
	suite.Equal(suite.serverName, survived.Name)

	actions, err := listActionsAtResource(suite.serverID, suite.bookingsID, 0, 10)
	suite.Require().NoError(err)
	suite.Len(actions.Actions, 1, "the action outlives the attempt to delete it")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// requireOwnerOnly asserts a write was refused as one the owning organization unit alone may make,
// rather than as a missing scope or a server the caller cannot see.
func (suite *ShareeWriteAccessTestSuite) requireOwnerOnly(resp *rawResponse, what string) {
	suite.T().Helper()

	suite.Require().Equal(http.StatusForbidden, resp.StatusCode,
		"%s must be refused, body: %s", what, resp.Body)
	suite.Equal(errCodeModificationRestrictedToOwner, resp.Error.Code,
		"%s must be refused as an ownership failure", what)
}

// doAsSharee sends a request carrying the sharee administrator's token.
func (suite *ShareeWriteAccessTestSuite) doAsSharee(
	method, path string, payload interface{},
) *rawResponse {
	suite.T().Helper()

	var reader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		suite.Require().NoError(err)
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, testServerURL+path, reader)
	suite.Require().NoError(err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := suite.shareeClient.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	result := &rawResponse{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	_ = json.Unmarshal(bodyBytes, &result.Error)
	return result
}
