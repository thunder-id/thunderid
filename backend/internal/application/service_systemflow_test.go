// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/entity"
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// secretRotationInput is what the flows around a client secret rotation are given for the application.
var secretRotationInput = map[string]string{
	entity.SystemFlowInputEntity:         testServiceAppID,
	entity.SystemFlowInputEntityCategory: string(providers.EntityCategoryApp),
}

// secretUpdateFixture is an application with a confidential OAuth client, about to be updated with a
// client secret, and what the update does recorded in order.
type secretUpdateFixture struct {
	service  *applicationService
	store    *inboundclientmock.InboundClientServiceInterfaceMock
	entities *entitymock.EntityServiceInterfaceMock
	runs     *[]*flowcommon.SystemFlowContext
	update   *model.ApplicationDTO
	order    []string
}

func (suite *ServiceTestSuite) newSecretUpdateFixture(secret string) *secretUpdateFixture {
	return suite.newSecretUpdateFixtureFor(secret, providers.TokenEndpointAuthMethodClientSecretBasic)
}

// newSecretUpdateFixtureFor is newSecretUpdateFixture with the application's client authenticating as
// method says; the update asks for client_secret_basic either way.
func (suite *ServiceTestSuite) newSecretUpdateFixtureFor(secret string,
	method providers.TokenEndpointAuthMethod) *secretUpdateFixture {
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
		JWT:                  engineconfig.JWTConfig{ValidityPeriod: 3600},
	})
	require.NoError(suite.T(), err)
	suite.T().Cleanup(config.ResetServerRuntime)

	service, mockStore := suite.setupTestService()
	f := &secretUpdateFixture{service: service, store: mockStore}
	f.entities = service.entityService.(*entitymock.EntityServiceInterfaceMock)
	f.runs = suite.recordSystemFlows(service, nil, &f.order)

	oauth := &providers.OAuthClient{
		ClientID:                testClientID,
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: method,
	}
	mockLoadFullApplication(mockStore, service, &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{Type: providers.OAuthInboundAuthType, OAuthConfig: oauth},
		},
	})
	f.update = &model.ApplicationDTO{
		ID:   testServiceAppID,
		Name: "Test App",
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:                testClientID,
				ClientSecret:            secret,
				RedirectURIs:            oauth.RedirectURIs,
				GrantTypes:              oauth.GrantTypes,
				ResponseTypes:           oauth.ResponseTypes,
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		}},
	}
	mockStore.On("UpdateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { f.order = append(f.order, "inbound") }).Return(nil).Maybe()
	creds := resetEntityServiceMethod(service, "UpdateSystemCredentials")
	creds.On("UpdateSystemCredentials", mock.Anything, testServiceAppID, mock.Anything).
		Run(func(mock.Arguments) { f.order = append(f.order, "secret") }).Return(nil).Maybe()
	return f
}

// storedSecret says the application already has a client secret, and whether the supplied one is it.
func (f *secretUpdateFixture) storedSecret(matches bool) {
	if matches {
		f.entities.On("AuthenticateEntityByID", mock.Anything, testServiceAppID, mock.Anything).
			Return(&entity.AuthenticateResult{EntityID: testServiceAppID}, "", nil)
		return
	}
	f.entities.On("AuthenticateEntityByID", mock.Anything, testServiceAppID, mock.Anything).
		Return(nil, testServiceAppID, entity.ErrAuthenticationFailed)
}

// An update that carries the secret the application already has, as every apply of a configuration
// version does, revokes nothing and leaves the stored secret alone: writing it again would mark the
// credential as changed and cut off the refresh tokens issued before.
func (suite *ServiceTestSuite) TestUpdateApplication_TheSameSecretRevokesNothing() {
	f := suite.newSecretUpdateFixture("the-same-secret")
	f.storedSecret(true)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Nil(suite.T(), svcErr)
	assert.Empty(suite.T(), *f.runs)
	assert.Equal(suite.T(), []string{"inbound"}, f.order, "the unchanged secret was written again")
}

// A different secret revokes what the old one issued before it is replaced, nothing is written until
// that run passes, and once it is written the post flow, with what the pre flow shared, revokes what the
// old secret issued meanwhile.
func (suite *ServiceTestSuite) TestUpdateApplication_ADifferentSecretRevokesAroundTheWrite() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.storedSecret(false)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), []*flowcommon.SystemFlowContext{
		{Handle: entity.SystemFlowSecretRotatePre, Inputs: secretRotationInput},
		{Handle: systemFlowRevocationPost, Inputs: secretRotationInput,
			RuntimeData: map[string]string{"plan": entity.SystemFlowSecretRotatePre}},
	}, *f.runs)
	assert.Equal(suite.T(), []string{entity.SystemFlowSecretRotatePre, "inbound", "secret",
		systemFlowRevocationPost}, f.order, "something was written before the revocation")
}

// A rotation the system flow refuses writes nothing.
func (suite *ServiceTestSuite) TestUpdateApplication_RefusedWhenTheRotationIsRefused() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.storedSecret(false)
	f.order = nil
	suite.recordSystemFlows(f.service, &ErrorApplicationHasNoClientSecret, &f.order)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Equal(suite.T(), &ErrorApplicationHasNoClientSecret, svcErr)
	assert.Equal(suite.T(), []string{entity.SystemFlowSecretRotatePre}, f.order,
		"something was written after the refusal")
}

// A post flow that fails once the new secret is written is reported, not passed over.
func (suite *ServiceTestSuite) TestUpdateApplication_FailedPostFlowIsReported() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.storedSecret(false)
	flows := newSystemFlowProviderMock(suite.T())
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == entity.SystemFlowSecretRotatePre
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowRevocationPost
	})).Return(nil, &tidcommon.InternalServerError).Once()
	f.service.systemFlows = flows

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
	assert.Equal(suite.T(), []string{"inbound", "secret"}, f.order)
}

// When the supplied secret cannot be compared with the stored one, the update is refused rather than
// guessed at.
func (suite *ServiceTestSuite) TestUpdateApplication_RefusedWhenTheSecretCannotBeCompared() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.entities.On("AuthenticateEntityByID", mock.Anything, testServiceAppID, mock.Anything).
		Return(nil, "", errors.New("store down"))

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
	assert.Empty(suite.T(), f.order, "something was written although the secret could not be compared")
}

// A client that authenticated without a secret is given one as its first, whatever an earlier
// configuration left stored: nothing was issued under a secret, so nothing is revoked, and the
// supplied value is not even compared.
func (suite *ServiceTestSuite) TestUpdateApplication_AClientThatHadNoSecretIsGivenOneWithoutRevoking() {
	f := suite.newSecretUpdateFixtureFor("a-first-secret", providers.TokenEndpointAuthMethodNone)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Nil(suite.T(), svcErr)
	assert.Empty(suite.T(), *f.runs)
	assert.Equal(suite.T(), []string{"inbound", "secret"}, f.order)
	f.entities.AssertNotCalled(suite.T(), "AuthenticateEntityByID", mock.Anything, testServiceAppID, mock.Anything)
}

// newRegenerationFixture returns a service whose application has an OAuth client authenticating as
// method, with the secret write recorded in order. The entity layer is mocked: the system flows it runs
// around the write are tested with the entity service.
func (suite *ServiceTestSuite) newRegenerationFixture(method providers.TokenEndpointAuthMethod) (
	*applicationService, *[]string) {
	service := suite.regeneratingService(appEntityWithClientID("secret-client"), nil, method, nil,
		regenerationNotStored)
	order := &[]string{}
	service.entityService.(*entitymock.EntityServiceInterfaceMock).
		On("UpdateSystemCredentials", mock.Anything, testServiceAppID, mock.Anything).
		Run(func(mock.Arguments) { *order = append(*order, "secret") }).Return(nil).Maybe()
	return service, order
}

// A regeneration is the entity service's, for applications as for agents: the application service hands
// it the application and returns the secret it generated.
func (suite *ServiceTestSuite) TestRegenerateClientSecret_RegeneratesThroughTheEntityService() {
	service, order := suite.newRegenerationFixture(providers.TokenEndpointAuthMethodClientSecretBasic)

	regenerated, svcErr := service.RegenerateClientSecret(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), svcErr)
	suite.Require().NotNil(regenerated)
	assert.Equal(suite.T(), testRegeneratedSecret, regenerated.ClientSecret)
	assert.False(suite.T(), regenerated.RevocationIncomplete)
	assert.Equal(suite.T(), []string{"secret"}, *order)
}

// A revocation that fails once the new secret is written still returns the secret: it is stored by then
// and can never be read again, so withholding it would leave the application with a secret no one holds.
// The failure is audited instead, and the response says the revocation is incomplete.
func (suite *ServiceTestSuite) TestRegenerateClientSecret_FailedRevocationStillReturnsTheSecret() {
	entities := entitymock.NewEntityServiceInterfaceMock(suite.T())
	entities.EXPECT().RegenerateClientSecret(mock.Anything, providers.EntityCategoryApp, testServiceAppID).
		Return(&entity.ClientSecretRegeneration{ClientSecret: testRegeneratedSecret,
			RevocationError: &tidcommon.InternalServerError}, nil).Once()
	bus, published := newRecordingBus(suite.T())
	service := &applicationService{entityService: entities, bus: bus}

	regenerated, svcErr := service.RegenerateClientSecret(adminContext(), testServiceAppID)

	assert.Nil(suite.T(), svcErr)
	suite.Require().NotNil(regenerated)
	assert.Equal(suite.T(), testRegeneratedSecret, regenerated.ClientSecret)
	assert.True(suite.T(), regenerated.RevocationIncomplete, "the caller is not told the revocation is incomplete")
	suite.requireOnlyEvent(published(), audit.EventApplicationUpdate, audit.ActionUpdate, testServiceAppID,
		audit.OutcomeFailed("sse_5000"))
}

// A client that authenticates without a secret has none to regenerate: it is refused before anything is
// written.
func (suite *ServiceTestSuite) TestRegenerateClientSecret_RefusedForAClientWithoutASecret() {
	for _, method := range []providers.TokenEndpointAuthMethod{
		providers.TokenEndpointAuthMethodPrivateKeyJWT, providers.TokenEndpointAuthMethodNone,
	} {
		suite.Run(string(method), func() {
			service, order := suite.newRegenerationFixture(method)

			secret, svcErr := regeneratedSecret(service.RegenerateClientSecret(context.Background(), testServiceAppID))

			assert.Equal(suite.T(), &ErrorApplicationHasNoClientSecret, svcErr)
			assert.Empty(suite.T(), secret)
			assert.Empty(suite.T(), *order)
		})
	}
}

// An application with no OAuth client has no secret to regenerate.
func (suite *ServiceTestSuite) TestValidateClientSecretRegeneration_RefusedForAnApplicationWithoutAnOAuthClient() {
	service, _ := suite.newFlowTargetTestService(&providers.Entity{Category: providers.EntityCategoryApp})

	profile, svcErr := service.ValidateClientSecretRegeneration(context.Background(), testServiceAppID)

	assert.Equal(suite.T(), &ErrorApplicationHasNoClientSecret, svcErr)
	assert.Nil(suite.T(), profile)
}

// An application declared in a file is read only, so its secret is not regenerated.
func (suite *ServiceTestSuite) TestValidateClientSecretRegeneration_RefusedForADeclarativeApplication() {
	service := suite.regeneratingService(appEntityWithClientID("secret-client"), nil,
		providers.TokenEndpointAuthMethodClientSecretBasic, nil, regenerationNotLoaded)
	service.entityService.(*entitymock.EntityServiceInterfaceMock).
		On("IsEntityDeclarative", mock.Anything, testServiceAppID).Return(true, nil).Once()

	profile, svcErr := service.ValidateClientSecretRegeneration(context.Background(), testServiceAppID)

	assert.Equal(suite.T(), &ErrorCannotModifyDeclarativeResource, svcErr)
	assert.Nil(suite.T(), profile)
}

// A caller without control over the application is refused, as for an update.
func (suite *ServiceTestSuite) TestValidateClientSecretRegeneration_RefusedWithoutControlOverTheApplication() {
	service := suite.regeneratingService(appEntityWithClientID("secret-client"), nil,
		providers.TokenEndpointAuthMethodClientSecretBasic, nil, regenerationNotLoaded)
	service.entityService.(*entitymock.EntityServiceInterfaceMock).
		On("IsEntityDeclarative", mock.Anything, testServiceAppID).Return(false, nil).Once()
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
	authz.On("CanControlEntity", mock.Anything, testServiceAppID).Return(&sysauthz.ErrorGrantNotPermitted).Once()
	service.authzService = authz

	profile, svcErr := service.ValidateClientSecretRegeneration(context.Background(), testServiceAppID)

	assert.Equal(suite.T(), &sysauthz.ErrorGrantNotPermitted, svcErr)
	assert.Nil(suite.T(), profile)
}

// The profile names the client whose tokens a regeneration revokes.
func (suite *ServiceTestSuite) TestValidateClientSecretRegeneration_ReturnsTheClient() {
	service := suite.regeneratingService(appEntityWithClientID("secret-client"), nil,
		providers.TokenEndpointAuthMethodClientSecretBasic, nil, regenerationNotStored)

	profile, svcErr := service.ValidateClientSecretRegeneration(context.Background(), testServiceAppID)

	assert.Nil(suite.T(), svcErr)
	suite.Require().NotNil(profile)
	assert.Equal(suite.T(), "secret-client", profile.ClientID)
}
