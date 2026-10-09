// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// Declarative fixtures: resources/declarative_resources/resource_servers/resource-declarative-shared.yaml
	// shares decl-shared-rs from decl-m2m-root with child A (the orders resource and reports:view),
	// with child B (everything except audit and orders:delete), and, through child A's reshare, with
	// grandchild A (orders:read alone).
	sharedResourceServerID = "decl-shared-rs"
	sharedRSOwnerOUID      = "decl-m2m-root"
	sharedRSChildAOUID     = "decl-m2m-child-a"
	sharedRSChildBOUID     = "decl-m2m-child-b"
	sharedRSGrandchildOUID = "decl-m2m-grandchild-a"
	// Reached by child B's reshare, which names no rule of its own.
	sharedRSGrandchildBOUID = "decl-m2m-grandchild-b"
	sharedRSUnreachedOUID   = "decl-ou-1"

	invalidPermissionsCode = "ROL-1012"
)

// ResourceServerSharingTestSuite covers what sharing a resource server means for role management:
// an organization unit may put a resource server's permissions on its roles only when it owns the
// server or a sharing policy reaching it shares those permissions.
type ResourceServerSharingTestSuite struct {
	suite.Suite
	roleIDs []string
}

func TestResourceServerSharingTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerSharingTestSuite))
}

func (suite *ResourceServerSharingTestSuite) TearDownTest() {
	for _, id := range suite.roleIDs {
		_ = testutils.DeleteRole(id)
	}
	suite.roleIDs = nil
}

// createRole creates a role in ouID carrying permissions of the shared resource server.
func (suite *ResourceServerSharingTestSuite) createRole(name, ouID string, permissions ...string) (string, error) {
	id, err := testutils.CreateRole(testutils.Role{
		Name: name,
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: sharedResourceServerID, Permissions: permissions},
		},
	})
	if err == nil {
		suite.roleIDs = append(suite.roleIDs, id)
	}
	return id, err
}

func (suite *ResourceServerSharingTestSuite) requireAccepted(ouID string, permissions ...string) {
	suite.T().Helper()
	_, err := suite.createRole(fmt.Sprintf("rs-sharing-%s-%d", ouID, len(suite.roleIDs)), ouID, permissions...)
	suite.Require().NoError(err, "role in %s with %v should be accepted", ouID, permissions)
}

func (suite *ResourceServerSharingTestSuite) requireRefused(ouID string, permissions ...string) {
	suite.T().Helper()
	_, err := suite.createRole(fmt.Sprintf("rs-sharing-refused-%s", ouID), ouID, permissions...)
	suite.Require().Error(err, "role in %s with %v should be refused", ouID, permissions)
	suite.Contains(err.Error(), invalidPermissionsCode)
}

// The owning organization unit may use every permission, including the ones it withholds from
// others.
func (suite *ResourceServerSharingTestSuite) TestOwnerMayUseEveryPermission() {
	suite.requireAccepted(sharedRSOwnerOUID,
		"orders", "orders:delete", "reports:export", "audit", "audit:read")
}

// A value shares the paths it names and everything beneath them, and nothing else.
func (suite *ResourceServerSharingTestSuite) TestValueSharesWhatItNames() {
	suite.requireAccepted(sharedRSChildAOUID, "orders", "orders:read", "orders:write", "orders:delete")
	suite.requireAccepted(sharedRSChildAOUID, "reports:view")

	suite.requireRefused(sharedRSChildAOUID, "reports:export")
	suite.requireRefused(sharedRSChildAOUID, "reports")
	suite.requireRefused(sharedRSChildAOUID, "audit:read")
}

// Excluded values withhold the paths they name and everything beneath them, sharing the rest.
func (suite *ResourceServerSharingTestSuite) TestExcludedValuesWithholdWhatTheyName() {
	suite.requireAccepted(sharedRSChildBOUID, "orders", "orders:read", "orders:write", "reports", "reports:export")

	suite.requireRefused(sharedRSChildBOUID, "orders:delete")
	suite.requireRefused(sharedRSChildBOUID, "audit")
	suite.requireRefused(sharedRSChildBOUID, "audit:read")
}

// A reshare can only narrow: grandchild A gets what child A chose to pass on, nothing more.
func (suite *ResourceServerSharingTestSuite) TestReshareNarrowsWhatTheResharerHolds() {
	suite.requireAccepted(sharedRSGrandchildOUID, "orders:read")

	suite.requireRefused(sharedRSGrandchildOUID, "orders:write")
	suite.requireRefused(sharedRSGrandchildOUID, "reports:view")
}

// A reshare naming no rule still hands on no more than the resharer holds. Grandchild B gets child
// B's bound, not the whole server that the rule's default would otherwise read as.
func (suite *ResourceServerSharingTestSuite) TestReshareWithoutRuleKeepsTheResharersBound() {
	suite.requireAccepted(sharedRSGrandchildBOUID, "orders:read", "orders:write", "reports:export")

	suite.requireRefused(sharedRSGrandchildBOUID, "orders:delete")
	suite.requireRefused(sharedRSGrandchildBOUID, "audit:read")
}

// Entitlement is strict: a unit the resource server was never shared with may use none of it.
func (suite *ResourceServerSharingTestSuite) TestUnreachedUnitMayUseNothing() {
	suite.requireRefused(sharedRSUnreachedOUID, "orders:read")
}

// A resource server no policy names is open to every organization unit. Until resource servers can
// be shared through the API, one created there can never be given a policy, so it keeps working
// across organization units as it did before sharing existed.
func (suite *ResourceServerSharingTestSuite) TestServerWithoutPolicyIsOpenToEveryUnit() {
	rsID, err := testutils.CreateSystemScopedResourceServer(sharedRSOwnerOUID,
		"RS Sharing Unshared Server", "https://localhost:8090/rs-sharing-unshared", "reports")
	suite.Require().NoError(err)
	defer func() {
		suite.NoError(testutils.DeleteResourceServerWithChildren(rsID))
	}()

	for _, ouID := range []string{sharedRSChildAOUID, sharedRSUnreachedOUID} {
		roleID, err := testutils.CreateRole(testutils.Role{
			Name: "rs-sharing-unshared-" + ouID,
			OUID: ouID,
			Permissions: []testutils.ResourcePermissions{
				{ResourceServerID: rsID, Permissions: []string{"system:reports:view"}},
			},
		})
		suite.Require().NoError(err, "a server with no policy is usable in %s", ouID)
		suite.roleIDs = append(suite.roleIDs, roleID)
	}
}

// One refused permission refuses the whole role, so nothing outside the unit's set slips in beside
// permissions it does hold.
func (suite *ResourceServerSharingTestSuite) TestOneUnavailablePermissionRefusesTheRole() {
	suite.requireRefused(sharedRSChildAOUID, "orders:read", "audit:read")
}

// An update is held to the same bound as a create.
func (suite *ResourceServerSharingTestSuite) TestUpdateIsBoundedToo() {
	roleID, err := suite.createRole("rs-sharing-update", sharedRSChildAOUID, "orders:read")
	suite.Require().NoError(err)

	err = testutils.UpdateRole(roleID, testutils.Role{
		Name: "rs-sharing-update",
		OUID: sharedRSChildAOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: sharedResourceServerID, Permissions: []string{"orders:read", "audit:read"}},
		},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), invalidPermissionsCode)

	err = testutils.UpdateRole(roleID, testutils.Role{
		Name: "rs-sharing-update",
		OUID: sharedRSChildAOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: sharedResourceServerID, Permissions: []string{"orders:write", "reports:view"}},
		},
	})
	suite.Require().NoError(err)
}
