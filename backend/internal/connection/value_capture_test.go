// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"net/http"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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

	_, svcErr := s.svc.createSMS(context.Background(), *s.twilioSender("the-token"))
	s.Require().Nil(svcErr)
	_, svcErr = s.svc.updateSMS(context.Background(), ncommon.NotificationProviderTypeTwilio, "tw-1",
		*s.twilioSender("rotated"))
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
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil)
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
