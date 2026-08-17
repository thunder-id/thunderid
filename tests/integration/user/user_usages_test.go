// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// resourceDependency mirrors one entry of the usages listing.
type resourceDependency struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

// dependenciesResponse mirrors the GET /users/{id}/usages payload. TotalResults is a pointer
// because nil means "dependency data unavailable", which is a different answer from zero.
type dependenciesResponse struct {
	TotalResults *int                 `json:"totalResults"`
	Count        int                  `json:"count"`
	Summary      map[string]int       `json:"summary"`
	Usages       []resourceDependency `json:"usages"`
}

// usagesErrorResponse mirrors an API error, including the interpolated description parameters.
type usagesErrorResponse struct {
	Code    string `json:"code"`
	Message struct {
		Key          string `json:"key"`
		DefaultValue string `json:"defaultValue"`
	} `json:"message"`
	Description struct {
		Key          string            `json:"key"`
		DefaultValue string            `json:"defaultValue"`
		Params       map[string]string `json:"params"`
	} `json:"description"`
}

// UserUsagesTestSuite covers GET /users/{id}/usages and the delete refusal it warns about.
//
// The endpoint is the pre-delete safety check the console consults before offering to delete a
// user: it reports the resources that reference the user and whether each one blocks deletion. A
// false "no blockers" would let an operator confirm a delete that the server then refuses, or worse,
// walk into a delete whose blocking dependants were never surfaced.
//
// An agent's owner is a restrict-behavior reference, so an owned agent is the realistic blocking
// dependency to test with. It is built through the real agent API rather than injected, because the
// owner lives in the agent entity's system attributes and is resolved by scanning agents, not by an
// indexed lookup.
//
// The suite asserts the informational read and the enforcement together, since they are the two
// halves of one contract: what /usages reports must be what DELETE actually does.
type UserUsagesTestSuite struct {
	suite.Suite

	ouID       string
	userTypeID string

	ownerUserID    string
	unusedUserID   string
	firstAgentID   string
	secondAgentID  string
	agentsRemoved  bool
	ownerRemovable bool
}

const (
	usagesOUHandle     = "user-usages-ou"
	usagesUserTypeName = "user-usages-person"

	usagesOwnerUsername  = "user-usages-owner"
	usagesUnusedUsername = "user-usages-unused"

	// errCodeUserHasBlockingDependencies is returned when a user cannot be deleted because a
	// restrict-behavior dependant still references it.
	errCodeUserHasBlockingDependencies = "USR-1027"
)

func TestUserUsagesTestSuite(t *testing.T) {
	suite.Run(t, new(UserUsagesTestSuite))
}

func (ts *UserUsagesTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      usagesOUHandle,
		Name:        "User Usages OU",
		Description: "Organization unit for the user usages tests",
	})
	ts.Require().NoError(err, "Failed to create the test organization unit")
	ts.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: usagesUserTypeName,
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "Failed to create the user type")
	ts.userTypeID = userTypeID

	// The server allows a single `default` agent type, shared across suites and never deleted.
	_, err = testutils.CreateAgentType(testutils.UserType{
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to create the agent type")

	ownerID, err := testutils.CreateUser(testutils.User{
		Type: usagesUserTypeName,
		OUID: ts.ouID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": "Usages@123"}`, usagesOwnerUsername)),
	})
	ts.Require().NoError(err, "Failed to create the owner user")
	ts.ownerUserID = ownerID

	unusedID, err := testutils.CreateUser(testutils.User{
		Type: usagesUserTypeName,
		OUID: ts.ouID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": "Usages@123"}`, usagesUnusedUsername)),
	})
	ts.Require().NoError(err, "Failed to create the unreferenced user")
	ts.unusedUserID = unusedID

	// Two agents, so the summary counts and the "N agent(s)" rendering are exercised on a value
	// greater than one. A single dependant would not distinguish a count from a boolean.
	firstAgentID, err := testutils.CreateAgent(testutils.Agent{
		OUID:        ts.ouID,
		Type:        "default",
		Name:        "user-usages-first-agent",
		Owner:       ts.ownerUserID,
		Description: "First agent owned by the usages test user",
	})
	ts.Require().NoError(err, "Failed to create the first owned agent")
	ts.firstAgentID = firstAgentID

	secondAgentID, err := testutils.CreateAgent(testutils.Agent{
		OUID:        ts.ouID,
		Type:        "default",
		Name:        "user-usages-second-agent",
		Owner:       ts.ownerUserID,
		Description: "Second agent owned by the usages test user",
	})
	ts.Require().NoError(err, "Failed to create the second owned agent")
	ts.secondAgentID = secondAgentID
}

func (ts *UserUsagesTestSuite) TearDownSuite() {
	if !ts.agentsRemoved {
		for _, id := range []string{ts.firstAgentID, ts.secondAgentID} {
			if id != "" {
				if err := testutils.DeleteAgent(id); err != nil {
					ts.T().Logf("Failed to delete agent %s: %v", id, err)
				}
			}
		}
	}
	if ts.ownerUserID != "" && !ts.ownerRemovable {
		if err := testutils.DeleteUser(ts.ownerUserID); err != nil {
			ts.T().Logf("Failed to delete the owner user: %v", err)
		}
	}
	if ts.unusedUserID != "" {
		if err := testutils.DeleteUser(ts.unusedUserID); err != nil {
			ts.T().Logf("Failed to delete the unreferenced user: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete the user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete the test organization unit: %v", err)
		}
	}
}

// getUsages fetches the usages of a user, asserting a 200.
func (ts *UserUsagesTestSuite) getUsages(userID string) dependenciesResponse {
	ts.T().Helper()

	req, err := http.NewRequest("GET", testServerURL+"/users/"+userID+"/usages", nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "unexpected status, body: %s", body)

	var usages dependenciesResponse
	ts.Require().NoError(json.Unmarshal(body, &usages))
	return usages
}

// deleteUser issues a delete and returns the status with the decoded error, if any.
func (ts *UserUsagesTestSuite) deleteUser(userID string) (int, usagesErrorResponse) {
	ts.T().Helper()

	req, err := http.NewRequest("DELETE", testServerURL+"/users/"+userID, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var errResp usagesErrorResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &errResp)
	}
	return resp.StatusCode, errResp
}

// TestUsagesReportsOwnedAgentsAsBlocking verifies the endpoint reports every owned agent and marks
// it as blocking. A dependant reported with the wrong behavior reads as harmless to the console.
func (ts *UserUsagesTestSuite) TestUsagesReportsOwnedAgentsAsBlocking() {
	usages := ts.getUsages(ts.ownerUserID)

	ts.Require().NotNil(usages.TotalResults,
		"a nil total means dependency data was unavailable, which must not be reported as no blockers")
	ts.Equal(2, *usages.TotalResults)
	ts.Equal(2, usages.Count)
	ts.Equal(map[string]int{"agent": 2}, usages.Summary)

	reported := make(map[string]resourceDependency, len(usages.Usages))
	for _, usage := range usages.Usages {
		reported[usage.ID] = usage
	}

	for _, agentID := range []string{ts.firstAgentID, ts.secondAgentID} {
		usage, found := reported[agentID]
		ts.Require().Truef(found, "agent %s missing from usages, got %v", agentID, usages.Usages)
		ts.Equal("agent", usage.ResourceType)
		ts.Equal("restrict", usage.BehaviorOnDelete,
			"an agent cannot exist without its owner, so ownership must block deletion")
		ts.NotEmpty(usage.DisplayName, "the console renders the display name in the warning")
	}
}

// TestUsagesForUnreferencedUserIsConfirmedEmpty verifies an unreferenced user reports a confirmed
// empty result, with a non-nil total, rather than the nil total that signals unavailable data.
func (ts *UserUsagesTestSuite) TestUsagesForUnreferencedUserIsConfirmedEmpty() {
	usages := ts.getUsages(ts.unusedUserID)

	ts.Require().NotNil(usages.TotalResults, "an empty result must be confirmed, not unknown")
	ts.Equal(0, *usages.TotalResults)
	ts.Equal(0, usages.Count)
	ts.Empty(usages.Usages)
}

// TestUsagesForNonExistentUser verifies the user is resolved before its usages are aggregated, so a
// mistyped ID is a 404 rather than an empty listing that reads as "safe to delete".
func (ts *UserUsagesTestSuite) TestUsagesForNonExistentUser() {
	req, err := http.NewRequest("GET", testServerURL+"/users/non-existent-user-id/usages", nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusNotFound, resp.StatusCode)
}

// TestUsagesRejectsAnAgentID verifies the endpoint is user-scoped: an agent ID is not a user, so it
// must not resolve just because both live in the entity table.
func (ts *UserUsagesTestSuite) TestUsagesRejectsAnAgentID() {
	req, err := http.NewRequest("GET", testServerURL+"/users/"+ts.firstAgentID+"/usages", nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusNotFound, resp.StatusCode)
}

// TestZDeleteIsRefusedThenAllowed walks the full pre-delete contract in the order an operator hits
// it: the delete is refused while the owned agents exist, the refusal names them, and the same
// delete succeeds once they are gone.
//
// Named with a Z prefix so testify's alphabetical ordering runs it after the read assertions, which
// depend on the agents still existing.
func (ts *UserUsagesTestSuite) TestZDeleteIsRefusedThenAllowed() {
	status, errResp := ts.deleteUser(ts.ownerUserID)
	ts.Require().Equal(http.StatusConflict, status,
		"deleting a user with restrict-behavior dependants must be refused")
	ts.Equal(errCodeUserHasBlockingDependencies, errResp.Code)
	ts.Equal("2 agent(s)", errResp.Description.Params["dependencies"],
		"the refusal must summarize the blockers so the operator knows what to fix")
	ts.Contains(errResp.Description.DefaultValue, "2 agent(s)")

	for _, agentID := range []string{ts.firstAgentID, ts.secondAgentID} {
		ts.Require().NoError(testutils.DeleteAgent(agentID), "Failed to delete the owned agent")
	}
	ts.agentsRemoved = true

	usages := ts.getUsages(ts.ownerUserID)
	ts.Require().NotNil(usages.TotalResults)
	ts.Equal(0, *usages.TotalResults, "usages must clear once the blocking agents are gone")

	status, errResp = ts.deleteUser(ts.ownerUserID)
	ts.Require().Equalf(http.StatusNoContent, status,
		"the delete must succeed once nothing blocks it, got code %q", errResp.Code)
	ts.ownerRemovable = true
}
