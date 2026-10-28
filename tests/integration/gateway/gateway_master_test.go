// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type masterEntry struct {
	Key           string `json:"key"`
	ResourceType  string `json:"resourceType"`
	ID            string `json:"id"`
	ContentHash   string `json:"contentHash"`
	SourceVersion string `json:"sourceVersion"`
	Origin        string `json:"origin"`
	State         string `json:"state"`
	Failure       *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"failure"`
}

type master struct {
	GatewayID string `json:"gatewayId"`
	Number    int    `json:"number"`
	Master    string `json:"master"`
	Resources string `json:"resources"`
	Summary   struct {
		Deployed int `json:"deployed"`
		Failed   int `json:"failed"`
		Removed  int `json:"removed"`
	} `json:"summary"`
	Entries []masterEntry `json:"entries"`
}

type resourceHistory struct {
	Key     string `json:"key"`
	History []struct {
		Master string      `json:"master"`
		Number int         `json:"number"`
		Entry  masterEntry `json:"entry"`
	} `json:"history"`
}

// A gateway nothing was applied to holds the empty master, and has no history.
func (ts *GatewayVersionsTestSuite) TestAGatewayNothingWasAppliedToHoldsAnEmptyMaster() {
	held := ts.master()
	ts.Equal(ts.gatewayID, held.GatewayID)
	ts.Zero(held.Number)
	ts.Empty(held.Master)
	ts.Empty(held.Entries)

	var history []master
	ts.Equal(http.StatusOK, ts.call(http.MethodGet, "/gateways/"+ts.gatewayID+"/master/history", nil, &history))
	ts.Empty(history)
	ts.Equal(http.StatusNotFound, ts.call(http.MethodGet,
		"/gateways/"+ts.gatewayID+"/master/resources/organization_unit/none", nil, nil))
}

// Each apply writes the gateway's master from the import's own outcomes: a resource the gateway
// accepted is deployed from the version that carried it, one it removed is recorded removed, and one
// that did not change keeps where it came from. The resource's history is its entry across the masters.
func (ts *GatewayVersionsTestSuite) TestEachApplyRecordsTheMasterResourceByResource() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("mastered-%d", time.Now().UnixNano()), Name: "Mastered",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()
	key := "organization_unit/" + ouID

	withUnit := ts.capture("master with the unit")
	ts.Require().NoError(testutils.DeleteOrganizationUnit(ouID))
	withoutUnit := ts.capture("master without the unit")

	var first applyResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": withUnit.Version, "origin": "dev"}, &first))
	ts.Require().True(first.Recorded, "the apply was not recorded: %s", first.Import)
	ts.Require().Len(first.Master, 64, "the apply did not answer with the master")

	held := ts.master()
	ts.Equal(first.Master, held.Master)
	ts.Equal(1, held.Number)
	ts.Equal(len(held.Entries), held.Summary.Deployed+held.Summary.Failed)
	ts.Contains(held.Resources, ouID)
	if entry := ts.entryIn(held, key); ts.NotNil(entry, "the master does not record the unit") {
		ts.Equal("deployed", entry.State)
		ts.Equal(withUnit.Version, entry.SourceVersion)
		ts.Equal("dev", entry.Origin)
		ts.Len(entry.ContentHash, 64)
	}

	second := ts.apply(withoutUnit.Version, false)
	ts.Require().True(second.Recorded, "the apply was not recorded: %s", second.Import)
	ts.NotEqual(first.Master, second.Master)

	held = ts.master()
	ts.Equal(2, held.Number)
	ts.Equal(1, held.Summary.Removed)
	ts.NotContains(held.Resources, ouID, "a removed resource stayed in the master's file")
	if entry := ts.entryIn(held, key); ts.NotNil(entry) {
		ts.Equal("removed", entry.State)
		ts.Equal(withoutUnit.Version, entry.SourceVersion)
		ts.Empty(entry.ContentHash)
	}
	// A resource the gateway refuses on every apply is recorded from each; the rest did not change.
	for _, entry := range held.Entries {
		if entry.Key != key && entry.State == "deployed" {
			ts.Equal(withUnit.Version, entry.SourceVersion,
				"%s did not change, yet it was recorded from the new version", entry.Key)
		}
	}

	var history []master
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/gateways/"+ts.gatewayID+"/master/history", nil, &history))
	ts.Require().Len(history, 2)
	ts.Equal([]int{2, 1}, []int{history[0].Number, history[1].Number})
	ts.Empty(history[0].Entries, "a listing carried the entries")

	var unit resourceHistory
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet,
		"/gateways/"+ts.gatewayID+"/master/resources/"+key, nil, &unit))
	ts.Equal(key, unit.Key)
	ts.Require().Len(unit.History, 2)
	ts.Equal("removed", unit.History[0].Entry.State)
	ts.Equal(second.Master, unit.History[0].Master)
	ts.Equal("deployed", unit.History[1].Entry.State)
	ts.Equal(first.Master, unit.History[1].Master)
}

// A resource the gateway refuses is recorded failed with what the gateway answered, beside the ones
// it accepted, and the master's file holds only what the gateway accepted. The server refuses to
// rewrite a resource it declares in a file, so the declared flow is such a resource on every apply.
func (ts *GatewayVersionsTestSuite) TestARefusedResourceIsRecordedFailedBesideTheRest() {
	const declaredFlow = "flow/decl-flow-1"
	captured := ts.capture("master with the declared flow")
	ts.Require().Contains(captured.Resources, "decl-flow-1")

	result := ts.apply(captured.Version, false)
	ts.Require().True(result.Recorded, "the apply was not recorded: %s", result.Import)

	held := ts.master()
	ts.Positive(held.Summary.Failed)
	ts.Positive(held.Summary.Deployed)
	if entry := ts.entryIn(held, declaredFlow); ts.NotNil(entry, "the declared flow is not recorded: %s", result.Import) {
		ts.Equal("failed", entry.State, "the declared flow was accepted: %s", result.Import)
		ts.Equal(captured.Version, entry.SourceVersion)
		if ts.NotNil(entry.Failure, "the failure was not recorded") {
			ts.NotEmpty(entry.Failure.Code)
			ts.NotEmpty(entry.Failure.Message)
		}
		ts.Empty(entry.ContentHash, "a resource the gateway never accepted has a document")
	}
	ts.NotContains(held.Resources, "id: decl-flow-1", "a refused resource is in the master's file")
	ts.Contains(held.Resources, "resource_type: flow", "the accepted flows are not in the master's file")

	var history resourceHistory
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet,
		"/gateways/"+ts.gatewayID+"/master/resources/"+declaredFlow, nil, &history))
	ts.Require().NotEmpty(history.History)
	ts.Equal("failed", history.History[0].Entry.State)
	ts.Equal(held.Master, history.History[0].Master)
}

func (ts *GatewayVersionsTestSuite) master() master {
	ts.T().Helper()
	var held master
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/gateways/"+ts.gatewayID+"/master", nil, &held))
	return held
}

func (ts *GatewayVersionsTestSuite) entryIn(held master, key string) *masterEntry {
	for i := range held.Entries {
		if held.Entries[i].Key == key {
			return &held.Entries[i]
		}
	}
	return nil
}
