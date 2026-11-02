// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type ResourceAPITestSuite struct {
	suite.Suite
	ouID             string
	resourceServerID string
}

func TestResourceAPITestSuite(t *testing.T) {
	suite.Run(t, new(ResourceAPITestSuite))
}

func (suite *ResourceAPITestSuite) SetupSuite() {
	// Create test organization unit
	ou := testutils.OrganizationUnit{
		Handle:      "test_resource_api_ou",
		Name:        "Test OU for Resource API",
		Description: "Organization unit for resource API testing",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	// Create test resource server
	rsReq := CreateResourceServerRequest{
		Name:        "Resource Test Server",
		Description: "Resource server for resource testing",
		OUID:        ouID,
	}
	rsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err, "Failed to create test resource server")
	suite.resourceServerID = rsID
}

func (suite *ResourceAPITestSuite) TearDownSuite() {
	if suite.resourceServerID != "" {
		deleteResourceServer(suite.resourceServerID)
	}
	if suite.ouID != "" {
		testutils.DeleteOrganizationUnit(suite.ouID)
	}
}

func (suite *ResourceAPITestSuite) TestCreateResource() {
	req := CreateResourceRequest{
		Name:        "Hotels",
		Handle:      "hotels",
		Description: "Hotel resources",
		Parent:      nil,
	}

	resourceID, err := createResource(suite.resourceServerID, req)
	suite.Require().NoError(err, "Failed to create resource")
	suite.NotEmpty(resourceID)

	defer deleteResource(suite.resourceServerID, resourceID)

	// Verify the created resource
	res, err := getResource(suite.resourceServerID, resourceID)
	suite.Require().NoError(err)
	suite.Equal(req.Name, res.Name)
	suite.Equal(req.Handle, res.Handle)
	suite.Equal(req.Description, res.Description)
	suite.Nil(res.Parent)
	suite.Equal("hotels", res.Permission, "Top-level resource permission should be just the handle")
}

func (suite *ResourceAPITestSuite) TestCreateResourceWithParent() {
	// Create parent resource
	parentReq := CreateResourceRequest{
		Name:        "Bookings",
		Handle:      "bookings",
		Description: "Booking resources",
		Parent:      nil,
	}
	parentID, err := createResource(suite.resourceServerID, parentReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, parentID)

	// Create child resource
	childReq := CreateResourceRequest{
		Name:        "Confirmed Bookings",
		Handle:      "confirmed",
		Description: "Confirmed bookings",
		Parent:      &parentID,
	}
	childID, err := createResource(suite.resourceServerID, childReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, childID)

	// Verify child resource
	child, err := getResource(suite.resourceServerID, childID)
	suite.Require().NoError(err)
	suite.Equal(childReq.Name, child.Name)
	suite.Equal(childReq.Handle, child.Handle)
	suite.NotNil(child.Parent)
	suite.Equal(parentID, *child.Parent)
	suite.Equal("bookings:confirmed", child.Permission, "Nested resource permission should be parent:child")
}

func (suite *ResourceAPITestSuite) TestCreateResourceDuplicateHandle() {
	req := CreateResourceRequest{
		Name:        "Duplicate Resource",
		Handle:      "duplicate-resource",
		Description: "First resource",
		Parent:      nil,
	}

	resourceID1, err := createResource(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID1)

	// Try to create another resource with the same handle under the same parent
	req2 := CreateResourceRequest{
		Name:        "Different Name",
		Handle:      "duplicate-resource",
		Description: "Second resource with same handle",
		Parent:      nil,
	}
	_, err = createResource(suite.resourceServerID, req2)
	suite.Error(err, "Should fail with duplicate handle")
	suite.Contains(err.Error(), "409")
}

func (suite *ResourceAPITestSuite) TestCreateResourceDuplicateHandleDifferentParent() {
	// Create two parent resources
	parent1Req := CreateResourceRequest{
		Name:   "Parent 1",
		Handle: "parent1",
		Parent: nil,
	}
	parent1ID, err := createResource(suite.resourceServerID, parent1Req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, parent1ID)

	parent2Req := CreateResourceRequest{
		Name:   "Parent 2",
		Handle: "parent2",
		Parent: nil,
	}
	parent2ID, err := createResource(suite.resourceServerID, parent2Req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, parent2ID)

	// Create child with same handle under different parents - should succeed
	child1Req := CreateResourceRequest{
		Name:   "Child Resource",
		Handle: "child",
		Parent: &parent1ID,
	}
	child1ID, err := createResource(suite.resourceServerID, child1Req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, child1ID)

	child2Req := CreateResourceRequest{
		Name:   "Another Child Resource",
		Handle: "child",
		Parent: &parent2ID,
	}
	child2ID, err := createResource(suite.resourceServerID, child2Req)
	suite.Require().NoError(err, "Should allow same handle under different parents")
	defer deleteResource(suite.resourceServerID, child2ID)
}

func (suite *ResourceAPITestSuite) TestCreateResourceInvalidParent() {
	req := CreateResourceRequest{
		Name:   "Invalid Parent Resource",
		Handle: "invalid-parent-resource",
		Parent: stringPtr("00000000-0000-0000-0000-000000000000"),
	}

	_, err := createResource(suite.resourceServerID, req)
	suite.Error(err, "Should fail with invalid parent")
	suite.Contains(err.Error(), "400")
}

func (suite *ResourceAPITestSuite) TestGetResource() {
	req := CreateResourceRequest{
		Name:        "get-test-resource",
		Handle:      "get-test-resource",
		Description: "Resource for get test",
		Parent:      nil,
	}

	resourceID, err := createResource(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID)

	res, err := getResource(suite.resourceServerID, resourceID)
	suite.Require().NoError(err)
	suite.Equal(resourceID, res.ID)
	suite.Equal(req.Name, res.Name)
	suite.Equal(req.Description, res.Description)
}

func (suite *ResourceAPITestSuite) TestGetResourceNotFound() {
	_, err := getResource(suite.resourceServerID, "00000000-0000-0000-0000-000000000000")
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ResourceAPITestSuite) TestListResources() {
	// Create multiple resources
	res1 := CreateResourceRequest{
		Name:        "List Resource 1",
		Handle:      "list-resource-1",
		Description: "First resource",
		Parent:      nil,
	}
	res1ID, err := createResource(suite.resourceServerID, res1)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, res1ID)

	res2 := CreateResourceRequest{
		Name:        "List Resource 2",
		Handle:      "list-resource-2",
		Description: "Second resource",
		Parent:      nil,
	}
	res2ID, err := createResource(suite.resourceServerID, res2)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, res2ID)

	// List all resources
	list, err := listResources(suite.resourceServerID, "", 0, 100)
	suite.Require().NoError(err)
	suite.GreaterOrEqual(list.TotalResults, 2)
	suite.Equal(1, list.StartIndex)

	// Verify our resources are in the list
	foundRes1 := false
	foundRes2 := false
	for _, res := range list.Resources {
		if res.ID == res1ID {
			foundRes1 = true
			suite.Equal(res1.Name, res.Name)
			suite.Equal("list-resource-1", res.Permission, "Permission should be returned in list response")
		}
		if res.ID == res2ID {
			foundRes2 = true
			suite.Equal(res2.Name, res.Name)
			suite.Equal("list-resource-2", res.Permission, "Permission should be returned in list response")
		}
	}
	suite.True(foundRes1, "Should find first resource")
	suite.True(foundRes2, "Should find second resource")
}

func (suite *ResourceAPITestSuite) TestListResourcesByParent() {
	// Create parent resource
	parentReq := CreateResourceRequest{
		Name:   "Parent for List",
		Handle: "parent-for-list",
		Parent: nil,
	}
	parentID, err := createResource(suite.resourceServerID, parentReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, parentID)

	// Create child resources
	child1Req := CreateResourceRequest{
		Name:   "Child 1",
		Handle: "child-1",
		Parent: &parentID,
	}
	child1ID, err := createResource(suite.resourceServerID, child1Req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, child1ID)

	child2Req := CreateResourceRequest{
		Name:   "Child 2",
		Handle: "child-2",
		Parent: &parentID,
	}
	child2ID, err := createResource(suite.resourceServerID, child2Req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, child2ID)

	// List resources by parent
	list, err := listResources(suite.resourceServerID, parentID, 0, 100)
	suite.Require().NoError(err)
	suite.Equal(2, list.TotalResults)

	// Verify only children are returned
	for _, res := range list.Resources {
		suite.NotNil(res.Parent)
		suite.Equal(parentID, *res.Parent)
	}
}

func (suite *ResourceAPITestSuite) TestListTopLevelResources() {
	// Create top-level resource
	topReq := CreateResourceRequest{
		Name:   "Top Level",
		Handle: "top-level",
		Parent: nil,
	}
	topID, err := createResource(suite.resourceServerID, topReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, topID)

	// Create child resource
	childReq := CreateResourceRequest{
		Name:   "Child of Top",
		Handle: "child-of-top",
		Parent: &topID,
	}
	childID, err := createResource(suite.resourceServerID, childReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, childID)

	// List top-level resources (parent = empty string for query param)
	list, err := listResources(suite.resourceServerID, "", 0, 100)
	suite.Require().NoError(err)

	// Verify only top-level resources are returned
	for _, res := range list.Resources {
		suite.Nil(res.Parent, "Top-level resources should have null parent")
	}

	// Verify our top-level resource is in the list
	foundTop := false
	for _, res := range list.Resources {
		if res.ID == topID {
			foundTop = true
		}
		// Child should not be in the list
		suite.NotEqual(childID, res.ID)
	}
	suite.True(foundTop, "Should find top-level resource")
}

func (suite *ResourceAPITestSuite) TestUpdateResource() {
	// Create resource
	req := CreateResourceRequest{
		Name:        "Update Test Resource",
		Handle:      "update-test-resource",
		Description: "Original description",
		Parent:      nil,
	}
	resourceID, err := createResource(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID)

	// Update resource - name is mutable, handle and parent are immutable
	updateReq := UpdateResourceRequest{
		Name:        "Updated Resource Name",
		Description: "Updated description",
	}
	err = updateResource(suite.resourceServerID, resourceID, updateReq)
	suite.Require().NoError(err)

	// Verify updates
	res, err := getResource(suite.resourceServerID, resourceID)
	suite.Require().NoError(err)
	suite.Equal(updateReq.Name, res.Name, "Name should be mutable")
	suite.Equal(req.Handle, res.Handle, "Handle should be immutable")
	suite.Equal(updateReq.Description, res.Description)
	suite.Nil(res.Parent, "Parent should remain immutable")
	suite.Equal("update-test-resource", res.Permission, "Permission should be immutable")
}

func (suite *ResourceAPITestSuite) TestUpdateResourceHandleIsImmutable() {
	// Create resource
	req := CreateResourceRequest{
		Name:   "Handle Immutability Test",
		Handle: "immutable-handle-resource",
		Parent: nil,
	}
	resourceID, err := createResource(suite.resourceServerID, req)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, resourceID)

	// Update name and description
	updateReq := UpdateResourceRequest{
		Name:        "Updated Name",
		Description: "New description",
	}
	err = updateResource(suite.resourceServerID, resourceID, updateReq)
	suite.Require().NoError(err)

	// Verify handle is unchanged, but name is updated
	res, err := getResource(suite.resourceServerID, resourceID)
	suite.Require().NoError(err)
	suite.Equal(updateReq.Name, res.Name, "Name should be mutable")
	suite.Equal(req.Handle, res.Handle, "Handle should remain immutable")
}

func (suite *ResourceAPITestSuite) TestUpdateResourceParentIsImmutable() {
	// Create parent resource
	parentReq := CreateResourceRequest{
		Name:   "Parent Resource",
		Handle: "parent",
		Parent: nil,
	}
	parentID, err := createResource(suite.resourceServerID, parentReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, parentID)

	// Create child resource
	childReq := CreateResourceRequest{
		Name:   "Child Resource",
		Handle: "child",
		Parent: &parentID,
	}
	childID, err := createResource(suite.resourceServerID, childReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, childID)

	// Update child - parent should remain unchanged (it's immutable)
	updateReq := UpdateResourceRequest{
		Name:        "Updated Child",
		Description: "Updated description",
	}
	err = updateResource(suite.resourceServerID, childID, updateReq)
	suite.Require().NoError(err)

	// Verify parent is unchanged
	res, err := getResource(suite.resourceServerID, childID)
	suite.Require().NoError(err)
	suite.NotNil(res.Parent, "Parent should still exist")
	suite.Equal(parentID, *res.Parent, "Parent should remain immutable")
}

func (suite *ResourceAPITestSuite) TestDeleteResource() {
	req := CreateResourceRequest{
		Name:   "Delete Test Resource",
		Handle: "delete-test-resource",
		Parent: nil,
	}

	resourceID, err := createResource(suite.resourceServerID, req)
	suite.Require().NoError(err)

	err = deleteResource(suite.resourceServerID, resourceID)
	suite.Require().NoError(err)

	// Verify resource is deleted
	_, err = getResource(suite.resourceServerID, resourceID)
	suite.Error(err)
	suite.Contains(err.Error(), "404")
}

func (suite *ResourceAPITestSuite) TestDeleteResourceNotFound() {
	err := deleteResource(suite.resourceServerID, "00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent")
}

func (suite *ResourceAPITestSuite) TestDeleteResourceServerNotFound() {
	err := deleteResource("00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000")
	suite.NoError(err, "Delete should be idempotent")
}

func (suite *ResourceAPITestSuite) TestResourcePermissionDerivationWithCustomDelimiter() {
	// Create resource server with custom delimiter
	delimiter := "."
	rsReq := CreateResourceServerRequest{
		Name:      "Custom Delimiter Server",
		OUID:      suite.ouID,
		Delimiter: &delimiter,
	}
	customRsID, err := createResourceServer(rsReq)
	suite.Require().NoError(err)
	defer deleteResourceServer(customRsID)

	// Create level 1: org
	level1Req := CreateResourceRequest{
		Name:   "Organization",
		Handle: "org",
		Parent: nil,
	}
	level1ID, err := createResource(customRsID, level1Req)
	suite.Require().NoError(err)
	defer deleteResource(customRsID, level1ID)

	level1, err := getResource(customRsID, level1ID)
	suite.Require().NoError(err)
	suite.Equal("org", level1.Permission, "Top-level resource permission should be just the handle")

	// Create level 2: org.dept
	level2Req := CreateResourceRequest{
		Name:   "Department",
		Handle: "dept",
		Parent: &level1ID,
	}
	level2ID, err := createResource(customRsID, level2Req)
	suite.Require().NoError(err)
	defer deleteResource(customRsID, level2ID)

	level2, err := getResource(customRsID, level2ID)
	suite.Require().NoError(err)
	suite.Equal("org.dept", level2.Permission, "Permission should use custom delimiter")

	// Create level 3: org.dept.team
	level3Req := CreateResourceRequest{
		Name:   "Team",
		Handle: "team",
		Parent: &level2ID,
	}
	level3ID, err := createResource(customRsID, level3Req)
	suite.Require().NoError(err)
	defer deleteResource(customRsID, level3ID)

	level3, err := getResource(customRsID, level3ID)
	suite.Require().NoError(err)
	suite.Equal("org.dept.team", level3.Permission, "Deeply nested permission should use custom delimiter")
}

func (suite *ResourceAPITestSuite) TestDeleteResourceWithChildren() {
	// Create parent with child
	parentReq := CreateResourceRequest{
		Name:   "Parent With Children",
		Handle: "parent-with-children",
		Parent: nil,
	}
	parentID, err := createResource(suite.resourceServerID, parentReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, parentID)

	childReq := CreateResourceRequest{
		Name:   "Dependent Child",
		Handle: "dependent-child",
		Parent: &parentID,
	}
	childID, err := createResource(suite.resourceServerID, childReq)
	suite.Require().NoError(err)
	defer deleteResource(suite.resourceServerID, childID)

	// Try to delete parent - should fail due to dependencies
	err = deleteResource(suite.resourceServerID, parentID)
	suite.Error(err, "Should fail when resource has children")
	suite.Contains(err.Error(), "400")
}

// PaginationTestSuite covers paginated list responses and the navigation links they carry.
type PaginationTestSuite struct {
	suite.Suite
	ouID             string
	resourceServerID string
	resourceID       string
}

func TestPaginationTestSuite(t *testing.T) {
	suite.Run(t, new(PaginationTestSuite))
}

func (suite *PaginationTestSuite) SetupSuite() {
	ou := testutils.OrganizationUnit{
		Handle:      "test_resource_pagination_ou",
		Name:        "Test OU for Resource Pagination",
		Description: "Organization unit for resource pagination testing",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	rsID, err := createResourceServer(CreateResourceServerRequest{
		Name:        "Pagination Test Server",
		Description: "Resource server for pagination testing",
		OUID:        ouID,
	})
	suite.Require().NoError(err, "Failed to create test resource server")
	suite.resourceServerID = rsID

	resID, err := createResource(rsID, CreateResourceRequest{
		Name:   "Pagination Parent",
		Handle: "pagination-parent",
		Parent: nil,
	})
	suite.Require().NoError(err, "Failed to create test resource")
	suite.resourceID = resID
}

func (suite *PaginationTestSuite) TearDownSuite() {
	if suite.resourceID != "" {
		deleteResource(suite.resourceServerID, suite.resourceID)
	}
	if suite.resourceServerID != "" {
		deleteResourceServer(suite.resourceServerID)
	}
	if suite.ouID != "" {
		testutils.DeleteOrganizationUnit(suite.ouID)
	}
}

func (suite *PaginationTestSuite) TestListResourceServersSecondPageLinks() {
	list, err := listResourceServers(1, 1)
	suite.Require().NoError(err)
	suite.Require().Greater(list.TotalResults, 2,
		"The suite fixtures and defaults should provide more than two resource servers")

	suite.Equal(2, list.StartIndex)
	suite.Equal(1, list.Count)

	links := linksByRel(list.Links)
	suite.Equal("/resource-servers?offset=0&limit=1", links["first"])
	suite.Equal("/resource-servers?offset=0&limit=1", links["prev"])
	suite.Equal("/resource-servers?offset=2&limit=1", links["next"])
	suite.Equal(fmt.Sprintf("/resource-servers?offset=%d&limit=1", list.TotalResults-1), links["last"])
}

// TestListResourceServersPreviousLinkClampedToStart covers the previous-page link when the offset
// is smaller than the page size, where the previous offset clamps to the start of the collection.
func (suite *PaginationTestSuite) TestListResourceServersPreviousLinkClampedToStart() {
	list, err := listResourceServers(1, 5)
	suite.Require().NoError(err)
	suite.Equal(2, list.StartIndex)

	var prev string
	for _, link := range list.Links {
		if link.Rel == "prev" {
			prev = link.Href
		}
	}
	suite.Equal("/resource-servers?offset=0&limit=5", prev)
}

func (suite *PaginationTestSuite) TestListResourcesSecondPageLinks() {
	for _, handle := range []string{"page-one", "page-two", "page-three"} {
		resID, err := createResource(suite.resourceServerID, CreateResourceRequest{
			Name:   handle,
			Handle: handle,
			Parent: &suite.resourceID,
		})
		suite.Require().NoError(err)
		defer deleteResource(suite.resourceServerID, resID)
	}

	list, err := listResources(suite.resourceServerID, suite.resourceID, 1, 1)
	suite.Require().NoError(err)
	suite.Equal(3, list.TotalResults)
	suite.Equal(2, list.StartIndex)
	suite.Equal(1, list.Count)

	links := linksByRel(list.Links)
	baseURL := fmt.Sprintf("/resource-servers/%s/resources", suite.resourceServerID)
	suite.Equal(baseURL+"?offset=0&limit=1", links["first"])
	suite.Equal(baseURL+"?offset=0&limit=1", links["prev"])
	suite.Equal(baseURL+"?offset=2&limit=1", links["next"])
	suite.Equal(baseURL+"?offset=2&limit=1", links["last"])
}

func (suite *PaginationTestSuite) TestListActionsAtResourceServerSecondPageLinks() {
	for _, handle := range []string{"action-one", "action-two", "action-three"} {
		actionID, err := createActionAtResourceServer(suite.resourceServerID, CreateActionRequest{
			Name:   handle,
			Handle: handle,
		})
		suite.Require().NoError(err)
		defer deleteAction(suite.resourceServerID, actionID)
	}

	list, err := listActionsAtResourceServer(suite.resourceServerID, 1, 1)
	suite.Require().NoError(err)
	suite.Equal(3, list.TotalResults)
	suite.Equal(2, list.StartIndex)
	suite.Equal(1, list.Count)

	links := linksByRel(list.Links)
	baseURL := fmt.Sprintf("/resource-servers/%s/actions", suite.resourceServerID)
	suite.Equal(baseURL+"?offset=0&limit=1", links["first"])
	suite.Equal(baseURL+"?offset=0&limit=1", links["prev"])
	suite.Equal(baseURL+"?offset=2&limit=1", links["next"])
	suite.Equal(baseURL+"?offset=2&limit=1", links["last"])
}

func (suite *PaginationTestSuite) TestListActionsAtResourceSecondPageLinks() {
	for _, handle := range []string{"nested-one", "nested-two", "nested-three"} {
		actionID, err := createActionAtResource(suite.resourceServerID, suite.resourceID, CreateActionRequest{
			Name:   handle,
			Handle: handle,
		})
		suite.Require().NoError(err)
		defer deleteActionAtResource(suite.resourceServerID, suite.resourceID, actionID)
	}

	list, err := listActionsAtResource(suite.resourceServerID, suite.resourceID, 1, 1)
	suite.Require().NoError(err)
	suite.Equal(3, list.TotalResults)
	suite.Equal(2, list.StartIndex)
	suite.Equal(1, list.Count)

	links := linksByRel(list.Links)
	baseURL := fmt.Sprintf("/resource-servers/%s/resources/%s/actions", suite.resourceServerID, suite.resourceID)
	suite.Equal(baseURL+"?offset=0&limit=1", links["first"])
	suite.Equal(baseURL+"?offset=0&limit=1", links["prev"])
	suite.Equal(baseURL+"?offset=2&limit=1", links["next"])
	suite.Equal(baseURL+"?offset=2&limit=1", links["last"])
}

// TestListDeclarativeResourcesWithLimit exercises pagination over the file-backed store. decl-rs-3
// declares two top level resources, so a limit of one must return a single page per offset.
func (suite *PaginationTestSuite) TestListDeclarativeResourcesWithLimit() {
	first, err := listResources(declarativeMCPServerID, "", 0, 1)
	suite.Require().NoError(err)
	suite.Equal(2, first.TotalResults)
	suite.Equal(1, first.Count)
	suite.Equal(1, first.StartIndex)
	suite.Require().Len(first.Resources, 1)
	suite.Equal("tools", first.Resources[0].Handle)

	second, err := listResources(declarativeMCPServerID, "", 1, 1)
	suite.Require().NoError(err)
	suite.Equal(2, second.TotalResults)
	suite.Equal(1, second.Count)
	suite.Equal(2, second.StartIndex)
	suite.Require().Len(second.Resources, 1)
	suite.Equal("prompts", second.Resources[0].Handle)

	links := linksByRel(second.Links)
	baseURL := fmt.Sprintf("/resource-servers/%s/resources", declarativeMCPServerID)
	suite.Equal(baseURL+"?offset=0&limit=1", links["first"])
	suite.Equal(baseURL+"?offset=0&limit=1", links["prev"])
}

// TestListDeclarativeActionsWithLimit exercises action pagination over the file-backed store.
func (suite *PaginationTestSuite) TestListDeclarativeActionsWithLimit() {
	resources, err := listResources(declarativeResourceServerID, "", 0, 100)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(resources.Resources)

	var declarativeResourceID string
	for _, res := range resources.Resources {
		if res.Handle == declarativeResourceHandle {
			declarativeResourceID = res.ID
		}
	}
	suite.Require().NotEmpty(declarativeResourceID)

	list, err := listActionsAtResource(declarativeResourceServerID, declarativeResourceID, 1, 1)
	suite.Require().NoError(err)
	suite.Equal(2, list.TotalResults)
	suite.Equal(1, list.Count)
	suite.Equal(2, list.StartIndex)

	links := linksByRel(list.Links)
	baseURL := fmt.Sprintf("/resource-servers/%s/resources/%s/actions", declarativeResourceServerID, declarativeResourceID)
	suite.Equal(baseURL+"?offset=0&limit=1", links["first"])
	suite.Equal(baseURL+"?offset=0&limit=1", links["prev"])
}

// Helper functions

// linksByRel returns a map from relation to href for a set of pagination links.
func linksByRel(links []LinkResponse) map[string]string {
	byRel := make(map[string]string, len(links))
	for _, link := range links {
		byRel[link.Rel] = link.Href
	}
	return byRel
}

func createResource(resourceServerID string, req CreateResourceRequest) (string, error) {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/resource-servers/%s/resources", testServerURL, resourceServerID)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
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

	var res ResourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return res.ID, nil
}

func getResource(resourceServerID, resourceID string) (*ResourceResponse, error) {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s", testServerURL, resourceServerID, resourceID)
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

	var res ResourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &res, nil
}

func listResources(resourceServerID string, parent string, offset, limit int) (*ResourceListResponse, error) {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/resources?offset=%d&limit=%d", testServerURL, resourceServerID, offset, limit)
	if parent != "" {
		url += "&parentId=" + parent
	}

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

	var list ResourceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return &list, nil
}

func updateResource(resourceServerID, resourceID string, req UpdateResourceRequest) error {
	client := testutils.GetHTTPClient()

	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s", testServerURL, resourceServerID, resourceID)
	httpReq, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
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

func deleteResource(resourceServerID, resourceID string) error {
	client := testutils.GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s", testServerURL, resourceServerID, resourceID)
	httpReq, err := http.NewRequest("DELETE", url, nil)
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

func stringPtr(s string) *string {
	return &s
}
