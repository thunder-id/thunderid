// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// GroupDeclarativeAPITestSuite validates how the group API treats YAML-declared groups while the
// group store runs in composite mode (the product default). Declarative groups live in the
// file-based store alongside runtime groups in the database: reads merge both stores, while every
// write against a declarative group must be rejected as immutable. The fixture IDs live in model.go.
const (
	declGroupClientID    = "CONSOLE"
	declGroupRedirectURI = "https://localhost:8095/console"
)

type GroupDeclarativeAPITestSuite struct {
	suite.Suite
}

func TestGroupDeclarativeAPITestSuite(t *testing.T) {
	suite.Run(t, new(GroupDeclarativeAPITestSuite))
}

// doDeclarative issues a request against the group API using the admin client.
func (suite *GroupDeclarativeAPITestSuite) doDeclarative(method, path string, body []byte) *http.Response {
	suite.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, testServerURL+path, bodyReader)
	suite.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	return resp
}

// TestGetDeclarativeGroup verifies a YAML-declared group is served from the file-based store and
// is flagged read-only.
func (suite *GroupDeclarativeAPITestSuite) TestGetDeclarativeGroup() {
	resp := suite.doDeclarative(http.MethodGet, "/groups/"+declGroupID, nil)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var group Group
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&group))

	suite.Equal(declGroupID, group.Id)
	suite.Equal(declGroupName, group.Name)
	suite.Equal(declGroupOUID, group.OUID)
	suite.True(group.IsReadOnly, "a YAML-declared group must be reported as read-only")
}

// TestListGroupsMergesDeclarativeAndDatabaseGroups verifies the composite store returns groups from
// both the file-based store and the database in a single listing.
func (suite *GroupDeclarativeAPITestSuite) TestListGroupsMergesDeclarativeAndDatabaseGroups() {
	dbGroupID, err := createGroup(CreateGroupRequest{
		Name:    "Declarative Merge Probe Group",
		OUID:    declGroupOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(dbGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up probe group: %v", deleteErr)
		}
	}()

	resp := suite.doDeclarative(http.MethodGet, "/groups?limit=100", nil)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResponse GroupListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResponse))

	readOnlyByID := make(map[string]bool, len(listResponse.Groups))
	ids := make([]string, 0, len(listResponse.Groups))
	for _, g := range listResponse.Groups {
		ids = append(ids, g.Id)
		readOnlyByID[g.Id] = g.IsReadOnly
	}

	suite.Containsf(ids, declGroupID, "listing must include the declarative group, got %v", ids)
	suite.Containsf(ids, dbGroupID, "listing must include the database group, got %v", ids)
	suite.True(readOnlyByID[declGroupID], "the declarative group must be flagged read-only")
	suite.False(readOnlyByID[dbGroupID], "a database-backed group must not be flagged read-only")
}

// TestGetDeclarativeGroupMembers verifies members declared in YAML are served through the members
// endpoint, for both the nested group member of the outer group and the entity member of the inner.
func (suite *GroupDeclarativeAPITestSuite) TestGetDeclarativeGroupMembers() {
	outerResp := suite.doDeclarative(http.MethodGet, "/groups/"+declGroupID+"/members", nil)
	defer outerResp.Body.Close()

	suite.Require().Equal(http.StatusOK, outerResp.StatusCode)

	var outerMembers MemberListResponse
	suite.Require().NoError(json.NewDecoder(outerResp.Body).Decode(&outerMembers))

	suite.Require().Equal(1, outerMembers.TotalResults, "the outer group declares one member")
	suite.Require().Len(outerMembers.Members, 1, "the declared member must be returned in the listing")
	suite.Equal(declNestedGroupID, outerMembers.Members[0].Id)
	suite.Equal(MemberTypeGroup, outerMembers.Members[0].Type, "the nested group member must be returned")

	nestedResp := suite.doDeclarative(http.MethodGet, "/groups/"+declNestedGroupID+"/members", nil)
	defer nestedResp.Body.Close()

	suite.Require().Equal(http.StatusOK, nestedResp.StatusCode)

	var nestedMembers MemberListResponse
	suite.Require().NoError(json.NewDecoder(nestedResp.Body).Decode(&nestedMembers))

	suite.Require().Equal(1, nestedMembers.TotalResults, "the nested group declares one member")
	suite.Require().Len(nestedMembers.Members, 1, "the declared member must be returned in the listing")
	suite.Equal(declGroupMemberID, nestedMembers.Members[0].Id)
	suite.Equal(MemberTypeUser, nestedMembers.Members[0].Type,
		"the declared entity member must resolve to its public 'user' type")
}

// TestDeclarativeGroupMemberDisplayResolves verifies that resolving display names for a runtime
// group's members reaches into the file-based store, so a nested declarative group is shown by name
// rather than by ID.
func (suite *GroupDeclarativeAPITestSuite) TestDeclarativeGroupMemberDisplayResolves() {
	hostID, err := createGroup(CreateGroupRequest{
		Name: "Runtime Host For Declarative Member",
		OUID: declGroupOUID,
		Members: []Member{
			{Id: declNestedGroupID, Type: MemberTypeGroup},
		},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(hostID); deleteErr != nil {
			suite.T().Logf("Failed to clean up host group: %v", deleteErr)
		}
	}()

	resp := suite.doDeclarative(http.MethodGet, "/groups/"+hostID+"/members?include=display", nil)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var memberList MemberListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&memberList))

	suite.Require().Len(memberList.Members, 1)
	suite.Equal(declNestedGroupID, memberList.Members[0].Id)
	suite.Equal(declNestedGroupName, memberList.Members[0].Display,
		"a declarative group member should display its declared name")
}

// TestExportAllGroupsIncludesDeclarativeGroups verifies the wildcard group export enumerates every
// group the service can list, including the YAML-declared ones.
func (suite *GroupDeclarativeAPITestSuite) TestExportAllGroupsIncludesDeclarativeGroups() {
	payload, err := json.Marshal(map[string]any{"groups": []string{"*"}})
	suite.Require().NoError(err)

	resp := suite.doDeclarative(http.MethodPost, "/export", payload)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	content := string(body)
	suite.Contains(content, declGroupName, "wildcard export should include the declarative group")
	suite.Contains(content, declNestedGroupName, "wildcard export should include the nested declarative group")
}

// TestUpdateDeclarativeGroupIsRejected verifies a YAML-declared group cannot be updated.
func (suite *GroupDeclarativeAPITestSuite) TestUpdateDeclarativeGroupIsRejected() {
	payload, err := json.Marshal(UpdateGroupRequest{
		Name: "Renamed Declarative Group",
		OUID: declGroupOUID,
	})
	suite.Require().NoError(err)

	resp := suite.doDeclarative(http.MethodPut, "/groups/"+declGroupID, payload)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	var errorResp ErrorResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&errorResp))

	suite.Equal("GRP-1015", errorResp.Code)
	suite.Equal("Cannot modify declarative group", errorResp.Message.DefaultValue)
}

// TestDeleteDeclarativeGroupIsRejected verifies a YAML-declared group cannot be deleted.
func (suite *GroupDeclarativeAPITestSuite) TestDeleteDeclarativeGroupIsRejected() {
	resp := suite.doDeclarative(http.MethodDelete, "/groups/"+declGroupID, nil)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	var errorResp ErrorResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&errorResp))

	suite.Equal("GRP-1015", errorResp.Code)
	suite.Equal("Cannot modify declarative group", errorResp.Message.DefaultValue)

	// The group must still be retrievable after the rejected delete.
	getResp := suite.doDeclarative(http.MethodGet, "/groups/"+declGroupID, nil)
	defer getResp.Body.Close()
	suite.Equal(http.StatusOK, getResp.StatusCode)
}

// TestGetDeclarativeGroupsByPath verifies declarative groups are listed under their OU handle path.
func (suite *GroupDeclarativeAPITestSuite) TestGetDeclarativeGroupsByPath() {
	resp := suite.doDeclarative(http.MethodGet, "/groups/tree/"+declGroupOUHandle+"?limit=100", nil)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResponse GroupListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResponse))

	ids := make([]string, 0, len(listResponse.Groups))
	for _, g := range listResponse.Groups {
		ids = append(ids, g.Id)
	}

	suite.Containsf(ids, declGroupID, "tree listing must include the declarative group, got %v", ids)
	suite.Containsf(ids, declNestedGroupID, "tree listing must include the nested declarative group, got %v", ids)
}

// TestCreateGroupConflictingWithDeclarativeName verifies the name-conflict check spans the
// file-based store, so a runtime group cannot shadow a declarative one in the same OU.
func (suite *GroupDeclarativeAPITestSuite) TestCreateGroupConflictingWithDeclarativeName() {
	payload, err := json.Marshal(CreateGroupRequest{
		Name:    declGroupName,
		OUID:    declGroupOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)

	resp := suite.doDeclarative(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	// If the conflict check ever stops spanning the file store the group is created instead, and
	// leaving it behind would shadow the declarative group in every later composite listing.
	if resp.StatusCode == http.StatusCreated {
		var created Group
		if decodeErr := json.NewDecoder(resp.Body).Decode(&created); decodeErr == nil && created.Id != "" {
			defer func() {
				if deleteErr := deleteGroup(created.Id); deleteErr != nil {
					suite.T().Logf("Failed to clean up conflicting group: %v", deleteErr)
				}
			}()
		}
	}

	suite.Require().Equal(http.StatusConflict, resp.StatusCode)

	var errorResp ErrorResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&errorResp))

	suite.Equal("GRP-1004", errorResp.Code)
	suite.Equal("Group name conflict", errorResp.Message.DefaultValue)
}

// TestDeclarativeUserInheritsPermissionThroughNestedGroups verifies permission resolution walks the
// declarative nesting chain: decl-user-1 is a member only of decl-group-2, which is nested inside
// decl-group-1, and only decl-group-1 carries the role. The user therefore inherits the permission
// transitively, which requires the file-based store to walk from the member's group up to its parent.
func (suite *GroupDeclarativeAPITestSuite) TestDeclarativeUserInheritsPermissionThroughNestedGroups() {
	const (
		rsIdentifier = "https://declarative-group-test.example.com/inherit"
		grantedScope = "system:group:view"
	)

	rsID, err := testutils.CreateSystemScopedResourceServer(
		declGroupOUID, "Declarative Inheritance RS", rsIdentifier, "group")
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteResourceServer(rsID); deleteErr != nil {
			suite.T().Logf("Failed to clean up resource server: %v", deleteErr)
		}
	}()

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Declarative Inheritance Role",
		OUID: declGroupOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{grantedScope}},
		},
		Assignments: []testutils.Assignment{
			{ID: declGroupID, Type: "group"},
		},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteRole(roleID); deleteErr != nil {
			suite.T().Logf("Failed to clean up role: %v", deleteErr)
		}
	}()

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		declGroupClientID,
		declGroupRedirectURI,
		grantedScope,
		declGroupMemberID,
		declGroupMemberPassword,
		true,
		"",
		rsIdentifier,
	)
	suite.Require().NoError(err, "the declarative user should be able to authenticate")
	suite.Require().NotEmpty(tokenResp.AccessToken)

	suite.Contains(tokenResp.Scope, grantedScope,
		"the permission held by the outer declarative group should reach a member of the nested group")
}

// TestCreateGroupWithDeclarativeGroupMember verifies a runtime group can nest a declarative group,
// which requires group ID validation to consult the file-based store.
func (suite *GroupDeclarativeAPITestSuite) TestCreateGroupWithDeclarativeGroupMember() {
	payload, err := json.Marshal(CreateGroupRequest{
		Name: "Runtime Group Nesting Declarative Group",
		OUID: declGroupOUID,
		Members: []Member{
			{Id: declNestedGroupID, Type: MemberTypeGroup},
		},
	})
	suite.Require().NoError(err)

	resp := suite.doDeclarative(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusCreated, resp.StatusCode)

	var created Group
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	defer func() {
		if deleteErr := deleteGroup(created.Id); deleteErr != nil {
			suite.T().Logf("Failed to clean up group: %v", deleteErr)
		}
	}()

	suite.Require().Len(created.Members, 1)
	suite.Equal(declNestedGroupID, created.Members[0].Id)
	suite.Equal(MemberTypeGroup, created.Members[0].Type)
}
