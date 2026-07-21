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

package resource_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/resource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

const defaultResourceServerConfigName = "defaultResourceServer"

type DefaultAwareProviderTestSuite struct {
	suite.Suite
	base    *resourcemock.ResourceServiceInterfaceMock
	config  *serverconfigmock.ServerConfigServiceMock
	subject providers.ResourceServerProvider
}

func TestDefaultAwareProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultAwareProviderTestSuite))
}

func (suite *DefaultAwareProviderTestSuite) SetupTest() {
	suite.base = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.config = serverconfigmock.NewServerConfigServiceMock(suite.T())
	suite.subject = resource.NewDefaultAwareResourceServerProvider(suite.base, suite.config)
}

// An explicit identifier is delegated to the wrapped provider verbatim; the server-config store is
// never consulted.
func (suite *DefaultAwareProviderTestSuite) TestExplicitIdentifier_DelegatesToBase() {
	rs := providers.ResourceServer{ID: "rs01", Identifier: "https://api.example.com"}
	suite.base.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(&rs, nil)

	resolved, err := suite.subject.GetResourceServerByIdentifier(context.Background(), "https://api.example.com")

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), &rs, resolved)
}

func (suite *DefaultAwareProviderTestSuite) TestExplicitIdentifier_PropagatesBaseError() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RES-1003"}
	suite.base.On("GetResourceServerByIdentifier", mock.Anything, "https://unknown.example.com").
		Return(nil, svcErr)

	resolved, err := suite.subject.GetResourceServerByIdentifier(context.Background(), "https://unknown.example.com")

	assert.Nil(suite.T(), resolved)
	assert.Same(suite.T(), svcErr, err)
}

// An empty identifier resolves the configured default resource server through the wrapped provider.
func (suite *DefaultAwareProviderTestSuite) TestEmptyIdentifier_DefaultConfigured_Resolves() {
	rs := providers.ResourceServer{ID: "rs-1", Identifier: "https://api.example.com"}
	suite.config.On("GetMergedConfig", mock.Anything, defaultResourceServerConfigName).
		Return(resource.DefaultResourceServerConfig{ResourceServerID: "rs-1"}, nil)
	suite.base.On("GetResourceServer", mock.Anything, "rs-1").Return(&rs, nil)

	resolved, err := suite.subject.GetResourceServerByIdentifier(context.Background(), "")

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), &rs, resolved)
}

// No default configured surfaces as a client error so the caller maps it to invalid_target.
func (suite *DefaultAwareProviderTestSuite) TestEmptyIdentifier_NoDefaultConfigured_ReturnsClientError() {
	suite.config.On("GetMergedConfig", mock.Anything, defaultResourceServerConfigName).
		Return(resource.DefaultResourceServerConfig{ResourceServerID: ""}, nil)

	resolved, err := suite.subject.GetResourceServerByIdentifier(context.Background(), "")

	assert.Nil(suite.T(), resolved)
	require.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.ClientErrorType, err.Type)
}

// A merged value of an unexpected type is treated as "no default configured" (client error), matching
// the pre-refactor behavior.
func (suite *DefaultAwareProviderTestSuite) TestEmptyIdentifier_ConfigTypeMismatch_ReturnsClientError() {
	suite.config.On("GetMergedConfig", mock.Anything, defaultResourceServerConfigName).
		Return("unexpected-type", nil)

	resolved, err := suite.subject.GetResourceServerByIdentifier(context.Background(), "")

	assert.Nil(suite.T(), resolved)
	require.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.ClientErrorType, err.Type)
}

// A failure reading the merged config is propagated as a server error.
func (suite *DefaultAwareProviderTestSuite) TestEmptyIdentifier_ConfigReadFailure_ReturnsServerError() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SCE-5000"}
	suite.config.On("GetMergedConfig", mock.Anything, defaultResourceServerConfigName).
		Return(nil, svcErr)

	resolved, err := suite.subject.GetResourceServerByIdentifier(context.Background(), "")

	assert.Nil(suite.T(), resolved)
	assert.Same(suite.T(), svcErr, err)
}

// A configured default that no longer exists fails closed with the wrapped provider's error.
func (suite *DefaultAwareProviderTestSuite) TestEmptyIdentifier_DefaultDeleted_PropagatesBaseError() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RES-1003"}
	suite.config.On("GetMergedConfig", mock.Anything, defaultResourceServerConfigName).
		Return(resource.DefaultResourceServerConfig{ResourceServerID: "rs-gone"}, nil)
	suite.base.On("GetResourceServer", mock.Anything, "rs-gone").Return(nil, svcErr)

	resolved, err := suite.subject.GetResourceServerByIdentifier(context.Background(), "")

	assert.Nil(suite.T(), resolved)
	assert.Same(suite.T(), svcErr, err)
}

// GetResourceServer and ValidatePermissions are promoted from the embedded provider unchanged.
func (suite *DefaultAwareProviderTestSuite) TestGetResourceServer_Delegates() {
	rs := providers.ResourceServer{ID: "rs01"}
	suite.base.On("GetResourceServer", mock.Anything, "rs01").Return(&rs, nil)

	resolved, err := suite.subject.GetResourceServer(context.Background(), "rs01")

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), &rs, resolved)
}

func (suite *DefaultAwareProviderTestSuite) TestValidatePermissions_Delegates() {
	suite.base.On("ValidatePermissions", mock.Anything, "rs01", []string{"read"}).
		Return([]string{}, nil)

	invalid, err := suite.subject.ValidatePermissions(context.Background(), "rs01", []string{"read"})

	assert.Nil(suite.T(), err)
	assert.Empty(suite.T(), invalid)
}

func (suite *DefaultAwareProviderTestSuite) TestConstructor_PanicsOnNilBase() {
	assert.Panics(suite.T(), func() {
		resource.NewDefaultAwareResourceServerProvider(nil, suite.config)
	})
}

func (suite *DefaultAwareProviderTestSuite) TestConstructor_PanicsOnNilServerConfig() {
	assert.Panics(suite.T(), func() {
		resource.NewDefaultAwareResourceServerProvider(suite.base, nil)
	})
}
