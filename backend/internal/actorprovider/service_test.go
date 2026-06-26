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
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
)

type ActorProviderTestSuite struct {
	suite.Suite
	mockInbound *inboundclientmock.InboundClientServiceInterfaceMock
	mockEntity  *entityprovidermock.EntityProviderInterfaceMock
	provider    providers.ActorProvider
}

func TestActorProviderTestSuite(t *testing.T) {
	suite.Run(t, new(ActorProviderTestSuite))
}

func (s *ActorProviderTestSuite) SetupTest() {
	s.mockInbound = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.mockEntity = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.provider = Initialize(s.mockInbound, s.mockEntity)
}

func (s *ActorProviderTestSuite) TestGetOAuthClientByClientID_Delegates() {
	expected := &providers.OAuthClient{ID: "app-1", ClientID: "client-1"}
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "client-1").Return(expected, nil)

	client, svcErr := s.provider.GetOAuthClientByClientID(context.Background(), "client-1")

	s.Nil(svcErr)
	s.Equal(toProviderOAuthClient(expected), client)
}

func (s *ActorProviderTestSuite) TestGetOAuthClientByClientID_NotFound() {
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "missing").
		Return((*providers.OAuthClient)(nil), inboundclient.ErrInboundClientNotFound)

	client, svcErr := s.provider.GetOAuthClientByClientID(context.Background(), "missing")

	s.Nil(client)
	s.Equal(ErrorActorNotFound.Code, svcErr.Code)
}

func (s *ActorProviderTestSuite) TestGetOAuthClientByClientID_FetchFailed() {
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "client-1").
		Return((*providers.OAuthClient)(nil), errors.New("db error"))

	client, svcErr := s.provider.GetOAuthClientByClientID(context.Background(), "client-1")

	s.Nil(client)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ActorProviderTestSuite) TestGetInboundClientByID_NotFound() {
	s.mockInbound.On("GetInboundClientByEntityID", mock.Anything, "missing").
		Return((*inboundmodel.InboundClient)(nil), inboundclient.ErrInboundClientNotFound)

	client, svcErr := s.provider.GetInboundClientByID(context.Background(), "missing")

	s.Nil(client)
	s.Equal(ErrorActorNotFound.Code, svcErr.Code)
}

func (s *ActorProviderTestSuite) TestGetInboundClientByID_FetchFailed() {
	s.mockInbound.On("GetInboundClientByEntityID", mock.Anything, "app-1").
		Return((*inboundmodel.InboundClient)(nil), errors.New("db error"))

	client, svcErr := s.provider.GetInboundClientByID(context.Background(), "app-1")

	s.Nil(client)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ActorProviderTestSuite) TestGetInboundClientByID_Delegates() {
	expected := &inboundmodel.InboundClient{ID: "app-1"}
	s.mockInbound.On("GetInboundClientByEntityID", mock.Anything, "app-1").Return(expected, nil)

	client, svcErr := s.provider.GetInboundClientByID(context.Background(), "app-1")

	s.Nil(svcErr)
	s.Equal(expected, client)
}

func (s *ActorProviderTestSuite) TestGetActor_Delegates() {
	expected := &providers.Entity{ID: "app-1"}
	s.mockEntity.On("GetEntity", "app-1").Return(expected, (*entityprovider.EntityProviderError)(nil))

	entity, err := s.provider.GetActor("app-1")

	s.Nil(err)
	s.Equal(expected, entity)
}

func (s *ActorProviderTestSuite) TestGetActorGroups_Delegates() {
	expected := []providers.EntityGroup{{ID: "group-1"}}
	s.mockEntity.On("GetTransitiveEntityGroups", "app-1").Return(expected, (*entityprovider.EntityProviderError)(nil))

	groups, err := s.provider.GetActorGroups("app-1")

	s.Nil(err)
	s.Equal(expected, groups)
}
