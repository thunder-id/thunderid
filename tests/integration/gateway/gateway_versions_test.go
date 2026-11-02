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
	Version   string `json:"version"`
	Number    int    `json:"number"`
	Name      string `json:"name"`
	Unchanged bool   `json:"unchanged"`
	Current   bool   `json:"current"`
	Note      string `json:"note"`
	Resources string `json:"resources"`
}

type appliedVersion struct {
	AppliedVersion  string `json:"appliedVersion"`
	PreviousVersion string `json:"previousVersion"`
}

type change struct {
	Key          string `json:"key"`
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	Change       string `json:"change"`
	Skipped      bool   `json:"skipped"`
	Excluded     bool   `json:"excluded"`
}

type diffResult struct {
	FromVersion string   `json:"fromVersion"`
	ToVersion   string   `json:"toVersion"`
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
	if removed := ts.changeIn(second.Diff, "organization_unit/"+ouID); ts.NotNil(removed, "the diff does not report the unit") {
		ts.Equal("deleted", removed.Change)
	}
	ts.Equal(http.StatusNotFound, ts.unitStatus(ouID), "the unit the version dropped was not removed")
	ts.Equal(appliedVersion{AppliedVersion: withoutUnit.Version, PreviousVersion: withUnit.Version},
		ts.applied())

	// A revert returns to the version held before, one step only: a second revert is refused.
	var reverted applyResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/revert",
		map[string]any{}, &reverted))
	ts.True(reverted.Recorded, "the revert was not recorded: %s", reverted.Import)
	ts.Equal(http.StatusOK, ts.unitStatus(ouID), "the revert did not bring the unit back")
	ts.Equal(appliedVersion{AppliedVersion: withUnit.Version}, ts.applied())
	ts.Equal(http.StatusBadRequest, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/revert",
		map[string]any{}, nil), "a revert was reverted")
	ts.Equal(http.StatusOK, ts.unitStatus(ouID), "a refused revert changed the gateway")
}

// Without a user lookup at a trusted issuer, a version names no one and neither does the gateway that
// holds it: the caller's id alone is not given out, though it is recorded.
func (ts *GatewayVersionsTestSuite) TestWithoutALookupNoUserIsNamed() {
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("actor-%d", time.Now().UnixNano()), Name: "actor",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(id) }()

	var captured struct {
		Version   string          `json:"version"`
		CreatedBy json.RawMessage `json:"createdBy"`
	}
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK},
		ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "who"}, &captured))
	ts.Nil(captured.CreatedBy, "the version gave out a user with no name to show")

	ts.True(ts.apply(captured.Version, false).Recorded)
	var held struct {
		AppliedBy json.RawMessage `json:"appliedBy"`
	}
	ts.Require().Equal(http.StatusOK,
		ts.call(http.MethodGet, "/gateways/"+ts.gatewayID+"/applied-version", nil, &held))
	ts.Nil(held.AppliedBy, "the gateway gave out a user with no name to show")
}

// A change left out of an apply is left alone on the gateway, and stays left alone in later applies
// until one selects it again.
func (ts *GatewayVersionsTestSuite) TestAChangeLeftOutIsLeftAloneUntilSelected() {
	units := map[string]string{}
	for _, name := range []string{"taken", "left"} {
		id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
			Handle: fmt.Sprintf("selection-%s-%d", name, time.Now().UnixNano()), Name: name,
		})
		ts.Require().NoError(err)
		units[name] = id
		defer func() { _ = testutils.DeleteOrganizationUnit(id) }()
	}
	captured := ts.capture("two units")
	for _, id := range units {
		ts.Require().NoError(testutils.DeleteOrganizationUnit(id))
	}
	leftKey := "organization_unit/" + units["left"]

	ts.Equal(http.StatusBadRequest, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": captured.Version, "selection": []string{"organization_unit/not-in-the-diff"}},
		nil), "a key the diff does not report was accepted")
	ts.Equal(http.StatusNotFound, ts.unitStatus(units["taken"]), "a refused selection was applied")

	ts.applySelecting(captured.Version, func(key string) bool { return key != leftKey })
	ts.Equal(http.StatusOK, ts.unitStatus(units["taken"]), "the selected unit was not applied")
	ts.Equal(http.StatusNotFound, ts.unitStatus(units["left"]), "the unit left out was applied")

	var diff struct {
		Changes []struct {
			Key      string `json:"key"`
			Excluded bool   `json:"excluded"`
		} `json:"changes"`
	}
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet,
		fmt.Sprintf("/gateways/%s/diff?version=%s", ts.gatewayID, captured.Version), nil, &diff))
	excluded := []string{}
	for _, c := range diff.Changes {
		if c.Excluded {
			excluded = append(excluded, c.Key)
		}
	}
	ts.Equal([]string{leftKey}, excluded, "the unit left out is not offered as left alone")

	ts.apply(captured.Version, false)
	ts.Equal(http.StatusNotFound, ts.unitStatus(units["left"]), "the choice was not kept for the next apply")

	ts.applySelecting(captured.Version, func(string) bool { return true })
	ts.Equal(http.StatusOK, ts.unitStatus(units["left"]), "selecting the unit again did not apply it")
}

// A skipped change is left alone for that apply only: the gateway does not keep it as its choice but
// is owed it, so the next diff offers it again, saying it was skipped, and the next apply writes it.
func (ts *GatewayVersionsTestSuite) TestASkippedChangeIsLeftAloneOnce() {
	units := map[string]string{}
	for _, name := range []string{"taken", "skipped"} {
		id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
			Handle: fmt.Sprintf("skip-%s-%d", name, time.Now().UnixNano()), Name: name,
		})
		ts.Require().NoError(err)
		units[name] = id
		defer func() { _ = testutils.DeleteOrganizationUnit(id) }()
	}
	captured := ts.capture("two units to skip from")
	for _, id := range units {
		ts.Require().NoError(testutils.DeleteOrganizationUnit(id))
	}
	skippedKey := "organization_unit/" + units["skipped"]

	var result applyResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": captured.Version, "skip": []string{skippedKey}}, &result))
	ts.True(result.Recorded, "the apply was not recorded: %s", result.Import)
	ts.Equal(http.StatusOK, ts.unitStatus(units["taken"]), "the unit not skipped was not applied")
	ts.Equal(http.StatusNotFound, ts.unitStatus(units["skipped"]), "the skipped unit was applied")

	offered := ts.changeIn(ts.apply(captured.Version, true).Diff, skippedKey)
	ts.Require().NotNil(offered, "the skipped change was not offered again")
	ts.Equal("updated", offered.Change)
	ts.True(offered.Skipped, "the change offered again does not say it was skipped")

	ts.True(ts.apply(captured.Version, false).Recorded)
	ts.Equal(http.StatusOK, ts.unitStatus(units["skipped"]), "a skip was kept for the next apply")
	taken := ts.changeIn(ts.apply(captured.Version, true).Diff, skippedKey)
	ts.Require().NotNil(taken)
	ts.Equal("unchanged", taken.Change)
	ts.False(taken.Skipped, "a change the apply wrote is still said to be skipped")
}

// A change owed from a skip that a later selection leaves out becomes the gateway's standing choice:
// the stored row is turned from owed into left alone, on the database itself.
func (ts *GatewayVersionsTestSuite) TestALeftOutOwedChangeBecomesTheStandingChoice() {
	units := map[string]string{}
	for _, name := range []string{"kept", "owed"} {
		id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
			Handle: fmt.Sprintf("owed-%s-%d", name, time.Now().UnixNano()), Name: name,
		})
		ts.Require().NoError(err)
		units[name] = id
		defer func() { _ = testutils.DeleteOrganizationUnit(id) }()
	}
	captured := ts.capture("two units, one to owe")
	for _, id := range units {
		ts.Require().NoError(testutils.DeleteOrganizationUnit(id))
	}
	keptKey, owedKey := "organization_unit/"+units["kept"], "organization_unit/"+units["owed"]

	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": captured.Version, "skip": []string{owedKey}}, nil))
	owed := ts.changeIn(ts.apply(captured.Version, true).Diff, owedKey)
	ts.Require().NotNil(owed)
	ts.Require().True(owed.Skipped, "the skipped change is not owed")

	var result applyResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": captured.Version, "selection": []string{keptKey}}, &result))
	ts.True(result.Recorded, "the apply was not recorded: %s", result.Import)

	left := ts.changeIn(ts.apply(captured.Version, true).Diff, owedKey)
	ts.Require().NotNil(left, "the change left out is no longer offered")
	ts.True(left.Excluded, "the change left out is not the gateway's choice")
	ts.False(left.Skipped, "the change left out is still owed")
	ts.Equal(http.StatusNotFound, ts.unitStatus(units["owed"]), "the change left out was applied")

	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": captured.Version, "selection": []string{keptKey, owedKey}}, nil))
}

// A skipped removal is owed to the gateway: the version is recorded, and the diff offers the removal
// until an apply removes the resource.
func (ts *GatewayVersionsTestSuite) TestASkippedRemovalIsOfferedUntilRemoved() {
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("skip-removal-%d", time.Now().UnixNano()), Name: "skip removal",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(id) }()
	withUnit := ts.capture("with the unit to be removed")
	ts.True(ts.apply(withUnit.Version, false).Recorded)
	ts.Require().NoError(testutils.DeleteOrganizationUnit(id))
	without := ts.capture("without the unit")
	key := "organization_unit/" + id

	var result applyResult
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": without.Version, "skip": []string{key}}, &result))
	ts.True(result.Recorded, "the apply was not recorded: %s", result.Import)
	ts.Equal(without.Version, ts.applied().AppliedVersion)

	owed := ts.changeIn(ts.apply(without.Version, true).Diff, key)
	ts.Require().NotNil(owed, "the skipped removal was not offered again")
	ts.Equal("deleted", owed.Change)
	ts.True(owed.Skipped)

	ts.True(ts.apply(without.Version, false).Recorded)
	ts.Nil(ts.changeIn(ts.apply(without.Version, true).Diff, key), "the removal is still offered after an apply took it")
}

// changeIn finds a change in a diff by key.
func (ts *GatewayVersionsTestSuite) changeIn(diff diffResult, key string) *change {
	for i := range diff.Changes {
		if diff.Changes[i].Key == key {
			return &diff.Changes[i]
		}
	}
	return nil
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
		fmt.Sprintf("/gateways/%s/diff?version=%s", ts.gatewayID, captured.Version), nil, &diff))

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
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("listed-%d", time.Now().UnixNano()), Name: "Listed",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()
	newer := ts.capture("newer")
	ts.NotEqual(older.Version, newer.Version, "a changed configuration was not a new version")

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
		fmt.Sprintf("/configuration-versions/%s", older.Version), nil, &read))
	ts.Equal(older.Version, read.Version)
	ts.NotEmpty(read.Resources)
	ts.Contains(read.Resources, "resource_type: user", "a version should carry the deployment's users")

	var latest version
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/configuration-versions/latest", nil, &latest))
	ts.Equal(newer.Version, latest.Version)
}

// A version keeps the name it was captured with and is read back by any seven characters of its hash.
func (ts *GatewayVersionsTestSuite) TestAVersionIsNamedAndReadByAPrefixOfItsHash() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("named-%d", time.Now().UnixNano()), Name: "Named",
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()

	var captured version
	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, "/configuration-versions",
		map[string]any{"name": "Spring release", "note": "first"}, &captured))
	ts.Equal("Spring release", captured.Name)
	ts.False(captured.Unchanged)

	var read version
	ts.Require().Equal(http.StatusOK,
		ts.call(http.MethodGet, "/configuration-versions/"+captured.Version[:7], nil, &read))
	ts.Equal(captured.Version, read.Version)
	ts.Equal("Spring release", read.Name)
}

// What cannot be done is refused with a reason rather than attempted.
func (ts *GatewayVersionsTestSuite) TestWhatCannotBeDoneIsRefused() {
	ts.Equal(http.StatusNotFound, ts.call(http.MethodGet, "/configuration-versions/0000000", nil, nil))
	ts.Equal(http.StatusBadRequest, ts.call(http.MethodGet, "/configuration-versions/first", nil, nil))
	ts.Equal(http.StatusBadRequest, ts.call(http.MethodGet, "/configuration-versions/abc", nil, nil))
	ts.Equal(http.StatusBadRequest, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/revert",
		map[string]any{}, nil), "a gateway holding nothing has nothing to revert to")
	ts.Equal(http.StatusNotFound, ts.call(http.MethodPost, "/gateways/not-a-gateway/apply",
		map[string]any{}, nil))
}

// A gateway that cannot be reached is reported as such, and nothing is recorded for it. That holds
// from the first call to it: for a version that refers to a value, a dry run asks the gateway which
// values it holds before anything else.
func (ts *GatewayVersionsTestSuite) TestAnUnreachableGatewayIsReported() {
	var unreachable gateway
	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, "/gateways", map[string]any{
		"name":    fmt.Sprintf("unreachable-%d", time.Now().UnixNano()),
		"baseUrl": "https://127.0.0.1:1",
	}, &unreachable))
	defer ts.call(http.MethodDelete, "/gateways/"+unreachable.ID, nil, nil)
	ts.referringUnit()
	captured := ts.capture("")

	ts.Equal(http.StatusBadGateway, ts.call(http.MethodPost, "/gateways/"+unreachable.ID+"/apply",
		map[string]any{"version": captured.Version, "dryRun": true}, nil),
		"a dry run that could not read the gateway's values was not reported as unreachable")
	status := ts.call(http.MethodPost, "/gateways/"+unreachable.ID+"/apply",
		map[string]any{"version": fmt.Sprint(captured.Version)}, nil)

	ts.Equal(http.StatusBadGateway, status)
	ts.Equal(http.StatusBadGateway, ts.call(http.MethodGet, "/gateways/"+unreachable.ID+"/variables", nil, nil),
		"a store read through an unreachable gateway was not reported as such")
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
	// A configuration already captured, as another test may have left it, is answered unchanged.
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status)
	ts.Require().Len(captured.Version, 64, "a version is named by its SHA-256")
	return captured
}

func (ts *GatewayVersionsTestSuite) apply(version string, dryRun bool) applyResult {
	ts.T().Helper()
	var result applyResult
	status := ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": version, "dryRun": dryRun}, &result)
	ts.Require().Equal(http.StatusOK, status)
	return result
}

// applySelecting applies a version selecting the changes on offer that keep accepts.
func (ts *GatewayVersionsTestSuite) applySelecting(version string, keep func(key string) bool) applyResult {
	ts.T().Helper()
	var diff struct {
		Changes []struct {
			Key      string `json:"key"`
			Change   string `json:"change"`
			Excluded bool   `json:"excluded"`
		} `json:"changes"`
	}
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet,
		fmt.Sprintf("/gateways/%s/diff?version=%s", ts.gatewayID, version), nil, &diff))
	selection := []string{}
	for _, c := range diff.Changes {
		if (c.Change != "unchanged" || c.Excluded) && keep(c.Key) {
			selection = append(selection, c.Key)
		}
	}
	var result applyResult
	status := ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": version, "selection": selection}, &result)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().True(result.Recorded, "the apply was not recorded: %s", result.Import)
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

// An apply settles the version it sends, so a capture meanwhile adds a version rather than rewriting
// one under the apply. When no gateway received it, the version is current again, and the next
// capture after a change replaces it in place; a version a gateway did receive stays as it is.
func (ts *GatewayVersionsTestSuite) TestAnApplyThatReachedNoGatewayLeavesTheVersionCurrent() {
	var unreachable gateway
	ts.Require().Equal(http.StatusCreated, ts.call(http.MethodPost, "/gateways", map[string]any{
		"name":    fmt.Sprintf("unreachable-%d", time.Now().UnixNano()),
		"baseUrl": "https://127.0.0.1:1",
	}, &unreachable))
	defer ts.call(http.MethodDelete, "/gateways/"+unreachable.ID, nil, nil)
	ts.unit("Settled")
	current := ts.captureCurrent()

	status := ts.call(http.MethodPost, "/gateways/"+unreachable.ID+"/apply",
		map[string]any{"version": current.Version}, nil)
	ts.Require().Equal(http.StatusBadGateway, status)

	listed := ts.listVersions()
	ts.Require().NotEmpty(listed)
	ts.Equal(listedVersion{Version: current.Version, Current: true}, listed[0],
		"the version an apply never delivered is no longer current")
	ts.unit("Settled again")
	replaced := ts.captureCurrent()
	ts.Equal(current.Number, replaced.Number, "a change after the failed apply added a version instead of replacing")
	ts.NotEqual(current.Version, replaced.Version)

	// Delivered to a gateway, the version is kept as it is from then on, whatever a later apply does.
	ts.apply(replaced.Version, false)
	status = ts.call(http.MethodPost, "/gateways/"+unreachable.ID+"/apply",
		map[string]any{"version": replaced.Version}, nil)
	ts.Require().Equal(http.StatusBadGateway, status)
	ts.unit("Settled once more")
	added := ts.captureCurrent()
	ts.Equal(replaced.Number+1, added.Number, "a version a gateway runs was replaced in place")
}

// A capture is refused while a resource refers to a secret from a field that holds no credential:
// every gateway would refuse to import it, so the configuration is refused here, naming the place.
func (ts *GatewayVersionsTestSuite) TestACaptureRefusesASecretOutsideACredentialField() {
	suffix := fmt.Sprint(time.Now().UnixNano())
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "misplaced-" + suffix, Name: "Misplaced " + suffix, Description: "sec:MISPLACED_" + suffix,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(id) }()

	encoded, err := json.Marshal(map[string]any{"note": "misplaced"})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/configuration-versions", bytes.NewReader(encoded))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, resp.StatusCode, string(body))
	ts.Contains(string(body), "GTW-1026")
	ts.Contains(string(body), "organization_unit/"+id+" description", "the refusal does not say where the secret stands")
}
