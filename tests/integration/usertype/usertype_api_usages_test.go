// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usertype

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// resourceUsage mirrors the ResourceUsage schema returned by GET /user-types/{id}/usages.
type resourceUsage struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

// resourceUsagesResponse mirrors the ResourceUsagesResponse schema.
type resourceUsagesResponse struct {
	TotalResults *int            `json:"totalResults"`
	Count        int             `json:"count"`
	Summary      map[string]int  `json:"summary"`
	Usages       []resourceUsage `json:"usages"`
}

type UserTypeUsagesTestSuite struct {
	suite.Suite
	client *http.Client
	oUID   string
}

var testUserTypeAPIUsagesOU = testutils.OrganizationUnit{
	Handle:      "test-user-type-api-usages-ou",
	Name:        "Test Organization Unit for User Type API Usages",
	Description: "Organization unit created for user type usages testing",
	Parent:      nil,
}

func TestUserTypeUsagesTestSuite(t *testing.T) {
	suite.Run(t, new(UserTypeUsagesTestSuite))
}

func (ts *UserTypeUsagesTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testUserTypeAPIUsagesOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.oUID = ouID
}

func (ts *UserTypeUsagesTestSuite) TearDownSuite() {
	if ts.oUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.oUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", ts.oUID, err)
		}
	}
}

func (ts *UserTypeUsagesTestSuite) getUsages(schemaID string) (int, *resourceUsagesResponse) {
	req, err := http.NewRequest("GET", testServerURL+"/user-types/"+schemaID+"/usages", nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var result resourceUsagesResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	return resp.StatusCode, &result
}

// TestGetUsages_NoUsers verifies GET /user-types/{id}/usages reports an empty, confirmed result
// when no users of that type exist.
func (ts *UserTypeUsagesTestSuite) TestGetUsages_NoUsers() {
	schema := CreateUserTypeRequest{
		Name:   "schema-usages-empty",
		OUID:   ts.oUID,
		Schema: json.RawMessage(`{"email": {"type": "string"}}`),
	}
	schemaID := ts.createTestSchema(schema)
	defer ts.deleteTestSchema(schemaID)

	status, result := ts.getUsages(schemaID)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().NotNil(result)
	ts.Require().NotNil(result.TotalResults)
	ts.Assert().Equal(0, *result.TotalResults)
	ts.Assert().Empty(result.Usages)
}

// TestGetUsages_WithExistingUsers verifies GET /user-types/{id}/usages reports blocking usages
// when users of that type exist, matching what DELETE would refuse.
func (ts *UserTypeUsagesTestSuite) TestGetUsages_WithExistingUsers() {
	schema := CreateUserTypeRequest{
		Name:   "schema-usages-blocked",
		OUID:   ts.oUID,
		Schema: json.RawMessage(`{"email": {"type": "string"}}`),
	}
	schemaID := ts.createTestSchema(schema)
	defer ts.deleteTestSchema(schemaID)

	user := testutils.User{
		OUID:       ts.oUID,
		Type:       schema.Name,
		Attributes: json.RawMessage(`{"email": "usages-test@example.com"}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err)
	defer func() {
		_ = testutils.DeleteUser(userID)
	}()

	status, result := ts.getUsages(schemaID)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().NotNil(result)
	ts.Require().NotNil(result.TotalResults)
	ts.Assert().Equal(1, *result.TotalResults)
	ts.Require().Len(result.Usages, 1)
	ts.Assert().Equal("restrict", result.Usages[0].BehaviorOnDelete)
	ts.Assert().Equal("user", result.Usages[0].ResourceType)
}

// TestGetUsages_NotFound verifies GET /user-types/{id}/usages returns 404 for an unknown ID.
func (ts *UserTypeUsagesTestSuite) TestGetUsages_NotFound() {
	status, _ := ts.getUsages("550e8400-e29b-41d4-a716-446655440000")
	ts.Assert().Equal(http.StatusNotFound, status)
}

// TestGetUsages_DeclarativeUserCountsAsBlocking verifies that a declarative (file-based) user
// counts toward a user type's usages the same way a database-backed one does. The suite reuses
// the pre-loaded declarative fixture "decl-user-1" (type: "person"), so this schema is
// deliberately named "person" (lowercase, distinct from the built-in "Person" type) to match it
// without touching shared declarative fixtures. This exercises the file-based and composite
// store paths behind GetEntityCountByType, which a purely database-backed user never reaches.
func (ts *UserTypeUsagesTestSuite) TestGetUsages_DeclarativeUserCountsAsBlocking() {
	schema := CreateUserTypeRequest{
		Name:   "person",
		OUID:   ts.oUID,
		Schema: json.RawMessage(`{"email": {"type": "string"}}`),
	}
	schemaID := ts.createTestSchema(schema)
	// The declarative user "decl-user-1" of type "person" always blocks deletion of this schema,
	// so cleanup here is best-effort only; deleteTestSchema already ignores failures.
	defer ts.deleteTestSchema(schemaID)

	status, result := ts.getUsages(schemaID)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().NotNil(result)
	ts.Require().NotNil(result.TotalResults)
	ts.Assert().GreaterOrEqual(*result.TotalResults, 1)
	ts.Require().NotEmpty(result.Usages)
	ts.Assert().Equal("restrict", result.Usages[0].BehaviorOnDelete)
	ts.Assert().Equal("user", result.Usages[0].ResourceType)

	// DELETE must also be refused, driven entirely by the declarative usage.
	deleteReq, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
	ts.Require().NoError(err)
	deleteResp, err := ts.client.Do(deleteReq)
	ts.Require().NoError(err)
	defer deleteResp.Body.Close()
	ts.Assert().Equal(http.StatusConflict, deleteResp.StatusCode)
}

// TestOptionsUsages verifies the CORS preflight route registered alongside
// GET /user-types/{id}/usages responds with 204 No Content.
func (ts *UserTypeUsagesTestSuite) TestOptionsUsages() {
	req, err := http.NewRequest("OPTIONS", testServerURL+"/user-types/some-id/usages", nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusNoContent, resp.StatusCode)
}

// createTestSchema creates a user type and returns its ID. Duplicated from
// usertype_api_delete_test.go's helper since each suite manages its own client/oUID.
func (ts *UserTypeUsagesTestSuite) createTestSchema(schema CreateUserTypeRequest) string {
	if schema.OUID == "" {
		schema.OUID = ts.oUID
	}

	jsonData, err := json.Marshal(schema)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/user-types", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "Response: %s", string(body))

	var createdSchema UserType
	ts.Require().NoError(json.Unmarshal(body, &createdSchema))
	return createdSchema.ID
}

func (ts *UserTypeUsagesTestSuite) deleteTestSchema(schemaID string) {
	req, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
	if err != nil {
		return
	}
	resp, err := ts.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
