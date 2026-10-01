// Copyright 2026 The ThunderID Authors
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
		"schemas":       []string{ts.extensionURN},
		ts.extensionURN: map[string]interface{}{"email": ts.memberUserName + "@example.com"},
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

// TestCreateAndGetGroupWithInitialMember verifies a group can be created with an initial member and then fetched.
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

// TestPatchAddThenRemoveMember verifies PATCH can add and then remove a member.
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

// TestPatchReplaceDisplayName verifies PATCH can replace displayName.
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

// TestPatchInvalidOpRejected verifies an unknown PATCH op is rejected.
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
func (ts *SCIMGroupsTestSuite) TestListWithFilterRejected() {
	status, _ := ts.createGroup("scim-it-group-filter", nil)
	ts.Require().Equal(http.StatusCreated, status)

	var err error
	status, _, err = scimRequest(http.MethodGet, `/Groups?filter=displayName+eq+%22scim-it-group-filter%22`, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "Groups filtering is not supported, unlike Users")
}

// TestDeleteGroupThenGetReturns404 verifies a deleted group is gone: a subsequent GET returns 404.
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

// TestCreateGroupMissingDisplayNameRejected verifies displayName is required to create a group.
func (ts *SCIMGroupsTestSuite) TestCreateGroupMissingDisplayNameRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas": []string{scimCoreGroupSchemaURN},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Groups", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "displayName is required to create a group")
}

// TestCreateGroupMissingSchemaURNRejected verifies the core Group schema URN is required to create a group.
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

// TestCreateGroupInvalidMemberTypeRejected verifies a member type other than User/Group is rejected.
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

// TestCreateGroupEmptyMemberValueRejected verifies an empty member value is rejected.
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

// TestCreateGroupNonexistentMemberRejected verifies a member value naming a nonexistent user is rejected.
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

// TestReplaceGroupIfMatchHeaderIgnored tests that a stale/bogus If-Match header is
// ignored rather than rejected, since ETag/version support is not implemented
// (ServiceProviderConfig advertises etag.supported=false).
func (ts *SCIMGroupsTestSuite) TestReplaceGroupIfMatchHeaderIgnored() {
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
	ts.Equal(http.StatusOK, status)
}

// TestPatchGroupMissingPatchOpSchemaRejected pins that a PATCH body whose
// "schemas" array does not declare the PatchOp schema URN is rejected
// (RFC 7644 §3.5.2), independent of whether the operations it carries are
// otherwise well-formed.
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

// ---------------------------------------------------------------------------
// List/request-shape edge cases
// ---------------------------------------------------------------------------

// TestListGroupsSortByRejected verifies sortBy/sortOrder is rejected when sorting is not supported.
func (ts *SCIMGroupsTestSuite) TestListGroupsSortByRejected() {
	status, _, err := scimRequest(http.MethodGet, "/Groups?sortBy=displayName", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "sortBy must be rejected when sorting is not supported")
}

// TestListGroupsZeroCountReturnsNoResources verifies count=0 (or negative) returns totalResults only, no Resources.
func (ts *SCIMGroupsTestSuite) TestListGroupsZeroCountReturnsNoResources() {
	status, created := ts.createGroup("scim-it-group-zero-count", nil)
	ts.Require().Equal(http.StatusCreated, status)
	_ = created

	status, body, err := scimRequest(http.MethodGet, "/Groups?count=-5", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var list scimGroupListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))
	ts.Empty(list.Resources, "count<=0 must return no Resources")
}

// TestCreateGroupWrongContentTypeRejected verifies a create request with the wrong Content-Type is rejected.
func (ts *SCIMGroupsTestSuite) TestCreateGroupWrongContentTypeRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-it-group-wrong-content-type",
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Groups", body, map[string]string{"Content-Type": "text/plain"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "wrong Content-Type must be rejected: %s", respBody)
}

// TestCreateGroupEmptyBodyRejected verifies an empty create body is rejected.
func (ts *SCIMGroupsTestSuite) TestCreateGroupEmptyBodyRejected() {
	status, _, err := scimRequest(http.MethodPost, "/Groups", []byte{}, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an empty body must be rejected")
}

// TestReplaceGroupWrongContentTypeRejected verifies a replace request with the wrong Content-Type is rejected.
func (ts *SCIMGroupsTestSuite) TestReplaceGroupWrongContentTypeRejected() {
	status, created := ts.createGroup("scim-it-group-replace-wrong-content-type", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "renamed",
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPut, "/Groups/"+id, body, map[string]string{"Content-Type": "text/plain"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "wrong Content-Type must be rejected: %s", respBody)
}

// TestReplaceGroupEmptyBodyRejected verifies an empty replace body is rejected.
func (ts *SCIMGroupsTestSuite) TestReplaceGroupEmptyBodyRejected() {
	status, created := ts.createGroup("scim-it-group-replace-empty-body", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(http.MethodPut, "/Groups/"+id, []byte{}, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an empty body must be rejected")
}

// TestReplaceGroupInvalidMemberTypeRejected verifies PUT rejects an invalid member type, same as POST.
func (ts *SCIMGroupsTestSuite) TestReplaceGroupInvalidMemberTypeRejected() {
	status, created := ts.createGroup("scim-it-group-replace-invalid-member-type", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-it-group-replace-invalid-member-type",
		"members": []map[string]interface{}{
			{"value": ts.memberUserID, "type": "Robot"},
		},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPut, "/Groups/"+id, body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "member type must be \"User\" or \"Group\"")
}

// TestPatchGroupWrongContentTypeRejected verifies a PATCH request with the wrong Content-Type is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupWrongContentTypeRejected() {
	status, created := ts.createGroup("scim-it-group-patch-wrong-content-type", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "replace", Path: "displayName", Value: "renamed"}},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody,
		map[string]string{"Content-Type": "text/plain"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "wrong Content-Type must be rejected: %s", respBody)
}

// TestPatchGroupEmptyBodyRejected verifies an empty PATCH body is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupEmptyBodyRejected() {
	status, created := ts.createGroup("scim-it-group-patch-empty-body", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(http.MethodPatch, "/Groups/"+id, []byte{}, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an empty body must be rejected")
}

// TestPatchGroupMalformedJSONRejected verifies a PATCH body that isn't valid JSON is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupMalformedJSONRejected() {
	status, created := ts.createGroup("scim-it-group-patch-malformed-json", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(http.MethodPatch, "/Groups/"+id, []byte(`{not valid json`), nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestGroupMemberTypeGroupAccepted verifies a Group-typed member is accepted, not just User.
func (ts *SCIMGroupsTestSuite) TestGroupMemberTypeGroupAccepted() {
	status, parent := ts.createGroup("scim-it-group-member-parent", nil)
	ts.Require().Equal(http.StatusCreated, status)
	parentID, _ := parent["id"].(string)

	member := map[string]interface{}{"value": parentID, "type": "Group"}
	status, created := ts.createGroup("scim-it-group-member-child", []map[string]interface{}{member})
	ts.Require().Equal(http.StatusCreated, status, "expected 201, got body: %v", created)

	var fetched scimGroup
	body, err := json.Marshal(created)
	ts.Require().NoError(err)
	ts.Require().NoError(json.Unmarshal(body, &fetched))
	ts.Require().Len(fetched.Members, 1)
	ts.Equal(parentID, fetched.Members[0].Value)
	ts.Equal("Group", fetched.Members[0].Type)
	ts.Contains(fetched.Members[0].Ref, "/Groups/"+parentID)
}

// TestReplaceGroupMembersChangesMembership verifies PUT with a disjoint member set removes the old member and adds the new one.
func (ts *SCIMGroupsTestSuite) TestReplaceGroupMembersChangesMembership() {
	member := map[string]interface{}{"value": ts.memberUserID, "type": "User"}
	status, created := ts.createGroup("scim-it-group-replace-membership", []map[string]interface{}{member})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	otherStatus, other := ts.createGroup("scim-it-group-replace-membership-other-member", nil)
	ts.Require().Equal(http.StatusCreated, otherStatus)
	otherID, _ := other["id"].(string)

	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-it-group-replace-membership",
		"members": []map[string]interface{}{
			{"value": otherID, "type": "Group"},
		},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPut, "/Groups/"+id, body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "replace failed: %s", respBody)

	var replaced scimGroup
	ts.Require().NoError(json.Unmarshal(respBody, &replaced))
	ts.Require().Len(replaced.Members, 1)
	ts.Equal(otherID, replaced.Members[0].Value)
}

// ---------------------------------------------------------------------------
// PATCH pathless / non-standard-path edge cases
// ---------------------------------------------------------------------------

// TestPatchGroupPathlessCombinedAttributes verifies a pathless PATCH can replace both displayName and members in one operation.
func (ts *SCIMGroupsTestSuite) TestPatchGroupPathlessCombinedAttributes() {
	status, created := ts.createGroup("scim-it-group-pathless-combined", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(map[string]interface{}{
		"schemas": []string{scimPatchOpSchemaURN},
		"Operations": []map[string]interface{}{
			{
				"op": "replace",
				"value": map[string]interface{}{
					"displayName": "scim-it-group-pathless-combined-renamed",
					"members": []map[string]interface{}{
						{"value": ts.memberUserID, "type": "User"},
					},
				},
			},
		},
	})
	ts.Require().NoError(err)

	status, body, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "pathless combined patch failed: %s", body)

	var patched scimGroup
	ts.Require().NoError(json.Unmarshal(body, &patched))
	ts.Equal("scim-it-group-pathless-combined-renamed", patched.DisplayName)
	ts.Require().Len(patched.Members, 1)
	ts.Equal(ts.memberUserID, patched.Members[0].Value)
}

// TestPatchGroupPathlessRemoveRejected verifies a pathless remove has no target and is rejected as noTarget.
func (ts *SCIMGroupsTestSuite) TestPatchGroupPathlessRemoveRejected() {
	status, created := ts.createGroup("scim-it-group-pathless-remove", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "remove"}},
	})
	ts.Require().NoError(err)

	status, body, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status)

	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("noTarget", errResp.ScimType)
}

// TestPatchGroupPathlessUnknownAttributeRejected verifies a pathless PATCH naming an unknown attribute is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupPathlessUnknownAttributeRejected() {
	status, created := ts.createGroup("scim-it-group-pathless-unknown-attr", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(map[string]interface{}{
		"schemas": []string{scimPatchOpSchemaURN},
		"Operations": []map[string]interface{}{
			{"op": "replace", "value": map[string]interface{}{"nickname": "x"}},
		},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestPatchGroupPathlessEmptyValueRejected verifies a pathless PATCH with an empty value object is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupPathlessEmptyValueRejected() {
	status, created := ts.createGroup("scim-it-group-pathless-empty-value", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(map[string]interface{}{
		"schemas": []string{scimPatchOpSchemaURN},
		"Operations": []map[string]interface{}{
			{"op": "replace", "value": map[string]interface{}{}},
		},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestPatchGroupUnknownPathRejected verifies a PATCH path that names neither displayName nor members is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupUnknownPathRejected() {
	status, created := ts.createGroup("scim-it-group-unknown-path", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "replace", Path: "nickname", Value: "x"}},
	})
	ts.Require().NoError(err)

	status, body, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status)

	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("invalidPath", errResp.ScimType)
}

// TestPatchGroupDisplayNameRemoveRejected verifies displayName cannot be removed via PATCH; it's required.
func (ts *SCIMGroupsTestSuite) TestPatchGroupDisplayNameRemoveRejected() {
	status, created := ts.createGroup("scim-it-group-displayname-remove", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "remove", Path: "displayName"}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "displayName is required and cannot be removed")
}

// TestPatchGroupDisplayNameEmptyValueRejected verifies a blank displayName value is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupDisplayNameEmptyValueRejected() {
	status, created := ts.createGroup("scim-it-group-displayname-empty-value", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "replace", Path: "displayName", Value: "  "}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a blank displayName must be rejected")
}

// ---------------------------------------------------------------------------
// PATCH members edge cases
// ---------------------------------------------------------------------------

// TestPatchGroupRemoveAllMembersNoFilter verifies PATCH remove on the members path with no filter clears all members.
func (ts *SCIMGroupsTestSuite) TestPatchGroupRemoveAllMembersNoFilter() {
	member := map[string]interface{}{"value": ts.memberUserID, "type": "User"}
	status, created := ts.createGroup("scim-it-group-remove-all-members", []map[string]interface{}{member})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "remove", Path: "members"}},
	})
	ts.Require().NoError(err)

	status, body, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "remove-all-members failed: %s", body)

	var patched scimGroup
	ts.Require().NoError(json.Unmarshal(body, &patched))
	ts.Empty(patched.Members)
}

// TestPatchGroupRemoveMembersWithUnexpectedValueRejected verifies remove members must not carry a value.
func (ts *SCIMGroupsTestSuite) TestPatchGroupRemoveMembersWithUnexpectedValueRejected() {
	status, created := ts.createGroup("scim-it-group-remove-members-unexpected-value", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:   "remove",
			Path: "members",
			Value: []map[string]interface{}{
				{"value": ts.memberUserID, "type": "User"},
			},
		}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "remove must not carry a value")
}

// TestPatchGroupRemoveFilteredMemberWithUnexpectedValueRejected verifies a filtered remove must not carry a value.
func (ts *SCIMGroupsTestSuite) TestPatchGroupRemoveFilteredMemberWithUnexpectedValueRejected() {
	member := map[string]interface{}{"value": ts.memberUserID, "type": "User"}
	status, created := ts.createGroup("scim-it-group-remove-filtered-unexpected-value", []map[string]interface{}{member})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:   "remove",
			Path: fmt.Sprintf(`members[value eq "%s"]`, ts.memberUserID),
			Value: []map[string]interface{}{
				{"value": ts.memberUserID, "type": "User"},
			},
		}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a filtered remove must not carry a value")
}

// TestPatchGroupFilteredAddRejected verifies add/replace do not support a filtered members[...] path.
func (ts *SCIMGroupsTestSuite) TestPatchGroupFilteredAddRejected() {
	status, created := ts.createGroup("scim-it-group-filtered-add", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:   "add",
			Path: fmt.Sprintf(`members[value eq "%s"]`, ts.memberUserID),
			Value: []map[string]interface{}{
				{"value": ts.memberUserID, "type": "User"},
			},
		}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a filtered path is not supported for add")
}

// TestPatchGroupMembersMalformedValueRejected verifies a non-array members value on add is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupMembersMalformedValueRejected() {
	status, created := ts.createGroup("scim-it-group-members-malformed-value", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "add", Path: "members", Value: "not-an-array"}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestPatchGroupAddEmptyMembersRejected verifies add with an empty members array is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupAddEmptyMembersRejected() {
	status, created := ts.createGroup("scim-it-group-add-empty-members", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op: "add", Path: "members", Value: []map[string]interface{}{},
		}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "add with an empty members array must be rejected")
}

// TestPatchGroupReplaceMembers verifies PATCH replace on the members path swaps the membership set.
func (ts *SCIMGroupsTestSuite) TestPatchGroupReplaceMembers() {
	member := map[string]interface{}{"value": ts.memberUserID, "type": "User"}
	status, created := ts.createGroup("scim-it-group-replace-members-patch", []map[string]interface{}{member})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	otherStatus, other := ts.createGroup("scim-it-group-replace-members-patch-other", nil)
	ts.Require().Equal(http.StatusCreated, otherStatus)
	otherID, _ := other["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:   "replace",
			Path: "members",
			Value: []map[string]interface{}{
				{"value": otherID, "type": "Group"},
			},
		}},
	})
	ts.Require().NoError(err)

	status, body, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "patch replace members failed: %s", body)

	var patched scimGroup
	ts.Require().NoError(json.Unmarshal(body, &patched))
	ts.Require().Len(patched.Members, 1)
	ts.Equal(otherID, patched.Members[0].Value)
}

// TestPatchGroupInvalidFilterPathRejected verifies a malformed members[...] filter path is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupInvalidFilterPathRejected() {
	status, created := ts.createGroup("scim-it-group-invalid-filter-path", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "remove", Path: `members(value eq "x")`}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestPatchGroupFilterPathWrongFieldRejected verifies the members filter must be on the "value" field.
func (ts *SCIMGroupsTestSuite) TestPatchGroupFilterPathWrongFieldRejected() {
	status, created := ts.createGroup("scim-it-group-filter-path-wrong-field", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "remove", Path: `members[type eq "User"]`}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "the filter must be on the \"value\" field")
}

// TestPatchGroupFilterPathEmptyValueRejected verifies an empty quoted value in the members filter is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupFilterPathEmptyValueRejected() {
	status, created := ts.createGroup("scim-it-group-filter-path-empty-value", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody, err := json.Marshal(scimPatchRequest{
		Schemas:    []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{Op: "remove", Path: `members[value eq ""]`}},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestPatchGroupInvalidOpTypeRejected verifies a PATCH body whose Operations field isn't an array is rejected.
func (ts *SCIMGroupsTestSuite) TestPatchGroupInvalidOpTypeRejected() {
	status, created := ts.createGroup("scim-it-group-invalid-op-type", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	patchBody := []byte(`{"schemas":["` + scimPatchOpSchemaURN + `"],"Operations":"not-an-array"}`)

	status, _, err := scimRequest(http.MethodPatch, "/Groups/"+id, patchBody, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestOptionsGroupsPreflightAccepted verifies the CORS preflight handler is wired for the Groups endpoint.
func (ts *SCIMGroupsTestSuite) TestOptionsGroupsPreflightAccepted() {
	status, _, err := scimRequest(http.MethodOptions, "/Groups", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNoContent, status)
}
