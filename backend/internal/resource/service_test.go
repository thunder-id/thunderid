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
	"context"
	"errors"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/consent"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/tests/mocks/consentmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

// newDisabledConsentServiceMock returns a consent service mock with IsEnabled returning false,
// suitable for resource tests that do not assert on consent sync behavior.
func newDisabledConsentServiceMock(t interface {
	mock.TestingT
	Cleanup(func())
}) *consentmock.ConsentServiceInterfaceMock {
	m := consentmock.NewConsentServiceInterfaceMock(t)
	m.On("IsEnabled").Return(false).Maybe()
	return m
}

const (
	testOriginalName    = "original-name"
	testOriginalHandle  = "original-handle"
	testUpdatedName     = "updated-name"
	testNewDescription  = "new description"
	testWrongResourceID = "res-wrong"
	declarativeRSID     = "declarative-rs"
)

var testParentResourceID = "parent-123"
var testEmptyResourceID = ""

// matchResourceServer is a matcher function that compares providers.ResourceServer ignoring the Delimiter field
// since it's set by the service before calling the store.
func matchResourceServer(expected providers.ResourceServer) interface{} {
	return mock.MatchedBy(func(actual providers.ResourceServer) bool {
		return actual.Name == expected.Name &&
			actual.Description == expected.Description &&
			actual.Identifier == expected.Identifier &&
			actual.OUID == expected.OUID &&
			actual.Delimiter != "" // Delimiter should be set
	})
}

// matchResource is a matcher function that compares providers.Resource ignoring the Permission field
// since it's computed by the service before calling the store.
func matchResource(expected providers.Resource) interface{} {
	return mock.MatchedBy(func(actual providers.Resource) bool {
		parentsMatch := expected.Parent == actual.Parent
		return actual.Name == expected.Name &&
			actual.Handle == expected.Handle &&
			actual.Description == expected.Description &&
			parentsMatch &&
			actual.Permission != "" // Permission should be computed
	})
}

// matchAction is a matcher function that compares providers.Action ignoring the Permission field
// since it's computed by the service before calling the store.
func matchAction(expected providers.Action) interface{} {
	return mock.MatchedBy(func(actual providers.Action) bool {
		return actual.Name == expected.Name &&
			actual.Handle == expected.Handle &&
			actual.Description == expected.Description &&
			actual.Permission != "" // Permission should be computed
	})
}

// Test Suite
type ResourceServiceTestSuite struct {
	suite.Suite
	mockStore         *resourceStoreInterfaceMock
	mockOU            *oumock.OrganizationUnitServiceInterfaceMock
	mockTransactioner *fakeTransactioner
	service           ResourceServiceInterface
}

func TestResourceServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServiceTestSuite))
}

func (suite *ResourceServiceTestSuite) SetupTest() {
	// Initialize runtime config for the test
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: engineconfig.ServerConfig{
			Identifier: "test-deployment",
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	require.NoError(suite.T(), err)
	defer config.ResetServerRuntime()

	suite.mockStore = newResourceStoreInterfaceMock(suite.T())
	suite.mockOU = new(oumock.OrganizationUnitServiceInterfaceMock)
	suite.mockTransactioner = &fakeTransactioner{}
	suite.service, err = newResourceService(
		suite.mockOU, newDisabledConsentServiceMock(suite.T()), suite.mockStore, suite.mockTransactioner,
	)
	suite.NoError(err)
	// The resource service is its own dependency provider: deletion consults the registry, which
	// resolves resource/action dependents back through the service.
	suite.service.SetDependencyRegistry(resourcedependency.Initialize(suite.service))
}

func (suite *ResourceServiceTestSuite) TearDownTest() {
	// Reset config to clear singleton state for next test
	config.ResetServerRuntime()
}

// Service Initialization Tests

func (suite *ResourceServiceTestSuite) TestNewResourceService_InvalidDelimiter() {
	// Test with an invalid delimiter character (e.g., " which is 0x22, not allowed)
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: engineconfig.ServerConfig{
			Identifier: "test-deployment",
		},
		Resource: engineconfig.ResourceConfig{
			DefaultDelimiter: "\"", // Invalid character (0x22)
		},
	}
	_ = config.InitializeServerRuntime("test-invalid-delimiter", testConfig)
	defer config.ResetServerRuntime()

	mockStore := newResourceStoreInterfaceMock(suite.T())
	mockOU := new(oumock.OrganizationUnitServiceInterfaceMock)

	mockTransactioner := &fakeTransactioner{}
	service, err := newResourceService(mockOU, newDisabledConsentServiceMock(suite.T()), mockStore, mockTransactioner)

	suite.Error(err)
	suite.Nil(service)
	suite.Contains(err.Error(), "configured permission delimiter is invalid")
}

// Resource Server Tests

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_Success() {
	rs := providers.ResourceServer{
		Name:        "test-rs",
		Description: "Test resource server",
		Identifier:  "test-identifier",
		OUID:        "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything,
		"test-rs").
		Return(false, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"test-identifier").
		Return(false, nil)
	suite.mockStore.On("CreateResourceServer", mock.Anything,
		mock.AnythingOfType("string"), matchResourceServer(rs)).
		Return(nil)

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotEmpty(result.ID)
	suite.Equal("test-rs", result.Name)
	suite.Equal("Test resource server", result.Description)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockOU.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_WithType() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		Type:       providers.ResourceServerTypeMCP,
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything, "test-rs").Return(false, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything, "test-identifier").Return(false, nil)
	suite.mockStore.On("CreateResourceServer", mock.Anything,
		mock.AnythingOfType("string"), mock.MatchedBy(func(r providers.ResourceServer) bool {
			return r.Type == providers.ResourceServerTypeMCP
		})).Return(nil)

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(providers.ResourceServerTypeMCP, result.Type)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_DefaultsToCustom() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything, "test-rs").Return(false, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything, "test-identifier").Return(false, nil)
	suite.mockStore.On("CreateResourceServer", mock.Anything,
		mock.AnythingOfType("string"), mock.MatchedBy(func(r providers.ResourceServer) bool {
			return r.Type == providers.ResourceServerTypeCustom
		})).Return(nil)

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(providers.ResourceServerTypeCustom, result.Type)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_InvalidType() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		Type:       providers.ResourceServerType("BOGUS"),
		OUID:       "ou-123",
	}

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRequestFormat.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_ValidationErrors() {
	testCases := []struct {
		name           string
		resourceServer providers.ResourceServer
		expectedError  tidcommon.ServiceError
	}{
		{
			name:           "EmptyName",
			resourceServer: providers.ResourceServer{Name: "", OUID: "ou-123"},
			expectedError:  ErrorInvalidRequestFormat,
		},
		{
			name:           "EmptyOU",
			resourceServer: providers.ResourceServer{Name: "test-rs", OUID: ""},
			expectedError:  ErrorInvalidRequestFormat,
		},
		{
			name: "InvalidDelimiter",
			resourceServer: providers.ResourceServer{
				Name:       "test-rs",
				Identifier: "test-identifier",
				Delimiter:  "::",
				OUID:       "ou-123",
			},
			expectedError: ErrorInvalidDelimiter,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := suite.service.CreateResourceServer(context.Background(), tc.resourceServer)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError.Code, err.Code)
		})
	}
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_OUNotFound() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound)

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorOrganizationUnitNotFound.Code, err.Code)
	suite.mockOU.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_OUServiceError() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{}, &tidcommon.InternalServerError)

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_NameConflict() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything,
		"test-rs").
		Return(true, nil)

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorNameConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_StoreError() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		OUID:       "ou-123",
		Identifier: "test-identifier",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything,
		"test-rs").
		Return(false, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"test-identifier").
		Return(false, nil)
	suite.mockStore.On("CreateResourceServer", mock.Anything,
		mock.AnythingOfType("string"), matchResourceServer(rs)).
		Return(errors.New("database error"))

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_IdentifierConflict() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything,
		"test-rs").
		Return(false, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"test-identifier").
		Return(true, nil)

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorIdentifierConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_CheckNameError() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything,
		"test-rs").
		Return(false, errors.New("database error"))

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_CheckIdentifierError() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything,
		"test-rs").
		Return(false, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"test-identifier").
		Return(false, errors.New("database error"))

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResourceServer_Success() {
	expectedRS := providers.ResourceServer{
		ID:          "rs-123",
		Name:        "test-rs",
		Description: "Test",
		OUID:        "ou-123",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").
		Return(expectedRS, nil)

	result, err := suite.service.GetResourceServer(context.Background(), "rs-123")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("rs-123", result.ID)
	suite.Equal("test-rs", result.Name)
}

func (suite *ResourceServiceTestSuite) TestGetResourceServer_MissingID() {
	result, err := suite.service.GetResourceServer(context.Background(), "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResourceServer_NotFound() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").
		Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := suite.service.GetResourceServer(context.Background(), "rs-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResourceServer_StoreError() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").
		Return(providers.ResourceServer{}, errors.New("database error"))

	result, err := suite.service.GetResourceServer(context.Background(), "rs-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResourceServerList_Success() {
	resourceServers := []providers.ResourceServer{
		{ID: "rs-1", Name: "RS 1"},
		{ID: "rs-2", Name: "RS 2"},
	}

	suite.mockStore.On("GetResourceServerListCount", mock.Anything).Return(2, nil)
	suite.mockStore.On("GetResourceServerList", mock.Anything,
		30, 0).Return(resourceServers, nil)

	result, err := suite.service.GetResourceServerList(context.Background(), 30, 0)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(2, result.TotalResults)
	suite.Equal(2, result.Count)
	suite.Equal(2, len(result.ResourceServers))
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_Success() {
	rs := providers.ResourceServer{
		Name:        "updated-rs",
		Description: "Updated",
		Identifier:  "new-identifier",
		OUID:        "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:          "rs-123",
		Name:        "old-name",
		Description: "Old",
		Identifier:  "original-identifier",
		OUID:        "ou-123",
		Delimiter:   ":",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(existingRS, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"new-identifier").Return(false, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything,
		"updated-rs").
		Return(false, nil)
	suite.mockStore.On("UpdateResourceServer", mock.Anything,
		"rs-123", mock.MatchedBy(func(r providers.ResourceServer) bool {
			return r.Name == rs.Name &&
				r.Identifier == "new-identifier" &&
				r.Description == rs.Description &&
				r.Delimiter == existingRS.Delimiter
		})).Return(nil)

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("rs-123", result.ID)
	suite.Equal("updated-rs", result.Name)
	suite.Equal("new-identifier", result.Identifier)
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_PreservesType() {
	rs := providers.ResourceServer{
		Name:        "updated-rs",
		Description: "Updated",
		Identifier:  "original-identifier",
		Type:        providers.ResourceServerTypeMCP, // Should be ignored; type is immutable
		OUID:        "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:          "rs-123",
		Name:        "old-name",
		Description: "Old",
		Identifier:  "original-identifier",
		Type:        providers.ResourceServerTypeAPI,
		OUID:        "ou-123",
		Delimiter:   ":",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(existingRS, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything, "updated-rs").Return(false, nil)
	suite.mockStore.On("UpdateResourceServer", mock.Anything,
		"rs-123", mock.MatchedBy(func(r providers.ResourceServer) bool {
			return r.Type == providers.ResourceServerTypeAPI
		})).Return(nil)

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(providers.ResourceServerTypeAPI, result.Type)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_NotFound() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_ValidationErrors() {
	testCases := []struct {
		name           string
		id             string
		resourceServer providers.ResourceServer
		expectedError  tidcommon.ServiceError
	}{
		{
			name:           "MissingID",
			id:             "",
			resourceServer: providers.ResourceServer{Name: "test-rs", OUID: "ou-123"},
			expectedError:  ErrorMissingID,
		},
		{
			name:           "EmptyName",
			id:             "rs-123",
			resourceServer: providers.ResourceServer{Name: "", OUID: "ou-123"},
			expectedError:  ErrorInvalidRequestFormat,
		},
		{
			name:           "EmptyOU",
			id:             "rs-123",
			resourceServer: providers.ResourceServer{Name: "test-rs", OUID: ""},
			expectedError:  ErrorInvalidRequestFormat,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := suite.service.UpdateResourceServer(context.Background(), tc.id, tc.resourceServer)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError.Code, err.Code)
		})
	}
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_OUNotFound() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:         "rs-123",
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-old",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(existingRS, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound)

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorOrganizationUnitNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_OUServiceError() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:         "rs-123",
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-old",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(existingRS, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{}, &tidcommon.InternalServerError)

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_NameConflict() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:         "rs-123",
		Name:       "old-name",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(existingRS, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything,
		"test-rs").
		Return(true, nil)

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorNameConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_StoreError() {
	rs := providers.ResourceServer{
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:         "rs-123",
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(existingRS, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("UpdateResourceServer", mock.Anything,
		"rs-123", mock.Anything).
		Return(errors.New("database error"))

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_GetResourceServerStoreError() {
	rs := providers.ResourceServer{
		Name:       "updated-name",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	// Mock GetResourceServer to return generic database error (not errResourceServerNotFound)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").
		Return(providers.ResourceServer{}, errors.New("database connection failed"))

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_Success() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything,
		"rs-123").Return(false, nil)
	suite.mockStore.On("DeleteResourceServer", mock.Anything,
		"rs-123").Return(nil)

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.Nil(err)
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_IdempotentWhenNotExists() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.Nil(err)
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_MissingID() {
	err := suite.service.DeleteResourceServer(context.Background(), "")

	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_CheckExistenceError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errors.New("database error"))

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_CheckDependenciesError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything,
		"rs-123").Return(false, errors.New("database error"))

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_DeleteError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything,
		"rs-123").Return(false, nil)
	suite.mockStore.On("DeleteResourceServer", mock.Anything,
		"rs-123").Return(errors.New("database error"))

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_HasDependencies() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything,
		"rs-123").Return(true, nil)

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_WithOnlyResources() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything,
		"rs-123").Return(true, nil)

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_WithOnlyActions() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything,
		"rs-123").Return(true, nil)

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_WithResourcesAndActions() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything,
		"rs-123").Return(true, nil)

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_WithNestedResources() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything,
		"rs-123").Return(true, nil)

	err := suite.service.DeleteResourceServer(context.Background(), "rs-123")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// providers.Resource Tests

func (suite *ResourceServiceTestSuite) TestCreateResource_Success() {
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
		Parent: nil,
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "test-handle", (*string)(nil)).
		Return(false, nil)
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", (*string)(nil), matchResource(res)).
		Return(nil)

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotEmpty(result.ID)
	suite.Equal("test-resource", result.Name)
	suite.Equal("test-handle", result.Handle)
}

func (suite *ResourceServiceTestSuite) TestCreateResource_ValidationErrors() {
	testCases := []struct {
		name          string
		resource      providers.Resource
		expectedError tidcommon.ServiceError
	}{
		{
			name:          "EmptyName",
			resource:      providers.Resource{Name: "", Handle: "test-handle"},
			expectedError: ErrorInvalidRequestFormat,
		},
		{
			name:          "EmptyHandle",
			resource:      providers.Resource{Name: "valid-name", Handle: ""},
			expectedError: ErrorInvalidRequestFormat,
		},
		{
			name:          "InvalidDelimiterInHandle",
			resource:      providers.Resource{Name: "valid-name", Handle: "invalid handle"},
			expectedError: ErrorInvalidHandle,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockStore.On("GetResourceServer", mock.Anything,
				"rs-123").Return(providers.ResourceServer{}, nil).Once()

			result, err := suite.service.CreateResource(context.Background(), "rs-123", tc.resource)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError.Code, err.Code)
		})
	}
}

// Parent-Child Advanced Tests

func (suite *ResourceServiceTestSuite) TestCreateResource_MultiLevelHierarchy() {
	// Create root resource
	rootRes := providers.Resource{
		Name:   "Root providers.Resource",
		Handle: "root",
		Parent: nil,
	}
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "root", (*string)(nil)).Return(false, nil).Once()
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123",
		(*string)(nil), matchResource(rootRes)).Return(nil).Once()

	result1, err1 := suite.service.CreateResource(context.Background(), "rs-123", rootRes)
	suite.Nil(err1)
	suite.NotNil(result1)

	// Use the generated root ID for child resource
	rootID := result1.ID
	childRes := providers.Resource{
		Name:   "Child providers.Resource",
		Handle: "child",
		Parent: &rootID,
	}
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("GetResource", mock.Anything,
		rootID, "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "child", &rootID).Return(false, nil).Once()
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", &rootID, matchResource(childRes)).
		Return(nil).Once()

	result2, err2 := suite.service.CreateResource(context.Background(), "rs-123", childRes)
	suite.Nil(err2)
	suite.NotNil(result2)

	// Use the generated child ID for grandchild resource
	childID := result2.ID
	grandchildRes := providers.Resource{
		Name:   "Grandchild providers.Resource",
		Handle: "grandchild",
		Parent: &childID,
	}
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("GetResource", mock.Anything,
		childID, "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "grandchild", &childID).Return(false, nil).Once()
	suite.mockStore.On(
		"CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", &childID, matchResource(grandchildRes),
	).Return(nil).Once()

	result3, err3 := suite.service.CreateResource(context.Background(), "rs-123", grandchildRes)
	suite.Nil(err3)
	suite.NotNil(result3)

	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_ChainDeletion() {
	// This test deletes resources in a chain and checks proper deletion
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	// Delete child first
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("GetResource", mock.Anything,
		"child-res", "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"child-res").Return(false, nil).Once()
	suite.mockStore.On("DeleteResource", mock.Anything,
		"child-res", "rs-123").Return(nil).Once()

	err1 := suite.service.DeleteResource(context.Background(), "rs-123", "child-res")
	suite.Nil(err1)

	// Now delete parent (should succeed since child is gone)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("GetResource", mock.Anything,
		"parent-res", "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"parent-res").Return(false, nil).Once()
	suite.mockStore.On("DeleteResource", mock.Anything,
		"parent-res", "rs-123").Return(nil).Once()

	err2 := suite.service.DeleteResource(context.Background(), "rs-123", "parent-res")
	suite.Nil(err2)

	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResource_WithParent_Success() {
	parentID := testParentResourceID
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
		Parent: &parentID,
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		testParentResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "test-handle", &parentID).
		Return(false, nil)
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", &parentID, matchResource(res)).
		Return(nil)

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("test-resource", result.Name)
	suite.Equal("test-handle", result.Handle)
	suite.Equal(&parentID, result.Parent)
}

func (suite *ResourceServiceTestSuite) TestCreateResource_ParentNotFound() {
	parentID := testParentResourceID
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
		Parent: &parentID,
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		testParentResourceID, "rs-123").Return(providers.Resource{}, errResourceNotFound)

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorParentResourceNotFound.Code, err.Code)
}

// Composite Foreign Key Validation Tests - Cross-Reference Validation

func (suite *ResourceServiceTestSuite) TestCreateResource_ParentFromDifferentServer() {
	parentID := "parent-in-other-server"
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
		Parent: &parentID,
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-server-a").Return(providers.ResourceServer{}, nil)
	// Parent lookup fails because parent-in-other-server doesn't exist under server A
	suite.mockStore.On("GetResource", mock.Anything,
		parentID, "rs-server-a").
		Return(providers.Resource{}, errResourceNotFound)

	result, err := suite.service.CreateResource(context.Background(), "rs-server-a", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorParentResourceNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_ResourceFromDifferentServer() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-server-a").Return(providers.ResourceServer{}, nil)
	// providers.Resource lookup fails because res-from-server-b doesn't exist under server A
	suite.mockStore.On("GetResource", mock.Anything,
		"res-from-server-b", "rs-server-a").
		Return(providers.Resource{}, errResourceNotFound)

	resourceID := "res-from-server-b"
	result, err := suite.service.CreateAction(context.Background(), "rs-server-a", &resourceID, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResource_ComplexCrossReference() {
	parentBFromServer2 := "parent-b-server2"
	res := providers.Resource{
		Name:   "resource-c",
		Handle: "resource-c-handle",
		Parent: &parentBFromServer2,
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-server-1").Return(providers.ResourceServer{}, nil)
	// Parent B lookup fails in server 1's context because it belongs to server 2
	suite.mockStore.On("GetResource", mock.Anything,
		parentBFromServer2, "rs-server-1").
		Return(providers.Resource{}, errResourceNotFound)

	result, err := suite.service.CreateResource(context.Background(), "rs-server-1", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorParentResourceNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResource_ResourceServerNotFound() {
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResource_HandleConflict() {
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "test-handle", (*string)(nil)).
		Return(true, nil)

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorHandleConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResource_StoreError() {
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "test-handle", (*string)(nil)).
		Return(false, nil)
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", (*string)(nil), matchResource(res)).
		Return(errors.New("database error"))

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// Handle Uniqueness Scope Tests

func (suite *ResourceServiceTestSuite) TestCreateResource_SameHandleDifferentParents() {
	parentA := "parent-a"
	parentB := "parent-b"
	res1 := providers.Resource{
		Name:   "Users providers.Resource under Parent A",
		Handle: "users",
		Parent: &parentA,
	}
	res2 := providers.Resource{
		Name:   "Users providers.Resource under Parent B",
		Handle: "users",
		Parent: &parentB,
	}

	// First resource creation
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("GetResource", mock.Anything,
		"parent-a", "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "users", &parentA).Return(false, nil).Once()
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", &parentA, matchResource(res1)).
		Return(nil).Once()

	result1, err1 := suite.service.CreateResource(context.Background(), "rs-123", res1)

	suite.Nil(err1)
	suite.NotNil(result1)

	// Second resource creation with same handle but different parent
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("GetResource", mock.Anything,
		"parent-b", "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "users", &parentB).Return(false, nil).Once()
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", &parentB, matchResource(res2)).
		Return(nil).Once()

	result2, err2 := suite.service.CreateResource(context.Background(), "rs-123", res2)

	suite.Nil(err2)
	suite.NotNil(result2)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResource_SameHandleRootAndChild() {
	rootRes := providers.Resource{
		Name:   "Users at Root",
		Handle: "users",
		Parent: nil,
	}
	parentX := "parent-x"
	childRes := providers.Resource{
		Name:   "Users under Parent",
		Handle: "users",
		Parent: &parentX,
	}

	// Root resource creation
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "users", (*string)(nil)).Return(false, nil).Once()
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123",
		(*string)(nil), matchResource(rootRes)).Return(nil).Once()

	result1, err1 := suite.service.CreateResource(context.Background(), "rs-123", rootRes)

	suite.Nil(err1)
	suite.NotNil(result1)

	// Child resource creation with same handle
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("GetResource", mock.Anything,
		"parent-x", "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "users", &parentX).Return(false, nil).Once()
	suite.mockStore.On(
		"CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", &parentX, matchResource(childRes),
	).Return(nil).Once()

	result2, err2 := suite.service.CreateResource(context.Background(), "rs-123", childRes)

	suite.Nil(err2)
	suite.NotNil(result2)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateAction_SameHandleDifferentScopes() {
	serverAction := providers.Action{
		Name:   "Read at Server Level",
		Handle: "read",
	}
	resourceAction := providers.Action{
		Name:   "Read at providers.Resource Level",
		Handle: "read",
	}

	// Server-level action creation
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", (*string)(nil), "read").Return(false, nil).Once()
	suite.mockStore.On("CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123",
		(*string)(nil), matchAction(serverAction)).Return(nil).Once()

	result1, err1 := suite.service.CreateAction(context.Background(), "rs-123", nil, serverAction)

	suite.Nil(err1)
	suite.NotNil(result1)

	// providers.Resource-level action creation with same handle
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	resourceID := "res-456"
	suite.mockStore.On("GetResource", mock.Anything,
		"res-456", "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", &resourceID, "read").Return(false, nil).Once()
	suite.mockStore.On(
		"CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", &resourceID, matchAction(resourceAction),
	).Return(nil).Once()
	result2, err2 := suite.service.CreateAction(context.Background(), "rs-123", &resourceID, resourceAction)

	suite.Nil(err2)
	suite.NotNil(result2)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateAction_SameHandleDifferentResources() {
	action1 := providers.Action{
		Name:   "Read at providers.Resource A",
		Handle: "read",
	}
	action2 := providers.Action{
		Name:   "Read at providers.Resource B",
		Handle: "read",
	}

	// providers.Action at resource A
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	resourceA := "res-a"
	suite.mockStore.On("GetResource", mock.Anything,
		"res-a", "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", &resourceA, "read").Return(false, nil).Once()
	suite.mockStore.On("CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123",
		&resourceA, matchAction(action1)).Return(nil).Once()
	result1, err1 := suite.service.CreateAction(context.Background(), "rs-123", &resourceA, action1)

	suite.Nil(err1)
	suite.NotNil(result1)

	// providers.Action at resource B with same handle
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil).Once()
	resourceB := "res-b"
	suite.mockStore.On("GetResource", mock.Anything,
		"res-b", "rs-123").Return(providers.Resource{}, nil).Once()
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", &resourceB, "read").Return(false, nil).Once()
	suite.mockStore.On("CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123",
		&resourceB, matchAction(action2)).Return(nil).Once()
	result2, err2 := suite.service.CreateAction(context.Background(), "rs-123", &resourceB, action2)

	suite.Nil(err2)
	suite.NotNil(result2)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResource_CheckHandleError() {
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "test-handle", (*string)(nil)).
		Return(false, errors.New("database error"))

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResource_ParentCheckError() {
	parentID := testParentResourceID
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
		Parent: &parentID,
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		parentID, "rs-123").
		Return(providers.Resource{}, errors.New("database error"))

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResource_CircularDependency_SelfReference() {
	// Test creating a resource with itself as parent
	res := providers.Resource{
		Name:   "test-resource",
		Handle: "test-handle",
		Parent: nil,
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "test-handle", (*string)(nil)).
		Return(false, nil)
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", (*string)(nil), matchResource(res)).
		Return(nil)

	result, err := suite.service.CreateResource(context.Background(), "rs-123", res)

	// Should succeed initially - circular check would need to be in update
	suite.Nil(err)
	suite.NotNil(result)
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_Success() {
	currentResource := providers.Resource{
		ID:          "res-123",
		Name:        testOriginalName,
		Handle:      testOriginalHandle,
		Description: "old description",
	}

	updateReq := providers.Resource{
		Name:        testUpdatedName,
		Description: testNewDescription,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(currentResource, nil).Once()
	suite.mockStore.On("UpdateResource", mock.Anything,
		"res-123", "rs-123", mock.MatchedBy(func(r providers.Resource) bool {
			return r.Name == testUpdatedName && r.Handle == testOriginalHandle && r.Description == testNewDescription
		})).Return(nil)

	result, err := suite.service.UpdateResource(context.Background(), "rs-123", "res-123", updateReq)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("res-123", result.ID)
	suite.Equal(testUpdatedName, result.Name)
	suite.Equal(testOriginalHandle, result.Handle) // Handle is immutable
	suite.Equal(testNewDescription, result.Description)
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_ParentIsImmutable() {
	currentResource := providers.Resource{
		ID:          "res-123",
		Name:        testOriginalName,
		Handle:      testOriginalHandle,
		Description: "old description",
		Parent:      nil,
	}

	newParentID := testParentResourceID
	updateReq := providers.Resource{
		Name:        testUpdatedName,
		Description: testNewDescription,
		Parent:      &newParentID, // Client attempts to set parent (should be ignored)
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(currentResource, nil).Once()
	suite.mockStore.On("UpdateResource", mock.Anything,
		"res-123", "rs-123", mock.MatchedBy(func(r providers.Resource) bool {
			// Verify parent is preserved from current resource (nil), NOT from updateReq
			// This validates immutability at the service layer
			return r.Name == testUpdatedName &&
				r.Handle == testOriginalHandle &&
				r.Parent == nil && // CRITICAL: Parent must remain nil
				r.Description == testNewDescription
		})).Return(nil)

	result, err := suite.service.UpdateResource(context.Background(), "rs-123", "res-123", updateReq)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("res-123", result.ID)
	suite.Equal(testUpdatedName, result.Name)
	suite.NotEqual(
		updateReq.Parent, result.Parent,
		"Parent field must be immutable - update request's parent should be ignored",
	)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_MissingID() {
	updateReq := providers.Resource{
		Description: testNewDescription,
	}

	result, err := suite.service.UpdateResource(context.Background(), "", "res-123", updateReq)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	result, err = suite.service.UpdateResource(context.Background(), "rs-123", "", updateReq)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_ResourceNotFound() {
	updateReq := providers.Resource{
		Description: testNewDescription,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, errResourceNotFound)

	result, err := suite.service.UpdateResource(context.Background(), "rs-123", "res-123", updateReq)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_HandleIsImmutable() {
	// Handle is immutable and preserved from current resource
	currentResource := providers.Resource{
		ID:          "res-123",
		Name:        testOriginalName,
		Handle:      testOriginalHandle,
		Description: "old description",
	}

	updateReq := providers.Resource{
		Name:        testUpdatedName,
		Handle:      "new-handle", // This will be ignored, handle is immutable
		Description: testNewDescription,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(currentResource, nil).Once()
	suite.mockStore.On("UpdateResource", mock.Anything,
		"res-123", "rs-123", mock.MatchedBy(func(r providers.Resource) bool {
			// Handle should be preserved from current resource, not from updateReq
			return r.Handle == testOriginalHandle && r.Name == testUpdatedName
		})).Return(nil)

	result, err := suite.service.UpdateResource(context.Background(), "rs-123", "res-123", updateReq)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testOriginalHandle, result.Handle) // Handle is preserved
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_StoreError() {
	currentResource := providers.Resource{
		ID:          "res-123",
		Name:        testOriginalName,
		Handle:      testOriginalHandle,
		Description: "old description",
	}

	updateReq := providers.Resource{
		Name:        testUpdatedName,
		Description: testNewDescription,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(currentResource, nil).Once()
	suite.mockStore.On("UpdateResource", mock.Anything,
		"res-123", "rs-123", mock.Anything).Return(errors.New("database error"))

	result, err := suite.service.UpdateResource(context.Background(), "rs-123", "res-123", updateReq)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_GetResourceError() {
	updateReq := providers.Resource{
		Description: testNewDescription,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, errors.New("database error"))

	result, err := suite.service.UpdateResource(context.Background(), "rs-123", "res-123", updateReq)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_ResourceServerNotFound() {
	updateReq := providers.Resource{
		Name:        testUpdatedName,
		Description: testNewDescription,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := suite.service.UpdateResource(context.Background(), "rs-123", "res-123", updateReq)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_CheckServerError() {
	updateReq := providers.Resource{
		Name:        testUpdatedName,
		Description: testNewDescription,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errors.New("database error"))

	result, err := suite.service.UpdateResource(context.Background(), "rs-123", "res-123", updateReq)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_Success() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"res-123").Return(false, nil)
	suite.mockStore.On("DeleteResource", mock.Anything,
		"res-123", "rs-123").Return(nil)

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-123")

	suite.Nil(err)
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_HasDependencies() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"res-123").Return(true, nil)

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-123")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
}

// providers.Resource Dependency Tests

func (suite *ResourceServiceTestSuite) TestDeleteResource_WithOnlyChildResources() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-parent", "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"res-parent").Return(true, nil)

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-parent")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_WithOnlyActions() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-with-actions", "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"res-with-actions").Return(true, nil)

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-with-actions")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_WithChildrenAndActions() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-complex", "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"res-complex").Return(true, nil)

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-complex")

	suite.NotNil(err)
	suite.Equal(ErrorCannotDelete.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_MissingID() {
	err := suite.service.DeleteResource(context.Background(), "", "res-123")
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	err = suite.service.DeleteResource(context.Background(), "rs-123", "")
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_Idempotent() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, errResourceNotFound)

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-123")

	suite.Nil(err)
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_DeleteError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"res-123").Return(false, nil)
	suite.mockStore.On("DeleteResource", mock.Anything,
		"res-123", "rs-123").Return(errors.New("database error"))

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_CheckExistenceError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, errors.New("database error"))

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_CheckResourceServerError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").
		Return(providers.ResourceServer{}, errors.New("database error"))

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_CheckDependenciesError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything,
		"res-123").Return(false, errors.New("database error"))

	err := suite.service.DeleteResource(context.Background(), "rs-123", "res-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// GetResource Tests

func (suite *ResourceServiceTestSuite) TestGetResource_Success() {
	expectedRes := providers.Resource{
		ID:          "res-123",
		Name:        "test-resource",
		Description: "Test",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(expectedRes, nil)

	result, err := suite.service.GetResource(context.Background(), "rs-123", "res-123")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("res-123", result.ID)
	suite.Equal("test-resource", result.Name)
}

func (suite *ResourceServiceTestSuite) TestGetResource_MissingID() {
	result, err := suite.service.GetResource(context.Background(), "", "res-123")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	result, err = suite.service.GetResource(context.Background(), "rs-123", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResource_ResourceServerNotFound() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := suite.service.GetResource(context.Background(), "rs-123", "res-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResource_NotFound() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, errResourceNotFound)

	result, err := suite.service.GetResource(context.Background(), "rs-123", "res-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResource_StoreError() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, errors.New("database error"))

	result, err := suite.service.GetResource(context.Background(), "rs-123", "res-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResource_CheckServerError() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errors.New("database error"))

	result, err := suite.service.GetResource(context.Background(), "rs-123", "res-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// Composite Foreign Key Validation Tests - Cross-Server providers.Resource Access

func (suite *ResourceServiceTestSuite) TestGetResource_WrongServerID() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-server-b").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-server-b").Return(providers.Resource{}, errResourceNotFound)

	result, err := suite.service.GetResource(context.Background(), "rs-server-b", "res-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_WrongServerID() {
	updateReq := providers.Resource{
		Name:        "updated-name",
		Handle:      "original-handle",
		Description: "updated description",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-wrong-server").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-wrong-server").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-wrong-server").Return(providers.Resource{}, errResourceNotFound)

	result, err := suite.service.UpdateResource(context.Background(), "rs-wrong-server", "res-123", updateReq)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_WrongServerID() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-wrong-server").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-wrong-server").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-wrong-server").
		Return(providers.Resource{}, errResourceNotFound)

	err := suite.service.DeleteResource(context.Background(), "rs-wrong-server", "res-123")

	suite.Nil(err) // Idempotent delete
	suite.mockStore.AssertExpectations(suite.T())
}

// GetResourceList Tests

func (suite *ResourceServiceTestSuite) TestGetResourceList() {
	testCases := []struct {
		name             string
		resourceServerID string
		parentID         *string
		limit            int
		offset           int
		setupMocks       func()
		expectedError    *tidcommon.ServiceError
		expectedCount    int
		validateResponse func(*ResourceList)
	}{
		{
			name:             "Success_NoFilter",
			resourceServerID: "rs-123",
			parentID:         nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResourceListCountByParent", mock.Anything,
					"rs-123", (*string)(nil)).Return(2, nil)
				suite.mockStore.On("GetResourceListByParent", mock.Anything,
					"rs-123", (*string)(nil), 30, 0).Return([]providers.Resource{
					{ID: "res-1", Name: "providers.Resource 1"},
					{ID: "res-2", Name: "providers.Resource 2"},
				}, nil)
			},
			expectedError: nil,
			expectedCount: 2,
			validateResponse: func(result *ResourceList) {
				suite.Equal(2, result.TotalResults)
				suite.Equal(2, result.Count)
				suite.Equal(2, len(result.Resources))
			},
		},
		{
			name:             "Success_WithParent",
			resourceServerID: "rs-123",
			parentID:         &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").Return(providers.Resource{}, nil)
				parentPtr := testParentResourceID
				suite.mockStore.On("GetResourceListCountByParent", mock.Anything,
					"rs-123", &parentPtr).Return(2, nil)
				suite.mockStore.On("GetResourceListByParent", mock.Anything,
					"rs-123", &parentPtr, 30, 0).Return([]providers.Resource{
					{ID: "res-1", Name: "providers.Resource 1"},
					{ID: "res-2", Name: "providers.Resource 2"},
				}, nil)
			},
			expectedError: nil,
			expectedCount: 2,
		},
		{
			name:             "Success_EmptyParent",
			resourceServerID: "rs-123",
			parentID:         &testEmptyResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					"", "rs-123").Return(providers.Resource{}, nil)
				emptyParent := ""
				suite.mockStore.On("GetResourceListCountByParent", mock.Anything,
					"rs-123", &emptyParent).Return(2, nil)
				suite.mockStore.On("GetResourceListByParent", mock.Anything,
					"rs-123", &emptyParent, 30, 0).Return([]providers.Resource{
					{ID: "res-1", Name: "Top Level 1"},
					{ID: "res-2", Name: "Top Level 2"},
				}, nil)
			},
			expectedError: nil,
			expectedCount: 2,
		},
		{
			name:             "Error_EmptyResourceServerID",
			resourceServerID: "",
			parentID:         nil,
			limit:            30,
			offset:           0,
			setupMocks:       func() {},
			expectedError:    &ErrorMissingID,
		},
		{
			name:             "Error_ResourceServerNotFound",
			resourceServerID: "rs-123",
			parentID:         nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			expectedError: &ErrorResourceServerNotFound,
		},
		{
			name:             "Error_ParentResourceNotFound",
			resourceServerID: "rs-123",
			parentID:         &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, errResourceNotFound)
			},
			expectedError: &ErrorResourceNotFound,
		},
		{
			name:             "Error_CheckResourceServerError",
			resourceServerID: "rs-123",
			parentID:         nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_CheckParentError",
			resourceServerID: "rs-123",
			parentID:         &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_CountByParentError",
			resourceServerID: "rs-123",
			parentID:         nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResourceListCountByParent", mock.Anything,
					"rs-123", (*string)(nil)).
					Return(0, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_ListByParentError",
			resourceServerID: "rs-123",
			parentID:         nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResourceListCountByParent", mock.Anything,
					"rs-123", (*string)(nil)).
					Return(10, nil)
				suite.mockStore.On("GetResourceListByParent", mock.Anything,
					"rs-123", (*string)(nil), 30, 0).
					Return(nil, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setupMocks()

			result, err := suite.service.GetResourceList(
				context.Background(), tc.resourceServerID, tc.parentID, tc.limit, tc.offset,
			)

			if tc.expectedError != nil {
				suite.Nil(result)
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
			} else {
				suite.Nil(err)
				suite.NotNil(result)
				if tc.validateResponse != nil {
					tc.validateResponse(result)
				}
			}
			suite.mockStore.AssertExpectations(suite.T())
		})
	}
}

func (suite *ResourceServiceTestSuite) TestGetAllResourceList() {
	testCases := []struct {
		name             string
		resourceServerID string
		setupMocks       func()
		expectedError    *tidcommon.ServiceError
		expectedCount    int
	}{
		{
			name:             "Success",
			resourceServerID: "rs-123",
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResourceListCount", mock.Anything,
					"rs-123").Return(2, nil)
				suite.mockStore.On("GetResourceList", mock.Anything,
					"rs-123", 2, 0).Return([]providers.Resource{
					{ID: "res-1", Name: "providers.Resource 1"},
					{ID: "res-2", Name: "providers.Resource 2"},
				}, nil)
			},
			expectedCount: 2,
		},
		{
			name:             "Success_Empty",
			resourceServerID: "rs-123",
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResourceListCount", mock.Anything,
					"rs-123").Return(0, nil)
			},
			expectedCount: 0,
		},
		{
			name:             "Error_EmptyResourceServerID",
			resourceServerID: "",
			setupMocks:       func() {},
			expectedError:    &ErrorMissingID,
		},
		{
			name:             "Error_ResourceServerNotFound",
			resourceServerID: "rs-123",
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			expectedError: &ErrorResourceServerNotFound,
		},
		{
			name:             "Error_CountError",
			resourceServerID: "rs-123",
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResourceListCount", mock.Anything,
					"rs-123").Return(0, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_ListError",
			resourceServerID: "rs-123",
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResourceListCount", mock.Anything,
					"rs-123").Return(5, nil)
				suite.mockStore.On("GetResourceList", mock.Anything,
					"rs-123", 5, 0).Return(nil, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setupMocks()

			result, err := suite.service.GetAllResourceList(context.Background(), tc.resourceServerID)

			if tc.expectedError != nil {
				suite.Nil(result)
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
			} else {
				suite.Nil(err)
				suite.NotNil(result)
				suite.Equal(tc.expectedCount, len(result))
			}
			suite.mockStore.AssertExpectations(suite.T())
		})
	}
}

// Action Tests

func (suite *ResourceServiceTestSuite) TestCreateActionAtResourceServer_Success() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", (*string)(nil), "test-handle").
		Return(false, nil)
	suite.mockStore.On("CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", (*string)(nil),
		mock.MatchedBy(func(a providers.Action) bool { return a.Handle != "" })).
		Return(nil)

	result, err := suite.service.CreateAction(context.Background(), "rs-123", nil, action)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotEmpty(result.ID)
	suite.Equal("test-action", result.Name)
	suite.Equal("test-handle", result.Handle)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResourceServer_ValidationErrors() {
	testCases := []struct {
		name          string
		action        providers.Action
		expectedError tidcommon.ServiceError
	}{
		{
			name:          "EmptyName",
			action:        providers.Action{Name: "", Handle: "test-handle"},
			expectedError: ErrorInvalidRequestFormat,
		},
		{
			name:          "EmptyHandle",
			action:        providers.Action{Name: "valid-name", Handle: ""},
			expectedError: ErrorInvalidRequestFormat,
		},
		{
			name:          "InvalidDelimiterInHandle",
			action:        providers.Action{Name: "valid-name", Handle: "invalid handle"},
			expectedError: ErrorInvalidHandle,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockStore.On("GetResourceServer", mock.Anything,
				"rs-123").Return(providers.ResourceServer{}, nil).Once()

			result, err := suite.service.CreateAction(context.Background(), "rs-123", nil, tc.action)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError.Code, err.Code)
		})
	}
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResourceServer_ResourceServerNotFound() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := suite.service.CreateAction(context.Background(), "rs-123", nil, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResourceServer_HandleConflict() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", (*string)(nil), "test-handle").
		Return(true, nil)

	result, err := suite.service.CreateAction(context.Background(), "rs-123", nil, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorHandleConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResourceServer_StoreError() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", (*string)(nil), "test-handle").
		Return(false, nil)
	suite.mockStore.On("CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", (*string)(nil),
		mock.MatchedBy(func(a providers.Action) bool { return a.Handle != "" })).
		Return(errors.New("database error"))

	result, err := suite.service.CreateAction(context.Background(), "rs-123", nil, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResourceServer_CheckHandleError() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", (*string)(nil), "test-handle").
		Return(false, errors.New("database error"))

	result, err := suite.service.CreateAction(context.Background(), "rs-123", nil, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_ValidationErrors() {
	testCases := []struct {
		name          string
		action        providers.Action
		expectedError tidcommon.ServiceError
	}{
		{
			name:          "EmptyName",
			action:        providers.Action{Name: "", Handle: "test-handle"},
			expectedError: ErrorInvalidRequestFormat,
		},
		{
			name:          "EmptyHandle",
			action:        providers.Action{Name: "valid-name", Handle: ""},
			expectedError: ErrorInvalidRequestFormat,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockStore.On("GetResourceServer", mock.Anything,
				"rs-123").Return(providers.ResourceServer{}, nil).Once()
			suite.mockStore.On("GetResource", mock.Anything,
				"res-123", "rs-123").Return(providers.Resource{}, nil).Once()

			resourceID := "res-123"
			result, err := suite.service.CreateAction(context.Background(), "rs-123", &resourceID, tc.action)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError.Code, err.Code)
		})
	}
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_Success() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resourceID := "res-123"
	suite.mockStore.On("GetResource", mock.Anything,
		"res-123", "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", &resourceID, "test-handle").Return(false, nil)
	suite.mockStore.On("CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123",
		&resourceID, matchAction(action)).Return(nil)
	result, err := suite.service.CreateAction(context.Background(), "rs-123", &resourceID, action)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("test-action", result.Name)
	suite.Equal("test-handle", result.Handle)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_ResourceServerNotFound() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	resID := testResourceID
	result, err := suite.service.CreateAction(context.Background(), "rs-123", &resID, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_ResourceNotFound() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, errResourceNotFound)

	resID := testResourceID
	result, err := suite.service.CreateAction(context.Background(), "rs-123", &resID, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_HandleConflict() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", &resID, "test-handle").Return(true, nil)
	result, err := suite.service.CreateAction(context.Background(), "rs-123", &resID, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorHandleConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_StoreError() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", &resID, "test-handle").Return(false, nil)
	suite.mockStore.On("CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", &resID, matchAction(action)).
		Return(errors.New("database error"))
	result, err := suite.service.CreateAction(context.Background(), "rs-123", &resID, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_CheckHandleError() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On(
		"CheckActionHandleExists", mock.Anything,
		"rs-123", &resID, "test-handle",
	).Return(false, errors.New("database error"))
	result, err := suite.service.CreateAction(context.Background(), "rs-123", &resID, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateActionAtResource_CheckResourceError() {
	action := providers.Action{
		Name:   "test-action",
		Handle: "test-handle",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").
		Return(providers.Resource{}, errors.New("database error"))

	resID := testResourceID
	result, err := suite.service.CreateAction(context.Background(), "rs-123", &resID, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestCreateAction_KindHandling covers kind defaulting and acceptance across resource server types:
// MCP defaults an omitted kind to "tool"; an explicit valid kind is preserved for any type; and an
// omitted kind on API/CUSTOM stays empty.
func (suite *ResourceServiceTestSuite) TestCreateAction_KindHandling() {
	cases := []struct {
		name         string
		rsID         string
		rsType       providers.ResourceServerType
		mcpCrossEnt  bool
		requestKind  providers.ActionKind
		expectedKind providers.ActionKind
	}{
		{"MCP omitted defaults to tool", "rs-mcp", providers.ResourceServerTypeMCP, true,
			"", providers.ActionKindTool},
		{"MCP explicit tool preserved", "rs-mcp", providers.ResourceServerTypeMCP, true,
			providers.ActionKindTool, providers.ActionKindTool},
		{"MCP explicit resource preserved", "rs-mcp", providers.ResourceServerTypeMCP, true,
			providers.ActionKindResource, providers.ActionKindResource},
		{"API explicit tool allowed", "rs-api", providers.ResourceServerTypeAPI, false,
			providers.ActionKindTool, providers.ActionKindTool},
		{"API omitted stays empty", "rs-api", providers.ResourceServerTypeAPI, false,
			"", ""},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			suite.mockStore.On("GetResourceServer", mock.Anything, tc.rsID).
				Return(providers.ResourceServer{Type: tc.rsType, Delimiter: ":"}, nil)
			suite.mockStore.On("CheckActionHandleExists", mock.Anything, tc.rsID, (*string)(nil), "h").
				Return(false, nil)
			if tc.mcpCrossEnt {
				suite.mockStore.On("CheckResourceHandleExists", mock.Anything, tc.rsID, "h", (*string)(nil)).
					Return(false, nil)
			}
			expectedKind := tc.expectedKind
			suite.mockStore.On("CreateAction", mock.Anything, mock.AnythingOfType("string"),
				tc.rsID, (*string)(nil), mock.MatchedBy(func(a providers.Action) bool {
					return a.Handle == "h" && a.Kind == expectedKind && a.Permission != ""
				})).Return(nil)

			result, err := suite.service.CreateAction(context.Background(), tc.rsID, nil,
				providers.Action{Name: "n", Handle: "h", Kind: tc.requestKind})

			suite.Nil(err)
			suite.NotNil(result)
			suite.Equal(tc.expectedKind, result.Kind)
			suite.mockStore.AssertExpectations(suite.T())
		})
	}
}

// TestCreateAction_InvalidKindRejected verifies a provided-but-unsupported kind is rejected with a
// 400 regardless of the resource server type.
func (suite *ResourceServiceTestSuite) TestCreateAction_InvalidKindRejected() {
	cases := []struct {
		name   string
		rsID   string
		rsType providers.ResourceServerType
	}{
		{"MCP", "rs-mcp", providers.ResourceServerTypeMCP},
		{"API", "rs-api", providers.ResourceServerTypeAPI},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			suite.mockStore.On("GetResourceServer", mock.Anything, tc.rsID).
				Return(providers.ResourceServer{Type: tc.rsType, Delimiter: ":"}, nil)

			result, err := suite.service.CreateAction(context.Background(), tc.rsID, nil,
				providers.Action{Name: "n", Handle: "h", Kind: providers.ActionKind("prompt")})

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(ErrorInvalidRequestFormat.Code, err.Code)
		})
	}
}

func (suite *ResourceServiceTestSuite) TestCreateAction_MCP_ActionHandleCollidesWithGroup() {
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-mcp").
		Return(providers.ResourceServer{Type: providers.ResourceServerTypeMCP, Delimiter: ":"}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything, "rs-mcp", (*string)(nil), "deploy").
		Return(false, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything, "rs-mcp", "deploy", (*string)(nil)).
		Return(true, nil)

	result, err := suite.service.CreateAction(context.Background(), "rs-mcp", nil,
		providers.Action{Name: "Deploy", Handle: "deploy", Kind: providers.ActionKindTool})

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorHandleConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateAction_API_NoCrossEntityCheck() {
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-api").
		Return(providers.ResourceServer{Type: providers.ResourceServerTypeAPI, Delimiter: ":"}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything, "rs-api", (*string)(nil), "deploy").
		Return(false, nil)
	suite.mockStore.On("CreateAction", mock.Anything, mock.AnythingOfType("string"),
		"rs-api", (*string)(nil), mock.MatchedBy(func(a providers.Action) bool {
			return a.Handle == "deploy" && a.Permission != ""
		})).Return(nil)

	result, err := suite.service.CreateAction(context.Background(), "rs-api", nil,
		providers.Action{Name: "Deploy", Handle: "deploy"})

	suite.Nil(err)
	suite.NotNil(result)
	suite.mockStore.AssertNotCalled(suite.T(), "CheckResourceHandleExists",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestCreateResource_MCP_GroupHandleCollidesWithAction() {
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-mcp").
		Return(providers.ResourceServer{Type: providers.ResourceServerTypeMCP, Delimiter: ":"}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything, "rs-mcp", "deploy", (*string)(nil)).
		Return(false, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything, "rs-mcp", (*string)(nil), "deploy").
		Return(true, nil)

	result, err := suite.service.CreateResource(context.Background(), "rs-mcp",
		providers.Resource{Name: "Deploy", Handle: "deploy"})

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorHandleConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResource_API_NoCrossEntityCheck() {
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-api").
		Return(providers.ResourceServer{Type: providers.ResourceServerTypeAPI, Delimiter: ":"}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything, "rs-api", "deploy", (*string)(nil)).
		Return(false, nil)
	suite.mockStore.On("CreateResource", mock.Anything, mock.AnythingOfType("string"),
		"rs-api", (*string)(nil), mock.MatchedBy(func(r providers.Resource) bool {
			return r.Handle == "deploy" && r.Permission != ""
		})).Return(nil)

	result, err := suite.service.CreateResource(context.Background(), "rs-api",
		providers.Resource{Name: "Deploy", Handle: "deploy"})

	suite.Nil(err)
	suite.NotNil(result)
	suite.mockStore.AssertNotCalled(suite.T(), "CheckActionHandleExists",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateAction_KindImmutable() {
	resID := testParentResourceID
	currentAction := providers.Action{
		ID:         "act-1",
		Name:       testOriginalName,
		Handle:     testOriginalHandle,
		Permission: "mcp:g:h",
		Kind:       providers.ActionKindResource,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-mcp").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-mcp").
		Return(providers.ResourceServer{Type: providers.ResourceServerTypeMCP, Delimiter: ":"}, nil)
	suite.mockStore.On("GetResource", mock.Anything, resID, "rs-mcp").Return(providers.Resource{}, nil)
	suite.mockStore.On("GetAction", mock.Anything, "act-1", "rs-mcp", &resID).
		Return(currentAction, nil)
	suite.mockStore.On("UpdateAction", mock.Anything, "act-1", "rs-mcp", &resID,
		mock.MatchedBy(func(a providers.Action) bool {
			return a.Kind == providers.ActionKindResource && a.Name == testUpdatedName
		})).Return(nil)

	result, err := suite.service.UpdateAction(context.Background(), "rs-mcp", &resID, "act-1",
		providers.Action{Name: testUpdatedName})

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(providers.ActionKindResource, result.Kind)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateAction_KindChangeRejected() {
	resID := testParentResourceID
	currentAction := providers.Action{
		ID:         "act-1",
		Name:       testOriginalName,
		Handle:     testOriginalHandle,
		Permission: "mcp:g:h",
		Kind:       providers.ActionKindResource,
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-mcp").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-mcp").
		Return(providers.ResourceServer{Type: providers.ResourceServerTypeMCP, Delimiter: ":"}, nil)
	suite.mockStore.On("GetResource", mock.Anything, resID, "rs-mcp").Return(providers.Resource{}, nil)
	suite.mockStore.On("GetAction", mock.Anything, "act-1", "rs-mcp", &resID).
		Return(currentAction, nil)

	result, err := suite.service.UpdateAction(context.Background(), "rs-mcp", &resID, "act-1",
		providers.Action{Name: testUpdatedName, Kind: providers.ActionKindTool})

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRequestFormat.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResourceServer_Success() {
	expectedAction := providers.Action{
		ID:   "action-123",
		Name: "test-action",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).
		Return(expectedAction, nil)

	result, err := suite.service.GetAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("action-123", result.ID)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResourceServer_MissingID() {
	result, err := suite.service.GetAction(context.Background(), "", nil, "action-123")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	result, err = suite.service.GetAction(context.Background(), "rs-123", nil, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResourceServer_NotFound() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).
		Return(providers.Action{}, errActionNotFound)

	result, err := suite.service.GetAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorActionNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionListAtResourceServer() {
	testCases := []struct {
		name             string
		resourceServerID string
		resourceID       *string
		limit            int
		offset           int
		setupMocks       func()
		expectedError    *tidcommon.ServiceError
		expectedCount    int
		validateResponse func(*ActionList)
	}{
		{
			name:             "Success_AtResourceServer",
			resourceServerID: "rs-123",
			resourceID:       nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetActionListCount", mock.Anything,
					"rs-123", (*string)(nil), providers.ActionKind("")).Return(2, nil)
				suite.mockStore.On("GetActionList", mock.Anything,
					"rs-123", (*string)(nil), providers.ActionKind(""), 30, 0).Return([]providers.Action{
					{ID: "action-1", Name: "Action 1"},
					{ID: "action-2", Name: "Action 2"},
				}, nil)
			},
			expectedError: nil,
			expectedCount: 2,
			validateResponse: func(result *ActionList) {
				suite.Equal(2, result.TotalResults)
				suite.Equal(2, len(result.Actions))
			},
		},
		{
			name:             "Error_EmptyResourceServerID",
			resourceServerID: "",
			resourceID:       nil,
			limit:            30,
			offset:           0,
			setupMocks:       func() {},
			expectedError:    &ErrorMissingID,
		},
		{
			name:             "Error_ResourceServerNotFound",
			resourceServerID: "rs-123",
			resourceID:       nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			expectedError: &ErrorResourceServerNotFound,
		},
		{
			name:             "Error_CheckResourceServerError",
			resourceServerID: "rs-123",
			resourceID:       nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_CountError",
			resourceServerID: "rs-123",
			resourceID:       nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetActionListCount", mock.Anything,
					"rs-123", (*string)(nil), providers.ActionKind("")).Return(0, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_ListError",
			resourceServerID: "rs-123",
			resourceID:       nil,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetActionListCount", mock.Anything,
					"rs-123", (*string)(nil), providers.ActionKind("")).Return(2, nil)
				suite.mockStore.On("GetActionList", mock.Anything,
					"rs-123", (*string)(nil), providers.ActionKind(""), 30, 0).Return(nil, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setupMocks()

			result, err := suite.service.GetActionList(
				context.Background(), tc.resourceServerID, tc.resourceID, "", tc.limit, tc.offset,
			)

			if tc.expectedError != nil {
				suite.Nil(result)
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
			} else {
				suite.Nil(err)
				suite.NotNil(result)
				if tc.validateResponse != nil {
					tc.validateResponse(result)
				}
			}
			suite.mockStore.AssertExpectations(suite.T())
		})
	}
}

func (suite *ResourceServiceTestSuite) TestGetResourceServerList_CountError() {
	suite.mockStore.On("GetResourceServerListCount", mock.Anything).Return(0, errors.New("database error"))

	result, err := suite.service.GetResourceServerList(context.Background(), 30, 0)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResourceServerList_ListError() {
	suite.mockStore.On("GetResourceServerListCount", mock.Anything).Return(2, nil)
	suite.mockStore.On("GetResourceServerList", mock.Anything,
		30, 0).Return(nil, errors.New("database error"))

	result, err := suite.service.GetResourceServerList(context.Background(), 30, 0)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResourceServer_ResourceServerNotFound() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := suite.service.GetAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResourceServer_StoreError() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).
		Return(providers.Action{}, errors.New("database error"))

	result, err := suite.service.GetAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateAction() {
	testCases := []struct {
		name             string
		resourceServerID string
		resourceID       *string
		actionID         string
		action           providers.Action
		setupMocks       func()
		expectedError    *tidcommon.ServiceError
		validateResponse func(*providers.Action)
	}{
		{
			name:             "Success_AtResourceServer",
			resourceServerID: "rs-123",
			resourceID:       nil,
			actionID:         "action-123",
			action: providers.Action{
				Name:        testUpdatedName,
				Description: testNewDescription,
			},
			setupMocks: func() {
				currentAction := providers.Action{
					ID:          "action-123",
					Name:        testOriginalName,
					Handle:      testOriginalHandle,
					Description: "old description",
				}
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetAction", mock.Anything,
					"action-123", "rs-123", (*string)(nil)).
					Return(currentAction, nil)
				suite.mockStore.On(
					"UpdateAction", mock.Anything,
					"action-123", "rs-123", (*string)(nil),
					mock.MatchedBy(func(a providers.Action) bool {
						return a.Name == testUpdatedName &&
							a.Handle == testOriginalHandle &&
							a.Description == testNewDescription
					})).Return(nil)
			},
			expectedError: nil,
			validateResponse: func(result *providers.Action) {
				suite.Equal(testUpdatedName, result.Name)
				suite.Equal(testOriginalHandle, result.Handle) // Handle is immutable
				suite.Equal(testNewDescription, result.Description)
			},
		},
		{
			name:             "Success_AtResource",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			actionID:         "action-123",
			action: providers.Action{
				Name:        testUpdatedName,
				Description: testNewDescription,
			},
			setupMocks: func() {
				currentAction := providers.Action{
					ID:          "action-123",
					Name:        testOriginalName,
					Handle:      testOriginalHandle,
					Description: "old description",
				}
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, nil)
				resIDLocal := testParentResourceID
				suite.mockStore.On("GetAction", mock.Anything,
					"action-123", "rs-123", &resIDLocal).
					Return(currentAction, nil)
				suite.mockStore.On("UpdateAction", mock.Anything,
					"action-123", "rs-123", &resIDLocal,
					mock.MatchedBy(func(a providers.Action) bool {
						return a.Name == testUpdatedName &&
							a.Handle == testOriginalHandle &&
							a.Description == testNewDescription
					})).Return(nil)
			},
			expectedError: nil,
			validateResponse: func(result *providers.Action) {
				suite.Equal(testUpdatedName, result.Name)
				suite.Equal(testOriginalHandle, result.Handle) // Handle is immutable
				suite.Equal(testNewDescription, result.Description)
			},
		},
		{
			name:             "Error_EmptyResourceServerID",
			resourceServerID: "",
			resourceID:       nil,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks:       func() {},
			expectedError:    &ErrorMissingID,
		},
		{
			name:             "Error_EmptyActionID",
			resourceServerID: "rs-123",
			resourceID:       nil,
			actionID:         "",
			action:           providers.Action{Description: testNewDescription},
			setupMocks:       func() {},
			expectedError:    &ErrorMissingID,
		},
		{
			name:             "Error_EmptyResourceID",
			resourceServerID: "rs-123",
			resourceID:       &testEmptyResourceID,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks:       func() {},
			expectedError:    &ErrorMissingID,
		},
		{
			name:             "Error_ResourceServerNotFound",
			resourceServerID: "rs-123",
			resourceID:       nil,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks: func() {
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			expectedError: &ErrorResourceServerNotFound,
		},
		{
			name:             "Error_ResourceNotFound",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks: func() {
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, errResourceNotFound)
			},
			expectedError: &ErrorResourceNotFound,
		},
		{
			name:             "Error_ActionNotFound_AtResourceServer",
			resourceServerID: "rs-123",
			resourceID:       nil,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks: func() {
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetAction", mock.Anything,
					"action-123", "rs-123", (*string)(nil)).Return(providers.Action{}, errActionNotFound)
			},
			expectedError: &ErrorActionNotFound,
		},
		{
			name:             "Error_ActionNotFound_AtResource",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks: func() {
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, nil)
				resID := testParentResourceID
				suite.mockStore.On("GetAction", mock.Anything,
					"action-123", "rs-123", &resID).
					Return(providers.Action{}, errActionNotFound)
			},
			expectedError: &ErrorActionNotFound,
		},
		{
			name:             "Error_CheckResourceServerError",
			resourceServerID: "rs-123",
			resourceID:       nil,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks: func() {
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_CheckResourceError",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks: func() {
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_GetActionError",
			resourceServerID: "rs-123",
			resourceID:       nil,
			actionID:         "action-123",
			action:           providers.Action{Description: testNewDescription},
			setupMocks: func() {
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetAction", mock.Anything,
					"action-123", "rs-123", (*string)(nil)).
					Return(providers.Action{}, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_UpdateError",
			resourceServerID: "rs-123",
			resourceID:       nil,
			actionID:         "action-123",
			action: providers.Action{
				Name:        testUpdatedName,
				Description: testNewDescription,
			},
			setupMocks: func() {
				currentAction := providers.Action{
					ID:          "action-123",
					Name:        testOriginalName,
					Handle:      testOriginalHandle,
					Description: "old description",
				}
				suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetAction", mock.Anything,
					"action-123", "rs-123", (*string)(nil)).Return(currentAction, nil)
				suite.mockStore.On("UpdateAction", mock.Anything,
					"action-123", "rs-123", (*string)(nil), mock.Anything).
					Return(errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setupMocks()

			result, err := suite.service.UpdateAction(
				context.Background(), tc.resourceServerID, tc.resourceID, tc.actionID, tc.action,
			)

			if tc.expectedError != nil {
				suite.Nil(result)
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
			} else {
				suite.Nil(err)
				suite.NotNil(result)
				if tc.validateResponse != nil {
					tc.validateResponse(result)
				}
			}
			suite.mockStore.AssertExpectations(suite.T())
		})
	}
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResourceServer_Success() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).Return(true, nil)
	suite.mockStore.On("DeleteAction", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).Return(nil)

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(err)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResourceServer_MissingID() {
	err := suite.service.DeleteAction(context.Background(), "", nil, "action-123")
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	err = suite.service.DeleteAction(context.Background(), "rs-123", nil, "")
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResourceServer_ResourceServerNotFound() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(err) // Idempotent delete
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResourceServer_StoreError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).Return(true, nil)
	suite.mockStore.On("DeleteAction", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).Return(errors.New("database error"))

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResourceServer_CheckServerError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errors.New("database error"))

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResourceServer_ActionNotFound() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).Return(false, nil)

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(err) // Idempotent delete
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResourceServer_CheckActionExistError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).Return(false, errors.New("database error"))

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_Success() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", &resID).Return(true, nil)
	suite.mockStore.On("DeleteAction", mock.Anything,
		"action-123", "rs-123", &resID).Return(nil)
	err := suite.service.DeleteAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(err)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_MissingID() {
	resID := testResourceID
	err := suite.service.DeleteAction(context.Background(), "", &resID, "action-123")
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	emptyResID := ""
	err = suite.service.DeleteAction(context.Background(), "rs-123", &emptyResID, "action-123")
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	err = suite.service.DeleteAction(context.Background(), "rs-123", &resID, "")
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_ResourceServerNotFound() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	resID := testResourceID
	err := suite.service.DeleteAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(err) // Idempotent delete
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_ResourceNotFound() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, errResourceNotFound)

	resID := testResourceID
	err := suite.service.DeleteAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(err) // Idempotent delete
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_StoreError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", &resID).Return(true, nil)
	suite.mockStore.On("DeleteAction", mock.Anything,
		"action-123", "rs-123", &resID).Return(errors.New("database error"))
	err := suite.service.DeleteAction(context.Background(), "rs-123", &resID, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_CheckServerError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").
		Return(providers.ResourceServer{}, errors.New("database error"))

	resID := testResourceID
	err := suite.service.DeleteAction(context.Background(), "rs-123", &resID, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_CheckResourceError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").
		Return(providers.Resource{}, errors.New("database error"))

	resID := testResourceID
	err := suite.service.DeleteAction(context.Background(), "rs-123", &resID, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_ActionNotFound() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", &resID).Return(false, nil)
	err := suite.service.DeleteAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(err) // Idempotent delete
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_CheckActionExistError() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", &resID).Return(false, errors.New("database error"))
	err := suite.service.DeleteAction(context.Background(), "rs-123", &resID, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// GetActionAtResource Tests

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_Success() {
	expectedAction := providers.Action{
		ID:   "action-123",
		Name: "test-action",
	}

	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", &resID).
		Return(expectedAction, nil)
	result, err := suite.service.GetAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("action-123", result.ID)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_MissingID() {
	resID := testResourceID
	result, err := suite.service.GetAction(context.Background(), "", &resID, "action-123")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	emptyResID := ""
	result, err = suite.service.GetAction(context.Background(), "rs-123", &emptyResID, "action-123")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)

	result, err = suite.service.GetAction(context.Background(), "rs-123", &resID, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingID.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_ResourceServerNotFound() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, errResourceServerNotFound)

	resID := testResourceID
	result, err := suite.service.GetAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_ResourceNotFound() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, errResourceNotFound)

	resID := testResourceID
	result, err := suite.service.GetAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_ActionNotFound() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", &resID).
		Return(providers.Action{}, errActionNotFound)
	result, err := suite.service.GetAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorActionNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_StoreError() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", &resID).
		Return(providers.Action{}, errors.New("database error"))
	result, err := suite.service.GetAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_CheckServerError() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").
		Return(providers.ResourceServer{}, errors.New("database error"))

	resID := testResourceID
	result, err := suite.service.GetAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_CheckResourceError() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").
		Return(providers.Resource{}, errors.New("database error"))

	resID := testResourceID
	result, err := suite.service.GetAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// Composite Foreign Key Validation Tests - Cross-Resource Action Access

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_WrongResourceID() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	wrongResID := testWrongResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testWrongResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", &wrongResID).
		Return(providers.Action{}, errActionNotFound)
	result, err := suite.service.GetAction(context.Background(), "rs-123", &wrongResID, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorActionNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateActionAtResource_WrongResourceID() {
	updateReq := providers.Action{
		Name:        "updated-action",
		Handle:      "original-handle",
		Description: "updated description",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	wrongResID := testWrongResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testWrongResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", &wrongResID).
		Return(providers.Action{}, errActionNotFound)
	result, err := suite.service.UpdateAction(context.Background(), "rs-123", &wrongResID, "action-123", updateReq)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorActionNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteActionAtResource_WrongResourceID() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	wrongResID := testWrongResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testWrongResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything,
		"action-123", "rs-123", &wrongResID).Return(false, nil)
	err := suite.service.DeleteAction(context.Background(), "rs-123", &wrongResID, "action-123")

	suite.Nil(err) // Idempotent delete
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResourceServer_WhenActionBelongsToResource() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", (*string)(nil)).
		Return(providers.Action{}, errActionNotFound)

	result, err := suite.service.GetAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorActionNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestGetActionAtResource_WhenActionBelongsToServer() {
	suite.mockStore.On("GetResourceServer", mock.Anything,
		"rs-123").Return(providers.ResourceServer{}, nil)
	resID := testResourceID
	suite.mockStore.On("GetResource", mock.Anything,
		testResourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("GetAction", mock.Anything,
		"action-123", "rs-123", &resID).
		Return(providers.Action{}, errActionNotFound)
	result, err := suite.service.GetAction(context.Background(), "rs-123", &resID, "action-123")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorActionNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// GetActionListAtResource Tests

func (suite *ResourceServiceTestSuite) TestGetActionListAtResource() {
	testCases := []struct {
		name             string
		resourceServerID string
		resourceID       *string
		limit            int
		offset           int
		setupMocks       func()
		expectedError    *tidcommon.ServiceError
		expectedCount    int
		validateResponse func(*ActionList)
	}{
		{
			name:             "Success_AtResource",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").Return(providers.Resource{}, nil)
				resID := testParentResourceID
				suite.mockStore.On("GetActionListCount", mock.Anything,
					"rs-123", &resID, providers.ActionKind("")).Return(2, nil)
				suite.mockStore.On("GetActionList", mock.Anything,
					"rs-123", &resID, providers.ActionKind(""), 30, 0).Return([]providers.Action{
					{ID: "action-1", Name: "Action 1"},
					{ID: "action-2", Name: "Action 2"},
				}, nil)
			},
			expectedError: nil,
			expectedCount: 2,
			validateResponse: func(result *ActionList) {
				suite.Equal(2, result.TotalResults)
				suite.Equal(2, len(result.Actions))
			},
		},
		{
			name:             "Success_EmptyList",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").Return(providers.Resource{}, nil)
				resID := testParentResourceID
				suite.mockStore.On("GetActionListCount", mock.Anything,
					"rs-123", &resID, providers.ActionKind("")).Return(0, nil)
				suite.mockStore.On("GetActionList", mock.Anything,
					"rs-123", &resID, providers.ActionKind(""), 30, 0).Return([]providers.Action{}, nil)
			},
			expectedError: nil,
			validateResponse: func(result *ActionList) {
				suite.Equal(0, result.TotalResults)
				suite.Equal(0, len(result.Actions))
			},
		},
		{
			name:             "Error_EmptyResourceID",
			resourceServerID: "rs-123",
			resourceID:       &testEmptyResourceID,
			limit:            30,
			offset:           0,
			setupMocks:       func() {},
			expectedError:    &ErrorMissingID,
		},
		{
			name:             "Error_ResourceServerNotFound",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			expectedError: &ErrorResourceServerNotFound,
		},
		{
			name:             "Error_ResourceNotFound",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, errResourceNotFound)
			},
			expectedError: &ErrorResourceNotFound,
		},
		{
			name:             "Error_CheckResourceServerError",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_CheckResourceError",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_CountError",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, nil)
				resID := testParentResourceID
				suite.mockStore.On("GetActionListCount", mock.Anything,
					"rs-123", &resID, providers.ActionKind("")).
					Return(0, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
		{
			name:             "Error_ListError",
			resourceServerID: "rs-123",
			resourceID:       &testParentResourceID,
			limit:            30,
			offset:           0,
			setupMocks: func() {
				suite.mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, nil)
				suite.mockStore.On("GetResource", mock.Anything,
					testParentResourceID, "rs-123").
					Return(providers.Resource{}, nil)
				resID := testParentResourceID
				suite.mockStore.On("GetActionListCount", mock.Anything,
					"rs-123", &resID, providers.ActionKind("")).Return(2, nil)
				suite.mockStore.On("GetActionList", mock.Anything,
					"rs-123", &resID, providers.ActionKind(""), 30, 0).
					Return(nil, errors.New("database error"))
			},
			expectedError: &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setupMocks()

			result, err := suite.service.GetActionList(
				context.Background(), tc.resourceServerID, tc.resourceID, "", tc.limit, tc.offset,
			)

			if tc.expectedError != nil {
				suite.Nil(result)
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
			} else {
				suite.Nil(err)
				suite.NotNil(result)
				if tc.validateResponse != nil {
					tc.validateResponse(result)
				}
			}
			suite.mockStore.AssertExpectations(suite.T())
		})
	}
}

// Validation Helper Tests

func (suite *ResourceServiceTestSuite) TestValidatePaginationParams() {
	// Valid params
	err := validatePaginationParams(30, 0)
	suite.Nil(err)

	// Invalid limit - too small
	err = validatePaginationParams(0, 0)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidLimit.Code, err.Code)

	// Invalid limit - too large (assuming MaxPageSize is defined)
	err = validatePaginationParams(10000, 0)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidLimit.Code, err.Code)

	// Invalid offset
	err = validatePaginationParams(30, -1)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidOffset.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestBuildPaginationLinks() {
	testCases := []struct {
		name          string
		base          string
		limit         int
		offset        int
		totalCount    int
		expectedLinks []Link
		description   string
	}{
		{
			name:        "FirstPage_HasMorePages",
			base:        "/test",
			limit:       10,
			offset:      0,
			totalCount:  25,
			description: "First page (offset=0) with more pages available",
			expectedLinks: []Link{
				{Href: "/test?offset=10&limit=10", Rel: "next"},
				{Href: "/test?offset=20&limit=10", Rel: "last"},
			},
		},
		{
			name:        "MiddlePage_WithPrevOffset_Negative",
			base:        "/test",
			limit:       10,
			offset:      5,
			totalCount:  30,
			description: "Middle page where prevOffset calculation goes negative (line 924-926)",
			expectedLinks: []Link{
				{Href: "/test?offset=0&limit=10", Rel: "first"},
				{Href: "/test?offset=0&limit=10", Rel: "prev"}, // prevOffset = 5-10 = -5, becomes 0
				{Href: "/test?offset=15&limit=10", Rel: "next"},
				{Href: "/test?offset=20&limit=10", Rel: "last"},
			},
		},
		{
			name:        "MiddlePage_NormalPrevOffset",
			base:        "/test",
			limit:       10,
			offset:      20,
			totalCount:  50,
			description: "Middle page with normal prevOffset calculation",
			expectedLinks: []Link{
				{Href: "/test?offset=0&limit=10", Rel: "first"},
				{Href: "/test?offset=10&limit=10", Rel: "prev"}, // prevOffset = 20-10 = 10
				{Href: "/test?offset=30&limit=10", Rel: "next"},
				{Href: "/test?offset=40&limit=10", Rel: "last"},
			},
		},
		{
			name:        "LastPage_NoNext",
			base:        "/test",
			limit:       10,
			offset:      20,
			totalCount:  25,
			description: "Last page (offset+limit >= totalCount), no next link",
			expectedLinks: []Link{
				{Href: "/test?offset=0&limit=10", Rel: "first"},
				{Href: "/test?offset=10&limit=10", Rel: "prev"},
				// No next link because offset(20) + limit(10) >= totalCount(25)
				// No last link because offset(20) >= lastPageOffset(20)
			},
		},
		{
			name:          "SinglePage_NoLinks",
			base:          "/test",
			limit:         10,
			offset:        0,
			totalCount:    5,
			description:   "Single page (totalCount <= limit), no pagination links",
			expectedLinks: []Link{
				// No links at all
			},
		},
		{
			name:          "ExactlyOnePage_OnLastPage",
			base:          "/test",
			limit:         10,
			offset:        0,
			totalCount:    10,
			description:   "Exactly one page of results, on last page (offset+limit == totalCount)",
			expectedLinks: []Link{
				// No next because offset(0) + limit(10) >= totalCount(10)
				// No last because offset(0) >= lastPageOffset(0)
			},
		},
		{
			name:        "LastPageOffset_EqualToOffset",
			base:        "/test",
			limit:       10,
			offset:      30,
			totalCount:  35,
			description: "On last page where offset equals lastPageOffset (line 942)",
			expectedLinks: []Link{
				{Href: "/test?offset=0&limit=10", Rel: "first"},
				{Href: "/test?offset=20&limit=10", Rel: "prev"},
				// No next because offset(30) + limit(10) > totalCount(35)
				// No last because offset(30) >= lastPageOffset(30)
			},
		},
		{
			name:        "SecondToLastPage_HasLastLink",
			base:        "/test",
			limit:       10,
			offset:      20,
			totalCount:  35,
			description: "Second to last page, has last link",
			expectedLinks: []Link{
				{Href: "/test?offset=0&limit=10", Rel: "first"},
				{Href: "/test?offset=10&limit=10", Rel: "prev"},
				{Href: "/test?offset=30&limit=10", Rel: "next"},
				{Href: "/test?offset=30&limit=10", Rel: "last"},
			},
		},
		{
			name:        "ExactlyAtBoundary_OffsetPlusLimitEqualsTotalCount",
			base:        "/test",
			limit:       10,
			offset:      10,
			totalCount:  20,
			description: "Exactly at boundary where offset+limit == totalCount (line 933)",
			expectedLinks: []Link{
				{Href: "/test?offset=0&limit=10", Rel: "first"},
				{Href: "/test?offset=0&limit=10", Rel: "prev"},
				// No next because offset(10) + limit(10) == totalCount(20), not < totalCount
				// No last because offset(10) >= lastPageOffset(10)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			links := buildPaginationLinks(tc.base, tc.limit, tc.offset, tc.totalCount)

			suite.Equal(len(tc.expectedLinks), len(links),
				"Expected %d links but got %d for: %s", len(tc.expectedLinks), len(links), tc.description)

			for i, expectedLink := range tc.expectedLinks {
				suite.Equal(expectedLink.Href, links[i].Href,
					"Link %d: Expected href %s but got %s", i, expectedLink.Href, links[i].Href)
				suite.Equal(expectedLink.Rel, links[i].Rel,
					"Link %d: Expected rel %s but got %s", i, expectedLink.Rel, links[i].Rel)
			}
		})
	}
}

func (suite *ResourceServiceTestSuite) TestListMethods_PaginationValidationErrors() {
	paginationTestCases := []struct {
		name          string
		limit         int
		offset        int
		expectedError tidcommon.ServiceError
	}{
		{
			name:          "Error_InvalidLimit_Zero",
			limit:         0,
			offset:        0,
			expectedError: ErrorInvalidLimit,
		},
		{
			name:          "Error_InvalidLimit_Negative",
			limit:         -1,
			offset:        0,
			expectedError: ErrorInvalidLimit,
		},
		{
			name:          "Error_InvalidLimit_ExceedsMax",
			limit:         101,
			offset:        0,
			expectedError: ErrorInvalidLimit,
		},
		{
			name:          "Error_InvalidOffset_Negative",
			limit:         30,
			offset:        -1,
			expectedError: ErrorInvalidOffset,
		},
	}

	// Test GetResourceServerList
	for _, tc := range paginationTestCases {
		suite.Run("GetResourceServerList_"+tc.name, func() {
			suite.SetupTest()

			result, err := suite.service.GetResourceServerList(context.Background(), tc.limit, tc.offset)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError.Code, err.Code)
			suite.mockStore.AssertExpectations(suite.T())
		})
	}

	// Test GetResourceList
	for _, tc := range paginationTestCases {
		suite.Run("GetResourceList_"+tc.name, func() {
			suite.SetupTest()

			result, err := suite.service.GetResourceList(context.Background(), "rs-123", nil, tc.limit, tc.offset)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError.Code, err.Code)
			suite.mockStore.AssertExpectations(suite.T())
		})
	}

	// Test GetActionList
	for _, tc := range paginationTestCases {
		suite.Run("GetActionList_"+tc.name, func() {
			suite.SetupTest()

			result, err := suite.service.GetActionList(context.Background(), "rs-123", nil, "", tc.limit, tc.offset)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError.Code, err.Code)
			suite.mockStore.AssertExpectations(suite.T())
		})
	}
}

// Delimiter Validation Tests

func (suite *ResourceServiceTestSuite) TestDelimiterValidation() {
	testCases := []struct {
		name        string
		delimiter   string
		expectError bool
	}{
		// Valid delimiters (._:-/)
		{"ValidSlash", "/", false},
		{"ValidColon", ":", false},
		{"ValidPeriod", ".", false},
		{"ValidDash", "-", false},
		{"ValidUnderscore", "_", false},
		// Invalid delimiters
		{"EmptyString", "", true},
		{"Space", " ", true},
		{"MultiChar", "::", true},
		{"NullChar", "\x00", true},
		{"DoubleQuote", "\"", true},
		{"Backslash", "\\", true},
		{"Tab", "\t", true},
		{"Newline", "\n", true},
		{"Hash", "#", true},
		{"Pipe", "|", true},
		{"Exclamation", "!", true},
		{"At", "@", true},
		{"Dollar", "$", true},
		{"Percent", "%", true},
		{"Ampersand", "&", true},
		{"Asterisk", "*", true},
		{"Plus", "+", true},
		{"Equals", "=", true},
		{"AlphaChar", "a", true},
		{"NumericChar", "1", true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			err := validateDelimiter(tc.delimiter)
			if tc.expectError {
				suite.NotNil(err, "delimiter '%s' should be invalid", tc.delimiter)
				suite.Equal(ErrorInvalidDelimiter.Code, err.Code)
			} else {
				suite.Nil(err, "delimiter %s should be valid", tc.delimiter)
			}
		})
	}
}

// Handle Validation Tests

func (suite *ResourceServiceTestSuite) TestHandleValidation() {
	testCases := []struct {
		name        string
		handle      string
		delimiter   string
		expectError bool
		errorCode   string
	}{
		// Valid handles
		{"SimpleHandle", "users", "/", false, ""},
		{"WithNumbers", "resource123", ":", false, ""},
		{"WithUnderscores", "my_resource", ".", false, ""},
		{"MixedCase", "MyResource", "|", false, ""},
		{"SingleChar", "u", "/", false, ""},
		{"WithDash", "my-resource", ".", false, ""},
		{"AllNumbers", "123", "/", false, ""},
		// Contains delimiter
		{"ContainsSlash", "users/read", "/", true, ErrorDelimiterInHandle.Code},
		{"ContainsColon", "resource:list", ":", true, ErrorDelimiterInHandle.Code},
		{"ContainsDot", "app.module", ".", true, ErrorDelimiterInHandle.Code},
		// Invalid characters
		{"WithSpace", "my resource", "/", true, ErrorInvalidHandle.Code},
		{"WithQuote", "resource\"name", "/", true, ErrorInvalidHandle.Code},
		{"WithBackslash", "resource\\name", "/", true, ErrorInvalidHandle.Code},
		{"WithTab", "resource\t", "/", true, ErrorInvalidHandle.Code},
		{"WithNewline", "resource\n", "/", true, ErrorInvalidHandle.Code},
		{"OnlyTab", "\t", "/", true, ErrorInvalidHandle.Code},
		{"OnlyNewline", "\n", "/", true, ErrorInvalidHandle.Code},
		// Invalid length
		{"TooLongHandle", string(make([]rune, 101)), "/", true, ErrorInvalidHandle.Code},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			err := validateHandle(tc.handle, tc.delimiter)
			if tc.expectError {
				suite.NotNil(err, "handle '%s' should fail validation", tc.handle)
				suite.Equal(tc.errorCode, err.Code)
			} else {
				suite.Nil(err, "handle %s should be valid", tc.handle)
			}
		})
	}
}

// Permission Derivation Tests

func (suite *ResourceServiceTestSuite) TestDerivePermission() {
	testCases := []struct {
		name               string
		resourceServer     providers.ResourceServer
		parent             *providers.Resource
		handle             string
		expectedPermission string
	}{
		{
			name:               "TopLevelResourceNoHandle",
			resourceServer:     providers.ResourceServer{Delimiter: ":"},
			parent:             nil,
			handle:             "users",
			expectedPermission: "users",
		},
		{
			name:               "ChildResourceWithColon",
			resourceServer:     providers.ResourceServer{Delimiter: ":"},
			parent:             &providers.Resource{Permission: "users"},
			handle:             "read",
			expectedPermission: "users:read",
		},
		{
			name:               "DeeplyNestedWithSlash",
			resourceServer:     providers.ResourceServer{Delimiter: "/"},
			parent:             &providers.Resource{Permission: "api/v1/users"},
			handle:             "read",
			expectedPermission: "api/v1/users/read",
		},
		{
			name:               "DotDelimiter",
			resourceServer:     providers.ResourceServer{Delimiter: "."},
			parent:             &providers.Resource{Permission: "admin.panel"},
			handle:             "delete",
			expectedPermission: "admin.panel.delete",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			permission := derivePermission(tc.resourceServer, tc.parent, tc.handle)
			suite.Equal(tc.expectedPermission, permission)
		})
	}
}

// Permission Character Validation Tests

func (suite *ResourceServiceTestSuite) TestPermissionCharacterValidation() {
	// Valid characters: a-zA-Z0-9._:-/
	validChars := []rune{
		'a', 'b', 'z', 'A', 'B', 'Z', // Letters
		'0', '1', '5', '9', // Numbers
		'.', '_', ':', '-', '/', // Special allowed characters
	}

	for _, c := range validChars {
		suite.True(isValidPermissionCharacter(c), "character %q (0x%02X) should be valid", c, c)
	}

	// Invalid characters: everything not in a-zA-Z0-9._:-/
	invalidChars := []rune{
		' ', '"', '\\', '\x00', '\x1F', '\x7F', // space, quote, backslash, control chars
		'!', '@', '#', '$', '%', '^', '&', '*', '(', ')', '+', '=', // special chars
		'[', ']', '{', '}', '<', '>', '|', '~', '`', '\'', // brackets and other
		';', ',', '?', // punctuation
	}

	for _, c := range invalidChars {
		suite.False(isValidPermissionCharacter(c), "character %q (0x%02X) should be invalid", c, c)
	}
}

// Pagination Validation Tests

func (suite *ResourceServiceTestSuite) TestPaginationValidation() {
	testCases := []struct {
		name         string
		limit        int
		offset       int
		expectError  bool
		expectedCode string
	}{
		// Valid params
		{"DefaultPagination", 10, 0, false, ""},
		{"SecondPage", 10, 10, false, ""},
		{"MaxLimit", 100, 0, false, ""},
		{"MaxLimitWithOffset", 100, 90, false, ""},
		// Invalid params
		{"NegativeLimit", -1, 0, true, "RES-1011"},
		{"NegativeOffset", 10, -1, true, "RES-1012"},
		{"ZeroLimit", 0, 0, true, "RES-1011"},
		{"LimitExceedsMax", 10001, 0, true, "RES-1011"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			err := validatePaginationParams(tc.limit, tc.offset)
			if tc.expectError {
				suite.NotNil(err)
				suite.Equal(tc.expectedCode, err.Code)
			} else {
				suite.Nil(err)
			}
		})
	}
}

// Integration Tests: Delimiter + Permission Hierarchy

func (suite *ResourceServiceTestSuite) TestPermissionHierarchyIntegration() {
	testCases := []struct {
		name           string
		delimiter      string
		expectedLevel1 string
		expectedLevel2 string
		expectedLevel3 string
	}{
		{
			name:           "SlashHierarchy",
			delimiter:      "/",
			expectedLevel1: "resource",
			expectedLevel2: "resource/get",
			expectedLevel3: "resource/get/admin",
		},
		{
			name:           "ColonHierarchy",
			delimiter:      ":",
			expectedLevel1: "scope",
			expectedLevel2: "scope:read",
			expectedLevel3: "scope:read:profile",
		},
		{
			name:           "DotHierarchy",
			delimiter:      ".",
			expectedLevel1: "admin",
			expectedLevel2: "admin.users",
			expectedLevel3: "admin.users.delete",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			rs := providers.ResourceServer{Identifier: "test", Delimiter: tc.delimiter}

			// Level 1
			perm1 := derivePermission(rs, nil, tc.expectedLevel1)
			suite.Equal(tc.expectedLevel1, perm1)

			// Level 2
			perm2 := derivePermission(rs, &providers.Resource{Permission: perm1},
				tc.expectedLevel2[len(perm1)+1:]) // Extract handle after delimiter
			suite.Equal(tc.expectedLevel2, perm2)

			// Level 3
			perm3 := derivePermission(rs, &providers.Resource{Permission: perm2},
				tc.expectedLevel3[len(perm2)+1:]) // Extract handle after delimiter
			suite.Equal(tc.expectedLevel3, perm3)
		})
	}
}

// ValidatePermissions Tests

func (suite *ResourceServiceTestSuite) TestValidatePermissions() {
	// Initialize runtime config once for all sub-tests
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: engineconfig.ServerConfig{
			Identifier: "test-deployment",
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test-validate-permissions", testConfig)
	suite.Require().NoError(err)
	defer config.ResetServerRuntime()

	testCases := []struct {
		name             string
		resourceServerID string
		permissions      []string
		setupMocks       func(*resourceStoreInterfaceMock)
		expectedInvalid  []string
		expectedError    *tidcommon.ServiceError
	}{
		{
			name:             "Success_EmptyPermissions",
			resourceServerID: "rs-123",
			permissions:      []string{},
			setupMocks:       func(mockStore *resourceStoreInterfaceMock) {},
			expectedInvalid:  []string{},
			expectedError:    nil,
		},
		{
			name:             "Success_AllPermissionsValid",
			resourceServerID: "rs-123",
			permissions:      []string{"read", "write", "delete"},
			setupMocks: func(mockStore *resourceStoreInterfaceMock) {
				mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{ID: "rs-123"}, nil)
				mockStore.On("ValidatePermissions", mock.Anything,
					"rs-123", []string{"read", "write", "delete"}).
					Return([]string{}, nil)
			},
			expectedInvalid: []string{},
			expectedError:   nil,
		},
		{
			name:             "Success_SomePermissionsInvalid",
			resourceServerID: "rs-123",
			permissions:      []string{"read", "write", "invalid1", "delete", "invalid2"},
			setupMocks: func(mockStore *resourceStoreInterfaceMock) {
				mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{ID: "rs-123"}, nil)
				mockStore.On("ValidatePermissions", mock.Anything,
					"rs-123", []string{"read", "write", "invalid1", "delete", "invalid2"}).
					Return([]string{"invalid1", "invalid2"}, nil)
			},
			expectedInvalid: []string{"invalid1", "invalid2"},
			expectedError:   nil,
		},
		{
			name:             "Success_AllPermissionsInvalid",
			resourceServerID: "rs-123",
			permissions:      []string{"badperm1", "badperm2", "badperm3"},
			setupMocks: func(mockStore *resourceStoreInterfaceMock) {
				mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{ID: "rs-123"}, nil)
				mockStore.On("ValidatePermissions", mock.Anything,
					"rs-123", []string{"badperm1", "badperm2", "badperm3"}).
					Return([]string{"badperm1", "badperm2", "badperm3"}, nil)
			},
			expectedInvalid: []string{"badperm1", "badperm2", "badperm3"},
			expectedError:   nil,
		},
		{
			name:             "Success_ResourceServerNotFound_ReturnsAllPermissionsInvalid",
			resourceServerID: "rs-nonexistent",
			permissions:      []string{"read", "write"},
			setupMocks: func(mockStore *resourceStoreInterfaceMock) {
				mockStore.On("GetResourceServer", mock.Anything,
					"rs-nonexistent").
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			expectedInvalid: []string{"read", "write"},
			expectedError:   nil,
		},
		{
			name:             "Error_GetResourceServerStoreError",
			resourceServerID: "rs-123",
			permissions:      []string{"read", "write"},
			setupMocks: func(mockStore *resourceStoreInterfaceMock) {
				mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{}, errors.New("database connection failed"))
			},
			expectedInvalid: nil,
			expectedError:   &tidcommon.InternalServerError,
		},
		{
			name:             "Error_ValidatePermissionsStoreError",
			resourceServerID: "rs-123",
			permissions:      []string{"read", "write"},
			setupMocks: func(mockStore *resourceStoreInterfaceMock) {
				mockStore.On("GetResourceServer", mock.Anything,
					"rs-123").
					Return(providers.ResourceServer{ID: "rs-123"}, nil)
				mockStore.On("ValidatePermissions", mock.Anything,
					"rs-123", []string{"read", "write"}).
					Return(nil, errors.New("database error"))
			},
			expectedInvalid: nil,
			expectedError:   &tidcommon.InternalServerError,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for this specific sub-test
			mockStore := newResourceStoreInterfaceMock(suite.T())
			mockOU := new(oumock.OrganizationUnitServiceInterfaceMock)

			// Create a fresh service instance with the fresh mocks
			mockTransactioner := &fakeTransactioner{}
			svc, err := newResourceService(
				mockOU, newDisabledConsentServiceMock(suite.T()), mockStore, mockTransactioner,
			)
			suite.Require().NoError(err)

			// Setup mocks for this test case
			tc.setupMocks(mockStore)

			// Execute the test
			invalidPerms, svcErr := svc.ValidatePermissions(context.Background(), tc.resourceServerID, tc.permissions)

			// Assert results
			if tc.expectedError != nil {
				suite.NotNil(svcErr)
				suite.Equal(tc.expectedError.Code, svcErr.Code)
				suite.Nil(invalidPerms)
			} else {
				suite.Nil(svcErr)
				suite.Equal(tc.expectedInvalid, invalidPerms)
			}

			// Verify all expected mock calls were made
			mockStore.AssertExpectations(suite.T())
		})
	}
}

// Test cases for declarative resource functionality

func (suite *ResourceServiceTestSuite) TestIsResourceServerDeclarative_True() {
	// Test when resource server is declarative
	suite.mockStore.On("IsResourceServerDeclarative", declarativeRSID).Return(true)

	result := suite.service.IsResourceServerDeclarative(declarativeRSID)

	suite.True(result)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestIsResourceServerDeclarative_False() {
	// Test when resource server is not declarative
	suite.mockStore.On("IsResourceServerDeclarative", "mutable-rs").Return(false)

	result := suite.service.IsResourceServerDeclarative("mutable-rs")

	suite.False(result)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_ImmutableDeclarativeResource() {
	// Test that updating a declarative resource server returns an immutability error
	resourceServerID := declarativeRSID
	updateReq := providers.ResourceServer{
		ID:         resourceServerID,
		Name:       "Updated Name",
		Identifier: "test-identifier",
		OUID:       "ou-1",
	}

	// Mock IsResourceServerDeclarative to return true
	suite.mockStore.On("GetResourceServer", mock.Anything, resourceServerID).
		Return(providers.ResourceServer{ID: resourceServerID}, nil)
	suite.mockStore.On("IsResourceServerDeclarative", resourceServerID).Return(true)

	// Execute the test
	result, svcErr := suite.service.UpdateResourceServer(context.Background(), resourceServerID, updateReq)

	// Assert immutability error
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal("RES-1018", svcErr.Code)
	suite.Equal(ErrorImmutableResourceServer.Code, svcErr.Code)

	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_MutableResource() {
	resourceServerID := "mutable-rs"
	updateReq := providers.ResourceServer{
		ID:         resourceServerID,
		Name:       "Updated Name",
		OUID:       "ou-1",
		Identifier: "original-identifier",
	}

	existingRS := providers.ResourceServer{
		ID:         resourceServerID,
		Name:       "Original Name",
		OUID:       "ou-1",
		Identifier: "original-identifier",
		Delimiter:  ":",
	}

	suite.mockStore.On("IsResourceServerDeclarative", resourceServerID).Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, resourceServerID).
		Return(existingRS, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-1").
		Return(providers.OrganizationUnit{ID: "ou-1"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything, "Updated Name").
		Return(false, nil)
	suite.mockStore.On("UpdateResourceServer", mock.Anything, resourceServerID,
		mock.MatchedBy(func(r providers.ResourceServer) bool {
			return r.Identifier == existingRS.Identifier
		})).Return(nil)

	result, svcErr := suite.service.UpdateResourceServer(context.Background(), resourceServerID, updateReq)

	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.Equal("Updated Name", result.Name)
	suite.Equal("original-identifier", result.Identifier)

	suite.mockStore.AssertExpectations(suite.T())
	suite.mockOU.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_ImmutableDeclarativeResource() {
	// Test that deleting a declarative resource server returns an immutability error
	resourceServerID := declarativeRSID

	// Mock IsResourceServerDeclarative to return true
	suite.mockStore.On("IsResourceServerDeclarative", resourceServerID).Return(true)

	// Execute the test
	svcErr := suite.service.DeleteResourceServer(context.Background(), resourceServerID)

	// Assert immutability error
	suite.NotNil(svcErr)
	suite.Equal("RES-1018", svcErr.Code)
	suite.Equal(ErrorImmutableResourceServer.Code, svcErr.Code)

	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResourceServer_MutableResource() {
	// Test that deleting a non-declarative resource server succeeds
	resourceServerID := "mutable-rs"

	// Mock IsResourceServerDeclarative to return false
	suite.mockStore.On("IsResourceServerDeclarative", resourceServerID).Return(false)

	// Mock the necessary store calls
	suite.mockStore.On("GetResourceServer", mock.Anything, resourceServerID).
		Return(providers.ResourceServer{ID: resourceServerID}, nil)
	suite.mockStore.On("CheckResourceServerHasDependencies", mock.Anything, resourceServerID).Return(false, nil)
	suite.mockStore.On("DeleteResourceServer", mock.Anything, resourceServerID).Return(nil)

	// Execute the test
	svcErr := suite.service.DeleteResourceServer(context.Background(), resourceServerID)

	// Assert success
	suite.Nil(svcErr)

	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateResource_ImmutableDeclarativeResource() {
	// Test that updating a resource in a declarative resource server returns an immutability error
	resourceServerID := declarativeRSID
	resourceID := "res-1"
	updateReq := providers.Resource{
		Name:        "Updated providers.Resource",
		Handle:      "updated",
		Description: "Updated",
	}

	// Mock IsResourceServerDeclarative to return true
	suite.mockStore.On("IsResourceServerDeclarative", resourceServerID).Return(true)

	// Execute the test
	result, svcErr := suite.service.UpdateResource(context.Background(), resourceServerID, resourceID, updateReq)

	// Assert immutability error
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal("RES-1019", svcErr.Code)
	suite.Equal(ErrorImmutableResource.Code, svcErr.Code)

	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_ImmutableDeclarativeResource() {
	// Test that deleting a resource in a declarative resource server returns an immutability error
	resourceServerID := declarativeRSID
	resourceID := "res-1"

	// Mock IsResourceServerDeclarative to return true
	suite.mockStore.On("IsResourceServerDeclarative", resourceServerID).Return(true)

	// Execute the test
	svcErr := suite.service.DeleteResource(context.Background(), resourceServerID, resourceID)

	// Assert immutability error
	suite.NotNil(svcErr)
	suite.Equal("RES-1019", svcErr.Code)
	suite.Equal(ErrorImmutableResource.Code, svcErr.Code)

	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateAction_ImmutableDeclarativeResource() {
	// Test that updating an action in a declarative resource server returns an immutability error
	resourceServerID := declarativeRSID
	actionID := "act-1"
	updateReq := providers.Action{
		Name:        "Updated providers.Action",
		Handle:      "updated",
		Description: "Updated",
	}

	// Mock IsResourceServerDeclarative to return true
	suite.mockStore.On("IsResourceServerDeclarative", resourceServerID).Return(true)

	// Execute the test
	result, svcErr := suite.service.UpdateAction(context.Background(), resourceServerID, nil, actionID, updateReq)

	// Assert immutability error
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal("RES-1020", svcErr.Code)
	suite.Equal(ErrorImmutableAction.Code, svcErr.Code)

	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestDeleteAction_ImmutableDeclarativeResource() {
	// Test that deleting an action in a declarative resource server returns an immutability error
	resourceServerID := declarativeRSID
	actionID := "act-1"

	// Mock IsResourceServerDeclarative to return true
	suite.mockStore.On("IsResourceServerDeclarative", resourceServerID).Return(true)

	// Execute the test
	svcErr := suite.service.DeleteAction(context.Background(), resourceServerID, nil, actionID)

	// Assert immutability error
	suite.NotNil(svcErr)
	suite.Equal("RES-1020", svcErr.Code)
	suite.Equal(ErrorImmutableAction.Code, svcErr.Code)

	suite.mockStore.AssertExpectations(suite.T())
}

// UpdateResourceServer identifier mutability tests

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_IdentifierChanged() {
	rs := providers.ResourceServer{
		Name:       "my-rs",
		Identifier: "https://api.example.com/new/",
		OUID:       "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:         "rs-123",
		Name:       "my-rs",
		Identifier: "https://api.example.com/old/",
		Delimiter:  ":",
		OUID:       "ou-123",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(existingRS, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"https://api.example.com/new/").Return(false, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("UpdateResourceServer", mock.Anything, "rs-123",
		mock.MatchedBy(func(r providers.ResourceServer) bool {
			return r.Identifier == "https://api.example.com/new/"
		})).Return(nil)

	result, svcErr := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.Equal("https://api.example.com/new/", result.Identifier)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_IdentifierConflict() {
	rs := providers.ResourceServer{
		Name:       "updated-rs",
		Identifier: "https://api.example.com/taken/",
		OUID:       "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:         "rs-123",
		Name:       "old-name",
		Identifier: "https://api.example.com/original/",
		Delimiter:  ":",
		OUID:       "ou-123",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(existingRS, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"https://api.example.com/taken/").Return(true, nil)

	result, svcErr := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorIdentifierConflict.Code, svcErr.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_IdentifierCheckStoreError() {
	rs := providers.ResourceServer{
		Name:       "updated-rs",
		Identifier: "https://api.example.com/new/",
		OUID:       "ou-123",
	}

	existingRS := providers.ResourceServer{
		ID:         "rs-123",
		Name:       "old-name",
		Identifier: "https://api.example.com/old/",
		Delimiter:  ":",
		OUID:       "ou-123",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(existingRS, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"https://api.example.com/new/").Return(false, errors.New("db error"))

	result, svcErr := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func newSyncTestService(t *testing.T, consentSvc consent.ConsentServiceInterface) *resourceService {
	t.Helper()
	return &resourceService{
		logger:         *log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
		consentService: consentSvc,
	}
}

func (suite *ResourceServiceTestSuite) TestSyncConsentOnPermissionCreate_CreatesElementWhenAbsent() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().IsEnabled().Return(true)
	cm.EXPECT().
		ValidateConsentElements(mock.Anything, "default", []string{"booking:reservations:read"}).
		Return([]string{}, nil)
	cm.EXPECT().
		CreateConsentElements(mock.Anything, "default", []consent.ConsentElementInput{{
			Name:        "booking:reservations:read",
			Description: "Read reservations",
			Namespace:   providers.NamespacePermission,
		}}).
		Return([]consent.ConsentElement{{ID: "el-1"}}, nil)

	svc := newSyncTestService(suite.T(), cm)
	err := svc.syncConsentOnPermissionCreate(
		context.Background(), "booking:reservations:read", "Read reservations",
	)
	require.NoError(suite.T(), err)
}

func (suite *ResourceServiceTestSuite) TestSyncConsentOnPermissionCreate_SkipsWhenElementExists() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().IsEnabled().Return(true)
	cm.EXPECT().
		ValidateConsentElements(mock.Anything, "default", []string{"p"}).
		Return([]string{"p"}, nil)

	svc := newSyncTestService(suite.T(), cm)
	require.NoError(suite.T(), svc.syncConsentOnPermissionCreate(context.Background(), "p", ""))
}

func (suite *ResourceServiceTestSuite) TestSyncConsentOnPermissionCreate_NoopWhenConsentDisabled() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().IsEnabled().Return(false)

	svc := newSyncTestService(suite.T(), cm)
	require.NoError(suite.T(), svc.syncConsentOnPermissionCreate(context.Background(), "p", ""))
}

func (suite *ResourceServiceTestSuite) TestSyncConsentOnPermissionCreate_NoopWhenPermissionEmpty() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.On("IsEnabled").Return(true).Maybe()

	svc := newSyncTestService(suite.T(), cm)
	require.NoError(suite.T(), svc.syncConsentOnPermissionCreate(context.Background(), "", ""))
}

func (suite *ResourceServiceTestSuite) TestSyncConsentOnPermissionCreate_WrapsConsentServiceError() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().IsEnabled().Return(true)
	se := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "CE-9999"}
	cm.EXPECT().
		ValidateConsentElements(mock.Anything, "default", []string{"p"}).
		Return(nil, se)

	svc := newSyncTestService(suite.T(), cm)
	err := svc.syncConsentOnPermissionCreate(context.Background(), "p", "")
	require.Error(suite.T(), err)
	var ce *consentSyncError
	require.True(suite.T(), errors.As(err, &ce))
	require.Equal(suite.T(), se, ce.Underlying)
}

func (suite *ResourceServiceTestSuite) TestSyncConsentOnPermissionDelete_DeletesExistingElement() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().IsEnabled().Return(true)
	cm.EXPECT().
		ListConsentElements(mock.Anything, "default", providers.NamespacePermission, "p").
		Return([]consent.ConsentElement{{ID: "el-1", Name: "p"}}, nil)
	cm.EXPECT().DeleteConsentElement(mock.Anything, "default", "el-1").Return(nil)

	svc := newSyncTestService(suite.T(), cm)
	require.NoError(suite.T(), svc.syncConsentOnPermissionDelete(context.Background(), "p"))
}

func (suite *ResourceServiceTestSuite) TestSyncConsentOnPermissionDelete_SuccessWhenElementAssociatedWithPurpose() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().IsEnabled().Return(true)
	cm.EXPECT().
		ListConsentElements(mock.Anything, "default", providers.NamespacePermission, "p").
		Return([]consent.ConsentElement{{ID: "el-1", Name: "p"}}, nil)
	cm.EXPECT().DeleteConsentElement(mock.Anything, "default", "el-1").
		Return(&consent.ErrorDeletingConsentElementWithAssociatedPurpose)

	svc := newSyncTestService(suite.T(), cm)
	require.NoError(suite.T(), svc.syncConsentOnPermissionDelete(context.Background(), "p"))
}

func (suite *ResourceServiceTestSuite) TestSyncConsentOnPermissionDelete_NoopWhenElementMissing() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().IsEnabled().Return(true)
	cm.EXPECT().
		ListConsentElements(mock.Anything, "default", providers.NamespacePermission, "p").
		Return([]consent.ConsentElement{}, nil)

	svc := newSyncTestService(suite.T(), cm)
	require.NoError(suite.T(), svc.syncConsentOnPermissionDelete(context.Background(), "p"))
}

// Ensure consentService=nil receivers are tolerated (declarative paths or partial setups).
func (suite *ResourceServiceTestSuite) TestSyncHelpers_TolerateNilConsentService() {
	svc := newSyncTestService(suite.T(), nil)
	require.NoError(suite.T(), svc.syncConsentOnPermissionCreate(context.Background(), "p", ""))
	require.NoError(suite.T(), svc.syncConsentOnPermissionDelete(context.Background(), "p"))
}

func (suite *ResourceServiceTestSuite) TestWrapConsentServiceError_NilPassthrough() {
	svc := newSyncTestService(suite.T(), nil)
	require.Nil(suite.T(), svc.wrapConsentServiceError(context.Background(), nil))
}

func (suite *ResourceServiceTestSuite) TestConsentSyncError_Error() {
	empty := &consentSyncError{}
	require.Equal(suite.T(), "consent sync failed", empty.Error())
	withUnderlying := &consentSyncError{Underlying: &tidcommon.ServiceError{
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "boom"},
	}}
	require.Equal(suite.T(), "boom", withUnderlying.Error())
}

func (suite *ResourceServiceTestSuite) TestConsentSyncError_IsClientError() {
	clientErr := &consentSyncError{Underlying: &tidcommon.ServiceError{Type: tidcommon.ClientErrorType}}
	require.True(suite.T(), clientErr.IsClientError())
	serverErr := &consentSyncError{Underlying: &tidcommon.ServiceError{Type: tidcommon.ServerErrorType}}
	require.False(suite.T(), serverErr.IsClientError())
	emptyErr := &consentSyncError{}
	require.False(suite.T(), emptyErr.IsClientError())
}

// TestResolveResourceServerOUHandle_OUHandleResolved verifies that when only ou_handle is set,
// it is resolved to ou_id via the OU service.
func (suite *ResourceServiceTestSuite) TestResolveResourceServerOUHandle_OUHandleResolved() {
	suite.mockOU.On("GetOrganizationUnitByPath", mock.Anything, "default").
		Return(providers.OrganizationUnit{ID: "ou-resolved"}, (*tidcommon.ServiceError)(nil)).Once()

	rs := &providers.ResourceServer{OUHandle: "default"}
	svcErr := suite.service.ResolveResourceServerOUHandle(context.Background(), rs)

	suite.Nil(svcErr)
	suite.Equal("ou-resolved", rs.OUID)
}

// TestResolveResourceServerOUHandle_OUIDAlreadySet verifies that no resolution happens when
// ou_id is set and ou_handle is empty.
func (suite *ResourceServiceTestSuite) TestResolveResourceServerOUHandle_OUIDAlreadySet() {
	rs := &providers.ResourceServer{OUID: "ou-direct"}
	svcErr := suite.service.ResolveResourceServerOUHandle(context.Background(), rs)

	suite.Nil(svcErr)
	suite.Equal("ou-direct", rs.OUID)
}

// TestResolveResourceServerOUHandle_BothProvided verifies that when both ou_id and ou_handle
// are provided, ou_id is retained and the OU service is never called.
func (suite *ResourceServiceTestSuite) TestResolveResourceServerOUHandle_BothProvided() {
	rs := &providers.ResourceServer{ID: "rs1", Name: "Server", OUID: "ou-direct", OUHandle: "default"}

	svcErr := suite.service.ResolveResourceServerOUHandle(context.Background(), rs)

	suite.Nil(svcErr)
	suite.Equal("ou-direct", rs.OUID)
	// AssertExpectations in t.Cleanup confirms GetOrganizationUnitByPath was never invoked.
}

// TestResolveResourceServerOUHandle_OUHandleNotFound verifies that a not-found response from
// the OU service is surfaced as ErrorInvalidRequestFormat.
func (suite *ResourceServiceTestSuite) TestResolveResourceServerOUHandle_OUHandleNotFound() {
	suite.mockOU.On("GetOrganizationUnitByPath", mock.Anything, "missing").
		Return(providers.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound).Once()

	rs := &providers.ResourceServer{OUHandle: "missing"}
	svcErr := suite.service.ResolveResourceServerOUHandle(context.Background(), rs)

	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
}

// TestResolveResourceServerOUHandle_NeitherProvided verifies the call is a no-op when neither
// ou_id nor ou_handle is provided.
func (suite *ResourceServiceTestSuite) TestResolveResourceServerOUHandle_NeitherProvided() {
	rs := &providers.ResourceServer{}
	svcErr := suite.service.ResolveResourceServerOUHandle(context.Background(), rs)

	suite.Nil(svcErr)
	suite.Empty(rs.OUID)
}

// TestResolveResourceServerOUHandle_NilOUService verifies that a clear error is returned when
// the OU service is nil and ou_handle is supplied (no nil-pointer panic).
func (suite *ResourceServiceTestSuite) TestResolveResourceServerOUHandle_NilOUService() {
	svc := &resourceService{
		logger:    *log.GetLogger(),
		ouService: nil,
	}
	rs := &providers.ResourceServer{OUHandle: "default"}

	svcErr := svc.ResolveResourceServerOUHandle(context.Background(), rs)

	suite.NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// TestResourceServerYAML_OUHandleParsed verifies that ou_handle is parsed off the YAML
// document into the providers.ResourceServer struct.
// newEnabledConsentServiceMock returns a consent service mock with IsEnabled returning true.
func (suite *ResourceServiceTestSuite) newEnabledConsentServiceMock() *consentmock.ConsentServiceInterfaceMock {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.On("IsEnabled").Return(true).Maybe()
	return cm
}

// newServiceWithConsent builds a resource service wired to the suite's store, OU, and
// transactioner mocks but with the supplied consent service.
func (suite *ResourceServiceTestSuite) newServiceWithConsent(
	cm consent.ConsentServiceInterface,
) ResourceServiceInterface {
	svc := &resourceService{
		logger:           *log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
		resourceStore:    suite.mockStore,
		ouService:        suite.mockOU,
		consentService:   cm,
		defaultDelimiter: ":",
		transactioner:    suite.mockTransactioner,
	}
	svc.SetDependencyRegistry(resourcedependency.Initialize(svc))
	return svc
}

func (suite *ResourceServiceTestSuite) TestCreateResourceServer_IDConflict() {
	rs := providers.ResourceServer{
		ID:         "rs-existing",
		Name:       "test-rs",
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything, "test-rs").
		Return(false, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything, "test-identifier").
		Return(false, nil)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-existing").
		Return(providers.ResourceServer{ID: "rs-existing"}, nil)

	result, err := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerIDConflict.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResourceServerByIdentifier_NotFound() {
	suite.mockStore.On("GetResourceServerByIdentifier", mock.Anything, "ident-1").
		Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := suite.service.GetResourceServerByIdentifier(context.Background(), "ident-1")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorResourceServerNotFound.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestGetResourceServerByIdentifier_StoreError() {
	suite.mockStore.On("GetResourceServerByIdentifier", mock.Anything, "ident-1").
		Return(providers.ResourceServer{}, errors.New("database error"))

	result, err := suite.service.GetResourceServerByIdentifier(context.Background(), "ident-1")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_CheckNameError() {
	rs := providers.ResourceServer{
		Name:       testUpdatedName,
		Identifier: "test-identifier",
		OUID:       "ou-123",
	}

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").
		Return(providers.ResourceServer{
			ID: "rs-123", Name: testOriginalName, Identifier: "test-identifier", OUID: "ou-123",
		}, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(providers.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything, testUpdatedName).
		Return(false, errors.New("database error"))

	result, err := suite.service.UpdateResourceServer(context.Background(), "rs-123", rs)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestFindResourceServersByPermissions_StoreError() {
	perms := []string{"booking:read"}
	suite.mockStore.On("FindResourceServersByPermissions", mock.Anything, perms).
		Return([]providers.ResourceServer(nil), errors.New("database error"))

	result, err := suite.service.FindResourceServersByPermissions(context.Background(), perms)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateResource_ConsentSyncError() {
	cm := suite.newEnabledConsentServiceMock()
	serverErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "CE-9999"}
	cm.On("ValidateConsentElements", mock.Anything, "default", mock.Anything).
		Return([]string(nil), serverErr)
	svc := suite.newServiceWithConsent(cm)

	res := providers.Resource{Name: "test-resource", Handle: "test-handle"}
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").
		Return(providers.ResourceServer{ID: "rs-123"}, nil)
	suite.mockStore.On("CheckResourceHandleExists", mock.Anything,
		"rs-123", "test-handle", (*string)(nil)).Return(false, nil)
	suite.mockStore.On("CreateResource", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", (*string)(nil), matchResource(res)).Return(nil)

	result, err := svc.CreateResource(context.Background(), "rs-123", res)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteResource_ConsentSyncError() {
	cm := suite.newEnabledConsentServiceMock()
	serverErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "CE-9999"}
	cm.On("ListConsentElements", mock.Anything, "default", providers.NamespacePermission, "perm-x").
		Return([]consent.ConsentElement(nil), serverErr)
	svc := suite.newServiceWithConsent(cm)

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").
		Return(providers.ResourceServer{ID: "rs-123"}, nil)
	suite.mockStore.On("GetResource", mock.Anything, "res-123", "rs-123").
		Return(providers.Resource{ID: "res-123", Permission: "perm-x"}, nil)
	suite.mockStore.On("CheckResourceHasDependencies", mock.Anything, "res-123").
		Return(false, nil)
	suite.mockStore.On("DeleteResource", mock.Anything, "res-123", "rs-123").Return(nil)

	err := svc.DeleteResource(context.Background(), "rs-123", "res-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestCreateAction_ConsentSyncError() {
	cm := suite.newEnabledConsentServiceMock()
	serverErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "CE-9999"}
	cm.On("ValidateConsentElements", mock.Anything, "default", mock.Anything).
		Return([]string(nil), serverErr)
	svc := suite.newServiceWithConsent(cm)

	action := providers.Action{Name: "test-action", Handle: "test-handle"}
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").
		Return(providers.ResourceServer{ID: "rs-123"}, nil)
	suite.mockStore.On("CheckActionHandleExists", mock.Anything,
		"rs-123", (*string)(nil), "test-handle").Return(false, nil)
	suite.mockStore.On("CreateAction", mock.Anything,
		mock.AnythingOfType("string"), "rs-123", (*string)(nil), matchAction(action)).Return(nil)

	result, err := svc.CreateAction(context.Background(), "rs-123", nil, action)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteAction_LoadForConsentSyncError() {
	cm := suite.newEnabledConsentServiceMock()
	svc := suite.newServiceWithConsent(cm)

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").
		Return(providers.ResourceServer{ID: "rs-123"}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything, "action-123", "rs-123", (*string)(nil)).
		Return(true, nil)
	suite.mockStore.On("GetAction", mock.Anything, "action-123", "rs-123", (*string)(nil)).
		Return(providers.Action{}, errors.New("database error"))

	err := svc.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *ResourceServiceTestSuite) TestDeleteAction_ConsentSyncError() {
	cm := suite.newEnabledConsentServiceMock()
	serverErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "CE-9999"}
	cm.On("ListConsentElements", mock.Anything, "default", providers.NamespacePermission, "perm-x").
		Return([]consent.ConsentElement(nil), serverErr)
	svc := suite.newServiceWithConsent(cm)

	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").
		Return(providers.ResourceServer{ID: "rs-123"}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything, "action-123", "rs-123", (*string)(nil)).
		Return(true, nil)
	suite.mockStore.On("GetAction", mock.Anything, "action-123", "rs-123", (*string)(nil)).
		Return(providers.Action{ID: "action-123", Permission: "perm-x"}, nil)
	suite.mockStore.On("DeleteAction", mock.Anything, "action-123", "rs-123", (*string)(nil)).
		Return(nil)

	err := svc.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func TestResourceServerYAML_OUHandleParsed(t *testing.T) {
	yamlData := []byte(`
id: rs1
name: Server
handle: server
ouHandle: default
`)
	rs, err := parseToResourceServer(yamlData)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rs.OUHandle != "default" {
		t.Errorf("OUHandle = %q, want %q", rs.OUHandle, "default")
	}
	if rs.OUID != "" {
		t.Errorf("OUID = %q, want empty (resolution happens later)", rs.OUID)
	}
}
