// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package mgmtauthz holds the management API behaviour that is asked of every plane. It runs under
// make test_integration against the all-in-one server and under make test_integration_cp against
// the Control Plane, and each case names what each plane must answer. A rule that makes the two
// planes differ is one cell of the table; a case whose planes are expected to agree fails the plane
// that stops agreeing.
package mgmtauthz

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Expectation is the status each plane must answer a case with.
type Expectation struct {
	Data    int
	Control int
}

// Same expects the status from both planes.
func Same(status int) Expectation {
	return Expectation{Data: status, Control: status}
}

// Differ expects a different status from each plane.
func Differ(data, control int) Expectation {
	return Expectation{Data: data, Control: control}
}

// forPlane returns the status the given plane must answer with.
func (e Expectation) forPlane(plane testutils.Plane) int {
	if plane == testutils.ControlPlane {
		return e.Control
	}
	return e.Data
}

// Case is one request a persona sends and the status each plane answers it with. A case whose
// persona the run's plane does not provide is skipped there, so a persona only one plane has is
// still given the status the other plane would answer with.
type Case struct {
	Name    string
	Persona testutils.Persona
	Method  string
	Path    string
	Body    string
	Want    Expectation
}

var cases = []Case{
	// Management reads the administrator may make on every plane.
	{Name: "admin lists applications", Persona: testutils.PersonaAdmin, Method: http.MethodGet,
		Path: "/applications", Want: Same(http.StatusOK)},
	{Name: "admin lists flows", Persona: testutils.PersonaAdmin, Method: http.MethodGet,
		Path: "/flows", Want: Same(http.StatusOK)},
	{Name: "admin lists organization units", Persona: testutils.PersonaAdmin, Method: http.MethodGet,
		Path: "/organization-units", Want: Same(http.StatusOK)},

	// The runtime endpoints are the all-in-one server's. A Control Plane serves none of them.
	{Name: "admin reads the OpenID configuration", Persona: testutils.PersonaAdmin, Method: http.MethodGet,
		Path: "/.well-known/openid-configuration", Want: Differ(http.StatusOK, http.StatusNotFound)},

	// A token that lacks the system scope is authenticated and then refused. Only the Control Plane
	// provides that persona.
	{Name: "no-scope identity lists applications", Persona: testutils.PersonaNoScope, Method: http.MethodGet,
		Path: "/applications", Want: Same(http.StatusForbidden)},
}

type ManagementAuthzTestSuite struct {
	suite.Suite
}

func TestManagementAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(ManagementAuthzTestSuite))
}

func (ts *ManagementAuthzTestSuite) TestEachPlaneAnswersAsExpected() {
	plane := testutils.CurrentPlane()
	for _, c := range cases {
		ts.Run(c.Name, func() {
			client := testutils.RequirePersonaClient(ts.T(), c.Persona)

			req, err := http.NewRequest(c.Method, testutils.TestServerURL+c.Path, strings.NewReader(c.Body))
			ts.Require().NoError(err)
			if c.Body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := client.Do(req)
			ts.Require().NoError(err)
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(resp.Body)

			ts.Equal(c.Want.forPlane(plane), resp.StatusCode, "%s %s on the %s plane: %s", c.Method, c.Path, plane,
				string(body))
		})
	}
}
