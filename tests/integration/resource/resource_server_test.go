/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	testServerURL = "https://localhost:8095"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "test_resource_ou",
		Name:        "Test Organization Unit for Resources",
		Description: "Organization unit created for resource API testing",
		Parent:      nil,
	}
)

var testOUID string

type ResourceServerAPITestSuite struct {
	suite.Suite
}

func TestResourceServerAPITestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerAPITestSuite))
}

func (suite *ResourceServerAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	testOUID = ouID
}

func (suite *ResourceServerAPITestSuite) TearDownSuite() {
	if testOUID != "" {
		err := testutils.DeleteOrganizationUnit(testOUID)
		if err != nil {
			suite.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServer() {
	reqBody := CreateResourceServerRequest{
		Name:               "Booking System",
		Description:        "Handles all booking operations",
		Handle:            "booking-system",
		OUID: testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err, "Failed to create resource server")
	suite.NotEmpty(rsID)

	defer deleteResourceServer(rsID)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(reqBody.Name, rs.Name)
	suite.Equal(reqBody.Description, rs.Description)
	suite.Equal(reqBody.Handle, rs.Handle)
	suite.Equal(reqBody.OUID, rs.OUID)
	suite.NotEmpty(rs.Delimiter, "Delimiter should be set to default value")
	suite.Equal(":", rs.Delimiter, "Default delimiter should be ':' based on default configuration")
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerWithoutOptionalFields() {
	reqBody := CreateResourceServerRequest{
		Name:               "Minimal Resource Server",
		Handle:            "minimal-rs",
		OUID: testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	suite.NotEmpty(rsID)

	defer deleteResourceServer(rsID)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(reqBody.Name, rs.Name)
	suite.Empty(rs.Description)
	suite.Equal(reqBody.Handle, rs.Handle)
	suite.Empty(rs.Identifier)
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerDuplicateName() {
	reqBody := CreateResourceServerRequest{
		Name:               "Duplicate Resource Server",
		Handle:            "dup-name-1",
		OUID: testOUID,
	}

	rsID1, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	// Try with same name - should fail
	reqBody2 := CreateResourceServerRequest{
		Name:               "Duplicate Resource Server",
		Handle:            "dup-name-2",
		OUID: testOUID,
	}
	_, err = createResourceServer(reqBody2)
	suite.Error(err, "Should fail with duplicate name")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerDuplicateHandle() {
	reqBody1 := CreateResourceServerRequest{
		Name:               "Resource Server 1",
		Handle:            "same-handler",
		OUID: testOUID,
	}

	rsID1, err := createResourceServer(reqBody1)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	reqBody2 := CreateResourceServerRequest{
		Name:               "Resource Server 2",
		Handle:            "same-handler",
		OUID: testOUID,
	}

	_, err = createResourceServer(reqBody2)
	suite.Error(err, "Should fail with duplicate handle")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerDuplicateIdentifier() {
	reqBody1 := CreateResourceServerRequest{
		Name:               "Resource Server With Identifier 1",
		Handle:            "dup-id-1",
		Identifier:         "https://api.example.com/booking/",
		OUID: testOUID,
	}

	rsID1, err := createResourceServer(reqBody1)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	reqBody2 := CreateResourceServerRequest{
		Name:               "Resource Server With Identifier 2",
		Handle:            "dup-id-2",
		Identifier:         "https://api.example.com/booking/",
		OUID: testOUID,
	}

	_, err = createResourceServer(reqBody2)
	suite.Error(err, "Should fail with duplicate identifier")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerInvalidOU() {
	reqBody := CreateResourceServerRequest{
		Name:               "Invalid OU Resource Server",
		Handle:            "invalid-ou-rs",
		OUID: "00000000-0000-0000-0000-000000000000",
	}

	_, err := createResourceServer(reqBody)
	suite.Error(err, "Should fail with invalid OU")
	suite.Contains(err.Error(), "400")
}

func (suite *ResourceServerAPITestSuite) TestGetResourceServer() {
	reqBody := CreateResourceServerRequest{
		Name:               "Get Test Resource Server",
		Description:        "Resource server for get test",
		Handle:            "get-test-rs",
		OUID: testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(rsID, rs.ID)
	suite.Equal(reqBody.Name, rs.Name)
	suite.Equal(reqBody.Description, rs.Description)
}

func (suite *ResourceServerAPITestSuite) TestGetResourceServerNotFound() {
	_, err := getResourceServer("00000000-0000-0000-0000-000000000000")
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ResourceServerAPITestSuite) TestListResourceServers() {
	delimiter := "-"
	rs1 := CreateResourceServerRequest{
		Name:               "List Resource Server 1",
		Description:        "First resource server",
		Handle:            "listrs1",
		OUID: testOUID,
		Delimiter:          &delimiter,
	}
	rs2 := CreateResourceServerRequest{
		Name:               "List Resource Server 2",
		Description:        "Second resource server",
		Handle:            "list-rs-2",
		OUID: testOUID,
	}

	rsID1, err := createResourceServer(rs1)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	rsID2, err := createResourceServer(rs2)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID2)

	list, err := listResourceServers(0, 100)
	suite.Require().NoError(err)
	suite.GreaterOrEqual(list.TotalResults, 2)
	suite.Equal(1, list.StartIndex)

	foundRS1 := false
	foundRS2 := false
	for _, rs := range list.ResourceServers {
		if rs.ID == rsID1 {
			foundRS1 = true
			suite.Equal(rs1.Name, rs.Name)
			suite.Equal("-", rs.Delimiter)
		}
		if rs.ID == rsID2 {
			foundRS2 = true
			suite.Equal(rs2.Name, rs.Name)
		}
	}
	suite.True(foundRS1, "Should find first resource server")
	suite.True(foundRS2, "Should find second resource server")
}

func (suite *ResourceServerAPITestSuite) TestListResourceServersWithPagination() {
	list, err := listResourceServers(0, 1)
	suite.Require().NoError(err)
	suite.LessOrEqual(list.Count, 1)
	suite.Equal(1, list.StartIndex)

	if list.TotalResults > 1 {
		suite.NotEmpty(list.Links)
	}
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServer() {
	delimiter := "/"
	reqBody := CreateResourceServerRequest{
		Name:        "Update Test Resource Server",
		Description: "Original description",
		Handle:      "original-handler",
		Identifier:  "https://api.example.com/original/",
		OUID:        testOUID,
		Delimiter:   &delimiter,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	updateReq := UpdateResourceServerRequest{
		Name:        "Updated Resource Server",
		Description: "Updated description",
		Handle:      "updated-handler",
		Identifier:  "https://api.example.com/updated/",
		OUID:        testOUID,
	}

	err = updateResourceServer(rsID, updateReq)
	suite.Require().NoError(err)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal(updateReq.Name, rs.Name)
	suite.Equal(updateReq.Description, rs.Description)
	suite.Equal(updateReq.Handle, rs.Handle, "Handle should be updated")
	suite.Equal(updateReq.Identifier, rs.Identifier, "Identifier should be updated")
	suite.Equal("/", rs.Delimiter, "Delimiter should remain unchanged after update")
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServerPreservesHandleWhenOmitted() {
	reqBody := CreateResourceServerRequest{
		Name:   "Preserve Handle RS",
		Handle: "preserve-me",
		OUID:   testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID)

	updateReq := UpdateResourceServerRequest{
		Name: "Preserve Handle RS Updated",
		OUID: testOUID,
	}

	err = updateResourceServer(rsID, updateReq)
	suite.Require().NoError(err)

	rs, err := getResourceServer(rsID)
	suite.Require().NoError(err)
	suite.Equal("preserve-me", rs.Handle, "Handle should be preserved when not provided in update")
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServerHandleConflict() {
	rs1 := CreateResourceServerRequest{
		Name:   "Handle Conflict RS 1",
		Handle: "handle-conflict-1",
		OUID:   testOUID,
	}
	rs2 := CreateResourceServerRequest{
		Name:   "Handle Conflict RS 2",
		Handle: "handle-conflict-2",
		OUID:   testOUID,
	}

	rsID1, err := createResourceServer(rs1)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	rsID2, err := createResourceServer(rs2)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID2)

	updateReq := UpdateResourceServerRequest{
		Name:   "Handle Conflict RS 2",
		Handle: "handle-conflict-1",
		OUID:   testOUID,
	}

	err = updateResourceServer(rsID2, updateReq)
	suite.Error(err, "Should fail with handle conflict")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServerNotFound() {
	updateReq := UpdateResourceServerRequest{
		Name:               "non-existent",
		OUID: testOUID,
	}

	err := updateResourceServer("00000000-0000-0000-0000-000000000000", updateReq)
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ResourceServerAPITestSuite) TestUpdateResourceServerNameConflict() {
	rs1 := CreateResourceServerRequest{
		Name:               "Conflict Resource Server 1",
		Handle:            "conflict-rs-1",
		OUID: testOUID,
	}
	rs2 := CreateResourceServerRequest{
		Name:               "Conflict Resource Server 2",
		Handle:            "conflict-rs-2",
		OUID: testOUID,
	}

	rsID1, err := createResourceServer(rs1)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID1)

	rsID2, err := createResourceServer(rs2)
	suite.Require().NoError(err)
	defer deleteResourceServer(rsID2)

	// Try to update second server to have the same name as first
	updateReq := UpdateResourceServerRequest{
		Name:               "Conflict Resource Server 1",
		OUID: testOUID,
	}

	err = updateResourceServer(rsID2, updateReq)
	suite.Error(err, "Should fail with name conflict")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceServerAPITestSuite) TestCreateResourceServerDelimiterInHandle() {
	reqBody := CreateResourceServerRequest{
		Name:    "Delimiter In Handle RS",
		Handle: "foo:bar",
		OUID:    testOUID,
	}

	_, err := createResourceServer(reqBody)
	suite.Error(err, "Should fail when handle contains the default delimiter")
	suite.Contains(err.Error(), "400")
}

func (suite *ResourceServerAPITestSuite) TestDeleteResourceServer() {
	reqBody := CreateResourceServerRequest{
		Name:               "Delete Test Resource Server",
		Handle:            "delete-test-rs",
		OUID: testOUID,
	}

	rsID, err := createResourceServer(reqBody)
	suite.Require().NoError(err)

	err = deleteResourceServer(rsID)
	suite.Require().NoError(err)

	_, err = getResourceServer(rsID)
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ResourceServerAPITestSuite) TestDeleteResourceServerNotFound() {
	err := deleteResourceServer("00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent")
}

// Delimiter Tests
func (suite *ResourceServerAPITestSuite) TestCreateResourceServerWithVariousDelimiters() {
	// Valid delimiters: a-zA-Z0-9._:-/
	validDelimiters := []string{":", ".", "-", "_", "/"}

	for i, delim := range validDelimiters {
		delimiter := delim
		reqBody := CreateResourceServerRequest{
			Name:               "Server With " + delim + " Delimiter",
			Handle:            fmt.Sprintf("delimtest%d", i),
			OUID: testOUID,
			Delimiter:          &delimiter,
		}

		rsID, err := createResourceServer(reqBody)
		suite.Require().NoError(err, "Should accept delimiter: %s", delim)
		defer deleteResourceServer(rsID)

		rs, err := getResourceServer(rsID)
		suite.Require().NoError(err)
		suite.Equal(delim, rs.Delimiter, "Delimiter should be %s", delim)
	}

	// Invalid delimiters - characters not in a-zA-Z0-9._:-/
	invalidDelimiters := []struct {
		value       string
		description string
	}{
		{"\"", "quote"},
		{"\\", "backslash"},
		{"::", "multi-character"},
		{"ñ", "non-ASCII"},
		{"#", "hash"},
		{"|", "pipe"},
		{"!", "exclamation"},
		{"@", "at"},
		{"$", "dollar"},
	}

	for _, tc := range invalidDelimiters {
		delimiter := tc.value
		reqBody := CreateResourceServerRequest{
			Name:               "Server With " + tc.description + " Delimiter",
			Handle:            "invalid" + tc.description,
			OUID: testOUID,
			Delimiter:          &delimiter,
		}

		_, err := createResourceServer(reqBody)
		suite.Error(err, "Should reject %s delimiter", tc.description)
		suite.Contains(err.Error(), "400", "Should return 400 for %s delimiter", tc.description)
	}
}

// Helper functions

func createResourceServer(req CreateResourceServerRequest) (string, error) {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", testServerURL+"/resource-servers", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var rs ResourceServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&rs); err != nil {
		return "", err
	}

	return rs.ID, nil
}

func getResourceServer(id string) (*ResourceServerResponse, error) {
	client := testutils.GetHTTPClient()

	httpReq, err := http.NewRequest("GET", testServerURL+"/resource-servers/"+id, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var rs ResourceServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&rs); err != nil {
		return nil, err
	}

	return &rs, nil
}

func listResourceServers(offset, limit int) (*ResourceServerListResponse, error) {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers?offset=%d&limit=%d", testServerURL, offset, limit)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var list ResourceServerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return &list, nil
}

func updateResourceServer(id string, req UpdateResourceServerRequest) error {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("PUT", testServerURL+"/resource-servers/"+id, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func deleteResourceServer(id string) error {
	client := testutils.GetHTTPClient()

	httpReq, err := http.NewRequest("DELETE", testServerURL+"/resource-servers/"+id, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
