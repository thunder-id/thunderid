// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"maps"
	"net/http"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
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

func (r *recordingCapturer) ReleaseSecrets(context.Context, string, interface{}) {}

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

// An HTTP connection is captured on create and on update, with each secret named as its export
// names it; an update that keeps the stored password captures it again under the new name.
func (s *ServiceTestSuite) TestHTTPWritesCaptureTheConnectionAsTheExportNamesIt() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	capturer := &recordingCapturer{}
	s.svc.valueCapturer = capturer
	values := export.Initialize(http.NewServeMux(),
		[]declarativeresource.ResourceExporter{newConnectionExporter(s.mockIDP, s.mockNotif, nil, mockHTTP)},
		export.ValueReferences)
	var stored *httpconnection.HTTPConnection
	write := func(connection httpconnection.HTTPConnection) *httpconnection.HTTPConnection {
		connection.ID = storedHTTPConnectionID
		stored = &connection
		return stored
	}
	mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(
		func(_ context.Context, connection httpconnection.HTTPConnection) *httpconnection.HTTPConnection {
			return write(connection)
		}, func(context.Context, httpconnection.HTTPConnection) *tidcommon.ServiceError { return nil })
	mockHTTP.On("GetHTTPConnection", mock.Anything, storedHTTPConnectionID).Return(
		func(context.Context, string) *httpconnection.HTTPConnection { return stored },
		func(context.Context, string) *tidcommon.ServiceError { return nil })
	mockHTTP.On("UpdateHTTPConnection", mock.Anything, storedHTTPConnectionID, mock.Anything).Return(
		func(_ context.Context, _ string, connection httpconnection.HTTPConnection) *httpconnection.HTTPConnection {
			return write(connection)
		}, func(context.Context, string, httpconnection.HTTPConnection) *tidcommon.ServiceError { return nil })

	created, err := httpToDTO(httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
		Authentication: basicAuthRequest("svc-user", "the-password")})
	s.Require().NoError(err)
	_, svcErr := s.svc.createHTTP(context.Background(), *created)
	s.Require().Nil(svcErr)
	renamed, err := httpUpdateToDTO(httpConnectionRequest{Name: "Fraud Engine", URL: storedHTTPURL,
		Authentication: basicAuthRequest("svc-user", "")})
	s.Require().NoError(err)
	_, svcErr = s.svc.updateHTTP(context.Background(), storedHTTPConnectionID, *renamed)
	s.Require().Nil(svcErr)

	s.Require().Len(capturer.resources, 2)
	for i, want := range []map[string]string{
		{"CONNECTION_RISK_ENGINE_PASSWORD": "the-password"},
		{"CONNECTION_FRAUD_ENGINE_PASSWORD": "the-password"},
	} {
		_, secrets, err := values.PlaceholderValues(context.Background(), capturer.resourceTypes[i],
			capturer.resources[i])
		s.Require().NoError(err)
		s.Equal(want, secrets)
	}
}

func (s *ServiceTestSuite) TestInitializeHandsTheHTTPConnectionWriter() {
	_, _, synced, err := Initialize(http.NewServeMux(), nil, s.mockIDP, s.mockNotif, &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), newHTTPConnectionServiceMock(s.T()), nil,
		newSharingServiceMock(s.T()))
	s.Require().NoError(err)
	s.IsType(&httpConnectionWriter{}, synced)
}

// gatewayStore keeps what the default gateway's store holds, by name, working each name out with the
// export service as the gateway's value capture does: a capture sets every value, and a release
// removes each secret of the released resource no stored connection refers to.
type gatewayStore struct {
	values export.ExportServiceInterface
	held   map[string]string
}

func (g *gatewayStore) CaptureValues(ctx context.Context, resourceType string, resource interface{}) {
	variables, secrets, err := g.values.PlaceholderValues(ctx, resourceType, resource)
	if err != nil {
		return
	}
	maps.Copy(g.held, variables)
	maps.Copy(g.held, secrets)
}

func (g *gatewayStore) ReleaseSecrets(ctx context.Context, resourceType string, resource interface{}) {
	_, secrets, err := g.values.PlaceholderValues(ctx, resourceType, resource)
	if err != nil {
		return
	}
	referenced, err := g.values.ReferencedSecretNames(ctx, resourceType)
	if err != nil {
		return
	}
	for name := range secrets {
		if !referenced[name] {
			delete(g.held, name)
		}
	}
}

// withHTTPStore gives the service an HTTP connection service holding the one connection each write
// replaces, the given message senders, and a gateway store, whose values it returns.
func (s *ServiceTestSuite) withHTTPStore(senders ...ncommon.NotificationSenderDTO) (map[string]string,
	*httpconnectionmock.HTTPConnectionServiceInterfaceMock) {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	store := &gatewayStore{held: map[string]string{}, values: export.Initialize(http.NewServeMux(),
		[]declarativeresource.ResourceExporter{newConnectionExporter(s.mockIDP, s.mockNotif, nil, mockHTTP)},
		export.ValueReferences)}
	s.svc.valueCapturer = store

	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{},
		(*tidcommon.ServiceError)(nil)).Maybe()
	s.mockIDP.On("GetIdentityProvider", mock.Anything, mock.Anything).Return((*providers.IDPDTO)(nil),
		&idp.ErrorIDPNotFound).Maybe()
	s.mockNotif.On("ListSenders", mock.Anything).Return(senders, (*tidcommon.ServiceError)(nil)).Maybe()
	for i := range senders {
		s.mockNotif.On("GetSender", mock.Anything, senders[i].ID).Return(&senders[i],
			(*tidcommon.ServiceError)(nil)).Maybe()
	}
	s.mockNotif.On("GetSender", mock.Anything, storedHTTPConnectionID).Return(
		(*ncommon.NotificationSenderDTO)(nil), &notification.ErrorSenderNotFound).Maybe()

	var stored *httpconnection.HTTPConnection
	write := func(connection httpconnection.HTTPConnection) *httpconnection.HTTPConnection {
		connection.ID = storedHTTPConnectionID
		stored = &connection
		return stored
	}
	mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(
		func(_ context.Context, connection httpconnection.HTTPConnection) *httpconnection.HTTPConnection {
			return write(connection)
		}, func(context.Context, httpconnection.HTTPConnection) *tidcommon.ServiceError { return nil }).Maybe()
	mockHTTP.On("UpdateHTTPConnection", mock.Anything, storedHTTPConnectionID, mock.Anything).Return(
		func(_ context.Context, _ string, connection httpconnection.HTTPConnection) *httpconnection.HTTPConnection {
			return write(connection)
		}, func(context.Context, string, httpconnection.HTTPConnection) *tidcommon.ServiceError { return nil }).Maybe()
	mockHTTP.On("GetHTTPConnection", mock.Anything, storedHTTPConnectionID).Return(
		func(context.Context, string) *httpconnection.HTTPConnection { return stored },
		func(context.Context, string) *tidcommon.ServiceError {
			if stored == nil {
				return &httpconnection.ErrorNotFound
			}
			return nil
		}).Maybe()
	mockHTTP.On("ListHTTPConnections", mock.Anything).Return(
		func(context.Context) []httpconnection.HTTPConnection {
			if stored == nil {
				return nil
			}
			return []httpconnection.HTTPConnection{*stored}
		}, func(context.Context) *tidcommon.ServiceError { return nil }).Maybe()
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, storedHTTPConnectionID).Return(
		func(context.Context, string) *tidcommon.ServiceError {
			stored = nil
			return nil
		}).Maybe()
	return store.held, mockHTTP
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
	_, svcErr := s.svc.updateHTTP(context.Background(), storedHTTPConnectionID, *dto)
	s.Require().Nil(svcErr)
}

func riskEngineBasic(password string) httpConnectionRequest {
	return httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
		Authentication: basicAuthRequest("svc-user", password)}
}

// riskEngineSMTP is an SMTP connection with the HTTP connection's name, so its password is named as the
// HTTP connection's is.
func (s *ServiceTestSuite) riskEngineSMTP() ncommon.NotificationSenderDTO {
	return ncommon.NotificationSenderDTO{
		ID: "sm-1", Name: "Risk Engine", Type: ncommon.NotificationSenderTypeEmail,
		Provider: ncommon.NotificationProviderTypeSMTP,
		Properties: []cmodels.Property{
			mustProperty(s.T(), ncommon.SMTPPropKeyHost, "smtp.example.com", false),
			mustProperty(s.T(), ncommon.SMTPPropKeyPort, "587", false),
			mustProperty(s.T(), ncommon.SMTPPropKeyFromAddress, "noreply@example.com", false),
			mustProperty(s.T(), "authentication_type", string(outboundauth.TypeBasic), false),
			mustProperty(s.T(), "authentication_username", "mailer", false),
			mustProperty(s.T(), "authentication_password", "smtp-password", true),
		},
	}
}

// AC2.12: a deleted connection leaves no copy of its secret in the gateway's store.
func (s *ServiceTestSuite) TestDeletingAnHTTPConnectionReleasesItsSecret() {
	held, _ := s.withHTTPStore()
	s.createHTTPThroughTheAPI(riskEngineBasic("old-password"))
	s.Require().Equal(map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"}, held)

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))

	s.Empty(held)
}

// A connection of another type whose secret is named alike still refers to the copy, so it stays.
func (s *ServiceTestSuite) TestDeletingAnHTTPConnectionKeepsASecretAnotherConnectionRefersTo() {
	held, _ := s.withHTTPStore(s.riskEngineSMTP())
	s.createHTTPThroughTheAPI(riskEngineBasic("old-password"))

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))

	s.Equal(map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"}, held)
}

// A delete that is refused, or of a connection that is not there, releases nothing.
func (s *ServiceTestSuite) TestAnHTTPDeleteThatRemovesNothingReleasesNothing() {
	held, mockHTTP := s.withHTTPStore()
	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
	s.createHTTPThroughTheAPI(riskEngineBasic("old-password"))
	mockHTTP.ExpectedCalls = withoutExpectations(mockHTTP.ExpectedCalls, "DeleteHTTPConnection")
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return(&declarativeresource.ErrorDeclarativeResourceDeleteOperation)

	s.Require().NotNil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))

	s.Equal(map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"}, held)
}

// A connection that cannot be read is not deleted, since what it held could not then be released.
func (s *ServiceTestSuite) TestAnHTTPDeleteWhoseReadFailsDeletesNothing() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	s.svc.valueCapturer = &recordingCapturer{}
	mockHTTP.On("GetHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return((*httpconnection.HTTPConnection)(nil), &tidcommon.InternalServerError)

	s.Equal(&tidcommon.InternalServerError, s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
	mockHTTP.AssertNotCalled(s.T(), "DeleteHTTPConnection", mock.Anything, mock.Anything)
}

// Without a capturer, which is every plane but a control plane, a delete still reads the connection first, so a
// delete naming a connection of another kind never reaches the policy cleanup, but releases nothing.
func (s *ServiceTestSuite) TestAnHTTPDeleteWithoutACapturerReleasesNothing() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	mockHTTP.On("GetHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return(&httpconnection.HTTPConnection{ID: storedHTTPConnectionID}, (*tidcommon.ServiceError)(nil))
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, storedHTTPConnectionID).Return((*tidcommon.ServiceError)(nil))

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
}

// AC2.12: an update releases only the secrets it dropped. One the connection still holds, under its
// name or a new one, stays, so a version captured before the update still finds it.
func (s *ServiceTestSuite) TestAnHTTPUpdateReleasesOnlyTheSecretsItDropped() {
	apiKeys := func(keys map[string]string) httpConnectionRequest {
		return httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
			Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey), Properties: keys}}
	}
	cases := map[string]struct {
		create, update httpConnectionRequest
		senders        []ncommon.NotificationSenderDTO
		want           map[string]string
	}{
		"the method changes": {
			create: riskEngineBasic("old-password"),
			update: httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
				Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBearer),
					Properties: map[string]string{outboundauth.FieldBearerToken: "new-token"}}},
			want: map[string]string{"CONNECTION_RISK_ENGINE_TOKEN": "new-token"},
		},
		"authentication is removed": {
			create: riskEngineBasic("old-password"),
			update: httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL},
			want:   map[string]string{},
		},
		"an API-key header is removed": {
			create: apiKeys(map[string]string{"Primary": "primary", "Secondary": "secondary"}),
			update: apiKeys(map[string]string{"Primary": "primary"}),
			want:   map[string]string{"CONNECTION_RISK_ENGINE_PRIMARY": "primary"},
		},
		"the password is replaced": {
			create: riskEngineBasic("old-password"),
			update: riskEngineBasic("new-password"),
			want:   map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "new-password"},
		},
		"the connection is renamed and keeps its password": {
			create: riskEngineBasic("old-password"),
			update: httpConnectionRequest{Name: "Fraud Engine", URL: storedHTTPURL,
				Authentication: basicAuthRequest("svc-user", "")},
			want: map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password",
				"CONNECTION_FRAUD_ENGINE_PASSWORD": "old-password"},
		},
		"another connection refers to the dropped secret": {
			create:  riskEngineBasic("old-password"),
			update:  httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL},
			senders: []ncommon.NotificationSenderDTO{s.riskEngineSMTP()},
			want:    map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"},
		},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			s.SetupTest()
			held, _ := s.withHTTPStore(tc.senders...)
			s.createHTTPThroughTheAPI(tc.create)

			s.updateHTTPThroughTheAPI(tc.update)

			s.Equal(tc.want, held)
		})
	}
}

// withoutExpectations drops the expectations set for a method, so a test can set it again.
func withoutExpectations(calls []*mock.Call, method string) []*mock.Call {
	kept := make([]*mock.Call, 0, len(calls))
	for _, call := range calls {
		if call.Method != method {
			kept = append(kept, call)
		}
	}
	return kept
}
