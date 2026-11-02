// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// An apply that would remove more resources than the deployment's threshold is refused until it is
// confirmed; a dry run shows the removals, and the confirmed apply removes them.
func (ts *GatewayVersionsTestSuite) TestAnApplyRemovingManyResourcesIsRefusedUntilConfirmed() {
	var ids []string
	for i := 0; i < 6; i++ {
		id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
			Handle: fmt.Sprintf("removal-%d-%d", i, time.Now().UnixNano()), Name: fmt.Sprintf("removal %d", i),
		})
		ts.Require().NoError(err)
		ids = append(ids, id)
	}
	defer func() {
		for _, id := range ids {
			_ = testutils.DeleteOrganizationUnit(id)
		}
	}()
	full := ts.capture("six units")
	ts.True(ts.apply(full.Version, false).Recorded)

	for _, id := range ids {
		ts.Require().NoError(testutils.DeleteOrganizationUnit(id))
	}
	ids = nil
	emptied := ts.capture("none of them")

	ts.Equal(http.StatusConflict, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": emptied.Version}, nil), "six removals went through unconfirmed")
	preview := ts.apply(emptied.Version, true)
	ts.Equal(6, ts.countChanges(preview.Diff, "deleted"))

	var confirmed applyResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": emptied.Version, "confirmRemovals": true}, &confirmed))
	ts.True(confirmed.Recorded)
	ts.Equal(6, ts.countChanges(confirmed.Diff, "deleted"))
}

// countChanges counts a diff's changes of one kind.
func (ts *GatewayVersionsTestSuite) countChanges(diff diffResult, kind string) int {
	count := 0
	for _, c := range diff.Changes {
		if c.Change == kind {
			count++
		}
	}
	return count
}

// An apply reports the values the version refers to that were set on the gateway after the version
// it holds was applied, by name and with what each is for, never by value.
func (ts *GatewayVersionsTestSuite) TestAnApplyReportsAValueSetSinceTheLastApply() {
	id, _, value := ts.referringUnit()
	base := "/gateways/" + ts.gatewayID + "/variables"
	defer ts.call(http.MethodDelete, base+"/"+value, nil, nil)
	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, base,
		map[string]any{"name": value, "value": "first"}, nil))
	captured := ts.capture("referring, held")
	ts.True(ts.apply(captured.Version, false).Recorded)

	// The store keeps when a value was set to the second, and the control plane records when it applied
	// the same way, so the change is set again until it lands in a later second than the apply did.
	var again struct {
		Changed struct {
			Variables  []string         `json:"variables"`
			References []valueReference `json:"references"`
		} `json:"changed"`
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		ts.Require().Equal(http.StatusOK, ts.call(http.MethodPut, base+"/"+value, map[string]any{"value": "second"}, nil))
		ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
			map[string]any{"version": captured.Version, "dryRun": true}, &again))
		if slices.Contains(again.Changed.Variables, value) || time.Now().After(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	ts.Contains(again.Changed.Variables, value)
	reference := ts.referenceTo(again.Changed.References, value)
	ts.Require().NotNil(reference, "the changed value was not said to be for anything")
	ts.Equal(id, reference.ResourceID)
	ts.Equal("description", reference.Field)
}
