// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

type SMTPTestSuite struct {
	suite.Suite
	handler   *handler
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

func TestSMTPSuite(t *testing.T) {
	suite.Run(t, new(SMTPTestSuite))
}

func (s *SMTPTestSuite) SetupTest() {
	s.handler, s.mockIDP, s.mockNotif = newConnectionTestHandler(s.T())
}

// basicAuthRequest builds the authentication block of a request. An empty password omits the
// field, which is how an update asks to keep the stored secret.
func basicAuthRequest(username, password string) *connectionAuthentication {
	properties := map[string]string{outboundauth.FieldBasicUsername: username}
	if password != "" {
		properties[outboundauth.FieldBasicPassword] = password
	}
	return &connectionAuthentication{
		Type:       string(outboundauth.TypeBasic),
		Properties: properties,
	}
}

// storedSMTPSender is the sender the notification service would return for an SMTP connection.
func (s *SMTPTestSuite) storedSMTPSender() *ncommon.NotificationSenderDTO {
	return &ncommon.NotificationSenderDTO{
		ID:       "sm-1",
		Name:     "Corp SMTP",
		Type:     ncommon.NotificationSenderTypeEmail,
		Provider: ncommon.NotificationProviderTypeSMTP,
		Properties: []cmodels.Property{
			mustProperty(s.T(), ncommon.SMTPPropKeyHost, "smtp.example.com", false),
			mustProperty(s.T(), ncommon.SMTPPropKeyPort, "587", false),
			mustProperty(s.T(), ncommon.SMTPPropKeyFromAddress, "noreply@example.com", false),
			mustProperty(s.T(), ncommon.SMTPPropKeyTLS, string(ncommon.TLSModeSTARTTLS), false),
			mustProperty(s.T(), outboundauth.PropertyKeyType, string(outboundauth.TypeBasic), false),
			mustProperty(s.T(), outboundauth.PropertyKey(outboundauth.FieldBasicUsername), "mailer", false),
			mustProperty(s.T(), outboundauth.PropertyKey(outboundauth.FieldBasicPassword), "s3cret", true),
		},
	}
}

func (s *SMTPTestSuite) TestToSenderDTOMapsFields() {
	dto, err := emailSMTPToSenderDTO(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Description: "Transactional mail",
		Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com", FromName: "Acme Support",
		TLS: string(ncommon.TLSModeSTARTTLS), Authentication: basicAuthRequest("mailer", "s3cret"),
	})
	s.Require().NoError(err)
	s.Equal(ncommon.NotificationSenderTypeEmail, dto.Type)
	s.Equal(ncommon.NotificationProviderTypeSMTP, dto.Provider)
	s.Equal("Transactional mail", dto.Description)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("smtp.example.com", values[ncommon.SMTPPropKeyHost])
	s.Equal("587", values[ncommon.SMTPPropKeyPort])
	s.Equal("noreply@example.com", values[ncommon.SMTPPropKeyFromAddress])
	s.Equal("Acme Support", values[ncommon.SMTPPropKeyFromName])
	s.Equal(string(ncommon.TLSModeSTARTTLS), values[ncommon.SMTPPropKeyTLS])
	s.Equal(string(outboundauth.TypeBasic), values[outboundauth.PropertyKeyType])
	s.Equal("mailer", values[outboundauth.PropertyKey(outboundauth.FieldBasicUsername)])
	// secret is encrypted at rest and masked on read
	s.Equal(maskedSecretValue, values[outboundauth.PropertyKey(outboundauth.FieldBasicPassword)])
}

// An omitted tls value must resolve to the secure default rather than to plaintext.
func (s *SMTPTestSuite) TestToSenderDTODefaultsTLSToSTARTTLS() {
	dto, err := emailSMTPToSenderDTO(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
	})
	s.Require().NoError(err)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal(string(ncommon.TLSModeSTARTTLS), values[ncommon.SMTPPropKeyTLS])
	// An omitted authentication block stores "none" rather than nothing, so a later update can
	// tell "switched off" from "not supplied".
	s.Equal(string(outboundauth.TypeNone), values[outboundauth.PropertyKeyType])
}

// An unrecognized tls value is stored verbatim so the sender service rejects it with a
// descriptive error, rather than being silently coerced to a valid mode here.
func (s *SMTPTestSuite) TestToSenderDTOPreservesInvalidTLSValue() {
	dto, err := emailSMTPToSenderDTO(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Host: "smtp.example.com", Port: 587,
		FromAddress: "noreply@example.com", TLS: "yes",
	})
	s.Require().NoError(err)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("yes", values[ncommon.SMTPPropKeyTLS])
}

// Port zero means "not supplied": it must be omitted so the sender service reports a missing
// property rather than accepting port 0.
func (s *SMTPTestSuite) TestToSenderDTOOmitsZeroPort() {
	dto, err := emailSMTPToSenderDTO(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Host: "smtp.example.com", FromAddress: "noreply@example.com",
	})
	s.Require().NoError(err)

	for _, prop := range dto.Properties {
		s.NotEqual(ncommon.SMTPPropKeyPort, prop.GetName())
	}
}

// The display name is optional. A blank one must store no property at all, so clearing it on
// update drops the name rather than storing an empty one.
func (s *SMTPTestSuite) TestToSenderDTOOmitsBlankFromName() {
	dto, err := emailSMTPToSenderDTO(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
	})
	s.Require().NoError(err)

	for _, prop := range dto.Properties {
		s.NotEqual(ncommon.SMTPPropKeyFromName, prop.GetName())
	}
}

func (s *SMTPTestSuite) TestFromSenderDTOTypesFields() {
	resp, err := emailSMTPFromSenderDTO(*s.storedSMTPSender())
	s.Require().NoError(err)

	s.Equal("sm-1", resp.ID)
	s.Equal(emailSMTPVendorName, resp.Type)
	s.Equal("smtp.example.com", resp.Host)
	s.Equal(587, resp.Port)
	s.Equal("noreply@example.com", resp.FromAddress)
	s.Equal(string(ncommon.TLSModeSTARTTLS), resp.TLS)
	s.Equal(string(outboundauth.TypeBasic), resp.Authentication.Type)
	s.Equal("mailer", resp.Authentication.Properties[outboundauth.FieldBasicUsername])
	s.Equal(maskedSecretValue, resp.Authentication.Properties[outboundauth.FieldBasicPassword])
}

func (s *SMTPTestSuite) TestCreateMasksSecret() {
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return(s.storedSMTPSender(), (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Host: "smtp.example.com", Port: 587,
		FromAddress: "noreply@example.com", TLS: string(ncommon.TLSModeSTARTTLS),
		Authentication: basicAuthRequest("mailer", "s3cret"),
	})
	req := httptest.NewRequest(http.MethodPost, "/connections/"+emailSMTPVendorName, bytes.NewReader(body))
	rr := httptest.NewRecorder()
	createSenderHandler(s.handler, emailSMTPToSenderDTO, emailSMTPFromSenderDTO)(rr, req)

	s.Equal(http.StatusCreated, rr.Code)
	var resp emailSMTPConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("sm-1", resp.ID)
	s.Equal(emailSMTPVendorName, resp.Type)
	s.Equal(587, resp.Port)
	s.Equal(maskedSecretValue, resp.Authentication.Properties[outboundauth.FieldBasicPassword])
	s.NotContains(rr.Body.String(), "s3cret")
}

func (s *SMTPTestSuite) TestGetRoundTrip() {
	s.mockNotif.On("GetSender", mock.Anything, "sm-1").
		Return(s.storedSMTPSender(), (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/"+emailSMTPVendorName+"/sm-1", nil)
	req.SetPathValue("id", "sm-1")
	rr := httptest.NewRecorder()
	getSenderHandler(s.handler, ncommon.NotificationSenderTypeEmail,
		ncommon.NotificationProviderTypeSMTP, emailSMTPFromSenderDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp emailSMTPConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("Corp SMTP", resp.Name)
	s.Equal("smtp.example.com", resp.Host)
	s.Equal(maskedSecretValue, resp.Authentication.Properties[outboundauth.FieldBasicPassword])
}

// A message sender must not be readable through the email endpoint, even by ID.
func (s *SMTPTestSuite) TestGetSenderTypeMismatchReturnsNotFound() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(&ncommon.NotificationSenderDTO{
			ID:       "tw-1",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeTwilio,
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/"+emailSMTPVendorName+"/tw-1", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	getSenderHandler(s.handler, ncommon.NotificationSenderTypeEmail,
		ncommon.NotificationProviderTypeSMTP, emailSMTPFromSenderDTO)(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
}

// Omitting the password on update keeps the stored one.
func (s *SMTPTestSuite) TestUpdateWithoutPasswordPreservesStoredSecret() {
	s.mockNotif.On("GetSender", mock.Anything, "sm-1").
		Return(s.storedSMTPSender(), (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("UpdateSender", mock.Anything, "sm-1",
		mock.MatchedBy(func(dto ncommon.NotificationSenderDTO) bool {
			for _, prop := range dto.Properties {
				if prop.GetName() != outboundauth.PropertyKey(outboundauth.FieldBasicPassword) {
					continue
				}
				value, err := prop.GetValue()
				return err == nil && value == "s3cret"
			}
			return false
		})).Return(s.storedSMTPSender(), (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Host: "smtp.example.com", Port: 587,
		FromAddress: "noreply@example.com", TLS: string(ncommon.TLSModeSTARTTLS),
		Authentication: basicAuthRequest("mailer", ""),
	})
	req := httptest.NewRequest(http.MethodPut, "/connections/"+emailSMTPVendorName+"/sm-1", bytes.NewReader(body))
	req.SetPathValue("id", "sm-1")
	rr := httptest.NewRecorder()
	updateSenderHandler(s.handler, ncommon.NotificationSenderTypeEmail,
		ncommon.NotificationProviderTypeSMTP, emailSMTPToSenderDTO, emailSMTPFromSenderDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
}

// Changing where or as whom the stored password is presented must require it to be entered
// again. Otherwise a caller who can edit the connection but not read its secrets could point it
// at a server they control and receive the stored password on the next send.
func (s *SMTPTestSuite) TestUpdateChangingCredentialTargetDropsStoredSecret() {
	cases := []struct {
		name     string
		host     string
		port     int
		username string
	}{
		{name: "host changed", host: "smtp.attacker.example", port: 587, username: "mailer"},
		{name: "port changed", host: "smtp.example.com", port: 2525, username: "mailer"},
		{name: "username changed", host: "smtp.example.com", port: 587, username: "other"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.mockNotif.On("GetSender", mock.Anything, "sm-1").
				Return(s.storedSMTPSender(), (*tidcommon.ServiceError)(nil))
			var captured ncommon.NotificationSenderDTO
			s.mockNotif.On("UpdateSender", mock.Anything, "sm-1", mock.Anything).
				Run(func(args mock.Arguments) { captured = args.Get(2).(ncommon.NotificationSenderDTO) }).
				Return(s.storedSMTPSender(), (*tidcommon.ServiceError)(nil))

			body, _ := json.Marshal(emailSMTPConnectionRequest{
				Name: "Corp SMTP", Host: tc.host, Port: tc.port,
				FromAddress: "noreply@example.com", TLS: string(ncommon.TLSModeSTARTTLS),
				Authentication: basicAuthRequest(tc.username, ""),
			})
			req := httptest.NewRequest(http.MethodPut, "/connections/"+emailSMTPVendorName+"/sm-1",
				bytes.NewReader(body))
			req.SetPathValue("id", "sm-1")
			rr := httptest.NewRecorder()
			updateSenderHandler(s.handler, ncommon.NotificationSenderTypeEmail,
				ncommon.NotificationProviderTypeSMTP, emailSMTPToSenderDTO, emailSMTPFromSenderDTO)(rr, req)

			s.Require().NotEmpty(captured.Properties)
			for _, prop := range captured.Properties {
				s.NotEqual(outboundauth.PropertyKey(outboundauth.FieldBasicPassword), prop.GetName(),
					"the stored password must not follow a changed credential target")
			}
		})
	}
}

func (s *SMTPTestSuite) TestDeleteSenderTypeMismatchReturnsNotFound() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(&ncommon.NotificationSenderDTO{
			ID:       "tw-1",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeTwilio,
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodDelete, "/connections/"+emailSMTPVendorName+"/tw-1", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	s.handler.deleteSenderInstance(ncommon.NotificationSenderTypeEmail,
		ncommon.NotificationProviderTypeSMTP)(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
	s.mockNotif.AssertNotCalled(s.T(), "DeleteSender", mock.Anything, "tw-1")
}
