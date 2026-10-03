// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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

// captured returns the one application handed to the capturer.
func (r *recordingCapturer) captured(suite *ServiceTestSuite) *providers.Application {
	require.Len(suite.T(), r.resources, 1)
	assert.Equal(suite.T(), resourceTypeApplication, r.resourceTypes[0])
	app, ok := r.resources[0].(*providers.Application)
	require.True(suite.T(), ok, "the capture was not handed the shape the exporter reads back")
	return app
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

// A regenerated secret is captured on the application it belongs to, which is read for its name
// since the regeneration returns only the secret.
func (suite *ServiceTestSuite) TestApplyCredentialAction_CapturesTheRegeneratedSecret() {
	service, mockStore := suite.setupTestService()
	capturer := &recordingCapturer{}
	service.valueCapturer = capturer
	mockStore.On("IsDeclarative", mock.Anything, testServiceAppID).Maybe().Return(false)
	mockLoadFullApplication(mockStore, service, &model.ApplicationProcessedDTO{
		ID:   testServiceAppID,
		Name: "My App",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthClient{
				ClientID:                testClientID,
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		}},
	})
	mockStore.On("GetOAuthClientByClientID", mock.Anything, testClientID).Return(
		&providers.OAuthClient{TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic}, nil)
	mockStore.EXPECT().GetCertificate(mock.Anything, cert.CertificateReferenceTypeOAuthApp, testClientID).
		Return(nil, nil)

	secret, svcErr := service.ApplyCredentialAction(context.Background(), testServiceAppID,
		model.CredentialActionRegenerate)
	require.Nil(suite.T(), svcErr)

	app := capturer.captured(suite)
	assert.Equal(suite.T(), "My App", app.Name)
	assert.Equal(suite.T(), testClientID, app.InboundAuthConfig[0].OAuthConfig.ClientID)
	assert.Equal(suite.T(), secret, app.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// A regenerated secret whose application cannot be read is not captured, and the regeneration
// still succeeds.
func (suite *ServiceTestSuite) TestCaptureRegeneratedSecret_UnreadableApplicationCapturesNothing() {
	service, mockStore := suite.setupTestService()
	capturer := &recordingCapturer{}
	service.valueCapturer = capturer
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return((*inboundmodel.InboundClient)(nil), nil)

	service.captureRegeneratedSecret(context.Background(), testServiceAppID, "the-secret")

	assert.Empty(suite.T(), capturer.resources)
}

// Without a capturer, which is every plane but a control plane, nothing is captured or read.
func (suite *ServiceTestSuite) TestCaptureWithoutACapturerDoesNothing() {
	service, _ := suite.setupTestService()

	assert.NotPanics(suite.T(), func() {
		service.captureValues(context.Background(), confidentialClientDTO("My App"))
		service.captureRegeneratedSecret(context.Background(), testServiceAppID, "the-secret")
	})
}
