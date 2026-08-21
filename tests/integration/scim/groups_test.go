// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// SCIMGroupsTestSuite exercises Group CRUD and PATCH — the mechanism a real
// IdP uses to sync role/entitlement membership (Groups is the only SCIM
// resource here with PATCH implemented; Users PATCH is covered as
// unsupported in SCIMDiscoveryTestSuite). One user is provisioned in
// SetupSuite purely as a member to add/remove; its own lifecycle is already
// covered by SCIMUsersTestSuite.
type SCIMGroupsTestSuite struct {
	suite.Suite
	ouID            string
	entityTypeID    string
	entityTypeName  string
	extensionURN    string
	memberUserID    string
	memberUserName  string
	createdGroupIDs []string
}

// TestSCIMGroupsTestSuite tests SCIM Groups Test Suite.
func TestSCIMGroupsTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMGroupsTestSuite))
}

// SetupSuite initializes the test suite environment.
func (ts *SCIMGroupsTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-it-groups-ou",
		Name:        "SCIM Groups Integration Test OU",
		Description: "Organization unit for SCIM Groups endpoint tests",
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	ts.entityTypeName = "scim-it-groups-person"
	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: ts.entityTypeName,
		OUID: ouID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{"type": "string", "required": true},
		},
	})
	ts.Require().NoError(err, "failed to create test entity type")
	ts.entityTypeID = entityTypeID

	urn, _, err := discoverExtensionSchema(ts.entityTypeName)
	ts.Require().NoError(err, "failed to discover extension schema via GET /Schemas")
	ts.extensionURN = urn

	ts.memberUserName = "scim.it.group-member"
	body, err := json.Marshal(map[string]interface{}{
		"schemas":  []string{scimCoreUserSchemaURN, ts.extensionURN},
		"userName": ts.memberUserName,
		"emails": []map[string]interface{}{
			{"value": ts.memberUserName + "@example.com", "type": "work"},
		},
		ts.extensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)
	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, status, "failed to provision the fixture member user: %s", respBody)

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	ts.memberUserID, _ = created["id"].(string)
	ts.Require().NotEmpty(ts.memberUserID)
}

// TearDownSuite cleans up the test suite environment.
func (ts *SCIMGroupsTestSuite) TearDownSuite() {
	for _, id := range ts.createdGroupIDs {
		_, _, _ = scimRequest(http.MethodDelete, "/Groups/"+id, nil, nil)
	}
	if ts.memberUserID != "" {
		_, _, _ = scimRequest(http.MethodDelete, "/Users/"+ts.memberUserID, nil, nil)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createGroup handles create group.
func (ts *SCIMGroupsTestSuite) createGroup(displayName string, members []map[string]interface{}) (int, map[string]interface{}) {
	payload := map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": displayName,
	}
	if members != nil {
		payload["members"] = members
	}
	body, err := json.Marshal(payload)
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Groups", body, nil)
	ts.Require().NoError(err)

	var resp map[string]interface{}
	if len(respBody) > 0 {
		ts.Require().NoError(json.Unmarshal(respBody, &resp))
	}
	if status == http.StatusCreated {
		id, _ := resp["id"].(string)
		ts.createdGroupIDs = append(ts.createdGroupIDs, id)
	}
	return status, resp
}

// TestCreateAndGetGroupWithInitialMember tests Create And Get Group With Initial Member.
func (ts *SCIMGroupsTestSuite) TestCreateAndGetGroupWithInitialMember() {
	member := map[string]interface{}{"value": ts.memberUserID, "display": ts.memberUserName, "type": "User"}
	status, created := ts.createGroup("scim-it-group-create", []map[string]interface{}{member})
	ts.Require().Equal(http.StatusCreated, status, "expected 201, got body: %v", created)
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)

	status, body, err := scimRequest(http.MethodGet, "/Groups/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var fetched scimGroup
	ts.Require().NoError(json.Unmarshal(body, &fetched))
	ts.Equal("scim-it-group-create", fetched.DisplayName)
	ts.Require().Len(fetched.Members, 1)
	ts.Equal(ts.memberUserID, fetched.Members[0].Value)
}

// TestPatchAddThenRemoveMember tests Patch Add Then Remove Member.
func (ts *SCIMGroupsTestSuite) TestPatchAddThenRemoveMember() {
	status, created := ts.createGroup("scim-it-group-patch", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	addBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:   "add",
			Path: "members",
			Value: []map[string]interface{}{
				{"value": ts.memberUserID, "display": ts.memberUserName, "type": "User"},
			},
		}},
	})
	ts.Require().NoError(err)
	status, body, err := scimRequest(http.MethodPatch, "/Groups/"+id, addBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "add member failed: %s", body)

	var afterAdd scimGroup
	ts.Require().NoError(json.Unmarshal(body, &afterAdd))
	ts.Require().Len(afterAdd.Members, 1)
	ts.Equal(ts.memberUserID, afterAdd.Members[0].Value)

	removeBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:   "remove",
			Path: fmt.Sprintf(`members[value eq "%s"]`, ts.memberUserID),
		}},
	})
	ts.Require().NoError(err)
	status, body, err = scimRequest(http.MethodPatch, "/Groups/"+id, removeBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "remove member failed: %s", body)

	var afterRemove scimGroup
	ts.Require().NoError(json.Unmarshal(body, &afterRemove))
	ts.Empty(afterRemove.Members)
}

// TestPatchReplaceDisplayName tests Patch Replace Display Name.
func (ts *SCIMGroupsTestSuite) TestPatchReplaceDisplayName() {
	status, created := ts.createGroup("scim-it-group-rename", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:    "replace",
			Path:  "displayName",
			Value: "scim-it-group-renamed",
		}},
	})
	ts.Require().NoError(err)
	status, body, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "rename failed: %s", body)

	var fetched scimGroup
	ts.Require().NoError(json.Unmarshal(body, &fetched))
	ts.Equal("scim-it-group-renamed", fetched.DisplayName)
}

// TestPatchInvalidOpRejected tests Patch Invalid Op Rejected.
func (ts *SCIMGroupsTestSuite) TestPatchInvalidOpRejected() {
	status, created := ts.createGroup("scim-it-group-invalid-op", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:    "upsert",
			Path:  "displayName",
			Value: "does-not-matter",
		}},
	})
	ts.Require().NoError(err)
	status, body, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status)

	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("invalidValue", errResp.ScimType)
}

// TestListWithFilterRejected pins a known limitation: unlike Users, Groups
// rejects any "filter" query param outright, even though
// ServiceProviderConfig currently advertises Filter.Supported=true for the
// service as a whole. A provisioning connector cannot rely on filtering
// Groups here — it must list and match client-side.
// TestListWithFilterRejected tests List With Filter Rejected.
func (ts *SCIMGroupsTestSuite) TestListWithFilterRejected() {
	status, _ := ts.createGroup("scim-it-group-filter", nil)
	ts.Require().Equal(http.StatusCreated, status)

	var err error
	status, _, err = scimRequest(http.MethodGet, `/Groups?filter=displayName+eq+%22scim-it-group-filter%22`, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "Groups filtering is not supported, unlike Users")
}

// TestDeleteGroupThenGetReturns404 tests Delete Group Then Get Returns 404.
func (ts *SCIMGroupsTestSuite) TestDeleteGroupThenGetReturns404() {
	status, created := ts.createGroup("scim-it-group-delete", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(http.MethodDelete, "/Groups/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusNoContent, status)

	status, _, err = scimRequest(http.MethodGet, "/Groups/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotFound, status)

	for i, cid := range ts.createdGroupIDs {
		if cid == id {
			ts.createdGroupIDs = append(ts.createdGroupIDs[:i], ts.createdGroupIDs[i+1:]...)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Create/Replace/Patch edge cases
// ---------------------------------------------------------------------------

// TestCreateGroupMissingDisplayNameRejected tests Create Group Missing Display Name Rejected.
func (ts *SCIMGroupsTestSuite) TestCreateGroupMissingDisplayNameRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas": []string{scimCoreGroupSchemaURN},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Groups", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "displayName is required to create a group")
}

// TestCreateGroupMissingSchemaURNRejected tests Create Group Missing Schema URN Rejected.
func (ts *SCIMGroupsTestSuite) TestCreateGroupMissingSchemaURNRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"displayName": "scim-it-group-missing-schema",
	})
	ts.Require().NoError(err)

	status, body2, err := scimRequest(http.MethodPost, "/Groups", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status)

	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body2, &errResp))
	ts.Equal("invalidValue", errResp.ScimType)
}

// TestCreateGroupInvalidMemberTypeRejected tests Create Group Invalid Member Type Rejected.
func (ts *SCIMGroupsTestSuite) TestCreateGroupInvalidMemberTypeRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-it-group-invalid-member-type",
		"members": []map[string]interface{}{
			{"value": ts.memberUserID, "type": "Robot"},
		},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Groups", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "member type must be \"User\" or \"Group\"")
}

// TestCreateGroupEmptyMemberValueRejected tests Create Group Empty Member Value Rejected.
func (ts *SCIMGroupsTestSuite) TestCreateGroupEmptyMemberValueRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-it-group-empty-member-value",
		"members": []map[string]interface{}{
			{"value": "", "type": "User"},
		},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Groups", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an empty member value must be rejected")
}

// TestCreateGroupNonexistentMemberRejected tests Create Group Nonexistent Member Rejected.
func (ts *SCIMGroupsTestSuite) TestCreateGroupNonexistentMemberRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-it-group-nonexistent-member",
		"members": []map[string]interface{}{
			{"value": "scim-it-does-not-exist", "type": "User"},
		},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Groups", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status)

	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(respBody, &errResp))
	ts.Equal("invalidValue", errResp.ScimType)
}

// TestReplaceGroupIfMatchMismatchRejected tests Replace Group If Match Mismatch Rejected.
func (ts *SCIMGroupsTestSuite) TestReplaceGroupIfMatchMismatchRejected() {
	status, created := ts.createGroup("scim-it-group-if-match", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-it-group-if-match-renamed",
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPut, "/Groups/"+id, body, map[string]string{"If-Match": `"bogus-etag"`})
	ts.Require().NoError(err)
	ts.Equal(http.StatusPreconditionFailed, status)
}

// TestPatchGroupMissingPatchOpSchemaRejected pins that a PATCH body whose
// "schemas" array does not declare the PatchOp schema URN is rejected
// (RFC 7644 §3.5.2), independent of whether the operations it carries are
// otherwise well-formed.
// TestPatchGroupMissingPatchOpSchemaRejected tests Patch Group Missing Patch Op Schema Rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupMissingPatchOpSchemaRejected() {
	status, created := ts.createGroup("scim-it-group-missing-patchop-schema", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{},
		Operations: []scimPatchOp{{
			Op:    "replace",
			Path:  "displayName",
			Value: "does-not-matter",
		}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}
