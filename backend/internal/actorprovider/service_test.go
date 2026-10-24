// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

type ActorProviderTestSuite struct {
	suite.Suite
	mockInbound *inboundclientmock.InboundClientServiceInterfaceMock
	mockEntity  *entityprovidermock.EntityProviderInterfaceMock
	mockAuthn   *managermock.AuthnProviderManagerMock
	mockRole    *rolemock.RoleServiceInterfaceMock
	provider    providers.ActorProvider
}

func TestActorProviderTestSuite(t *testing.T) {
	suite.Run(t, new(ActorProviderTestSuite))
}

func (s *ActorProviderTestSuite) SetupTest() {
	s.mockInbound = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.mockEntity = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.mockAuthn = managermock.NewAuthnProviderManagerMock(s.T())
	s.mockRole = rolemock.NewRoleServiceInterfaceMock(s.T())
	s.provider = Initialize(s.mockInbound, s.mockEntity, s.mockAuthn, s.mockRole, nil, nil)
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
		Return((*providers.OAuthClient)(nil), nil)

	client, svcErr := s.provider.GetOAuthClientByClientID(context.Background(), "missing")

	s.Nil(client)
	s.Nil(svcErr)
}

func (s *ActorProviderTestSuite) TestGetOAuthClientByClientID_FetchFailed() {
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "client-1").
		Return((*providers.OAuthClient)(nil), errors.New("db error"))

	client, svcErr := s.provider.GetOAuthClientByClientID(context.Background(), "client-1")

	s.Nil(client)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ActorProviderTestSuite) TestGetOAuthClientByID_Delegates() {
	expected := &providers.OAuthClient{ID: "app-1", ClientID: "client-1"}
	s.mockInbound.On("GetOAuthClientByEntityID", mock.Anything, "app-1").Return(expected, nil)

	client, svcErr := s.provider.GetOAuthClientByID(context.Background(), "app-1")

	s.Nil(svcErr)
	s.Equal(toProviderOAuthClient(expected), client)
}

func (s *ActorProviderTestSuite) TestGetOAuthClientByID_NotFound() {
	s.mockInbound.On("GetOAuthClientByEntityID", mock.Anything, "missing").
		Return((*providers.OAuthClient)(nil), nil)

	client, svcErr := s.provider.GetOAuthClientByID(context.Background(), "missing")

	s.Nil(client)
	s.Nil(svcErr)
}

func (s *ActorProviderTestSuite) TestGetOAuthClientByID_FetchFailed() {
	s.mockInbound.On("GetOAuthClientByEntityID", mock.Anything, "app-1").
		Return((*providers.OAuthClient)(nil), errors.New("db error"))

	client, svcErr := s.provider.GetOAuthClientByID(context.Background(), "app-1")

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

func (s *ActorProviderTestSuite) TestAuthenticateActor_Delegates_Success() {
	identifiers := map[string]interface{}{"userID": "app-1"}
	creds := map[string]interface{}{"flowSecret": "s3cret"}
	s.mockAuthn.On("AuthenticateUser", mock.Anything,
		identifiers, creds,
		(*providers.RequestedAttributes)(nil), (*providers.AuthnMetadata)(nil),
		providers.AuthUser{}).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims(nil),
			(*tidcommon.ServiceError)(nil))

	svcErr := s.provider.AuthenticateActor(context.Background(), identifiers, creds)

	s.Nil(svcErr)
}

func (s *ActorProviderTestSuite) TestAuthenticateActor_Delegates_Failure() {
	identifiers := map[string]interface{}{"userID": "app-1"}
	creds := map[string]interface{}{"flowSecret": "wrong"}
	authFailed := &tidcommon.ServiceError{Code: "AUTH-FAIL", Type: tidcommon.ClientErrorType}
	s.mockAuthn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims(nil), authFailed)

	svcErr := s.provider.AuthenticateActor(context.Background(), identifiers, creds)

	s.NotNil(svcErr)
	s.Equal("AUTH-FAIL", svcErr.Code)
}

func (s *ActorProviderTestSuite) TestGetActor_Delegates() {
	expected := &providers.Entity{ID: "app-1"}
	s.mockEntity.On("GetEntityProfile", "app-1").Return(expected, (*entityprovider.EntityProviderError)(nil))

	entity, err := s.provider.GetActor("app-1")

	s.Nil(err)
	s.Equal(expected, entity)
}

func (s *ActorProviderTestSuite) TestGetActor_AgentDisplayComesFromTheDisplayAttribute() {
	for name, tc := range map[string]struct {
		displayPath string
		want        string
	}{
		"display attribute set":     {displayPath: "name", want: "Ledger Agent"},
		"display attribute removed": {displayPath: "", want: "agent-1"},
	} {
		s.Run(name, func() {
			mockType := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
			mockType.On("GetDisplayAttributesByHandles", mock.Anything, entitytype.TypeCategoryAgent,
				[]string{"default"}).Return(map[string]string{"default": tc.displayPath},
				(*tidcommon.ServiceError)(nil)).Once()
			provider := Initialize(s.mockInbound, s.mockEntity, s.mockAuthn, s.mockRole, mockType, nil)
			s.mockEntity.On("GetEntityProfile", "agent-1").Return(&providers.Entity{
				ID: "agent-1", Category: providers.EntityCategoryAgent, Type: "default",
				Attributes: []byte(`{"name":"Ledger Agent"}`),
			}, (*entityprovider.EntityProviderError)(nil)).Once()

			entity, err := provider.GetActor("agent-1")

			s.Nil(err)
			s.Equal(tc.want, entity.Display)
		})
	}
}

func (s *ActorProviderTestSuite) TestGetActorGroups_Delegates() {
	expected := []providers.EntityGroup{{ID: "group-1"}}
	s.mockEntity.On("GetTransitiveEntityGroups", "app-1").Return(expected, (*entityprovider.EntityProviderError)(nil))

	groups, err := s.provider.GetActorGroups("app-1")

	s.Nil(err)
	s.Equal(expected, groups)
}

func (s *ActorProviderTestSuite) TestGetActorRoles_Delegates() {
	expected := []string{"admin", "editor"}
	groupIDs := []string{"group-1"}
	s.mockRole.On("GetUserRoles", mock.Anything, "app-1", groupIDs).
		Return(expected, (*tidcommon.ServiceError)(nil))

	roles, err := s.provider.GetActorRoles("app-1", groupIDs)

	s.Nil(err)
	s.Equal(expected, roles)
}

func (s *ActorProviderTestSuite) TestGetActorRoles_PropagatesError() {
	svcErr := &tidcommon.ServiceError{
		Code:  "ROLE-0001",
		Error: tidcommon.I18nMessage{Key: "error.test.role", DefaultValue: "role lookup failed"},
	}
	s.mockRole.On("GetUserRoles", mock.Anything, "app-1", []string{"group-1"}).Return(nil, svcErr)

	roles, err := s.provider.GetActorRoles("app-1", []string{"group-1"})

	s.Nil(roles)
	s.Equal(svcErr, err)
}

func (s *ActorProviderTestSuite) TestGetActorRoles_NilRoleService_ReturnsNil() {
	provider := Initialize(s.mockInbound, s.mockEntity, s.mockAuthn, nil, nil, nil)

	roles, err := provider.GetActorRoles("app-1", []string{"group-1"})

	s.Nil(err)
	s.Nil(roles)
}

// --- client rules ---

// stubRulesResolver returns fixed rules or a fixed error, and records the entity it was asked about.
type stubRulesResolver struct {
	rules  *providers.ClientRules
	err    *tidcommon.ServiceError
	called string
}

func (r *stubRulesResolver) GetClientRules(_ context.Context, entityID string) (
	*providers.ClientRules, *tidcommon.ServiceError) {
	r.called = entityID
	return r.rules, r.err
}

// providerWithAgentRules returns an actor provider whose agent clients get their rules from the resolver.
func (s *ActorProviderTestSuite) providerWithAgentRules(resolver ClientRulesResolver) providers.ActorProvider {
	return Initialize(s.mockInbound, s.mockEntity, s.mockAuthn, s.mockRole, nil,
		map[providers.EntityCategory]ClientRulesResolver{providers.EntityCategoryAgent: resolver})
}

// Both client reads add the rules of the client's entity category.
func (s *ActorProviderTestSuite) TestGetOAuthClient_AddsClientRules() {
	rules := providers.ClientRules{RequiredSubjectID: "owner-1", Disabled: true, MaxAccessTokenValidity: 600}
	agentClient := &providers.OAuthClient{ID: "agent-1", ClientID: "agent-client",
		EntityCategory: providers.EntityCategoryAgent}
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "agent-client").Return(agentClient, nil)
	s.mockInbound.On("GetOAuthClientByEntityID", mock.Anything, "agent-1").Return(agentClient, nil)
	resolver := &stubRulesResolver{rules: &rules}
	provider := s.providerWithAgentRules(resolver)

	byClientID, svcErr := provider.GetOAuthClientByClientID(context.Background(), "agent-client")
	s.Require().Nil(svcErr)
	s.Equal(rules, byClientID.Rules)
	s.Equal("agent-1", resolver.called)

	byID, svcErr := provider.GetOAuthClientByID(context.Background(), "agent-1")
	s.Require().Nil(svcErr)
	s.Equal(rules, byID.Rules)
}

// A client of a category without a resolver, or one the resolver gives no rules, has none.
func (s *ActorProviderTestSuite) TestGetOAuthClient_NoClientRules() {
	appClient := &providers.OAuthClient{ID: "app-1", ClientID: "app-client",
		EntityCategory: providers.EntityCategoryApp}
	agentClient := &providers.OAuthClient{ID: "agent-1", ClientID: "agent-client",
		EntityCategory: providers.EntityCategoryAgent}
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "app-client").Return(appClient, nil)
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "agent-client").Return(agentClient, nil)
	resolver := &stubRulesResolver{}
	provider := s.providerWithAgentRules(resolver)

	client, svcErr := provider.GetOAuthClientByClientID(context.Background(), "app-client")
	s.Require().Nil(svcErr)
	s.Equal(providers.ClientRules{}, client.Rules)
	s.Empty(resolver.called, "an application is not looked up")

	client, svcErr = provider.GetOAuthClientByClientID(context.Background(), "agent-client")
	s.Require().Nil(svcErr)
	s.Equal(providers.ClientRules{}, client.Rules)
}

// A failed rules lookup is a server error, so no client is served without rules that may apply to it.
func (s *ActorProviderTestSuite) TestGetOAuthClient_ClientRulesLookupFails() {
	agentClient := &providers.OAuthClient{ID: "agent-1", ClientID: "agent-client",
		EntityCategory: providers.EntityCategoryAgent}
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "agent-client").Return(agentClient, nil)
	provider := s.providerWithAgentRules(&stubRulesResolver{err: &tidcommon.InternalServerError})

	client, svcErr := provider.GetOAuthClientByClientID(context.Background(), "agent-client")

	s.Nil(client)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}
