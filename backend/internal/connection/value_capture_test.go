// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/declarativeresourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

const storedHTTPConnectionID = "http-1"

type ValueCaptureTestSuite struct {
	suite.Suite
	svc       *service
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

func TestValueCaptureTestSuite(t *testing.T) {
	suite.Run(t, new(ValueCaptureTestSuite))
}

func (s *ValueCaptureTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), newHTTPConnectionServiceMock(s.T()))
}

func (s *ValueCaptureTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// captureValues gives the service a value releaser and returns every connection handed to it.
func (s *ValueCaptureTestSuite) captureValues() *[]interface{} {
	releaser := declarativeresourcemock.NewValueReleaserMock(s.T())
	captured := &[]interface{}{}
	releaser.EXPECT().CaptureValues(mock.Anything, resourceTypeConnection, mock.Anything).
		Run(func(_ context.Context, _ string, resource interface{}) {
			*captured = append(*captured, resource)
		}).Maybe()
	releaser.EXPECT().ReleaseSecrets(mock.Anything, resourceTypeConnection, mock.Anything).Maybe()
	s.svc.valueReleaser = releaser
	return captured
}

// captured returns the one connection handed to the releaser, in the shape its exporter reads back.
func (s *ValueCaptureTestSuite) captured(resources []interface{}) *connectionExportModel {
	s.Require().Len(resources, 1)
	model, ok := resources[0].(*connectionExportModel)
	s.Require().True(ok, "the capture was not handed the shape the exporter reads back")
	return model
}

func (s *ValueCaptureTestSuite) clientSecret(value string) []cmodels.Property {
	return []cmodels.Property{mustProperty(s.T(), idp.PropClientSecret, value, true)}
}

func (s *ValueCaptureTestSuite) twilioSender(authToken string) *ncommon.NotificationSenderDTO {
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
func (s *ValueCaptureTestSuite) TestCreateCapturesTheConnection() {
	captured := s.captureValues()
	created := &providers.IDPDTO{ID: "g-1", Name: "My Google", Type: providers.IDPTypeGoogle,
		Properties: s.clientSecret("the-secret")}
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return(created, (*tidcommon.ServiceError)(nil))

	_, svcErr := s.svc.create(context.Background(), created)
	s.Require().Nil(svcErr)

	model := s.captured(*captured)
	s.Equal("My Google", model.Name)
	s.Equal("the-secret", model.ClientSecret)
}

// An update that leaves the secret out carries the stored one forward, so it is captured again under
// the connection's current name.
func (s *ValueCaptureTestSuite) TestUpdateCapturesTheStoredSecretUnderTheCurrentName() {
	captured := s.captureValues()
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

	model := s.captured(*captured)
	s.Equal("Renamed", model.Name)
	s.Equal("stored", model.ClientSecret)
}

// A write that fails captures nothing.
func (s *ValueCaptureTestSuite) TestFailedCreateCapturesNothing() {
	captured := s.captureValues()
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPAlreadyExists)

	_, svcErr := s.svc.create(context.Background(), &providers.IDPDTO{Name: "g", Type: providers.IDPTypeGoogle})

	s.Require().NotNil(svcErr)
	s.Empty(*captured)
}

// A message sender is captured on create and on update.
func (s *ValueCaptureTestSuite) TestSMSWritesCaptureTheSender() {
	captured := s.captureValues()
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

	s.Require().Len(*captured, 2)
	s.Equal("the-token", (*captured)[0].(*connectionExportModel).AuthToken)
	s.Equal("rotated", (*captured)[1].(*connectionExportModel).AuthToken)
}

// Without a releaser, which is every plane but a control plane, nothing is captured.
func (s *ValueCaptureTestSuite) TestCaptureWithoutAReleaserDoesNothing() {
	s.NotPanics(func() {
		s.svc.captureIDPValues(context.Background(), &providers.IDPDTO{Type: providers.IDPTypeGoogle})
		s.svc.captureSenderValues(context.Background(), s.twilioSender("t"))
	})
}

// The names a connection's values are captured under are the ones its reference export writes.
func (s *ValueCaptureTestSuite) TestACapturedSecretIsNamedAsTheExportNamesIt() {
	captured := s.captureValues()
	s.svc.captureSenderValues(context.Background(), s.twilioSender("the-token"))
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(s.twilioSender("the-token"), (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "tw-1").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil, nil)
	values := export.Initialize(http.NewServeMux(), []declarativeresource.ResourceExporter{exporter},
		export.ValueReferences)

	variables, secrets, err := values.PlaceholderValues(context.Background(), resourceTypeConnection,
		s.captured(*captured))
	s.Require().NoError(err)
	exported, svcErr := values.ExportResources(context.Background(),
		&export.ExportRequest{Connections: []string{"tw-1"}})
	s.Require().Nil(svcErr)

	s.Empty(variables)
	s.Equal(map[string]string{"CONNECTION_MY_SENDER_AUTH_TOKEN": "the-token"}, secrets)
	s.Require().Len(exported.Files, 1)
	s.Contains(exported.Files[0].Content, "sec:CONNECTION_MY_SENDER_AUTH_TOKEN")
}

// An HTTP connection is captured on create and on update, with each secret named as its export
// names it; an update that keeps the stored password captures it again under the new name.
func (s *ValueCaptureTestSuite) TestHTTPWritesCaptureTheConnectionAsTheExportNamesIt() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	captured := s.captureValues()
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

	s.Require().Len(*captured, 2)
	for i, want := range []map[string]string{
		{"CONNECTION_RISK_ENGINE_PASSWORD": "the-password"},
		{"CONNECTION_FRAUD_ENGINE_PASSWORD": "the-password"},
	} {
		_, secrets, err := values.PlaceholderValues(context.Background(), resourceTypeConnection,
			(*captured)[i])
		s.Require().NoError(err)
		s.Equal(want, secrets)
	}
}

// withHTTPStore returns the secrets handed to ReleaseSecrets, named as the reference export names them.
func (s *ValueCaptureTestSuite) withHTTPStore(senders ...ncommon.NotificationSenderDTO) (*[]map[string]string,
	*httpconnectionmock.HTTPConnectionServiceInterfaceMock, export.ExportServiceInterface) {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	values := export.Initialize(http.NewServeMux(),
		[]declarativeresource.ResourceExporter{newConnectionExporter(s.mockIDP, s.mockNotif, nil, mockHTTP)},
		export.ValueReferences)
	releaser := declarativeresourcemock.NewValueReleaserMock(s.T())
	releaser.EXPECT().CaptureValues(mock.Anything, resourceTypeConnection, mock.Anything).Maybe()
	released := &[]map[string]string{}
	releaser.EXPECT().ReleaseSecrets(mock.Anything, resourceTypeConnection, mock.Anything).
		Run(func(ctx context.Context, resourceType string, resource interface{}) {
			_, secrets, err := values.PlaceholderValues(ctx, resourceType, resource)
			s.Require().NoError(err)
			*released = append(*released, secrets)
		}).Maybe()
	s.svc.valueReleaser = releaser

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
	return released, mockHTTP, values
}

func (s *ValueCaptureTestSuite) createHTTPThroughTheAPI(request httpConnectionRequest) {
	s.T().Helper()
	dto, err := httpToDTO(request)
	s.Require().NoError(err)
	_, svcErr := s.svc.createHTTP(context.Background(), *dto)
	s.Require().Nil(svcErr)
}

func (s *ValueCaptureTestSuite) updateHTTPThroughTheAPI(request httpConnectionRequest) {
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
func (s *ValueCaptureTestSuite) riskEngineSMTP() ncommon.NotificationSenderDTO {
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

func (s *ValueCaptureTestSuite) TestDeletingAnHTTPConnectionReleasesItsSecret() {
	released, _, _ := s.withHTTPStore()
	s.createHTTPThroughTheAPI(riskEngineBasic("old-password"))

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))

	s.Equal([]map[string]string{{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"}}, *released)
}

func (s *ValueCaptureTestSuite) TestAnotherConnectionKindStillRefersToADeletedConnectionsSecret() {
	_, _, values := s.withHTTPStore(s.riskEngineSMTP())
	s.createHTTPThroughTheAPI(riskEngineBasic("old-password"))

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))

	referenced, err := values.ReferencedSecretNames(context.Background(), resourceTypeConnection)
	s.Require().NoError(err)
	s.True(referenced["CONNECTION_RISK_ENGINE_PASSWORD"])
}

func (s *ValueCaptureTestSuite) TestAnHTTPDeleteThatRemovesNothingReleasesNothing() {
	released, mockHTTP, _ := s.withHTTPStore()
	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
	s.createHTTPThroughTheAPI(riskEngineBasic("old-password"))
	mockHTTP.ExpectedCalls = withoutExpectations(mockHTTP.ExpectedCalls, "DeleteHTTPConnection")
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return(&declarativeresource.ErrorDeclarativeResourceDeleteOperation)

	s.Require().NotNil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))

	s.Empty(*released)
}

func (s *ValueCaptureTestSuite) TestAnHTTPDeleteWhoseReadFailsDeletesNothing() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	s.svc.valueReleaser = declarativeresourcemock.NewValueReleaserMock(s.T())
	mockHTTP.On("GetHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return((*httpconnection.HTTPConnection)(nil), &tidcommon.InternalServerError)

	s.Equal(&tidcommon.InternalServerError, s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
	mockHTTP.AssertNotCalled(s.T(), "DeleteHTTPConnection", mock.Anything, mock.Anything)
}

func (s *ValueCaptureTestSuite) TestAnHTTPDeleteWithoutAReleaserReleasesNothing() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, newAuthZENPDPServiceMock(s.T()), mockHTTP)
	mockHTTP.On("GetHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return(&httpconnection.HTTPConnection{ID: storedHTTPConnectionID}, (*tidcommon.ServiceError)(nil))
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, storedHTTPConnectionID).Return((*tidcommon.ServiceError)(nil))

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
}

func (s *ValueCaptureTestSuite) TestAnHTTPUpdateReleasesOnlyTheSecretsItDropped() {
	apiKeys := func(keys map[string]string) httpConnectionRequest {
		return httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
			Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey), Properties: keys}}
	}
	cases := map[string]struct {
		create, update httpConnectionRequest
		released       map[string]string
	}{
		"the method changes": {
			create: riskEngineBasic("old-password"),
			update: httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
				Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBearer),
					Properties: map[string]string{outboundauth.FieldBearerToken: "new-token"}}},
			released: map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"},
		},
		"authentication is removed": {
			create:   riskEngineBasic("old-password"),
			update:   httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL},
			released: map[string]string{"CONNECTION_RISK_ENGINE_PASSWORD": "old-password"},
		},
		"an API-key header is removed": {
			create:   apiKeys(map[string]string{"Primary": "primary", "Secondary": "secondary"}),
			update:   apiKeys(map[string]string{"Primary": "primary"}),
			released: map[string]string{"CONNECTION_RISK_ENGINE_SECONDARY": "secondary"},
		},
		"the password is replaced": {
			create:   riskEngineBasic("old-password"),
			update:   riskEngineBasic("new-password"),
			released: map[string]string{},
		},
		"the connection is renamed and keeps its password": {
			create: riskEngineBasic("old-password"),
			update: httpConnectionRequest{Name: "Fraud Engine", URL: storedHTTPURL,
				Authentication: basicAuthRequest("svc-user", "")},
			released: map[string]string{},
		},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			s.SetupTest()
			released, _, _ := s.withHTTPStore()
			s.createHTTPThroughTheAPI(tc.create)

			s.updateHTTPThroughTheAPI(tc.update)

			s.Equal([]map[string]string{tc.released}, *released)
		})
	}
}

func withoutExpectations(calls []*mock.Call, method string) []*mock.Call {
	kept := make([]*mock.Call, 0, len(calls))
	for _, call := range calls {
		if call.Method != method {
			kept = append(kept, call)
		}
	}
	return kept
}
