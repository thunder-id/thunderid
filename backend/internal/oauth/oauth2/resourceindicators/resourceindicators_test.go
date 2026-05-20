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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
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
	rs := resource.ResourceServer{ID: "rs01", Identifier: "https://api.example.com"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(&rs, nil)

	resolved, err := ResolveResourceServers(context.Background(), suite.mockResourceService,
		[]string{"https://api.example.com"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []*resource.ResourceServer{&rs}, resolved)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveResourceServers_NotFound_ReturnsInvalidTarget() {
	svcErr := &serviceerror.ServiceError{Type: serviceerror.ClientErrorType, Code: "RSE-4041"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://unknown.example.com").
		Return(nil, svcErr)

	resolved, err := ResolveResourceServers(context.Background(), suite.mockResourceService,
		[]string{"https://unknown.example.com"})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *ResourceIndicatorsTestSuite) TestResolveResourceServers_StoreFailure_ReturnsServerError() {
	svcErr := &serviceerror.ServiceError{Type: serviceerror.ServerErrorType, Code: "SSE-5000"}
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(nil, svcErr)

	resolved, err := ResolveResourceServers(context.Background(), suite.mockResourceService,
		[]string{"https://api.example.com"})

	assert.Nil(suite.T(), resolved)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to resolve resource server", err.ErrorDescription)
}

// ComposeAudiences §4 fallback-only clientID tests

func (suite *ResourceIndicatorsTestSuite) TestComposeAudiences_RSContributes_NoClientID() {
	// When at least one RS contributes, clientID must NOT appear in aud.
	rs := &resource.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	auds, err := ComposeAudiences(context.Background(), suite.mockResourceService,
		"client123", []*resource.ResourceServer{rs}, []string{"read"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []string{"https://rs01.example.com"}, auds)
}

func (suite *ResourceIndicatorsTestSuite) TestComposeAudiences_NoRS_FallbackToClientID() {
	// When no RS contributes (explicit empty resolvedRSes), aud falls back to clientID.
	auds, err := ComposeAudiences(context.Background(), suite.mockResourceService,
		"client123", []*resource.ResourceServer{}, []string{})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []string{"client123"}, auds)
}

func (suite *ResourceIndicatorsTestSuite) TestComposeAudiences_NilResolvedRSes_NoScopes_FallbackToClientID() {
	// resolvedRSes==nil, no scopes → implicit discovery skipped → fallback to clientID.
	suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, mock.Anything).
		Return([]resource.ResourceServer{}, nil).Maybe()

	auds, err := ComposeAudiences(context.Background(), suite.mockResourceService,
		"client123", nil, []string{})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []string{"client123"}, auds)
}

func (suite *ResourceIndicatorsTestSuite) TestComposeAudiences_ImplicitDiscovery_RSFound() {
	// resolvedRSes==nil with scopes → implicit discovery returns RS → aud contains RS only, no clientID.
	suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, []string{"read"}).
		Return([]resource.ResourceServer{
			{ID: "rs01", Identifier: "https://rs01.example.com"},
		}, nil)

	auds, err := ComposeAudiences(context.Background(), suite.mockResourceService,
		"client123", nil, []string{"read"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []string{"https://rs01.example.com"}, auds)
}

func (suite *ResourceIndicatorsTestSuite) TestComposeAudiences_ImplicitDiscovery_NoRSFound_FallbackToClientID() {
	// resolvedRSes==nil with scopes → implicit discovery returns nothing → fallback to clientID.
	suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, []string{"openid"}).
		Return([]resource.ResourceServer{}, nil)

	auds, err := ComposeAudiences(context.Background(), suite.mockResourceService,
		"client123", nil, []string{"openid"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []string{"client123"}, auds)
}

func (suite *ResourceIndicatorsTestSuite) TestComposeAudiences_EmptyClientID_NoRS_ReturnsEmptySlice() {
	// No RS and empty clientID → return empty slice (no fallback possible).
	auds, err := ComposeAudiences(context.Background(), suite.mockResourceService,
		"", []*resource.ResourceServer{}, []string{})

	assert.Nil(suite.T(), err)
	assert.Empty(suite.T(), auds)
}

func (suite *ResourceIndicatorsTestSuite) TestComposeAudiences_MultipleRS_Deduped() {
	// Multiple RSes with duplicate identifiers are deduped; clientID is absent.
	rs1 := &resource.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	rs2 := &resource.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	auds, err := ComposeAudiences(context.Background(), suite.mockResourceService,
		"client123", []*resource.ResourceServer{rs1, rs2}, []string{"read"})

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), []string{"https://rs01.example.com"}, auds)
}

func (suite *ResourceIndicatorsTestSuite) TestComposeAudiences_ImplicitDiscovery_ServiceError() {
	// FindResourceServersByPermissions failure returns server error.
	svcErr := &serviceerror.ServiceError{Code: "internal_error"}
	suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, []string{"read"}).
		Return(nil, svcErr)

	auds, err := ComposeAudiences(context.Background(), suite.mockResourceService,
		"client123", nil, []string{"read"})

	assert.Nil(suite.T(), auds)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

// ContributingAudiences tests

func (suite *ResourceIndicatorsTestSuite) TestContributingAudiences_Empty() {
	auds := ContributingAudiences([]*resource.ResourceServer{})
	assert.Nil(suite.T(), auds)
}

func (suite *ResourceIndicatorsTestSuite) TestContributingAudiences_SkipsEmptyIdentifier() {
	rs := &resource.ResourceServer{ID: "rs01", Identifier: ""}
	auds := ContributingAudiences([]*resource.ResourceServer{rs})
	assert.Empty(suite.T(), auds)
}

func (suite *ResourceIndicatorsTestSuite) TestContributingAudiences_PreservesOrder() {
	rs1 := &resource.ResourceServer{ID: "rs01", Identifier: "https://b.example.com"}
	rs2 := &resource.ResourceServer{ID: "rs02", Identifier: "https://a.example.com"}
	auds := ContributingAudiences([]*resource.ResourceServer{rs1, rs2})
	assert.Equal(suite.T(), []string{"https://b.example.com", "https://a.example.com"}, auds)
}

// FilterByIdentifiers tests

func (suite *ResourceIndicatorsTestSuite) TestFilterByIdentifiers_Empty() {
	result := FilterByIdentifiers([]*resource.ResourceServer{}, []string{"https://api.example.com"})
	assert.Empty(suite.T(), result)
}

func (suite *ResourceIndicatorsTestSuite) TestFilterByIdentifiers_AllMatch() {
	rs1 := &resource.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	rs2 := &resource.ResourceServer{ID: "rs02", Identifier: "https://rs02.example.com"}
	result := FilterByIdentifiers([]*resource.ResourceServer{rs1, rs2},
		[]string{"https://rs01.example.com", "https://rs02.example.com"})
	assert.Equal(suite.T(), []*resource.ResourceServer{rs1, rs2}, result)
}

func (suite *ResourceIndicatorsTestSuite) TestFilterByIdentifiers_Subset() {
	rs1 := &resource.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	rs2 := &resource.ResourceServer{ID: "rs02", Identifier: "https://rs02.example.com"}
	result := FilterByIdentifiers([]*resource.ResourceServer{rs1, rs2},
		[]string{"https://rs01.example.com"})
	assert.Equal(suite.T(), []*resource.ResourceServer{rs1}, result)
}

func (suite *ResourceIndicatorsTestSuite) TestFilterByIdentifiers_NoMatch() {
	rs1 := &resource.ResourceServer{ID: "rs01", Identifier: "https://rs01.example.com"}
	result := FilterByIdentifiers([]*resource.ResourceServer{rs1}, []string{"https://other.example.com"})
	assert.Empty(suite.T(), result)
}

func (suite *ResourceIndicatorsTestSuite) TestFilterByIdentifiers_PreservesOrder() {
	rs1 := &resource.ResourceServer{ID: "rs01", Identifier: "https://b.example.com"}
	rs2 := &resource.ResourceServer{ID: "rs02", Identifier: "https://a.example.com"}
	result := FilterByIdentifiers([]*resource.ResourceServer{rs1, rs2},
		[]string{"https://b.example.com", "https://a.example.com"})
	assert.Equal(suite.T(), []*resource.ResourceServer{rs1, rs2}, result)
}

// UnionScopes tests

func (suite *ResourceIndicatorsTestSuite) TestUnionScopes_Empty() {
	result := UnionScopes(map[string][]string{})
	assert.Empty(suite.T(), result)
}

func (suite *ResourceIndicatorsTestSuite) TestUnionScopes_Deduped() {
	input := map[string][]string{
		"rs01": {"read", "write"},
		"rs02": {"write", "delete"},
	}
	result := UnionScopes(input)
	assert.Contains(suite.T(), result, "read")
	assert.Contains(suite.T(), result, "write")
	assert.Contains(suite.T(), result, "delete")
	assert.Equal(suite.T(), 3, len(result))
}
