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

// The steps the entity service's rotation records around the write it is handed.
const (
	rotationBefore = "revoke-before"
	rotationAfter  = "revoke-after"
)

// recordSecretRotation has the entity service's rotation record its revocation before and after the write
// it is handed, in order. A refusal stops it before the write; revocationErr is what the revocation after
// the write reports.
func recordSecretRotation(entities *entitymock.EntityServiceInterfaceMock, refusal,
	revocationErr *tidcommon.ServiceError, order *[]string) {
	entities.EXPECT().RotateClientSecret(mock.Anything, testServiceAppID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string, write func() *tidcommon.ServiceError) (*tidcommon.ServiceError,
			*tidcommon.ServiceError) {
			*order = append(*order, rotationBefore)
			if refusal != nil {
				return refusal, nil
			}
			if svcErr := write(); svcErr != nil {
				return svcErr, nil
			}
			*order = append(*order, rotationAfter)
			return nil, revocationErr
		}).Maybe()
}

// secretUpdateFixture is an application with a confidential OAuth client, about to be updated with a
// client secret, and what the update does recorded in order.
type secretUpdateFixture struct {
	service  *applicationService
	store    *inboundclientmock.InboundClientServiceInterfaceMock
	entities *entitymock.EntityServiceInterfaceMock
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
	assert.Equal(suite.T(), []string{"inbound"}, f.order, "the unchanged secret was written again")
}

// A different secret has the entity service revoke what the old one issued around the whole change, so the
// revocation starts before the client is reconfigured and ends once the secret is written.
func (suite *ServiceTestSuite) TestUpdateApplication_ADifferentSecretRevokesAroundTheWrite() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.storedSecret(false)
	recordSecretRotation(f.entities, nil, nil, &f.order)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), []string{rotationBefore, "inbound", "secret", rotationAfter}, f.order,
		"something was written outside the revocation")
}

// A rotation the entity service refuses writes nothing.
func (suite *ServiceTestSuite) TestUpdateApplication_RefusedWhenTheRotationIsRefused() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.storedSecret(false)
	recordSecretRotation(f.entities, &tidcommon.InternalServerError, nil, &f.order)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
	assert.Equal(suite.T(), []string{rotationBefore}, f.order, "something was written after the refusal")
}

// The caller chose the secret, so a revocation that fails once it is written is reported as the update's
// failure.
func (suite *ServiceTestSuite) TestUpdateApplication_FailedRevocationAfterTheWriteIsReported() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.storedSecret(false)
	recordSecretRotation(f.entities, nil, &tidcommon.InternalServerError, &f.order)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
	assert.Equal(suite.T(), []string{rotationBefore, "inbound", "secret", rotationAfter}, f.order)
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

// The response says whether the entity service revoked what the old secret issued.
func (suite *ServiceTestSuite) TestRegenerateClientSecret_ReportsTheRevocation() {
	for _, revoked := range []bool{true, false} {
		entities := entitymock.NewEntityServiceInterfaceMock(suite.T())
		entities.EXPECT().RegenerateClientSecret(mock.Anything, providers.EntityCategoryApp, testServiceAppID).
			Return(&entity.ClientSecretRegeneration{ClientSecret: testRegeneratedSecret, Revoked: revoked}, nil).Once()
		bus, _ := newRecordingBus(suite.T())
		service := &applicationService{entityService: entities, bus: bus}

		regenerated, svcErr := service.RegenerateClientSecret(adminContext(), testServiceAppID)

		suite.Require().Nil(svcErr)
		suite.Equal(revoked, regenerated.Revoked)
		suite.False(regenerated.RevocationIncomplete)
	}
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

	svcErr := service.ValidateClientSecretRegeneration(context.Background(), testServiceAppID)

	assert.Equal(suite.T(), &ErrorApplicationHasNoClientSecret, svcErr)
}

// An application declared in a file is read only, so its secret is not regenerated.
func (suite *ServiceTestSuite) TestValidateClientSecretRegeneration_RefusedForADeclarativeApplication() {
	service := suite.regeneratingService(appEntityWithClientID("secret-client"), nil,
		providers.TokenEndpointAuthMethodClientSecretBasic, nil, regenerationNotLoaded)
	service.entityService.(*entitymock.EntityServiceInterfaceMock).
		On("IsEntityDeclarative", mock.Anything, testServiceAppID).Return(true, nil).Once()

	svcErr := service.ValidateClientSecretRegeneration(context.Background(), testServiceAppID)

	assert.Equal(suite.T(), &ErrorCannotModifyDeclarativeResource, svcErr)
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

	svcErr := service.ValidateClientSecretRegeneration(context.Background(), testServiceAppID)

	assert.Equal(suite.T(), &sysauthz.ErrorGrantNotPermitted, svcErr)
}

// A client that authenticates with a secret may have it regenerated.
func (suite *ServiceTestSuite) TestValidateClientSecretRegeneration_AllowsAClientWithASecret() {
	service := suite.regeneratingService(appEntityWithClientID("secret-client"), nil,
		providers.TokenEndpointAuthMethodClientSecretBasic, nil, regenerationNotStored)

	assert.Nil(suite.T(), service.ValidateClientSecretRegeneration(context.Background(), testServiceAppID))
}
