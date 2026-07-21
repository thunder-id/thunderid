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

package actorprovider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
)

type UtilsTestSuite struct {
	suite.Suite
	mockInbound *inboundclientmock.InboundClientServiceInterfaceMock
	mockEntity  *entityprovidermock.EntityProviderInterfaceMock
	provider    providers.ActorProvider
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) SetupTest() {
	s.mockInbound = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.mockEntity = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.provider = Initialize(s.mockInbound, s.mockEntity, managermock.NewAuthnProviderManagerMock(s.T()))
}

func (s *UtilsTestSuite) TestBuildApplication_Success() {
	client := &inboundmodel.InboundClient{
		ID: "app-1",
		Properties: map[string]interface{}{
			"metadata": map[string]interface{}{"key": "value"},
		},
	}
	attrs, _ := json.Marshal(map[string]interface{}{"name": "My App", "clientId": "public-client"})
	entity := &providers.Entity{ID: "app-1", SystemAttributes: attrs}

	s.mockInbound.On("GetInboundClientByEntityID", mock.Anything, "app-1").Return(client, nil)
	s.mockEntity.On("GetEntity", "app-1").Return(entity, (*entityprovider.EntityProviderError)(nil))

	app, svcErr := BuildApplication(context.Background(), s.provider, "app-1")

	s.Nil(svcErr)
	s.Equal("app-1", app.ID)
	s.Equal("My App", app.Name)
	s.Equal("value", app.Metadata["key"])
	s.Require().Len(app.InboundAuthConfig, 1)
	s.Equal("public-client", app.InboundAuthConfig[0].OAuthConfig.ClientID)
}

func (s *UtilsTestSuite) TestBuildApplication_NilClient() {
	mockProvider := actorprovidermock.NewActorProviderMock(s.T())
	mockProvider.EXPECT().GetInboundClientByID(mock.Anything, "app-1").
		Return((*providers.InboundClient)(nil), (*tidcommon.ServiceError)(nil))

	app, svcErr := BuildApplication(context.Background(), mockProvider, "app-1")

	s.Nil(app)
	s.Equal(ErrorActorNotFound.Code, svcErr.Code)
}

func (s *UtilsTestSuite) TestBuildApplication_EntityLoadError() {
	client := &inboundmodel.InboundClient{ID: "app-1"}
	s.mockInbound.On("GetInboundClientByEntityID", mock.Anything, "app-1").Return(client, nil)
	s.mockEntity.On("GetEntity", "app-1").Return(
		(*providers.Entity)(nil),
		entityprovider.NewEntityProviderError("INTERNAL_ERROR", "boom", ""))

	app, svcErr := BuildApplication(context.Background(), s.provider, "app-1")

	s.Nil(app)
	s.NotNil(svcErr)
}

func (s *UtilsTestSuite) TestBuildApplication_EntityNotFound() {
	client := &inboundmodel.InboundClient{ID: "app-1", AllowedUserTypes: []string{"customer"}}
	s.mockInbound.On("GetInboundClientByEntityID", mock.Anything, "app-1").Return(client, nil)
	s.mockEntity.On("GetEntity", "app-1").Return(
		(*providers.Entity)(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "missing", ""))

	app, svcErr := BuildApplication(context.Background(), s.provider, "app-1")

	s.Nil(svcErr)
	s.Equal("app-1", app.ID)
	s.Equal([]string{"customer"}, app.AllowedUserTypes)
}

func (s *UtilsTestSuite) TestBuildApplicationMetadata_NilEntityAndProps() {
	meta := BuildApplicationMetadata("app-1", nil, nil)
	s.Equal("app-1", meta.ID)
	s.Empty(meta.Name)
}

func (s *UtilsTestSuite) TestBuildApplicationMetadata_InvalidEntityJSON() {
	entity := &providers.Entity{SystemAttributes: []byte("not-json")}
	meta := BuildApplicationMetadata("app-1", entity, nil)
	s.Equal("app-1", meta.ID)
	s.Empty(meta.Name)
}

func (s *UtilsTestSuite) TestAssembleApplication_NoClientID() {
	client := &providers.InboundClient{ID: "app-1"}
	app := assembleApplication(client, nil)
	s.Equal("app-1", app.ID)
	s.Empty(app.InboundAuthConfig)
}

func (s *UtilsTestSuite) TestAssembleApplication_CarriesFlowIDs() {
	client := &providers.InboundClient{
		ID:                   "app-1",
		AuthFlowID:           "auth-flow",
		SignOutFlowID:        "signout-flow",
		IsSignOutFlowEnabled: true,
	}

	app := assembleApplication(client, nil)

	s.Equal("auth-flow", app.AuthFlowID)
	s.Equal("signout-flow", app.SignOutFlowID)
	s.True(app.IsSignOutFlowEnabled)
}

func (s *UtilsTestSuite) TestBuildApplication_NotFound() {
	s.mockInbound.On("GetInboundClientByEntityID", mock.Anything, "missing").
		Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound)

	app, svcErr := BuildApplication(context.Background(), s.provider, "missing")

	s.Nil(app)
	s.Equal(ErrorActorNotFound.Code, svcErr.Code)
}

func (s *UtilsTestSuite) TestBuildApplication_InboundClientError() {
	s.mockInbound.On("GetInboundClientByEntityID", mock.Anything, "app-1").
		Return((*inboundmodel.InboundClient)(nil), errors.New("db error"))

	app, svcErr := BuildApplication(context.Background(), s.provider, "app-1")

	s.Nil(app)
	s.NotNil(svcErr)
	s.NotEqual(ErrorActorNotFound.Code, svcErr.Code)
}

func (s *UtilsTestSuite) TestBuildApplicationMetadata() {
	attrs, _ := json.Marshal(map[string]interface{}{"name": "App", "description": "Desc"})
	entity := &providers.Entity{SystemAttributes: attrs}
	props := map[string]interface{}{
		"logo_url":   "https://logo",
		"url":        "https://app",
		"tos_uri":    "https://tos",
		"policy_uri": "https://policy",
	}

	meta := BuildApplicationMetadata("app-1", entity, props)

	assert.Equal(s.T(), "app-1", meta.ID)
	assert.Equal(s.T(), "App", meta.Name)
	assert.Equal(s.T(), "Desc", meta.Description)
	assert.Equal(s.T(), "https://logo", meta.LogoURL)
	assert.Equal(s.T(), "https://app", meta.URL)
	assert.Equal(s.T(), "https://tos", meta.TosURI)
	assert.Equal(s.T(), "https://policy", meta.PolicyURI)
}

func (s *UtilsTestSuite) TestReadEntitySystemAttributes_NilEntity() {
	attrs := readEntitySystemAttributes(nil)
	s.Empty(attrs)
}

func (s *UtilsTestSuite) TestReadEntitySystemAttributes_InvalidJSON() {
	entity := &providers.Entity{SystemAttributes: []byte("not-json")}
	attrs := readEntitySystemAttributes(entity)
	s.Empty(attrs)
}

func (s *UtilsTestSuite) TestReadEntitySystemAttributes_EmptyAttributes() {
	entity := &providers.Entity{SystemAttributes: []byte{}}
	attrs := readEntitySystemAttributes(entity)
	s.Empty(attrs)
}

func TestBuildApplication_InboundClientStoreError(t *testing.T) {
	mockInbound := inboundclientmock.NewInboundClientServiceInterfaceMock(t)
	mockEntity := entityprovidermock.NewEntityProviderInterfaceMock(t)
	provider := Initialize(mockInbound, mockEntity, managermock.NewAuthnProviderManagerMock(t))

	mockInbound.On("GetInboundClientByEntityID", mock.Anything, "app-1").
		Return((*inboundmodel.InboundClient)(nil), errors.New("db error"))

	app, svcErr := BuildApplication(context.Background(), provider, "app-1")

	assert.Nil(t, app)
	assert.NotNil(t, svcErr)
	assert.NotEqual(t, ErrorActorNotFound.Code, svcErr.Code)
}
