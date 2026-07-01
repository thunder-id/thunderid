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

package ou

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DeclarativeResourceTestSuite struct {
	suite.Suite
	mockService *OrganizationUnitServiceInterfaceMock
	exporter    declarativeresource.ResourceExporter
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) SetupTest() {
	s.mockService = NewOrganizationUnitServiceInterfaceMock(s.T())
	s.exporter = NewOUExporterForTest(s.mockService)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceType() {
	resourceType := s.exporter.GetResourceType()
	assert.Equal(s.T(), "organization_unit", resourceType)
}

func (s *DeclarativeResourceTestSuite) TestGetParameterizerType() {
	paramType := s.exporter.GetParameterizerType()
	assert.Equal(s.T(), "OrganizationUnit", paramType)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByID() {
	ou := providers.OrganizationUnit{
		ID:          "test-ou-1",
		Handle:      "test",
		Name:        "Test OU",
		Description: "Test organization unit",
		Parent:      nil,
	}

	s.mockService.EXPECT().GetOrganizationUnit(mock.Anything, "test-ou-1").
		Return(ou, (*tidcommon.ServiceError)(nil))

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "test-ou-1")
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test OU", name)
	assert.NotNil(s.T(), resource)

	retrievedOU, ok := resource.(*providers.OrganizationUnit)
	assert.True(s.T(), ok)
	assert.Equal(s.T(), "test-ou-1", retrievedOU.ID)
}

func (s *DeclarativeResourceTestSuite) TestValidateResource() {
	ou := &providers.OrganizationUnit{
		ID:     "test-ou-1",
		Handle: "test",
		Name:   "Test OU",
	}

	name, err := s.exporter.ValidateResource(context.Background(), ou, "test-ou-1", nil)
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test OU", name)
}

func (s *DeclarativeResourceTestSuite) TestValidateResourceInvalidType() {
	invalidResource := "not an OU"

	name, err := s.exporter.ValidateResource(context.Background(), invalidResource, "test-id", nil)
	assert.NotNil(s.T(), err)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *DeclarativeResourceTestSuite) TestParseToOU() {
	yamlData := []byte(`
id: test-ou-1
handle: test
name: Test OU
description: Test organization unit
parent: parent-id
`)

	ou, err := parseToOU(yamlData)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), ou)
	assert.Equal(s.T(), "test-ou-1", ou.ID)
	assert.Equal(s.T(), "test", ou.Handle)
	assert.Equal(s.T(), "Test OU", ou.Name)
	assert.Equal(s.T(), "Test organization unit", ou.Description)
	assert.NotNil(s.T(), ou.Parent)
	assert.Equal(s.T(), "parent-id", *ou.Parent)
}

func (s *DeclarativeResourceTestSuite) TestParseToOUWithoutParent() {
	yamlData := []byte(`
id: root-ou
handle: root
name: Root OU
description: Root organization unit
`)

	ou, err := parseToOU(yamlData)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), ou)
	assert.Equal(s.T(), "root-ou", ou.ID)
	assert.Equal(s.T(), "root", ou.Handle)
	assert.Equal(s.T(), "Root OU", ou.Name)
	assert.Nil(s.T(), ou.Parent)
}

func (s *DeclarativeResourceTestSuite) TestValidateOUWrapper() {
	fbStore, _ := newFileBasedStore()
	store := fbStore.(*fileBasedStore)
	ou := &providers.OrganizationUnit{
		ID:     "test-ou-1",
		Handle: "test",
		Name:   "Test OU",
	}

	err := validateOUWrapper(ou, store, nil)
	assert.NoError(s.T(), err)
}

func (s *DeclarativeResourceTestSuite) TestValidateOUWrapperMissingID() {
	fbStore, _ := newFileBasedStore()
	store := fbStore.(*fileBasedStore)
	ou := &providers.OrganizationUnit{
		Handle: "test",
		Name:   "Test OU",
	}

	err := validateOUWrapper(ou, store, nil)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "ID is required")
}

func (s *DeclarativeResourceTestSuite) TestValidateOUWrapperMissingName() {
	fbStore, _ := newFileBasedStore()
	store := fbStore.(*fileBasedStore)
	ou := &providers.OrganizationUnit{
		ID:     "test-ou-1",
		Handle: "test",
	}

	err := validateOUWrapper(ou, store, nil)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "name is required")
}

func (s *DeclarativeResourceTestSuite) TestValidateOUWrapperMissingHandle() {
	fbStore, _ := newFileBasedStore()
	store := fbStore.(*fileBasedStore)
	ou := &providers.OrganizationUnit{
		ID:   "test-ou-1",
		Name: "Test OU",
	}

	err := validateOUWrapper(ou, store, nil)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "handle is required")
}

func (s *DeclarativeResourceTestSuite) TestValidateOUWrapperDuplicateID() {
	fbStore, _ := newFileBasedStore()
	store := fbStore.(*fileBasedStore)

	// First OU - should succeed
	ou1 := &providers.OrganizationUnit{
		ID:     "test-ou-duplicate",
		Handle: "test1",
		Name:   "Test OU 1",
	}

	err := store.CreateOrganizationUnit(context.Background(), *ou1)
	assert.NoError(s.T(), err)

	// Second OU with same ID - should fail validation
	ou2 := &providers.OrganizationUnit{
		ID:     "test-ou-duplicate",
		Handle: "test2",
		Name:   "Test OU 2",
	}

	err = validateOUWrapper(ou2, store, nil)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "duplicate organization unit ID")
	assert.Contains(s.T(), err.Error(), "test-ou-duplicate")
	assert.Contains(s.T(), err.Error(), "declarative resources")
}

func (s *DeclarativeResourceTestSuite) TestValidateOUWrapperDuplicateIDInDBStore() {
	fs, _ := newFileBasedStore()
	fileStore := fs.(*fileBasedStore)
	dbStore := newOrganizationUnitStoreInterfaceMock(s.T())

	// Mock dbStore to return that the ID exists
	dbStore.On("IsOrganizationUnitExists", mock.Anything, "test-ou-db-duplicate").
		Return(true, nil).
		Once()

	// Try to add an OU with an ID that exists in DB
	ou := &providers.OrganizationUnit{
		ID:     "test-ou-db-duplicate",
		Handle: "test",
		Name:   "Test OU",
	}

	err := validateOUWrapper(ou, fileStore, dbStore)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "duplicate organization unit ID")
	assert.Contains(s.T(), err.Error(), "test-ou-db-duplicate")
	assert.Contains(s.T(), err.Error(), "database")

	dbStore.AssertExpectations(s.T())
}

func (s *DeclarativeResourceTestSuite) TestValidateOUWrapperNoDuplicateInCompositeMode() {
	fs, _ := newFileBasedStore()
	fileStore := fs.(*fileBasedStore)
	dbStore := newOrganizationUnitStoreInterfaceMock(s.T())

	// Mock dbStore to return that the ID does not exist
	dbStore.On("IsOrganizationUnitExists", mock.Anything, "test-ou-new").
		Return(false, nil).
		Once()

	// Valid OU that doesn't exist in either store
	ou := &providers.OrganizationUnit{
		ID:     "test-ou-new",
		Handle: "test",
		Name:   "Test OU",
	}

	err := validateOUWrapper(ou, fileStore, dbStore)
	assert.NoError(s.T(), err)

	dbStore.AssertExpectations(s.T())
}

func (s *DeclarativeResourceTestSuite) TestValidateOUWrapperErrorInDBStore() {
	fs, _ := newFileBasedStore()
	fileStore := fs.(*fileBasedStore)
	dbStore := newOrganizationUnitStoreInterfaceMock(s.T())

	// Mock dbStore to return an error
	dbStore.On("IsOrganizationUnitExists", mock.Anything, "test-ou-db-error").
		Return(false, fmt.Errorf("database connection failed")).
		Once()

	// Try to add an OU when DB check fails
	ou := &providers.OrganizationUnit{
		ID:     "test-ou-db-error",
		Handle: "test",
		Name:   "Test OU",
	}

	err := validateOUWrapper(ou, fileStore, dbStore)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to check organization unit existence")
	assert.Contains(s.T(), err.Error(), "database connection failed")

	dbStore.AssertExpectations(s.T())
}

func (s *DeclarativeResourceTestSuite) TestGetResourceRules() {
	rules := s.exporter.GetResourceRules()
	assert.NotNil(s.T(), rules)
	assert.Empty(s.T(), rules.Variables)
	assert.Empty(s.T(), rules.ArrayVariables)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_NoOUs() {
	// Test with empty result
	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.Nil(s.T(), err)
	assert.Empty(s.T(), ids)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_RootOUsOnly() {
	// Test with only root OUs (no children)
	rootOU1 := providers.OrganizationUnitBasic{
		ID:     "root-1",
		Handle: "root1",
		Name:   "Root 1",
	}
	rootOU2 := providers.OrganizationUnitBasic{
		ID:     "root-2",
		Handle: "root2",
		Name:   "Root 2",
	}

	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{rootOU1, rootOU2},
		}, (*tidcommon.ServiceError)(nil))

	// Mock IsOrganizationUnitDeclarative to indicate these are mutable OUs
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "root-1").Return(false)
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "root-2").Return(false)

	// Mock GetOrganizationUnitChildren to return empty lists for both roots
	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "root-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "root-2", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Contains(s.T(), ids, "root-1")
	assert.Contains(s.T(), ids, "root-2")
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_WithChildren() {
	// Test with nested OUs
	rootOU := providers.OrganizationUnitBasic{
		ID:     "root-1",
		Handle: "root",
		Name:   "Root OU",
	}
	childOU := providers.OrganizationUnitBasic{
		ID:     "child-1",
		Handle: "child",
		Name:   "Child OU",
	}
	grandchildOU := providers.OrganizationUnitBasic{
		ID:     "grandchild-1",
		Handle: "grandchild",
		Name:   "Grandchild OU",
	}

	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{rootOU},
		}, (*tidcommon.ServiceError)(nil))

	// Mock IsOrganizationUnitDeclarative to indicate these are mutable OUs
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "root-1").Return(false)

	// Mock children at each level
	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "root-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{childOU},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "child-1").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "child-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{grandchildOU},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "grandchild-1").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "grandchild-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 3)
	assert.Contains(s.T(), ids, "root-1")
	assert.Contains(s.T(), ids, "child-1")
	assert.Contains(s.T(), ids, "grandchild-1")
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_MultipleRootsWithChildren() {
	// Test with multiple root OUs each having children
	rootOU1 := providers.OrganizationUnitBasic{
		ID:     "root-1",
		Handle: "root1",
		Name:   "Root 1",
	}
	rootOU2 := providers.OrganizationUnitBasic{
		ID:     "root-2",
		Handle: "root2",
		Name:   "Root 2",
	}
	child1 := providers.OrganizationUnitBasic{
		ID:     "child-1",
		Handle: "child1",
		Name:   "Child 1",
	}
	child2 := providers.OrganizationUnitBasic{
		ID:     "child-2",
		Handle: "child2",
		Name:   "Child 2",
	}

	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{rootOU1, rootOU2},
		}, (*tidcommon.ServiceError)(nil))

	// Mock IsOrganizationUnitDeclarative to indicate these are mutable OUs
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "root-1").Return(false)
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "root-2").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "root-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{child1},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "child-1").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "child-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "root-2", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{child2},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "child-2").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "child-2", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 4)
	assert.Contains(s.T(), ids, "root-1")
	assert.Contains(s.T(), ids, "root-2")
	assert.Contains(s.T(), ids, "child-1")
	assert.Contains(s.T(), ids, "child-2")
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_ErrorGettingList() {
	// Test error handling when getting the OU list fails
	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(
			(*providers.OrganizationUnitListResponse)(nil),
			&tidcommon.InternalServerError,
		)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.NotNil(s.T(), err)
	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, err.Code)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_ErrorGettingChildren() {
	// Test error handling when getting children fails
	rootOU := providers.OrganizationUnitBasic{
		ID:     "root-1",
		Handle: "root",
		Name:   "Root OU",
	}

	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{rootOU},
		}, (*tidcommon.ServiceError)(nil))

	// Mock IsOrganizationUnitDeclarative to indicate this is a mutable OU
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "root-1").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "root-1", 100, 0, mock.Anything).
		Return(
			(*providers.OrganizationUnitListResponse)(nil),
			&tidcommon.InternalServerError,
		)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.NotNil(s.T(), err)
	assert.Nil(s.T(), ids)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_DeepNesting() {
	// Test with deeply nested hierarchy (5 levels)
	level1 := providers.OrganizationUnitBasic{ID: "level-1", Handle: "l1", Name: "Level 1"}
	level2 := providers.OrganizationUnitBasic{ID: "level-2", Handle: "l2", Name: "Level 2"}
	level3 := providers.OrganizationUnitBasic{ID: "level-3", Handle: "l3", Name: "Level 3"}
	level4 := providers.OrganizationUnitBasic{ID: "level-4", Handle: "l4", Name: "Level 4"}
	level5 := providers.OrganizationUnitBasic{ID: "level-5", Handle: "l5", Name: "Level 5"}

	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{level1},
		}, (*tidcommon.ServiceError)(nil))

	// Mock IsOrganizationUnitDeclarative for all levels
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "level-1").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "level-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{level2},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "level-2").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "level-2", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{level3},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "level-3").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "level-3", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{level4},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "level-4").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "level-4", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{level5},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "level-5").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "level-5", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 5)
	for i := 1; i <= 5; i++ {
		assert.Contains(s.T(), ids, "level-"+strconv.Itoa(i))
	}
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_MultipleChildrenPerLevel() {
	// Test with multiple children at same level
	rootOU := providers.OrganizationUnitBasic{
		ID:     "root-1",
		Handle: "root",
		Name:   "Root OU",
	}
	child1 := providers.OrganizationUnitBasic{
		ID:     "child-1",
		Handle: "child1",
		Name:   "Child 1",
	}
	child2 := providers.OrganizationUnitBasic{
		ID:     "child-2",
		Handle: "child2",
		Name:   "Child 2",
	}
	child3 := providers.OrganizationUnitBasic{
		ID:     "child-3",
		Handle: "child3",
		Name:   "Child 3",
	}

	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{rootOU},
		}, (*tidcommon.ServiceError)(nil))

	// Mock IsOrganizationUnitDeclarative for root
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "root-1").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "root-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{child1, child2, child3},
		}, (*tidcommon.ServiceError)(nil))

	// Mock IsOrganizationUnitDeclarative for all children
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "child-1").Return(false)
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "child-2").Return(false)
	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "child-3").Return(false)

	// Each child has no children
	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "child-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "child-2", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "child-3", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{},
		}, (*tidcommon.ServiceError)(nil))

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 4)
	assert.Contains(s.T(), ids, "root-1")
	assert.Contains(s.T(), ids, "child-1")
	assert.Contains(s.T(), ids, "child-2")
	assert.Contains(s.T(), ids, "child-3")
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDs_ErrorGettingGrandchildren() {
	rootOU := providers.OrganizationUnitBasic{
		ID:     "root-1",
		Handle: "root",
		Name:   "Root OU",
	}
	childOU := providers.OrganizationUnitBasic{
		ID:     "child-1",
		Handle: "child",
		Name:   "Child OU",
	}

	s.mockService.EXPECT().
		GetOrganizationUnitList(mock.Anything, 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{rootOU},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "root-1").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "root-1", 100, 0, mock.Anything).
		Return(&providers.OrganizationUnitListResponse{
			OrganizationUnits: []providers.OrganizationUnitBasic{childOU},
		}, (*tidcommon.ServiceError)(nil))

	s.mockService.EXPECT().IsOrganizationUnitDeclarative(mock.Anything, "child-1").Return(false)

	s.mockService.EXPECT().
		GetOrganizationUnitChildren(mock.Anything, "child-1", 100, 0, mock.Anything).
		Return(
			(*providers.OrganizationUnitListResponse)(nil),
			&tidcommon.InternalServerError,
		)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())
	assert.NotNil(s.T(), err)
	assert.Nil(s.T(), ids)
}
