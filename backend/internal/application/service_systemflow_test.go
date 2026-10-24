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
	"github.com/thunder-id/thunderid/internal/system/systemflow"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
)

// secretUpdateFixture is an application with a confidential OAuth client, about to be updated with a
// client secret, and what the update does recorded in order.
type secretUpdateFixture struct {
	service  *applicationService
	store    *inboundclientmock.InboundClientServiceInterfaceMock
	entities *entitymock.EntityServiceInterfaceMock
	flows    *recordingSystemFlows
	update   *model.ApplicationDTO
	order    []string
}

func (suite *ServiceTestSuite) newSecretUpdateFixture(secret string) *secretUpdateFixture {
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
	f.flows = &recordingSystemFlows{onRun: func() { f.order = append(f.order, "flow") }}
	service.SetSystemFlowRunner(f.flows)

	oauth := &providers.OAuthClient{
		ClientID:                testClientID,
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
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
				TokenEndpointAuthMethod: oauth.TokenEndpointAuthMethod,
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
	resetEntityServiceMethod(f.service, "GetCredentialsByType").
		On("GetCredentialsByType", mock.Anything, testServiceAppID, fieldClientSecret).
		Return([]entity.StoredCredential{{Value: "hash"}}, nil)
	if matches {
		f.entities.On("AuthenticateEntityByID", mock.Anything, testServiceAppID, mock.Anything).
			Return(&entity.AuthenticateResult{EntityID: testServiceAppID}, nil)
		return
	}
	f.entities.On("AuthenticateEntityByID", mock.Anything, testServiceAppID, mock.Anything).
		Return(nil, entity.ErrAuthenticationFailed)
}

// An update that carries the secret the application already has, as every apply of a configuration
// version does, revokes nothing and leaves the stored secret alone: writing it again would mark the
// credential as changed and cut off the refresh tokens issued before.
func (suite *ServiceTestSuite) TestUpdateApplication_TheSameSecretRevokesNothing() {
	f := suite.newSecretUpdateFixture("the-same-secret")
	f.storedSecret(true)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Nil(suite.T(), svcErr)
	assert.Empty(suite.T(), f.flows.runs, "an unchanged secret ran a system flow")
	assert.Equal(suite.T(), []string{"inbound"}, f.order, "the unchanged secret was written again")
}

// A different secret revokes what the old one issued before it is replaced, and nothing is written
// until that run passes.
func (suite *ServiceTestSuite) TestUpdateApplication_ADifferentSecretRevokesBeforeTheWrite() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.storedSecret(false)

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Nil(suite.T(), svcErr)
	assert.Equal(suite.T(), []systemflow.Operation{systemflow.ApplicationSecretRotate}, f.flows.runs)
	assert.Equal(suite.T(), map[string]string{systemflow.TargetApplication: testServiceAppID}, f.flows.targets[0])
	assert.Equal(suite.T(), []string{"flow", "inbound", "secret"}, f.order,
		"something was written before the revocation")
}

// A rotation the system flow refuses writes nothing.
func (suite *ServiceTestSuite) TestUpdateApplication_RefusedWhenTheRotationIsRefused() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	f.storedSecret(false)
	f.flows.err = &ErrorApplicationHasNoClientSecret

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Equal(suite.T(), &ErrorApplicationHasNoClientSecret, svcErr)
	assert.Equal(suite.T(), []string{"flow"}, f.order, "something was written after the refusal")
}

// An application that had no secret gets one without a revocation: nothing was issued under one.
func (suite *ServiceTestSuite) TestUpdateApplication_TheFirstSecretIsWrittenWithoutRevoking() {
	f := suite.newSecretUpdateFixture("a-first-secret")

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Nil(suite.T(), svcErr)
	assert.Empty(suite.T(), f.flows.runs)
	assert.Equal(suite.T(), []string{"inbound", "secret"}, f.order)
}

// When the stored secret cannot be read or compared, the update is refused rather than guessed at.
func (suite *ServiceTestSuite) TestUpdateApplication_RefusedWhenTheSecretCannotBeCompared() {
	f := suite.newSecretUpdateFixture("a-new-secret")
	resetEntityServiceMethod(f.service, "GetCredentialsByType").
		On("GetCredentialsByType", mock.Anything, testServiceAppID, fieldClientSecret).
		Return([]entity.StoredCredential{{Value: "hash"}}, nil)
	f.entities.On("AuthenticateEntityByID", mock.Anything, testServiceAppID, mock.Anything).
		Return(nil, errors.New("store down"))

	_, svcErr := f.service.UpdateApplication(context.Background(), testServiceAppID, f.update)

	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
	assert.Empty(suite.T(), f.order, "something was written although the secret could not be compared")
}

// A regenerated secret is always new, so the regeneration revokes what the old one issued before the
// write.
func (suite *ServiceTestSuite) TestApplyCredentialAction_RevokesBeforeTheWrite() {
	service, mockStore := suite.newFlowTargetTestService(appEntityWithClientID(testClientID))
	mockStore.On("GetOAuthClientByClientID", mock.Anything, testClientID).Return(&providers.OAuthClient{
		ClientID: testClientID, TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
	}, nil)
	var order []string
	flows := &recordingSystemFlows{onRun: func() { order = append(order, "flow") }}
	service.SetSystemFlowRunner(flows)
	entities := service.entityService.(*entitymock.EntityServiceInterfaceMock)
	entities.On("UpdateSystemCredentials", mock.Anything, testServiceAppID, mock.Anything).
		Run(func(mock.Arguments) { order = append(order, "secret") }).Return(nil)

	secret, svcErr := service.ApplyCredentialAction(context.Background(), testServiceAppID,
		model.CredentialActionRegenerate)

	assert.Nil(suite.T(), svcErr)
	assert.NotEmpty(suite.T(), secret)
	assert.Equal(suite.T(), []systemflow.Operation{systemflow.ApplicationSecretRotate}, flows.runs)
	assert.Equal(suite.T(), []string{"flow", "secret"}, order, "something was written before the revocation")
}

// A regeneration whose revocation is refused keeps the current secret.
func (suite *ServiceTestSuite) TestApplyCredentialAction_RefusedWhenTheRevocationIsRefused() {
	service, mockStore := suite.newFlowTargetTestService(appEntityWithClientID(testClientID))
	mockStore.On("GetOAuthClientByClientID", mock.Anything, testClientID).Return(&providers.OAuthClient{
		ClientID: testClientID, TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
	}, nil)
	service.SetSystemFlowRunner(&recordingSystemFlows{err: &tidcommon.InternalServerError})

	secret, svcErr := service.ApplyCredentialAction(context.Background(), testServiceAppID,
		model.CredentialActionRegenerate)

	assert.Empty(suite.T(), secret)
	assert.Equal(suite.T(), &tidcommon.InternalServerError, svcErr)
	service.entityService.(*entitymock.EntityServiceInterfaceMock).
		AssertNotCalled(suite.T(), "UpdateSystemCredentials", mock.Anything, mock.Anything, mock.Anything)
}
