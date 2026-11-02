// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// This file extends GroupAPITestSuite with coverage for the `include=display` query parameter and
// for application-typed members. Both feed the same resolution path in the group service: OU
// handles are resolved for group payloads, while member displays are resolved per member category
// (user displays come from the user type's display attribute, app displays from the app name, and
// group displays from the group name).

// testUserDisplay returns the display expected for testUserID. The user type resolves displays
// through its `email` system attribute, so the expectation is read back off the fixture itself.
func (suite *GroupAPITestSuite) testUserDisplay() string {
	suite.T().Helper()

	var attributes struct {
		Email string `json:"email"`
	}
	suite.Require().NoError(json.Unmarshal(testUser.Attributes, &attributes))
	return attributes.Email
}

// TestListGroupsWithDisplay verifies `include=display` populates the OU handle on every listed group.
func (suite *GroupAPITestSuite) TestListGroupsWithDisplay() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups?limit=100&include=display", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResponse GroupListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResponse))

	found := false
	for _, group := range listResponse.Groups {
		if group.Id == createdGroupID {
			found = true
			suite.Equal(testOU.Handle, group.OUHandle, "OU handle should be resolved with include=display")
		}
	}
	suite.True(found, "Created group should be in the list")
}

// TestListGroupsWithoutDisplayOmitsOUHandle verifies the OU handle is only resolved on request.
func (suite *GroupAPITestSuite) TestListGroupsWithoutDisplayOmitsOUHandle() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups?limit=100", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResponse GroupListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResponse))

	found := false
	for _, group := range listResponse.Groups {
		if group.Id == createdGroupID {
			found = true
		}
		suite.Emptyf(group.OUHandle, "OU handle should be omitted without include=display for group %s", group.Id)
	}
	suite.True(found, "Created group should be in the list")
}

// TestGetGroupWithDisplay verifies `include=display` resolves the OU handle on the single-group
// endpoint, and that it is omitted otherwise.
func (suite *GroupAPITestSuite) TestGetGroupWithDisplay() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"?include=display", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var group Group
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&group))

	suite.Equal(testOU.Handle, group.OUHandle, "OU handle should be resolved with include=display")

	plainReq, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID, nil)
	suite.Require().NoError(err)

	plainResp, err := client.Do(plainReq)
	suite.Require().NoError(err)
	defer plainResp.Body.Close()

	suite.Require().Equal(http.StatusOK, plainResp.StatusCode)

	var plainGroup Group
	suite.Require().NoError(json.NewDecoder(plainResp.Body).Decode(&plainGroup))

	suite.Empty(plainGroup.OUHandle, "OU handle should be omitted without include=display")
}

// TestGetGroupMembersWithDisplay verifies member displays are resolved per member category: a user
// member resolves through its user type's display attribute, a group member through the group name.
func (suite *GroupAPITestSuite) TestGetGroupMembersWithDisplay() {
	memberGroupID, err := createGroup(CreateGroupRequest{
		Name:    "Display Member Group",
		OUID:    testOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(memberGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up member group: %v", deleteErr)
		}
	}()

	parentGroupID, err := createGroup(CreateGroupRequest{
		Name: "Display Parent Group",
		OUID: testOUID,
		Members: []Member{
			{Id: testUserID, Type: MemberTypeUser},
			{Id: memberGroupID, Type: MemberTypeGroup},
		},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(parentGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up parent group: %v", deleteErr)
		}
	}()

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/"+parentGroupID+"/members?include=display", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var memberList MemberListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&memberList))

	displayByID := make(map[string]string, len(memberList.Members))
	for _, member := range memberList.Members {
		displayByID[member.Id] = member.Display
	}

	suite.Equal(suite.testUserDisplay(), displayByID[testUserID],
		"user member display should come from the user type's display attribute")
	suite.Equal("Display Member Group", displayByID[memberGroupID],
		"group member display should be the group name")
}

// TestGetGroupMembersWithoutDisplayOmitsDisplay verifies member displays are only resolved on request.
func (suite *GroupAPITestSuite) TestGetGroupMembersWithoutDisplayOmitsDisplay() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var memberList MemberListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&memberList))

	found := false
	for _, member := range memberList.Members {
		if member.Id == testUserID {
			found = true
		}
		suite.Emptyf(member.Display, "Display should be omitted without include=display for member %s", member.Id)
	}
	suite.True(found, "The test user should be a member of the group")
}

// TestGetGroupsByPathWithDisplay verifies `include=display` populates the OU handle on the
// path-based listing.
func (suite *GroupAPITestSuite) TestGetGroupsByPathWithDisplay() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/tree/"+testOU.Handle+"?include=display", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResponse GroupListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResponse))

	suite.Require().NotEmpty(listResponse.Groups, "The test OU should contain at least one group")
	for _, group := range listResponse.Groups {
		suite.Equal(testOU.Handle, group.OUHandle, "OU handle should be resolved with include=display")
	}
}

// TestCreateGroupWithApplicationMember verifies an application can be a group member and that its
// display name resolves to the application name rather than its ID.
func (suite *GroupAPITestSuite) TestCreateGroupWithApplicationMember() {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:        "Group Member Application",
		Description: "Application added as a group member",
		OUID:        testOUID,
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteApplication(appID); deleteErr != nil {
			suite.T().Logf("Failed to clean up application: %v", deleteErr)
		}
	}()

	appGroupID, err := createGroup(CreateGroupRequest{
		Name: "Group with Application Member",
		OUID: testOUID,
		Members: []Member{
			{Id: appID, Type: MemberTypeApp},
		},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(appGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up group: %v", deleteErr)
		}
	}()

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/"+appGroupID+"/members?include=display", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var memberList MemberListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&memberList))

	suite.Require().Len(memberList.Members, 1)
	suite.Equal(appID, memberList.Members[0].Id)
	suite.Equal(MemberTypeApp, memberList.Members[0].Type,
		"an application member should be returned with the public 'app' type")
	suite.Equal("Group Member Application", memberList.Members[0].Display,
		"application member display should be the application name")
}
