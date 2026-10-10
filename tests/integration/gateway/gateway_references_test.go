// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type valueReference struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName"`
	Field        string `json:"field"`
}

type referencingVersion struct {
	Version    string           `json:"version"`
	References []valueReference `json:"references"`
}

// referringUnit creates an organization unit whose description is a reference, so the configuration
// refers to a value by name, and returns the unit's id, its name and the value's name.
func (ts *GatewayVersionsTestSuite) referringUnit() (string, string, string) {
	suffix := fmt.Sprint(time.Now().UnixNano())
	name := "Referring Unit " + suffix
	value := "REFERRING_UNIT_" + suffix
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "referring-unit-" + suffix,
		Name:        name,
		Description: "var:" + value,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(id) })
	return id, name, value
}

func (ts *GatewayVersionsTestSuite) referenceTo(references []valueReference, name string) *valueReference {
	for i := range references {
		if references[i].Name == name {
			return &references[i]
		}
	}
	return nil
}

// The configuration as it stands says which resource refers to each value, and in which field, before
// anything is captured; reading it captures nothing.
func (ts *GatewayVersionsTestSuite) TestTheCurrentConfigurationSaysWhatEachValueIsFor() {
	id, name, value := ts.referringUnit()
	var before []version
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/configuration-versions", nil, &before))

	var current referencingVersion
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/configuration-versions/current", nil, &current))

	ts.Len(current.Version, 64, "the current configuration did not carry the hash a capture would give it")
	reference := ts.referenceTo(current.References, value)
	ts.Require().NotNil(reference, "the current configuration did not say where it refers to %s", value)
	ts.Equal(valueReference{Name: value, Kind: "variable", ResourceType: "organization_unit", ResourceID: id,
		ResourceName: name, Field: "description"}, *reference)

	var after []version
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/configuration-versions", nil, &after))
	ts.Equal(len(before), len(after), "reading the current configuration captured a version")
}

// A dry run names each value the gateway lacks with what it is for, and an apply is refused for it.
func (ts *GatewayVersionsTestSuite) TestAMissingValueIsReportedWithWhatItIsFor() {
	id, name, value := ts.referringUnit()
	captured := ts.capture("referring")

	var dryRun struct {
		Missing struct {
			Variables  []string         `json:"variables"`
			References []valueReference `json:"references"`
		} `json:"missing"`
	}
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": captured.Version, "dryRun": true}, &dryRun))

	ts.Contains(dryRun.Missing.Variables, value)
	reference := ts.referenceTo(dryRun.Missing.References, value)
	ts.Require().NotNil(reference, "the missing value was not said to be for anything")
	ts.Equal("organization_unit", reference.ResourceType)
	ts.Equal(id, reference.ResourceID)
	ts.Equal(name, reference.ResourceName)
	ts.Equal("description", reference.Field)

	var read referencingVersion
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet,
		"/configuration-versions/"+captured.Version, nil, &read))
	ts.NotNil(ts.referenceTo(read.References, value), "a captured version did not say where it refers to the value")

	ts.Equal(http.StatusConflict, ts.call(http.MethodPost, "/gateways/"+ts.gatewayID+"/apply",
		map[string]any{"version": captured.Version}, nil))
}
