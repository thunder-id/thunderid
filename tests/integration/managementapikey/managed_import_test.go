// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package managementapikey

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ManagedImportTestSuite deploys a group through the managed import, as a control plane does, and checks
// that this deployment keeps it read only: only a later deploy changes it.
type ManagedImportTestSuite struct {
	suite.Suite
	ouID     string
	deployed string
	// cpMember is a group the control plane sends as a member; dpMember one added here.
	cpMember string
	dpMember string
}

func TestManagedImportTestSuite(t *testing.T) {
	suite.Run(t, new(ManagedImportTestSuite))
}

func (ts *ManagedImportTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "managed-import-ou", Name: "Managed import OU",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	ts.cpMember, err = testutils.CreateGroup(testutils.Group{Name: "managed-import-cp-member", OUID: ouID})
	ts.Require().NoError(err)
	ts.dpMember, err = testutils.CreateGroup(testutils.Group{Name: "managed-import-dp-member", OUID: ouID})
	ts.Require().NoError(err)
	ts.deployed = newUUID(ts.T())
}

func (ts *ManagedImportTestSuite) TearDownSuite() {
	_, _ = ts.deploy(fmt.Sprintf(`{"deletions":[{"resourceType":"group","id":%q}]}`, ts.deployed))
	_ = testutils.DeleteGroup(ts.cpMember)
	_ = testutils.DeleteGroup(ts.dpMember)
	_ = testutils.DeleteOrganizationUnit(ts.ouID)
}

// One deploy, then every rule in order, since each step depends on what the one before left.
func (ts *ManagedImportTestSuite) TestADeployedGroupIsReadOnlyHere() {
	status, results := ts.deploy(ts.groupDocument(ts.cpMember))
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal("success", results[0]["status"], "%v", results[0])

	ts.Run("it reads as read only", func() {
		body := ts.admin(http.MethodGet, "/groups/"+ts.deployed, nil, http.StatusOK)
		var group struct {
			IsReadOnly bool `json:"isReadOnly"`
		}
		ts.Require().NoError(json.Unmarshal(body, &group))
		ts.True(group.IsReadOnly)
	})

	ts.Run("it is not updated or deleted here", func() {
		update := fmt.Sprintf(`{"name":"renamed","ouId":%q}`, ts.ouID)
		ts.Equal("GRP-1023", ts.code(ts.admin(http.MethodPut, "/groups/"+ts.deployed, []byte(update),
			http.StatusBadRequest)))
		ts.Equal("GRP-1023", ts.code(ts.admin(http.MethodDelete, "/groups/"+ts.deployed, nil,
			http.StatusBadRequest)))
	})

	ts.Run("its deployed member is not removed here", func() {
		ts.Equal("GRP-1024", ts.code(ts.admin(http.MethodPost, "/groups/"+ts.deployed+"/members/remove",
			members(ts.cpMember), http.StatusBadRequest)))
	})

	ts.Run("a member added here stays editable", func() {
		ts.admin(http.MethodPost, "/groups/"+ts.deployed+"/members/add", members(ts.dpMember), http.StatusOK)
		ts.admin(http.MethodPost, "/groups/"+ts.deployed+"/members/remove", members(ts.dpMember), http.StatusOK)
		ts.admin(http.MethodPost, "/groups/"+ts.deployed+"/members/add", members(ts.dpMember), http.StatusOK)
	})

	ts.Run("the plain import is refused it", func() {
		body := ts.admin(http.MethodPost, "/import",
			mustJSON(ts.T(), map[string]string{"content": ts.groupYAML(ts.cpMember)}), http.StatusOK)
		var response struct {
			Results []map[string]interface{} `json:"results"`
		}
		ts.Require().NoError(json.Unmarshal(body, &response))
		ts.Require().Len(response.Results, 1)
		ts.Equal("failed", response.Results[0]["status"])
		ts.Equal("GRP-1023", response.Results[0]["code"])
	})

	ts.Run("a deploy that drops a member removes it and keeps the one added here", func() {
		status, results := ts.deploy(ts.groupDocument())
		ts.Require().Equal(http.StatusOK, status)
		ts.Require().Equal("success", results[0]["status"], "%v", results[0])

		listed, err := testutils.GetGroupMembers(ts.deployed)
		ts.Require().NoError(err)
		ids := make([]string, 0, len(listed))
		for _, member := range listed {
			ids = append(ids, member.ID)
		}
		ts.Equal([]string{ts.dpMember}, ids)
	})

	ts.Run("a deploy removes it", func() {
		status, results := ts.deploy(fmt.Sprintf(`{"deletions":[{"resourceType":"group","id":%q}]}`,
			ts.deployed))
		ts.Require().Equal(http.StatusOK, status)
		ts.Require().Equal("success", results[0]["status"], "%v", results[0])
		ts.admin(http.MethodGet, "/groups/"+ts.deployed, nil, http.StatusNotFound)
	})
}

func (ts *ManagedImportTestSuite) groupYAML(memberIDs ...string) string {
	document := fmt.Sprintf("resource_type: group\nid: %s\nname: managed-import-group\nouId: %s\n",
		ts.deployed, ts.ouID)
	if len(memberIDs) > 0 {
		document += "members:\n"
		for _, id := range memberIDs {
			document += fmt.Sprintf("  - id: %s\n    type: group\n", id)
		}
	}
	return document
}

func (ts *ManagedImportTestSuite) groupDocument(memberIDs ...string) string {
	return string(mustJSON(ts.T(), map[string]string{"content": ts.groupYAML(memberIDs...)}))
}

// A deploy that changes a deployed group updates it, though the group is read only here.
func (ts *ManagedImportTestSuite) TestADeployUpdatesADeployedGroup() {
	id := newUUID(ts.T())
	document := func(name string) string {
		return string(mustJSON(ts.T(), map[string]string{"content": fmt.Sprintf(
			"resource_type: group\nid: %s\nname: %s\nouId: %s\n", id, name, ts.ouID)}))
	}
	ts.T().Cleanup(func() {
		_, _ = ts.deploy(fmt.Sprintf(`{"deletions":[{"resourceType":"group","id":%q}]}`, id))
	})

	status, results := ts.deploy(document("managed-import-first"))
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal("success", results[0]["status"], "%v", results[0])
	status, results = ts.deploy(document("managed-import-second"))
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal("success", results[0]["status"], "%v", results[0])

	var group struct {
		Name       string `json:"name"`
		IsReadOnly bool   `json:"isReadOnly"`
	}
	ts.Require().NoError(json.Unmarshal(ts.admin(http.MethodGet, "/groups/"+id, nil, http.StatusOK), &group))
	ts.Equal("managed-import-second", group.Name)
	ts.True(group.IsReadOnly)
}

// An agent type a control plane deployed reads as read only and is not changed here: it is marked as an agent
// type, which is what the agent type API checks.
func (ts *ManagedImportTestSuite) TestADeployedAgentTypeIsReadOnlyHere() {
	id := newUUID(ts.T())
	document := string(mustJSON(ts.T(), map[string]string{"content": fmt.Sprintf(
		"resource_type: agent_type\nid: %s\nhandle: managed-import-agent-type\ndisplayName: Managed import\n"+
			"ouId: %s\nschema:\n  name:\n    type: string\n", id, ts.ouID)}))
	ts.T().Cleanup(func() {
		_, _ = ts.deploy(fmt.Sprintf(`{"deletions":[{"resourceType":"agent_type","id":%q}]}`, id))
	})

	status, results := ts.deploy(document)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal("success", results[0]["status"], "%v", results[0])

	var agentType struct {
		IsReadOnly bool `json:"isReadOnly"`
	}
	ts.Require().NoError(json.Unmarshal(ts.admin(http.MethodGet, "/agent-types/"+id, nil, http.StatusOK),
		&agentType))
	ts.True(agentType.IsReadOnly)

	update := fmt.Sprintf(`{"displayName":"Changed","ouId":%q,"schema":{"name":{"type":"string"}}}`, ts.ouID)
	ts.Equal("USRS-1023", ts.code(ts.admin(http.MethodPut, "/agent-types/"+id, []byte(update),
		http.StatusForbidden)))
}

// A group created through the API here has nothing marking it, so it stays editable.
func (ts *ManagedImportTestSuite) TestAGroupCreatedHereStaysEditable() {
	id, err := testutils.CreateGroup(testutils.Group{Name: "managed-import-local", OUID: ts.ouID})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteGroup(id) })

	var group struct {
		IsReadOnly bool `json:"isReadOnly"`
	}
	ts.Require().NoError(json.Unmarshal(ts.admin(http.MethodGet, "/groups/"+id, nil, http.StatusOK), &group))
	ts.False(group.IsReadOnly)

	update := fmt.Sprintf(`{"name":"managed-import-local-renamed","ouId":%q}`, ts.ouID)
	ts.admin(http.MethodPut, "/groups/"+id, []byte(update), http.StatusOK)
}

// The plain import with the key, as a pipeline that deploys editable resources calls it, marks nothing:
// what it writes is editable here.
func (ts *ManagedImportTestSuite) TestAGroupWrittenByThePlainImportStaysEditable() {
	id := newUUID(ts.T())
	content := fmt.Sprintf("resource_type: group\nid: %s\nname: managed-import-plain\nouId: %s\n", id, ts.ouID)
	ts.T().Cleanup(func() { _ = testutils.DeleteGroup(id) })

	status, results := ts.send("/import", string(mustJSON(ts.T(), map[string]string{"content": content})))
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal("success", results[0]["status"], "%v", results[0])

	var group struct {
		IsReadOnly bool `json:"isReadOnly"`
	}
	ts.Require().NoError(json.Unmarshal(ts.admin(http.MethodGet, "/groups/"+id, nil, http.StatusOK), &group))
	ts.False(group.IsReadOnly)
	update := fmt.Sprintf(`{"name":"managed-import-plain-renamed","ouId":%q}`, ts.ouID)
	ts.admin(http.MethodPut, "/groups/"+id, []byte(update), http.StatusOK)
}

// deploy sends a body to the managed import with the management API key, as a control plane does.
func (ts *ManagedImportTestSuite) deploy(body string) (int, []map[string]interface{}) {
	return ts.send(managedImportPath, body)
}

// send posts a body to an import path with the management API key.
func (ts *ManagedImportTestSuite) send(path, body string) (int, []map[string]interface{}) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodPost, testServerURL+path, bytes.NewReader([]byte(body)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(apiKeyHeader, managementAPIKey)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	var response struct {
		Results []map[string]interface{} `json:"results"`
	}
	payload, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	if resp.StatusCode == http.StatusOK {
		ts.Require().NoError(json.Unmarshal(payload, &response), string(payload))
	}
	return resp.StatusCode, response.Results
}

// admin calls the API as an administrator and checks the status it answers with.
func (ts *ManagedImportTestSuite) admin(method, path string, body []byte, want int) []byte {
	ts.T().Helper()
	req, err := http.NewRequest(method, testServerURL+path, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(want, resp.StatusCode, "%s %s: %s", method, path, payload)
	return payload
}

func (ts *ManagedImportTestSuite) code(body []byte) string {
	var failure struct {
		Code string `json:"code"`
	}
	ts.Require().NoError(json.Unmarshal(body, &failure), string(body))
	return failure.Code
}

func members(ids ...string) []byte {
	type member struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	list := make([]member, 0, len(ids))
	for _, id := range ids {
		list = append(list, member{ID: id, Type: "group"})
	}
	encoded, _ := json.Marshal(map[string]interface{}{"members": list})
	return encoded
}

func mustJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return encoded
}

// errorCode reads the code of an error response.
func errorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	var failure struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&failure))
	return failure.Code
}

// newUUID makes a version 4 UUID for the deployed group's id, which a control plane chooses.
func newUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	_, err := rand.Read(b)
	require.NoError(t, err)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
