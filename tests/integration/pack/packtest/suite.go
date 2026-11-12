// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package packtest holds what the suites that run against the quickstart pack share: a base suite
// with a client for the Control Plane and one for the Data Plane, and the helpers that author on the
// Control Plane and wait for the sync.
package packtest

import (
	"fmt"
	"net/http"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// DefaultOUID is the organization unit the pack bootstraps on every plane.
	DefaultOUID = "01900000-0000-7000-8000-000000000001"
	// DefaultAuthFlowID is the login flow every default application of the pack signs in with.
	DefaultAuthFlowID = "01900000-0000-7000-8000-000000000068"
)

// Suite is the base of the suites that author on the Control Plane, wait for the sync, and prove the
// result on the Data Plane. Embed it in a suite and run that with suite.Run.
type Suite struct {
	suite.Suite
	CP *testutils.PackPlane
	DP *testutils.PackPlane
}

// SetupSuite gives the suite a client for each plane.
func (s *Suite) SetupSuite() {
	s.Require().True(testutils.IsPackRun(), "the pack suite runs through make test_integration_pack")
	s.CP = testutils.PackCP()
	s.DP = testutils.PackDP()
}

// Unique returns a name no other test, and no earlier run of this one, has used.
func Unique(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// Call sends a request to a plane and fails the test when it cannot be sent.
func (s *Suite) Call(plane *testutils.PackPlane, method, path string, in, out any) (int, []byte) {
	s.T().Helper()
	status, raw, err := plane.Do(method, path, in, out)
	s.Require().NoError(err, "%s %s", method, path)
	return status, raw
}

// Sync waits for the Data Plane to hold what the Control Plane holds.
func (s *Suite) Sync() {
	s.T().Helper()
	s.Require().NoError(testutils.PackSyncToDP())
}

// RemoveOnCleanup deletes a resource from the Control Plane when the test ends, and waits for the
// removal to reach the Data Plane so the next test starts from the same configuration.
func (s *Suite) RemoveOnCleanup(path string) {
	s.T().Cleanup(func() {
		s.Call(s.CP, http.MethodDelete, path, nil, nil)
		s.Assert().NoError(testutils.PackSyncToDP(), "the removal of %s did not reach the data plane", path)
	})
}

// CreateClientCredentialsApp creates a confidential application on the Control Plane and removes it,
// and the removal's sync, when the test ends.
func (s *Suite) CreateClientCredentialsApp(clientID, clientSecret string) string {
	s.T().Helper()
	var created struct {
		ID string `json:"id"`
	}
	status, raw := s.Call(s.CP, http.MethodPost, "/applications", map[string]any{
		"name":                      clientID,
		"description":               "Created on the control plane by the pack suite",
		"ouId":                      DefaultOUID,
		"type":                      "m2m",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId":                clientID,
				"clientSecret":            clientSecret,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	}, &created)
	s.Require().Equal(http.StatusCreated, status, string(raw))
	s.RemoveOnCleanup("/applications/" + created.ID)
	return created.ID
}
