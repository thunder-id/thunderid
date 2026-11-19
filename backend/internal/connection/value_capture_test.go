// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"net/http"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/varname"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/declarativeresourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

// recordingCapturer records every resource handed to it.
type recordingCapturer struct {
	resourceTypes []string
	resources     []interface{}
}

func (r *recordingCapturer) ReleaseSecrets(context.Context, string, interface{}) {}

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
	s.svc.valueReleaser = capturer
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
	s.svc.valueReleaser = capturer
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
	s.svc.valueReleaser = capturer
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPAlreadyExists)

	_, svcErr := s.svc.create(context.Background(), &providers.IDPDTO{Name: "g", Type: providers.IDPTypeGoogle})

	s.Require().NotNil(svcErr)
	s.Empty(capturer.resources)
}

// A message sender is captured on create and on update.
func (s *ServiceTestSuite) TestSMSWritesCaptureTheSender() {
	capturer := &recordingCapturer{}
	s.svc.valueReleaser = capturer
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
	s.svc.valueReleaser = capturer
	s.svc.captureSenderValues(context.Background(), s.twilioSender("the-token"))
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(s.twilioSender("the-token"), (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "tw-1").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil, nil)
	values, initErr := export.Initialize(http.NewServeMux(), []declarativeresource.ResourceExporter{exporter},
		export.ValueReferences, eventbusmock.NewEventBusInterfaceMock(s.T()))
	s.Require().NoError(initErr)

	variables, secrets, err := values.PlaceholderValues(context.Background(), resourceTypeConnection,
		s.captured(capturer))
	s.Require().NoError(err)
	exported, svcErr := values.ReadResources(context.Background(),
		&export.ExportRequest{Connections: []string{"tw-1"}})
	s.Require().Nil(svcErr)

	s.Empty(variables)
	authTokenVar := varname.DeriveVariableName("connection", "tw-1", ncommon.TwilioPropKeyAuthToken)
	s.Equal(map[string]string{authTokenVar: "the-token"}, secrets)
	s.Require().Len(exported.Files, 1)
	s.Contains(exported.Files[0].Content, "sec:"+authTokenVar)
}

const storedHTTPConnectionID = "http-1"

// An HTTP connection is captured on create and on update, with each secret named as its export
// names it; an update that keeps the stored password captures it again, under the same name a rename
// leaves unchanged.
func (s *ServiceTestSuite) TestHTTPWritesCaptureTheConnectionAsTheExportNamesIt() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = withAllowAllAuthz(s.T(), newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), mockHTTP, newDiscardingBus(s.T())))
	capturer := &recordingCapturer{}
	s.svc.valueReleaser = capturer
	values, initErr := export.Initialize(http.NewServeMux(),
		[]declarativeresource.ResourceExporter{newConnectionExporter(s.mockIDP, s.mockNotif, nil, mockHTTP)},
		export.ValueReferences, eventbusmock.NewEventBusInterfaceMock(s.T()))
	s.Require().NoError(initErr)
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
	want := map[string]string{varname.DeriveVariableName("connection", storedHTTPConnectionID,
		outboundauth.FieldBasicPassword): "the-password"}
	for i := range capturer.resources {
		_, secrets, err := values.PlaceholderValues(context.Background(), capturer.resourceTypes[i],
			capturer.resources[i])
		s.Require().NoError(err)
		s.Equal(want, secrets)
	}
}

func (s *ServiceTestSuite) TestInitializeHandsTheHTTPConnectionWriter() {
	_, _, synced, err := Initialize(http.NewServeMux(), nil, s.mockIDP, s.mockNotif, &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), newHTTPConnectionServiceMock(s.T()), nil,
		newSharingServiceMock(s.T()), allowAllAuthz(s.T()), newDiscardingBus(s.T()))
	s.Require().NoError(err)
	s.IsType(&httpConnectionWriter{}, synced)
}

// withHTTPStore returns the secrets handed to ReleaseSecrets, named as the reference export names them.
func (s *ServiceTestSuite) withHTTPStore() (*[]map[string]string,
	*httpconnectionmock.HTTPConnectionServiceInterfaceMock) {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = withAllowAllAuthz(s.T(), newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), mockHTTP, newDiscardingBus(s.T())))
	values, initErr := export.Initialize(http.NewServeMux(),
		[]declarativeresource.ResourceExporter{newConnectionExporter(s.mockIDP, s.mockNotif, nil, mockHTTP)},
		export.ValueReferences, eventbusmock.NewEventBusInterfaceMock(s.T()))
	s.Require().NoError(initErr)
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
	return released, mockHTTP
}

func (s *ServiceTestSuite) createHTTPThroughTheAPI(request httpConnectionRequest) {
	s.T().Helper()
	dto, err := httpToDTO(request)
	s.Require().NoError(err)
	_, svcErr := s.svc.createHTTP(context.Background(), *dto)
	s.Require().Nil(svcErr)
}

func (s *ServiceTestSuite) updateHTTPThroughTheAPI(request httpConnectionRequest) {
	s.T().Helper()
	dto, err := httpUpdateToDTO(request)
	s.Require().NoError(err)
	_, svcErr := s.svc.updateHTTP(context.Background(), storedHTTPConnectionID, *dto)
	s.Require().Nil(svcErr)
}

// httpSecretName is how the export names a secret of the stored HTTP connection.
func httpSecretName(field string) string {
	return varname.DeriveVariableName("connection", storedHTTPConnectionID, field)
}

func riskEngineBasic(password string) httpConnectionRequest {
	return httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL,
		Authentication: basicAuthRequest("svc-user", password)}
}

func (s *ServiceTestSuite) TestDeletingAnHTTPConnectionReleasesItsSecret() {
	released, _ := s.withHTTPStore()
	s.createHTTPThroughTheAPI(riskEngineBasic("old-password"))

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))

	s.Equal([]map[string]string{{httpSecretName(outboundauth.FieldBasicPassword): "old-password"}}, *released)
}

func (s *ServiceTestSuite) TestAnHTTPDeleteThatRemovesNothingReleasesNothing() {
	released, mockHTTP := s.withHTTPStore()
	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
	s.createHTTPThroughTheAPI(riskEngineBasic("old-password"))
	mockHTTP.ExpectedCalls = withoutExpectations(mockHTTP.ExpectedCalls, "DeleteHTTPConnection")
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return(&declarativeresource.ErrorDeclarativeResourceDeleteOperation)

	s.Require().NotNil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))

	s.Empty(*released)
}

func (s *ServiceTestSuite) TestAnHTTPDeleteWhoseReadFailsDeletesNothing() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = withAllowAllAuthz(s.T(), newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), mockHTTP, newDiscardingBus(s.T())))
	s.svc.valueReleaser = declarativeresourcemock.NewValueReleaserMock(s.T())
	mockHTTP.On("GetHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return((*httpconnection.HTTPConnection)(nil), &tidcommon.InternalServerError)

	s.Equal(&tidcommon.InternalServerError, s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
	mockHTTP.AssertNotCalled(s.T(), "DeleteHTTPConnection", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAnHTTPDeleteWithoutAReleaserReleasesNothing() {
	mockHTTP := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.svc = withAllowAllAuthz(s.T(), newService(s.mockIDP, s.mockNotif, &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), mockHTTP, newDiscardingBus(s.T())))
	mockHTTP.On("GetHTTPConnection", mock.Anything, storedHTTPConnectionID).
		Return(&httpconnection.HTTPConnection{ID: storedHTTPConnectionID}, (*tidcommon.ServiceError)(nil))
	mockHTTP.On("DeleteHTTPConnection", mock.Anything, storedHTTPConnectionID).Return((*tidcommon.ServiceError)(nil))

	s.Require().Nil(s.svc.deleteHTTP(context.Background(), storedHTTPConnectionID))
}

func (s *ServiceTestSuite) TestAnHTTPUpdateReleasesOnlyTheSecretsItDropped() {
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
			released: map[string]string{httpSecretName(outboundauth.FieldBasicPassword): "old-password"},
		},
		"authentication is removed": {
			create:   riskEngineBasic("old-password"),
			update:   httpConnectionRequest{Name: "Risk Engine", URL: storedHTTPURL},
			released: map[string]string{httpSecretName(outboundauth.FieldBasicPassword): "old-password"},
		},
		"an API-key header is removed": {
			create:   apiKeys(map[string]string{"Primary": "primary", "Secondary": "secondary"}),
			update:   apiKeys(map[string]string{"Primary": "primary"}),
			released: map[string]string{httpSecretName("Secondary"): "secondary"},
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
			released, _ := s.withHTTPStore()
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
