/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package resourceindicators

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

type ResourceIndicatorsTestSuite struct {
	suite.Suite
	mockResourceService *resourcemock.ResourceServiceInterfaceMock
}

func TestResourceIndicatorsTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceIndicatorsTestSuite))
}

func (suite *ResourceIndicatorsTestSuite) SetupTest() {
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
}

// ValidateResourceURIs tests

func (suite *ResourceIndicatorsTestSuite) TestValidateResourceURIs_Valid() {
	err := ValidateResourceURIs([]string{"https://api.example.com/resource"})
	assert.Nil(suite.T(), err)
}

func (suite *ResourceIndicatorsTestSuite) TestValidateResourceURIs_Empty() {
	err := ValidateResourceURIs([]string{})
	assert.Nil(suite.T(), err)
}

func (suite *ResourceIndicatorsTestSuite) TestValidateResourceURIs_MissingScheme() {
	err := ValidateResourceURIs([]string{"api.example.com/resource"})
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestValidateResourceURIs_WithFragment() {
	err := ValidateResourceURIs([]string{"https://api.example.com/resource#frag"})
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Contains(suite.T(), err.ErrorDescription, "fragment")
}

func (suite *ResourceIndicatorsTestSuite) TestValidateResourceURIs_InvalidURI() {
	err := ValidateResourceURIs([]string{"://bad"})
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

// ResolveResourceServers tests

func (suite *ResourceIndicatorsTestSuite) TestResolveResourceServers_Empty() {
	resolved, err := ResolveResourceServers(context.Background(), suite.mockResourceService, []string{})
	assert.Nil(suite.T(), err)
	assert.Nil(suite.T(), resolved)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveResourceServers_Found() {
	rs := providers.ResourceServer{ID: "rs01", Identifier: "https://api.example.com"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(&rs, nil)

	resolved, err := ResolveResourceServers(context.Background(), suite.mockResourceService,
		[]string{"https://api.example.com"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []*providers.ResourceServer{&rs}, resolved)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveResourceServers_NotFound_ReturnsInvalidTarget() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RSE-4041"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://unknown.example.com").
		Return(nil, svcErr)

	resolved, err := ResolveResourceServers(context.Background(), suite.mockResourceService,
		[]string{"https://unknown.example.com"})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveResourceServers_StoreFailure_ReturnsServerError() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SSE-5000"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(nil, svcErr)

	resolved, err := ResolveResourceServers(context.Background(), suite.mockResourceService,
		[]string{"https://api.example.com"})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to resolve resource server", err.ErrorDescription)
}

// ResolveTargetResourceServer tests

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_SingleResource_Resolves() {
	rs := providers.ResourceServer{ID: "rs01", Identifier: "https://api.example.com"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(&rs, nil)
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{"https://api.example.com"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), &rs, resolved)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_MultipleResources_ReturnsInvalidTarget() {
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{"https://a.example.com", "https://b.example.com"})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_UnknownIdentifier_ReturnsInvalidTarget() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RSE-4041"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://unknown.example.com").
		Return(nil, svcErr)
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{"https://unknown.example.com"})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_LookupServerError_ReturnsServerError() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SSE-5000"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(nil, svcErr)
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{"https://api.example.com"})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_NoResource_DefaultConfigured_Resolves() {
	rs := providers.ResourceServer{ID: "rs-1", Identifier: "https://api.example.com"}
	suite.mockResourceService.On("GetResourceServer", mock.Anything, "rs-1").
		Return(&rs, nil)
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
	serverConfig.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(resource.DefaultResourceServerConfig{ResourceServerID: "rs-1"}, nil)

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), &rs, resolved)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_EmptyDefaultID_ReturnsInvalidTarget() {
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
	serverConfig.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(resource.DefaultResourceServerConfig{ResourceServerID: ""}, nil)

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_NilServerConfig_ReturnsInvalidTarget() {
	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		nil, []string{})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_DefaultRSNotFound_ReturnsInvalidTarget() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RSE-4041"}
	suite.mockResourceService.On("GetResourceServer", mock.Anything, "rs-1").
		Return(nil, svcErr)
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
	serverConfig.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(resource.DefaultResourceServerConfig{ResourceServerID: "rs-1"}, nil)

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_MergedConfigError_ReturnsServerError() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SCE-5000"}
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
	serverConfig.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(nil, svcErr)

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

// DownscopeToResourceServer tests

func (suite *ResourceIndicatorsTestSuite) TestDownscopeToResourceServer_DropsInvalidScopes() {
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, "rs01", []string{"read", "write", "delete"}).
		Return([]string{"write"}, nil)

	scopes, err := DownscopeToResourceServer(context.Background(), suite.mockResourceService, "rs01",
		[]string{"read", "write", "delete"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []string{"read", "delete"}, scopes)
}

func (suite *ResourceIndicatorsTestSuite) TestDownscopeToResourceServer_PreservesOrder() {
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, "rs01", []string{"c", "a", "b"}).
		Return([]string{}, nil)

	scopes, err := DownscopeToResourceServer(context.Background(), suite.mockResourceService, "rs01",
		[]string{"c", "a", "b"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []string{"c", "a", "b"}, scopes)
}

func (suite *ResourceIndicatorsTestSuite) TestDownscopeToResourceServer_EmptyScopes_Unchanged() {
	scopes, err := DownscopeToResourceServer(context.Background(), suite.mockResourceService, "rs01",
		[]string{})

	assert.Nil(suite.T(), err)
	assert.Empty(suite.T(), scopes)
}

func (suite *ResourceIndicatorsTestSuite) TestDownscopeToResourceServer_ValidatePermissionsError_ReturnsServerError() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "RSE-5000"}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, "rs01", []string{"read"}).
		Return(nil, svcErr)

	scopes, err := DownscopeToResourceServer(context.Background(), suite.mockResourceService, "rs01",
		[]string{"read"})

	assert.Nil(suite.T(), scopes)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

// ComputeRSValidScopes tests

func (suite *ResourceIndicatorsTestSuite) TestComputeRSValidScopes_Empty() {
	result, err := ComputeRSValidScopes(context.Background(), suite.mockResourceService,
		[]*providers.ResourceServer{}, []string{"read"})
	assert.Nil(suite.T(), err)
	assert.Empty(suite.T(), result)
}

func (suite *ResourceIndicatorsTestSuite) TestComputeRSValidScopes_DropsInvalidPerRS() {
	rs := &providers.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, "rs01", []string{"read", "write"}).
		Return([]string{"write"}, nil)

	result, err := ComputeRSValidScopes(context.Background(), suite.mockResourceService,
		[]*providers.ResourceServer{rs}, []string{"read", "write"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), map[string][]string{"rs01": {"read"}}, result)
}

func (suite *ResourceIndicatorsTestSuite) TestComputeRSValidScopes_ValidatePermissionsError_ReturnsServerError() {
	rs := &providers.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "RSE-5000"}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, "rs01", []string{"read"}).
		Return(nil, svcErr)

	result, err := ComputeRSValidScopes(context.Background(), suite.mockResourceService,
		[]*providers.ResourceServer{rs}, []string{"read"})

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

// ResolveAndDownscope tests

func (suite *ResourceIndicatorsTestSuite) TestResolveAndDownscope_NoResources_Unchanged() {
	resolved, scopes, err := ResolveAndDownscope(context.Background(), suite.mockResourceService,
		[]string{}, []string{"read", "write"})

	assert.Nil(suite.T(), err)
	assert.Nil(suite.T(), resolved)
	assert.Equal(suite.T(), []string{"read", "write"}, scopes)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveAndDownscope_Downscopes() {
	rs := providers.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://rs01.example.com").
		Return(&rs, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, "rs01", []string{"read", "write"}).
		Return([]string{"write"}, nil)

	resolved, scopes, err := ResolveAndDownscope(context.Background(), suite.mockResourceService,
		[]string{"https://rs01.example.com"}, []string{"read", "write"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []*providers.ResourceServer{&rs}, resolved)
	assert.Equal(suite.T(), []string{"read"}, scopes)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveAndDownscope_UnknownIdentifier_ReturnsInvalidTarget() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RSE-4041"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://unknown.example.com").
		Return(nil, svcErr)

	resolved, scopes, err := ResolveAndDownscope(context.Background(), suite.mockResourceService,
		[]string{"https://unknown.example.com"}, []string{"read"})

	assert.Nil(suite.T(), resolved)
	assert.Nil(suite.T(), scopes)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveTargetResourceServer_InvalidResourceURI_ReturnsInvalidTarget() {
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())

	resolved, err := ResolveTargetResourceServer(context.Background(), suite.mockResourceService,
		serverConfig, []string{"api.example.com/resource"})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

// ResolveAudienceBinding tests

func (suite *ResourceIndicatorsTestSuite) TestResolveAudienceBinding_NoResourceNoPermissionScopes_ReturnsNil() {
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())

	rs, err := ResolveAudienceBinding(context.Background(), suite.mockResourceService, serverConfig, nil, nil)

	assert.Nil(suite.T(), rs)
	assert.Nil(suite.T(), err)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveAudienceBinding_PermissionScopes_ResolvesDefault() {
	suite.mockResourceService.On("GetResourceServer", mock.Anything, "rs-1").
		Return(&providers.ResourceServer{ID: "rs-1", Identifier: "https://api.example.com"}, nil)
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())
	serverConfig.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(resource.DefaultResourceServerConfig{ResourceServerID: "rs-1"}, nil)

	rs, err := ResolveAudienceBinding(context.Background(), suite.mockResourceService, serverConfig,
		nil, []string{"read"})

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), rs)
	assert.Equal(suite.T(), "rs-1", rs.ID)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveAudienceBinding_ExplicitResourceNoPermissionScopes_Resolves() {
	rs := providers.ResourceServer{ID: "rs01", Identifier: "https://api.example.com"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(&rs, nil)
	serverConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())

	resolved, err := ResolveAudienceBinding(context.Background(), suite.mockResourceService, serverConfig,
		[]string{"https://api.example.com"}, nil)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), &rs, resolved)
}
