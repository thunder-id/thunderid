// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type masterEntry struct {
	Key           string `json:"key"`
	SourceVersion string `json:"sourceVersion"`
	Origin        string `json:"origin"`
	State         string `json:"state"`
	ContentHash   string `json:"contentHash"`
}

type gatewayMaster struct {
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

// registerDataPlane registers the suite's Data Plane as a gateway of this Control Plane for one test.
func (ts *ControlPlaneTestSuite) registerDataPlane() string {
	ts.T().Helper()
	// The data plane is bootstrapped in a home of its own, beside the control plane's, with a
	// certificate of its own.
	outDir := filepath.Dir(filepath.Dir(testutils.GetExtractedProductHome()))
	certificates, err := filepath.Glob(filepath.Join(outDir, filepath.Base(testutils.DataPlaneExtractedDir), "*",
		"config", "certs", "server.cert"))
	ts.Require().NoError(err)
	ts.Require().Len(certificates, 1, "the data plane's certificate was not found")
	certificate, err := os.ReadFile(certificates[0])
	ts.Require().NoError(err, "the data plane's certificate is what it is trusted by")
	var registered struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, "/gateways", map[string]any{
		"name":          unique("data-plane"),
		"baseUrl":       testutils.DataPlaneURL,
		"key":           testutils.ManagementAPIKey,
		"caCertificate": string(certificate),
	}, &registered)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/gateways/"+registered.ID, nil, nil) })
	return registered.ID
}

func (ts *ControlPlaneTestSuite) masterOf(gatewayID string) gatewayMaster {
	ts.T().Helper()
	var held gatewayMaster
	status, raw := ts.call(http.MethodGet, "/gateways/"+gatewayID+"/master", nil, &held)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	return held
}

// Applied to the Data Plane, a version the Control Plane captured leaves a master that records the
// resource deployed from it, none refused; a later version that dropped the resource leaves it
// recorded removed.
//
// The Data Plane is shared with the suite's other tests, so the apply sends only the test's own unit,
// which needs no variable or secret: the rest of the configuration is left alone, and the second apply
// removes the unit, leaving the Data Plane as it found it.
func (ts *ControlPlaneTestSuite) TestApplyingToTheDataPlaneRecordsItsMaster() {
	gatewayID := ts.registerDataPlane()
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("mastered"), Name: "Mastered on the data plane",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()
	key := "organization_unit/" + ouID

	var withUnit, withoutUnit struct {
		Version string `json:"version"`
	}
	status, raw := ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "with the unit"}, &withUnit)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status, string(raw))
	ts.Require().NoError(testutils.DeleteOrganizationUnit(ouID))
	status, raw = ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "without the unit"},
		&withoutUnit)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status, string(raw))

	var applied struct {
		Recorded bool   `json:"recorded"`
		Master   string `json:"master"`
		Import   any    `json:"import"`
	}
	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply", map[string]any{
		"version": withUnit.Version, "origin": "control-plane", "selection": []string{key},
	}, &applied)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().True(applied.Recorded, "the apply was not recorded: %s", raw)

	held := ts.masterOf(gatewayID)
	ts.Equal(applied.Master, held.Master)
	ts.Equal(1, held.Number)
	ts.Zero(held.Summary.Failed, "the data plane refused something: %v", applied.Import)
	ts.Zero(held.Summary.Removed)
	ts.Equal(1, held.Summary.Deployed, "more than the selected unit was recorded deployed")
	ts.Contains(held.Resources, ouID)
	ts.Require().Len(held.Entries, 1)
	entry := held.Entries[0]
	ts.Equal(key, entry.Key)
	ts.Equal("deployed", entry.State)
	ts.Equal(withUnit.Version, entry.SourceVersion)
	ts.Equal("control-plane", entry.Origin)
	ts.Len(entry.ContentHash, 64)

	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		map[string]any{"version": withoutUnit.Version, "selection": []string{key}}, &applied)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().True(applied.Recorded, "the apply was not recorded: %s", raw)

	held = ts.masterOf(gatewayID)
	ts.Equal(2, held.Number)
	ts.Zero(held.Summary.Failed, "the data plane refused something: %v", applied.Import)
	ts.Zero(held.Summary.Deployed)
	ts.Equal(1, held.Summary.Removed)
	ts.NotContains(held.Resources, ouID)
	ts.Require().Len(held.Entries, 1)
	ts.Equal(key, held.Entries[0].Key)
	ts.Equal("removed", held.Entries[0].State)
	ts.Equal(withoutUnit.Version, held.Entries[0].SourceVersion)
	ts.Empty(held.Entries[0].ContentHash)
}
