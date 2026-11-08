// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// RoleOUConfinementTestSuite verifies that the Control Plane does not confine a token holding only
// `system:role` to an organization unit: it reads and lists roles of every organization unit, and the
// unfiltered list carries each role once, under the organization unit that owns it.
type RoleOUConfinementTestSuite struct {
	suite.Suite

	ouRoot, ouChild, ouOther string
	sharedRole, otherRole    string

	roleClient *http.Client
}

func TestRoleOUConfinementTestSuite(t *testing.T) {
	suite.Run(t, new(RoleOUConfinementTestSuite))
}

func (s *RoleOUConfinementTestSuite) SetupSuite() {
	token := os.Getenv(testutils.RoleScopedTokenEnv)
	s.Require().NotEmpty(token, "the harness must export a system:role token")
	s.roleClient = testutils.GetHTTPClientWithToken(token)

	var err error
	s.ouRoot, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "cp-role-confinement-root", Name: "CP Role Confinement Root"})
	s.Require().NoError(err)
	s.ouChild, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "cp-role-confinement-child", Name: "CP Role Confinement Child", Parent: &s.ouRoot})
	s.Require().NoError(err)
	s.ouOther, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "cp-role-confinement-other", Name: "CP Role Confinement Other"})
	s.Require().NoError(err)

	s.sharedRole, err = testutils.CreateRole(testutils.Role{Name: "CP Role Confinement Shared", OUID: s.ouRoot})
	s.Require().NoError(err)
	s.otherRole, err = testutils.CreateRole(testutils.Role{Name: "CP Role Confinement Other", OUID: s.ouOther})
	s.Require().NoError(err)

	status, body := s.send(testutils.GetHTTPClient(), http.MethodPost,
		"/roles/"+s.sharedRole+"/sharing-policies", map[string]interface{}{"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]interface{}{{"ouId": s.ouChild, "reach": "unit"}},
		}})
	s.Require().Equal(http.StatusCreated, status, "share the role into the child: %s", string(body))
}

func (s *RoleOUConfinementTestSuite) TearDownSuite() {
	for _, id := range []string{s.sharedRole, s.otherRole} {
		if id != "" {
			s.NoError(testutils.DeleteRole(id), "teardown: delete role")
		}
	}
	for _, id := range []string{s.ouChild, s.ouRoot, s.ouOther} {
		if id != "" {
			s.NoError(testutils.DeleteOrganizationUnit(id), "teardown: delete organization unit")
		}
	}
}

func (s *RoleOUConfinementTestSuite) TestReadsRolesOfEveryOU() {
	for _, id := range []string{s.sharedRole, s.otherRole} {
		status, body := s.send(s.roleClient, http.MethodGet, "/roles/"+id, nil)
		s.Equal(http.StatusOK, status, "role %s: %s", id, string(body))
	}
}

func (s *RoleOUConfinementTestSuite) TestListsOriginalsOnly() {
	counts := map[string]int{}
	owners := map[string]string{}
	for offset := 0; ; offset += 100 {
		status, body := s.send(s.roleClient, http.MethodGet, fmt.Sprintf("/roles?limit=100&offset=%d", offset), nil)
		s.Require().Equal(http.StatusOK, status, string(body))
		var page struct {
			Roles []struct {
				ID   string `json:"id"`
				OUID string `json:"ouId"`
			} `json:"roles"`
		}
		s.Require().NoError(json.Unmarshal(body, &page))
		for _, r := range page.Roles {
			counts[r.ID]++
			owners[r.ID] = r.OUID
		}
		if len(page.Roles) < 100 {
			break
		}
	}

	s.Equal(1, counts[s.sharedRole], "a shared role is listed once")
	s.Equal(s.ouRoot, owners[s.sharedRole], "under the organization unit that owns it")
	s.Equal(1, counts[s.otherRole])
}

// send issues a request with the client and returns the status and body.
func (s *RoleOUConfinementTestSuite) send(client *http.Client, method, path string,
	body interface{}) (int, []byte) {
	s.T().Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, respBody
}
