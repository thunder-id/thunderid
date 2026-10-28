// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Selecting a child unit sends its parent along when the gateway lacks it, marked as required by the
// child; without dependencies the same selection is refused, naming the parent. Removing the parent
// while the gateway keeps the child is refused too.
func (ts *GatewayVersionsTestSuite) TestDeployingAUnitBringsItsParent() {
	parent := fmt.Sprintf("needed-%d", time.Now().UnixNano())
	parentID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: parent, Name: parent})
	ts.Require().NoError(err)
	childID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: parent + "-child", Name: parent + " child", Parent: &parentID})
	ts.Require().NoError(err)
	defer func() {
		_ = testutils.DeleteOrganizationUnit(childID)
		_ = testutils.DeleteOrganizationUnit(parentID)
	}()
	withBoth := ts.capture("a unit under another")
	ts.Require().NoError(testutils.DeleteOrganizationUnit(childID))
	ts.Require().NoError(testutils.DeleteOrganizationUnit(parentID))
	withNeither := ts.capture("without the units")
	parentKey, childKey := "organization_unit/"+parentID, "organization_unit/"+childID

	if child := ts.changeIn(ts.diff(withBoth.Version), childKey); ts.NotNil(child) {
		ts.Equal([]string{parentKey}, child.Needs, "the child does not say it needs its parent")
	}

	ts.Equal(http.StatusConflict, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": withBoth.Version, "selection": []string{childKey}, "withDependencies": false},
		nil), "a child was accepted without its parent")
	ts.Equal(http.StatusNotFound, ts.unitStatus(childID), "a refused apply reached the gateway")

	var result applyResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": withBoth.Version, "selection": []string{childKey}}, &result))
	ts.True(result.Recorded, "the apply was not recorded: %s", result.Import)
	ts.Equal(http.StatusOK, ts.unitStatus(parentID), "the parent was not sent along")
	ts.Equal(http.StatusOK, ts.unitStatus(childID), "the child was not deployed")
	if sent := ts.changeIn(result.Diff, parentKey); ts.NotNil(sent) {
		ts.True(sent.Selected)
		ts.True(sent.Required, "the parent is not marked as sent for the child")
		ts.Equal(childKey, sent.RequiredBy)
	}
	if sent := ts.changeIn(result.Diff, childKey); ts.NotNil(sent) {
		ts.True(sent.Selected)
		ts.False(sent.Required)
	}

	// The gateway keeps the child, which names the parent, so the parent cannot go alone.
	ts.Equal(http.StatusConflict, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": withNeither.Version, "selection": []string{parentKey}}, nil),
		"the parent was removed from under its child")
	ts.Equal(http.StatusOK, ts.unitStatus(parentID), "a refused removal reached the gateway")

	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": withNeither.Version, "selection": []string{parentKey, childKey}}, &result))
	ts.True(result.Recorded, "the apply was not recorded: %s", result.Import)
	ts.Equal(http.StatusNotFound, ts.unitStatus(childID))
	ts.Equal(http.StatusNotFound, ts.unitStatus(parentID))
}
