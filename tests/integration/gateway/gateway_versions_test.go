// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// selfManagementKey is the management API key the integration server trusts. Registering the server
// as a gateway of itself with this key lets a version be applied without a second server.
const selfManagementKey = "integration-management-api-key"

type version struct {
	Version   int    `json:"version"`
	Note      string `json:"note"`
	Resources string `json:"resources"`
}

type appliedVersion struct {
	AppliedVersion  int `json:"appliedVersion"`
	PreviousVersion int `json:"previousVersion"`
}

type change struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	Change       string `json:"change"`
}

type diffResult struct {
	FromVersion int      `json:"fromVersion"`
	ToVersion   int      `json:"toVersion"`
	Changes     []change `json:"changes"`
}

type applyResult struct {
	DryRun   bool            `json:"dryRun"`
	Recorded bool            `json:"recorded"`
	Diff     diffResult      `json:"diff"`
	Import   json.RawMessage `json:"import"`
}

// GatewayVersionsTestSuite captures this server's configuration as versions and applies them to the
// server itself, registered as a gateway.
type GatewayVersionsTestSuite struct {
	suite.Suite
	gatewayID string
}

func TestGatewayVersionsTestSuite(t *testing.T) {
	suite.Run(t, new(GatewayVersionsTestSuite))
}

func (ts *GatewayVersionsTestSuite) SetupTest() {
	certificate, err := os.ReadFile(filepath.Join(testutils.GetExtractedProductHome(), "config", "certs",
		"server.cert"))
	ts.Require().NoError(err, "the server certificate is what this gateway is trusted by")

	var registered gateway
	status := ts.call(http.MethodPost, "/gateways", map[string]any{
		"name":          fmt.Sprintf("self-%d", time.Now().UnixNano()),
		"baseUrl":       testServerURL,
		"key":           selfManagementKey,
		"caCertificate": string(certificate),
	}, &registered)
	ts.Require().Equal(http.StatusCreated, status)
	ts.gatewayID = registered.ID
}

func (ts *GatewayVersionsTestSuite) TearDownTest() {
	ts.call(http.MethodDelete, "/gateways/"+ts.gatewayID, nil, nil)
}

// A version records the configuration as it was, so applying it brings back what was removed since,
// and applying a later one removes what that one no longer has. A revert returns to the version held
// before.
func (ts *GatewayVersionsTestSuite) TestApplyAddsAndRemovesWhatTheVersionsDiffer() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("versioned-%d", time.Now().UnixNano()), Name: "Versioned",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()

	withUnit := ts.capture("with the unit")
	ts.Contains(withUnit.Resources, ouID, "the captured version does not hold the unit")

	ts.Require().NoError(testutils.DeleteOrganizationUnit(ouID))
	withoutUnit := ts.capture("without the unit")
	ts.NotContains(withoutUnit.Resources, ouID)

	// Nothing has been applied yet, so the unit is something the gateway does not hold.
	first := ts.apply(withUnit.Version, false)
	ts.True(first.Recorded, "the apply was not recorded: %s", first.Import)
	ts.Equal(http.StatusOK, ts.unitStatus(ouID), "applying the version did not bring the unit back")
	ts.Equal(appliedVersion{AppliedVersion: withUnit.Version}, ts.applied())

	// The later version no longer has the unit, so applying it removes it.
	second := ts.apply(withoutUnit.Version, false)
	ts.True(second.Recorded, "the apply was not recorded: %s", second.Import)
	ts.Equal(withUnit.Version, second.Diff.FromVersion)
	ts.Contains(second.Diff.Changes, change{ResourceType: "organization_unit", ID: ouID, Change: "deleted"})
	ts.Equal(http.StatusNotFound, ts.unitStatus(ouID), "the unit the version dropped was not removed")
	ts.Equal(appliedVersion{AppliedVersion: withoutUnit.Version, PreviousVersion: withUnit.Version},
		ts.applied())

	// A revert returns to the version held before, and keeps the one reverted from to return to.
	var reverted applyResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/revert",
		map[string]any{}, &reverted))
	ts.True(reverted.Recorded, "the revert was not recorded: %s", reverted.Import)
	ts.Equal(http.StatusOK, ts.unitStatus(ouID), "the revert did not bring the unit back")
	ts.Equal(appliedVersion{AppliedVersion: withUnit.Version, PreviousVersion: withoutUnit.Version},
		ts.applied())
}

// A dry run reports what would change and records nothing.
func (ts *GatewayVersionsTestSuite) TestADryRunChangesNothing() {
	captured := ts.capture("")

	result := ts.apply(captured.Version, true)

	ts.True(result.DryRun)
	ts.False(result.Recorded)
	ts.Equal(appliedVersion{}, ts.applied(), "a dry run recorded an applied version")
}

// The diff shows what an apply would send without sending it.
func (ts *GatewayVersionsTestSuite) TestTheDiffComparesWithWhatTheGatewayHolds() {
	captured := ts.capture("")

	var diff diffResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet,
		fmt.Sprintf("/gateways/%s/diff?version=%d", ts.gatewayID, captured.Version), nil, &diff))

	ts.Equal(captured.Version, diff.ToVersion)
	ts.Zero(diff.FromVersion, "the gateway holds nothing yet")
	ts.NotEmpty(diff.Changes)
	for _, c := range diff.Changes {
		ts.Equal("added", c.Change, "nothing held, so everything is added")
	}
}

// Captured versions are listed newest first and read back with their content.
func (ts *GatewayVersionsTestSuite) TestVersionsAreListedAndReadBack() {
	older := ts.capture("older")
	newer := ts.capture("newer")

	var listed []version
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/configuration-versions", nil, &listed))
	// Found by number rather than by position, so a capture made elsewhere in the meantime does not
	// move what is asserted.
	newerAt, olderAt := -1, -1
	for i, listedVersion := range listed {
		switch listedVersion.Version {
		case newer.Version:
			newerAt = i
		case older.Version:
			olderAt = i
		}
	}
	ts.Require().NotEqual(-1, newerAt, "the newer version is not listed")
	ts.Require().NotEqual(-1, olderAt, "the older version is not listed")
	ts.Less(newerAt, olderAt, "versions are not listed newest first")
	ts.Equal("newer", listed[newerAt].Note)
	ts.Empty(listed[newerAt].Resources, "a listing should leave the content out")

	var read version
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet,
		fmt.Sprintf("/configuration-versions/%d", older.Version), nil, &read))
	ts.Equal(older.Version, read.Version)
	ts.NotEmpty(read.Resources)
	ts.Contains(read.Resources, "resource_type: user", "a version should carry the deployment's users")

	var latest version
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/configuration-versions/latest", nil, &latest))
	ts.GreaterOrEqual(latest.Version, newer.Version)
}

// What cannot be done is refused with a reason rather than attempted.
func (ts *GatewayVersionsTestSuite) TestWhatCannotBeDoneIsRefused() {
	ts.Equal(http.StatusNotFound, ts.call(http.MethodGet, "/configuration-versions/999999", nil, nil))
	ts.Equal(http.StatusBadRequest, ts.call(http.MethodGet, "/configuration-versions/first", nil, nil))
	ts.Equal(http.StatusBadRequest, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/revert",
		map[string]any{}, nil), "a gateway holding nothing has nothing to revert to")
	ts.Equal(http.StatusNotFound, ts.call(http.MethodPost, "/gateways/not-a-gateway/apply",
		map[string]any{}, nil))
}

// A gateway that cannot be reached is reported as such, and nothing is recorded for it.
func (ts *GatewayVersionsTestSuite) TestAnUnreachableGatewayIsReported() {
	var unreachable gateway
	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, "/gateways", map[string]any{
		"name":    fmt.Sprintf("unreachable-%d", time.Now().UnixNano()),
		"baseUrl": "https://127.0.0.1:1",
	}, &unreachable))
	defer ts.call(http.MethodDelete, "/gateways/"+unreachable.ID, nil, nil)
	captured := ts.capture("")

	status := ts.call(http.MethodPost, "/gateways/"+unreachable.ID+"/apply",
		map[string]any{"version": fmt.Sprint(captured.Version)}, nil)

	ts.Equal(http.StatusBadGateway, status)
}

// A gateway's variables are managed through this plane, which presents the gateway's key: what is
// written lands in the gateway's own store, and what the gateway refuses is refused as it answered.
func (ts *GatewayVersionsTestSuite) TestAGatewaysVariablesAreManagedThroughThisPlane() {
	name := fmt.Sprintf("GW_VAR_%d", time.Now().UnixNano())
	base := "/gateways/" + ts.gatewayID + "/variables"
	defer ts.call(http.MethodDelete, base+"/"+name, nil, nil)

	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, base,
		map[string]any{"name": name, "value": "first"}, nil))
	ts.Equal(http.StatusOK, ts.call(http.MethodPut, base+"/"+name, map[string]any{"value": "second"}, nil))

	var direct struct {
		Value string `json:"value"`
	}
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/variables/"+name, nil, &direct),
		"the variable did not land in the gateway's own store")
	ts.Equal("second", direct.Value)

	var listed struct {
		Variables []struct {
			Name string `json:"name"`
		} `json:"variables"`
	}
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, base+"?names="+name, nil, &listed))
	ts.Require().Len(listed.Variables, 1)
	ts.Equal(name, listed.Variables[0].Name)

	ts.Equal(http.StatusBadRequest, ts.call(http.MethodPost, base, map[string]any{"name": "not a name"}, nil),
		"the gateway's own validation did not come back")
	ts.Equal(http.StatusNoContent, ts.call(http.MethodDelete, base+"/"+name, nil, nil))
	ts.Equal(http.StatusNotFound, ts.call(http.MethodGet, "/variables/"+name, nil, nil))
}

// A gateway's secrets are written through this plane and, as on the gateway, never read back.
func (ts *GatewayVersionsTestSuite) TestAGatewaysSecretsAreWrittenButNeverRead() {
	name := fmt.Sprintf("GW_SECRET_%d", time.Now().UnixNano())
	base := "/gateways/" + ts.gatewayID + "/secrets"
	defer ts.call(http.MethodDelete, base+"/"+name, nil, nil)

	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, base,
		map[string]any{"name": name, "value": "s3cret"}, nil))

	raw := ts.rawBody(http.MethodGet, base+"/"+name)
	ts.Contains(raw, name)
	ts.NotContains(raw, "s3cret", "a secret's value was read back")
}

func (ts *GatewayVersionsTestSuite) rawBody(method, path string) string {
	ts.T().Helper()
	req, err := http.NewRequest(method, testServerURL+path, nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	return string(body)
}

func (ts *GatewayVersionsTestSuite) capture(note string) version {
	ts.T().Helper()
	var captured version
	status := ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": note}, &captured)
	ts.Require().Equal(http.StatusCreated, status)
	ts.Require().Positive(captured.Version)
	return captured
}

func (ts *GatewayVersionsTestSuite) apply(seq int, dryRun bool) applyResult {
	ts.T().Helper()
	var result applyResult
	status := ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": fmt.Sprint(seq), "dryRun": dryRun}, &result)
	ts.Require().Equal(http.StatusOK, status)
	return result
}

func (ts *GatewayVersionsTestSuite) applied() appliedVersion {
	ts.T().Helper()
	var held appliedVersion
	ts.Require().Equal(http.StatusOK,
		ts.call(http.MethodGet, "/gateways/"+ts.gatewayID+"/applied-version", nil, &held))
	return held
}

func (ts *GatewayVersionsTestSuite) unitStatus(id string) int {
	ts.T().Helper()
	return ts.call(http.MethodGet, "/organization-units/"+id, nil, nil)
}

// call sends a request and decodes a successful answer into out.
func (ts *GatewayVersionsTestSuite) call(method, path string, body any, out any) int {
	ts.T().Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	if out != nil && resp.StatusCode < http.StatusBadRequest {
		ts.Require().NoError(json.NewDecoder(resp.Body).Decode(out))
	}
	return resp.StatusCode
}
