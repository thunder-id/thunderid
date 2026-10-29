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

// supplyMissingValues sets, on the data plane through this Control Plane, every value a dry run of
// the version reports the data plane lacks, so the apply can go ahead.
func (ts *ControlPlaneTestSuite) supplyMissingValues(gatewayID, version string) {
	ts.T().Helper()
	var dry struct {
		Missing *struct {
			Variables  []string `json:"variables"`
			Secrets    []string `json:"secrets"`
			References []struct {
				Name string `json:"name"`
				Kind string `json:"kind"`
				List bool   `json:"list"`
			} `json:"references"`
		} `json:"missing"`
	}
	status, raw := ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		map[string]any{"version": version, "dryRun": true}, &dry)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	if dry.Missing == nil {
		return
	}
	lists := map[string]bool{}
	for _, reference := range dry.Missing.References {
		if reference.List {
			lists[reference.Name] = true
		}
	}
	for _, name := range dry.Missing.Variables {
		value := unique("value")
		if lists[name] {
			value = `["https://` + unique("dp") + `.example.com/callback"]`
		}
		status, raw := ts.call(http.MethodPut, "/gateways/"+gatewayID+"/variables/"+name,
			map[string]any{"value": value}, nil)
		ts.Require().Less(status, http.StatusBadRequest, string(raw))
	}
	for _, name := range dry.Missing.Secrets {
		status, raw := ts.call(http.MethodPut, "/gateways/"+gatewayID+"/secrets/"+name,
			map[string]any{"value": unique("secret")}, nil)
		ts.Require().Less(status, http.StatusBadRequest, string(raw))
	}
}

func (ts *ControlPlaneTestSuite) masterOf(gatewayID string) gatewayMaster {
	ts.T().Helper()
	var held gatewayMaster
	status, raw := ts.call(http.MethodGet, "/gateways/"+gatewayID+"/master", nil, &held)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	return held
}

// Applied to the Data Plane, a version the Control Plane captured leaves a master that records every
// resource deployed from it, none refused; a later version that dropped a resource leaves it recorded
// removed, with the rest keeping where they came from.
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

	ts.supplyMissingValues(gatewayID, withUnit.Version)
	var applied struct {
		Recorded bool   `json:"recorded"`
		Master   string `json:"master"`
		Import   any    `json:"import"`
	}
	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		map[string]any{"version": withUnit.Version, "origin": "control-plane", "confirmRemovals": true}, &applied)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().True(applied.Recorded, "the apply was not recorded: %s", raw)

	held := ts.masterOf(gatewayID)
	ts.Equal(applied.Master, held.Master)
	ts.Equal(1, held.Number)
	ts.Zero(held.Summary.Failed, "the data plane refused something: %v", applied.Import)
	ts.Zero(held.Summary.Removed)
	ts.Equal(len(held.Entries), held.Summary.Deployed)
	ts.Contains(held.Resources, ouID)
	for _, entry := range held.Entries {
		ts.Equal("deployed", entry.State, entry.Key)
		ts.Equal(withUnit.Version, entry.SourceVersion, entry.Key)
		ts.Equal("control-plane", entry.Origin, entry.Key)
		ts.Len(entry.ContentHash, 64, entry.Key)
	}

	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		map[string]any{"version": withoutUnit.Version}, &applied)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().True(applied.Recorded, "the apply was not recorded: %s", raw)

	held = ts.masterOf(gatewayID)
	ts.Equal(2, held.Number)
	ts.Zero(held.Summary.Failed, "the data plane refused something: %v", applied.Import)
	ts.Equal(1, held.Summary.Removed)
	ts.NotContains(held.Resources, ouID)
	for _, entry := range held.Entries {
		if entry.Key == key {
			ts.Equal("removed", entry.State)
			ts.Equal(withoutUnit.Version, entry.SourceVersion)
			ts.Empty(entry.ContentHash)
			continue
		}
		ts.Equal("deployed", entry.State, entry.Key)
		ts.Equal(withUnit.Version, entry.SourceVersion, "%s did not change, yet it was recorded from the new version",
			entry.Key)
	}
}
