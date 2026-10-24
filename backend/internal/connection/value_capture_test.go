// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"maps"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/declarativeresourcemock"
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

// captured returns the one connection handed to the capturer, in the shape its exporter reads back.
func (s *ServiceTestSuite) captured(r *recordingCapturer) *connectionExportModel {
	s.Require().Len(r.resources, 1)
	s.Equal(resourceTypeConnection, r.resourceTypes[0])
	model, ok := r.resources[0].(*connectionExportModel)
	s.Require().True(ok, "the capture was not handed the shape the exporter reads back")
	return model
}

func (s *ServiceTestSuite) twilioSender(authToken string) *ncommon.NotificationSenderDTO {
	return &ncommon.NotificationSenderDTO{
		ID: "tw-1", Name: "My Sender", Type: ncommon.NotificationSenderTypeMessage,
		Provider: ncommon.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			mustProperty(s.T(), ncommon.TwilioPropKeyAccountSID, "AC123", false),
			mustProperty(s.T(), ncommon.TwilioPropKeyAuthToken, authToken, true),
		},
	}
}

// A created identity-provider connection is captured with its client secret in the clear.
func (s *ServiceTestSuite) TestCreateCapturesTheConnection() {
	capturer := &recordingCapturer{}
	s.svc.valueCapturer = capturer
	created := &providers.IDPDTO{ID: "g-1", Name: "My Google", Type: providers.IDPTypeGoogle,
		Properties: s.clientSecret("the-secret")}
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return(created, (*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.create(context.Background(), created)
	s.Require().Nil(svcErr)

	model := s.captured(capturer)
	s.Equal("My Google", model.Name)
	s.Equal("the-secret", model.ClientSecret)
}

// An update that leaves the secret out carries the stored one forward, so it is captured again under
// the connection's current name.
func (s *ServiceTestSuite) TestUpdateCapturesTheStoredSecretUnderTheCurrentName() {
	capturer := &recordingCapturer{}
	s.svc.valueCapturer = capturer
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle, Properties: s.clientSecret("stored")},
			(*tidcommon.ServiceError)(nil))
	s.mockIDP.On("UpdateIdentityProvider", mock.Anything, "g-1", mock.Anything).
		Return(func(_ context.Context, _ string, dto *providers.IDPDTO) *providers.IDPDTO {
			dto.ID = "g-1"
			return dto
		}, func(context.Context, string, *providers.IDPDTO) *tidcommon.ServiceError { return nil })

	_, svcErr := s.svc.update(context.Background(), providers.IDPTypeGoogle, "g-1",
		&providers.IDPDTO{Name: "Renamed", Type: providers.IDPTypeGoogle})
	s.Require().Nil(svcErr)

	model := s.captured(capturer)
	s.Equal("Renamed", model.Name)
	s.Equal("stored", model.ClientSecret)
}

// A write that fails captures nothing.
func (s *ServiceTestSuite) TestFailedCreateCapturesNothing() {
	capturer := &recordingCapturer{}
	s.svc.valueCapturer = capturer
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPAlreadyExists)

	_, svcErr := s.svc.create(context.Background(), &providers.IDPDTO{Name: "g", Type: providers.IDPTypeGoogle})

	s.Require().NotNil(svcErr)
	s.Empty(capturer.resources)
}

// A message sender is captured on create and on update.
func (s *ServiceTestSuite) TestSMSWritesCaptureTheSender() {
	capturer := &recordingCapturer{}
	s.svc.valueCapturer = capturer
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return(s.twilioSender("the-token"), (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(s.twilioSender("the-token"), (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("UpdateSender", mock.Anything, "tw-1", mock.Anything).
		Return(s.twilioSender("rotated"), (*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.createSender(context.Background(), *s.twilioSender("the-token"))
	s.Require().Nil(svcErr)
	_, svcErr = s.svc.updateSender(context.Background(), ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeTwilio, "tw-1", *s.twilioSender("rotated"))
	s.Require().Nil(svcErr)

	s.Require().Len(capturer.resources, 2)
	s.Equal("the-token", capturer.resources[0].(*connectionExportModel).AuthToken)
	s.Equal("rotated", capturer.resources[1].(*connectionExportModel).AuthToken)
}

// Without a capturer, which is every plane but a control plane, nothing is captured.
func (s *ServiceTestSuite) TestCaptureWithoutACapturerDoesNothing() {
	s.NotPanics(func() {
		s.svc.captureIDPValues(context.Background(), &providers.IDPDTO{Type: providers.IDPTypeGoogle})
		s.svc.captureSenderValues(context.Background(), s.twilioSender("t"))
	})
}

// The names a connection's values are captured under are the ones its reference export writes.
func (s *ServiceTestSuite) TestACapturedSecretIsNamedAsTheExportNamesIt() {
	capturer := &recordingCapturer{}
	s.svc.valueCapturer = capturer
	s.svc.captureSenderValues(context.Background(), s.twilioSender("the-token"))
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(s.twilioSender("the-token"), (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "tw-1").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil, nil)
	values := export.Initialize(http.NewServeMux(), []declarativeresource.ResourceExporter{exporter},
		export.ValueReferences)

	variables, secrets, err := values.PlaceholderValues(context.Background(), resourceTypeConnection,
		s.captured(capturer))
	s.Require().NoError(err)
	exported, svcErr := values.ExportResources(context.Background(),
		&export.ExportRequest{Connections: []string{"tw-1"}})
	s.Require().Nil(svcErr)

	s.Empty(variables)
	s.Equal(map[string]string{"CONNECTION_MY_SENDER_AUTH_TOKEN": "the-token"}, secrets)
	s.Require().Len(exported.Files, 1)
	s.Contains(exported.Files[0].Content, "sec:CONNECTION_MY_SENDER_AUTH_TOKEN")
}

const storedHTTPConnectionID = "http-1"

// mockGatewayStore returns a value releaser that keeps held as gateway.ValueCapture keeps the default
// gateway's store, naming each value as values does.
func mockGatewayStore(t *testing.T, values export.ExportServiceInterface,
	held map[string]string) *declarativeresourcemock.ValueReleaserMock {
	store := declarativeresourcemock.NewValueReleaserMock(t)
	store.EXPECT().CaptureValues(mock.Anything, mock.Anything, mock.Anything).Run(
		func(ctx context.Context, resourceType string, resource interface{}) {
			variables, secrets, err := values.PlaceholderValues(ctx, resourceType, resource)
			if err != nil {
				return
			}
			maps.Copy(held, variables)
			maps.Copy(held, secrets)
		}).Maybe()
	store.EXPECT().ReleaseValues(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(
		func(ctx context.Context, resourceType string, previous, current interface{}) {
			variables, secrets, err := values.OwnedNames(ctx, resourceType, previous)
			if err != nil {
				return
			}
			keptVariables, keptSecrets := map[string]bool{}, map[string]bool{}
			if current != nil {
				if v, sc, err := values.ReferencedNames(ctx, resourceType, current); err == nil {
					keptVariables, keptSecrets = v, sc
				}
			}
			for name := range variables {
				if !keptVariables[name] {
					delete(held, name)
				}
			}
			for name := range secrets {
				if !keptSecrets[name] {
					delete(held, name)
				}
			}
		}).Maybe()
	return store
}

// withHTTPStore gives the service an HTTP connection service holding one connection that each write
// replaces, and a capturer that keeps what the default gateway's store would, returned as held.
func (s *ServiceTestSuite) withHTTPStore() (map[string]string,
	*httpconnectionmock.HTTPConnectionServiceInterfaceMock) {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, &authZENPDPServiceStub{}, mockHTTP)
	held := map[string]string{}
	s.svc.valueCapturer = mockGatewayStore(s.T(), export.Initialize(http.NewServeMux(),
		[]declarativeresource.ResourceExporter{newConnectionExporter(s.mockIDP, s.mockNotif, nil, mockHTTP)},
		export.ValueReferences), held)

	var stored *httpconnection.HTTPConnection
	write := func(_ context.Context, connection httpconnection.HTTPConnection) *httpconnection.HTTPConnection {
		connection.ID = storedHTTPConnectionID
		stored = &connection
		return stored
	}
	mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(write,
		func(context.Context, httpconnection.HTTPConnection) *tidcommon.ServiceError { return nil }).Maybe()
	mockHTTP.On("UpdateHTTPConnection", mock.Anything, "http-1", mock.Anything).Return(
		func(ctx context.Context, _ string, connection httpconnection.HTTPConnection) *httpconnection.HTTPConnection {
			return write(ctx, connection)
		}, func(context.Context, string, httpconnection.HTTPConnection) *tidcommon.ServiceError { return nil }).Maybe()
	mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").Return(
		func(context.Context, string) *httpconnection.HTTPConnection { return stored },
		func(context.Context, string) *tidcommon.ServiceError { return nil }).Maybe()
	return held, mockHTTP
}

// createHTTPThroughTheAPI creates the connection as the create route maps it.
func (s *ServiceTestSuite) createHTTPThroughTheAPI(request httpConnectionRequest) {
	s.T().Helper()
	dto, err := httpToDTO(request)
	s.Require().NoError(err)
	_, svcErr := s.svc.createHTTP(context.Background(), *dto)
	s.Require().Nil(svcErr)
}

// updateHTTPThroughTheAPI updates the connection as the update route maps it.
func (s *ServiceTestSuite) updateHTTPThroughTheAPI(request httpConnectionRequest) {
	s.T().Helper()
	dto, err := httpUpdateToDTO(request)
	s.Require().NoError(err)
	_, svcErr := s.svc.updateHTTP(context.Background(), "http-1", *dto)
	s.Require().Nil(svcErr)
}

func riskEngineBasic() httpConnectionRequest {
	return httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{
				outboundauth.FieldBasicUsername: "svc-user", outboundauth.FieldBasicPassword: "old-password",
			}}}
}

func (s *ServiceTestSuite) TestDeletingAnHTTPConnectionReleasesItsSecret() {
	held, mockHTTP := s.withHTTPStore()
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, "http-1").Return((*tidcommon.ServiceError)(nil))
	s.createHTTPThroughTheAPI(riskEngineBasic())
	s.Require().Equal(map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"}, held)

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), "http-1"))

	s.Empty(held)
}

func (s *ServiceTestSuite) TestARefusedHTTPDeleteReleasesNothing() {
	held, mockHTTP := s.withHTTPStore()
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, "http-1").
		Return(&declarativeresource.ErrorDeclarativeResourceDeleteOperation)
	s.createHTTPThroughTheAPI(riskEngineBasic())

	s.Require().NotNil(s.svc.deleteHTTP(context.Background(), "http-1"))

	s.Equal(map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"}, held)
}

func (s *ServiceTestSuite) TestAnHTTPUpdateThatDropsTheSecretReleasesItsCopy() {
	cases := map[string]struct {
		create, update httpConnectionRequest
		want           map[string]string
	}{
		"the method changes": {
			create: riskEngineBasic(),
			update: httpConnectionRequest{Name: "Risk Engine", URL: "https://collector.example.net/evaluate",
				Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBearer),
					Properties: map[string]string{outboundauth.FieldBearerToken: "new-token"}}},
			want: map[string]string{"CONNECTION_RISK_ENGINE_TOKEN": "new-token"},
		},
		"authentication is removed": {
			create: riskEngineBasic(),
			update: httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL},
			want:   map[string]string{},
		},
		"the origin changes with a new password": {
			create: riskEngineBasic(),
			update: httpConnectionRequest{Name: "Risk Engine", URL: "https://collector.example.net/evaluate",
				Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBasic),
					Properties: map[string]string{
						outboundauth.FieldBasicUsername: "svc-user", outboundauth.FieldBasicPassword: "new-password",
					}}},
			want: map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "new-password"},
		},
		"an API-key header is removed": {
			create: httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
				Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey),
					Properties: map[string]string{"X-Primary-Key": "primary", "X-Secondary-Key": "secondary"}}},
			update: httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
				Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey),
					Properties: map[string]string{"X-Primary-Key": maskedSecretValue}}},
			want: map[string]string{"CONNECTION_RISK_ENGINE_X_PRIMARY_KEY": "primary"},
		},
		"the connection is renamed and keeps its password": {
			create: riskEngineBasic(),
			update: httpConnectionRequest{Name: "Fraud Engine", URL: storedHTTPURL,
				Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBasic),
					Properties: map[string]string{outboundauth.FieldBasicUsername: "svc-user"}}},
			want: map[string]string{"CONNECTION_FRAUD_ENGINE_PASSWORD": "old-password"},
		},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			held, _ := s.withHTTPStore()
			s.createHTTPThroughTheAPI(tc.create)

			s.updateHTTPThroughTheAPI(tc.update)

			s.Equal(tc.want, held)
		})
	}
}

func (s *ServiceTestSuite) TestReleaseWithoutAReleaserDoesNothing() {
	capturer := &recordingCapturer{}
	s.svc.valueCapturer = capturer
	connection := &httpconnection.HTTPConnection{ID: "http-1", Name: "Risk", URL: storedHTTPURL,
		AuthProperties: s.storedBasic()}

	s.NotPanics(func() { s.svc.releaseHTTPValues(context.Background(), connection, nil) })
	s.Empty(capturer.resources)
}

func (s *ServiceTestSuite) TestAnHTTPUpdateReferringToTheCopyKeepsIt() {
	held, _ := s.withHTTPStore()
	s.createHTTPThroughTheAPI(riskEngineBasic())

	s.updateHTTPThroughTheAPI(httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{
				outboundauth.FieldBasicUsername: "svc-user",
				outboundauth.FieldBasicPassword: "sec:CONNECTION_RISK_ENGINE_PASSWORD",
			}}})

	s.Equal(map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"}, held)
}

func (s *ServiceTestSuite) TestInitializeHandsTheHTTPConnectionWriter() {
	_, synced, err := Initialize(http.NewServeMux(), nil, s.mockIDP, s.mockNotif, &testResourceServerLister{},
		&authZENPDPServiceStub{}, newHTTPConnectionServiceMock(s.T()), nil)
	s.Require().NoError(err)
	s.IsType(&httpConnectionWriter{}, synced)
}

func (s *ServiceTestSuite) TestDeletingAnHTTPConnectionThatReferredToItsOwnCopyReleasesIt() {
	held, mockHTTP := s.withHTTPStore()
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, "http-1").Return((*tidcommon.ServiceError)(nil))
	s.createHTTPThroughTheAPI(riskEngineBasic())
	s.updateHTTPThroughTheAPI(httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{
				outboundauth.FieldBasicUsername: "svc-user",
				outboundauth.FieldBasicPassword: "sec:CONNECTION_RISK_ENGINE_PASSWORD",
			}}})
	s.Require().Equal(map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"}, held)

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), "http-1"))

	s.Empty(held)
}

func (s *ServiceTestSuite) TestAnHTTPDeleteWithoutAReleaserDoesNotReadTheConnection() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, &authZENPDPServiceStub{}, mockHTTP)
	s.svc.valueCapturer = &recordingCapturer{}
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, "http-1").Return((*tidcommon.ServiceError)(nil))

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), "http-1"))

	mockHTTP.AssertNotCalled(s.T(), "GetHTTPConnection", mock.Anything, mock.Anything)
}
