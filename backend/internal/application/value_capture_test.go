// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
)

// recordingCapturer records every resource handed to it.
type recordingCapturer struct {
	resourceTypes []string
	resources     []interface{}
}

func (r *recordingCapturer) CaptureValues(_ context.Context, resourceType string, resource interface{}) {
	r.resourceTypes = append(r.resourceTypes, resourceType)
	r.resources = append(r.resources, resource)
}

// captured returns the one application handed to the capturer, in either shape the exporter reads.
func (r *recordingCapturer) captured(suite *ServiceTestSuite) *providers.Application {
	require.Len(suite.T(), r.resources, 1)
	assert.Equal(suite.T(), resourceTypeApplication, r.resourceTypes[0])
	app, ok := applicationOf(r.resources[0])
	require.True(suite.T(), ok, "the capture was not handed the shape the exporter reads back")
	return app
}

// capturedDocument returns the one document handed to the capturer.
func (r *recordingCapturer) capturedDocument(suite *ServiceTestSuite) *applicationDeclarativeResource {
	require.Len(suite.T(), r.resources, 1)
	assert.Equal(suite.T(), resourceTypeApplication, r.resourceTypes[0])
	doc, ok := r.resources[0].(*applicationDeclarativeResource)
	require.True(suite.T(), ok, "the capture was not handed the shape the exporter reads back")
	return doc
}

// embeddedAppDTO is an application with no OAuth configuration, which holds a Flow Secret.
func embeddedAppDTO(name string) *model.ApplicationDTO {
	return &model.ApplicationDTO{Type: model.ApplicationTypeCustom, Name: name, OUID: testOUID}
}

// mockLoadForUpdate makes stored the application an update replaces.
func mockLoadForUpdate(service *applicationService,
	mockStore *inboundclientmock.InboundClientServiceInterfaceMock, stored *model.ApplicationProcessedDTO) {
	mockStore.On("IsDeclarative", mock.Anything, stored.ID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, stored)
	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
}

func confidentialClientDTO(name string) *model.ApplicationDTO {
	return &model.ApplicationDTO{
		Type: model.ApplicationTypeM2M,
		Name: name,
		OUID: testOUID,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:                testClientID,
				GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		}},
	}
}

// A created application is captured with the client secret generated for it, which only the create
// still holds.
func (suite *ServiceTestSuite) TestCreateApplication_CapturesTheGeneratedSecret() {
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test",
		&config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}))
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()
	capturer := &recordingCapturer{}
	service.valueCapturer = capturer
	mockStore.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, svcErr := service.CreateApplication(context.Background(), confidentialClientDTO("My App"))
	require.Nil(suite.T(), svcErr)

	app := capturer.captured(suite)
	assert.Equal(suite.T(), result.ID, app.ID)
	assert.Equal(suite.T(), "My App", app.Name)
	require.Len(suite.T(), app.InboundAuthConfig, 1)
	assert.Equal(suite.T(), testClientID, app.InboundAuthConfig[0].OAuthConfig.ClientID)
	secret := app.InboundAuthConfig[0].OAuthConfig.ClientSecret
	assert.NotEmpty(suite.T(), secret)
	assert.Equal(suite.T(), result.InboundAuthConfig[0].OAuthConfig.ClientSecret, secret)
}

// A create that fails captures nothing.
func (suite *ServiceTestSuite) TestCreateApplication_FailureCapturesNothing() {
	service, _ := suite.setupTestService()
	capturer := &recordingCapturer{}
	service.valueCapturer = capturer

	_, svcErr := service.CreateApplication(context.Background(), nil)

	require.NotNil(suite.T(), svcErr)
	assert.Empty(suite.T(), capturer.resources)
}

// An updated application is captured as the update returns it.
func (suite *ServiceTestSuite) TestUpdateApplication_CapturesTheUpdatedApplication() {
	config.ResetServerRuntime()
	require.NoError(suite.T(), config.InitializeServerRuntime("/tmp/test",
		&config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}))
	defer config.ResetServerRuntime()

	service, mockStore := suite.setupTestService()
	capturer := &recordingCapturer{}
	service.valueCapturer = capturer
	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "My App",
		Type: model.ApplicationTypeM2M,
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthClient{
				ClientID:                testClientID,
				GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		}},
	})
	mockStore.On("UpdateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	update := confidentialClientDTO("My App Renamed")
	update.ID = testServiceAppID

	_, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, update)
	require.Nil(suite.T(), svcErr)

	app := capturer.captured(suite)
	assert.Equal(suite.T(), testServiceAppID, app.ID)
	assert.Equal(suite.T(), "My App Renamed", app.Name)
	assert.Equal(suite.T(), testClientID, app.InboundAuthConfig[0].OAuthConfig.ClientID)
}

// A created application is captured with the Flow Secret issued to it, which only the create still
// holds, so the gateway is given the value the export refers to. One that cannot hold a Flow Secret is
// captured without one.
func (suite *ServiceTestSuite) TestCreateApplication_CapturesTheIssuedFlowSecret() {
	cases := []struct {
		name           string
		app            *model.ApplicationDTO
		wantFlowSecret bool
	}{
		{name: "an application that holds one", app: embeddedAppDTO("My App"), wantFlowSecret: true},
		{name: "an application that cannot hold one", app: confidentialClientDTO("My App")},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			config.ResetServerRuntime()
			suite.Require().NoError(config.InitializeServerRuntime("/tmp/test",
				&config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}))
			defer config.ResetServerRuntime()

			service, mockStore := suite.setupTestService()
			capturer := &recordingCapturer{}
			service.valueCapturer = capturer
			mockStore.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			result, svcErr := service.CreateApplication(context.Background(), tc.app)
			suite.Require().Nil(svcErr)

			captured := capturer.capturedDocument(suite).FlowSecret
			suite.Equal(result.FlowSecret, captured)
			suite.Equal(tc.wantFlowSecret, captured != "")
		})
	}
}

// An update captures the Flow Secret it sets, as a regenerate does, so the gateway is given the new
// value. One that leaves the Flow Secret alone captures none, so the gateway keeps the value it holds,
// and a Flow Secret supplied for an application that cannot hold one is neither stored nor captured.
func (suite *ServiceTestSuite) TestUpdateApplication_CapturesTheFlowSecretItSets() {
	embedded := &model.ApplicationProcessedDTO{
		ID: testServiceAppID, Name: "My App", Type: model.ApplicationTypeCustom, OUID: testOUID,
	}
	confidential := &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "My App",
		Type: model.ApplicationTypeM2M,
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthClient{
				ClientID:                testClientID,
				GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		}},
	}
	cases := []struct {
		name           string
		stored         *model.ApplicationProcessedDTO
		update         *model.ApplicationDTO
		flowSecret     string
		wantFlowSecret string
	}{
		{name: "an update that sets one", stored: embedded, update: embeddedAppDTO("My App"),
			flowSecret: "the-new-flow-secret", wantFlowSecret: "the-new-flow-secret"},
		{name: "an update that leaves it alone", stored: embedded, update: embeddedAppDTO("My App")},
		{name: "an application that cannot hold one", stored: confidential,
			update: confidentialClientDTO("My App"), flowSecret: "ignored-flow-secret"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			config.ResetServerRuntime()
			suite.Require().NoError(config.InitializeServerRuntime("/tmp/test",
				&config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: false}}))
			defer config.ResetServerRuntime()

			service, mockStore := suite.setupTestService()
			capturer := &recordingCapturer{}
			service.valueCapturer = capturer
			mockLoadForUpdate(service, mockStore, tc.stored)
			tc.update.ID = testServiceAppID
			tc.update.FlowSecret = tc.flowSecret

			_, svcErr := service.UpdateApplication(context.Background(), testServiceAppID, tc.update)
			suite.Require().Nil(svcErr)

			suite.Equal(tc.wantFlowSecret, capturer.capturedDocument(suite).FlowSecret)
		})
	}
}

// Without a capturer, which is every plane but a control plane, nothing is captured or read.
func (suite *ServiceTestSuite) TestCaptureWithoutACapturerDoesNothing() {
	service, _ := suite.setupTestService()

	assert.NotPanics(suite.T(), func() {
		service.captureValues(context.Background(), confidentialClientDTO("My App"), "")
	})
}

// A regenerated secret is captured with the application, as an update that sets one is, so a
// configuration version carries the new secret: only the regeneration still holds it.
func (suite *ServiceTestSuite) TestRegenerateClientSecret_CapturesTheNewSecret() {
	service, _ := suite.newRegenerationFixture(providers.TokenEndpointAuthMethodClientSecretBasic)
	capturer := &recordingCapturer{}
	service.valueCapturer = capturer
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(suite.T())
	governance.EXPECT().ReadEntityAccess(mock.Anything, mock.Anything).
		Return(identitygovernance.AccessRead{}).Maybe()
	service.governance = governance
	mockLoadFullApplication(service.inboundClientService.(*inboundclientmock.InboundClientServiceInterfaceMock),
		service, &model.ApplicationProcessedDTO{
			ID:   testServiceAppID,
			Name: "My App",
			Type: model.ApplicationTypeM2M,
			InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					ClientID:                testClientID,
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			}},
		})

	secret, svcErr := regeneratedSecret(service.RegenerateClientSecret(context.Background(), testServiceAppID))
	require.Nil(suite.T(), svcErr)

	app := capturer.captured(suite)
	assert.Equal(suite.T(), testServiceAppID, app.ID)
	assert.Equal(suite.T(), "My App", app.Name)
	require.Len(suite.T(), app.InboundAuthConfig, 1)
	assert.Equal(suite.T(), testClientID, app.InboundAuthConfig[0].OAuthConfig.ClientID)
	assert.Equal(suite.T(), secret, app.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// A refused regeneration writes no secret, so it captures none.
func (suite *ServiceTestSuite) TestRegenerateClientSecret_RefusalCapturesNothing() {
	service, _ := suite.newRegenerationFixture(providers.TokenEndpointAuthMethodPrivateKeyJWT)
	capturer := &recordingCapturer{}
	service.valueCapturer = capturer

	_, svcErr := regeneratedSecret(service.RegenerateClientSecret(context.Background(), testServiceAppID))

	require.NotNil(suite.T(), svcErr)
	assert.Empty(suite.T(), capturer.resources)
}
