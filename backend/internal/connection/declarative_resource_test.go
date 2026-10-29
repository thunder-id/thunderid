// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/authzenpdpmock"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"

	"gopkg.in/yaml.v3"
)

type DeclarativeResourceTestSuite struct {
	suite.Suite
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
	exporter  *connectionExporter
}

func TestDeclarativeResourceSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.T().Cleanup(config.ResetServerRuntime)
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.exporter = NewConnectionExporterForTest(s.mockIDP, s.mockNotif, nil, nil)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceType() {
	s.Equal("connection", s.exporter.GetResourceType())
	s.Equal("Connection", s.exporter.GetParameterizerType())
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelFromIDPDTOUnmasksSecret() {
	dto := providers.IDPDTO{
		ID: "google-1", Name: "My Google", Type: providers.IDPTypeGoogle,
		Properties: []cmodels.Property{
			mustProperty(s.T(), idp.PropClientID, "client-123", false),
			mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
			mustProperty(s.T(), idp.PropRedirectURI, "https://app/cb", false),
		},
	}
	model, err := connectionModelFromIDPDTO(dto)
	s.Require().NoError(err)
	s.Equal("google", model.Type)
	s.Equal("google-1", model.ID)
	s.Equal("client-123", model.ClientID)
	s.Equal("s3cret", model.ClientSecret, "export model must carry the plaintext secret, not the mask")
	s.Equal("https://app/cb", model.RedirectURI)
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelFromIDPDTORejectsUnregisteredType() {
	_, err := connectionModelFromIDPDTO(providers.IDPDTO{ID: "x", Type: providers.IDPType("SAML")})
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelFromSenderDTOUnmasksSecret() {
	dto := ncommon.NotificationSenderDTO{
		ID: "tw-1", Name: "My Twilio", Type: ncommon.NotificationSenderTypeMessage,
		Provider: ncommon.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			mustProperty(s.T(), ncommon.TwilioPropKeyAccountSID, "AC00000000000000000000000000000000", false),
			mustProperty(s.T(), ncommon.TwilioPropKeyAuthToken, "tok", true),
			mustProperty(s.T(), ncommon.TwilioPropKeySenderID, "+15005550006", false),
		},
	}
	model, err := connectionModelFromSenderDTO(dto)
	s.Require().NoError(err)
	s.Equal("twilio", model.Type)
	s.Equal("tok", model.AuthToken)
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelFromSenderDTOSMSGateway() {
	dto := ncommon.NotificationSenderDTO{
		ID: "sg-1", Name: "Gateway", Type: ncommon.NotificationSenderTypeMessage,
		Provider: ncommon.NotificationProviderTypeCustom,
		Properties: []cmodels.Property{
			mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
		},
	}
	model, err := connectionModelFromSenderDTO(dto)
	s.Require().NoError(err)
	s.Equal(smsGatewayVendorName, model.Type)
	s.Equal("https://sms.example.com/send", model.URL)
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelFromSenderDTOLegacySMSGatewayHeaders() {
	cases := []struct {
		name   string
		value  string
		secret bool
	}{
		{"plain headers", "X-API-Key: legacy-secret, X-Tenant: tenant-1", false},
		{"encrypted headers", "X-API-Key: legacy-secret, X-Tenant: tenant-1", true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			dto := ncommon.NotificationSenderDTO{
				ID: "sg-1", Name: "Legacy SMS", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderTypeCustom,
				Properties: []cmodels.Property{
					mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
					mustProperty(s.T(), ncommon.CustomPropKeyHTTPHeaders, tc.value, tc.secret),
				},
			}

			model, err := connectionModelFromSenderDTO(dto)
			s.Require().NoError(err)
			s.Require().NotNil(model.Authentication)
			s.Equal(string(outboundauth.TypeNone), model.Authentication.Type)
			s.Equal(tc.value, model.HTTPHeaders)
			s.Empty(s.exporter.GetResourceRulesForResource(&model).SecretVariables)

			_, imported, err := connectionModelToDTO(model)
			s.Require().NoError(err)
			s.Require().NotNil(imported)
			roundTripped, err := connectionModelFromSenderDTO(*imported)
			s.Require().NoError(err)
			s.Equal(model.Authentication, roundTripped.Authentication)
			s.Equal(model.HTTPHeaders, roundTripped.HTTPHeaders)
		})
	}
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelToDTORoundTripsEachVendor() {
	cases := []struct {
		name     string
		model    connectionExportModel
		wantIDP  bool
		wantType string
	}{
		{
			"google",
			connectionExportModel{
				ID: "1", Type: "google", Name: "n", ClientID: "c", ClientSecret: "s", RedirectURI: "r",
			},
			true, "GOOGLE",
		},
		{
			"github",
			connectionExportModel{
				ID: "2", Type: "github", Name: "n", ClientID: "c", ClientSecret: "s", RedirectURI: "r",
			},
			true, "GITHUB",
		},
		{
			"oidc",
			connectionExportModel{
				ID: "3", Type: "oidc", Name: "n", ClientID: "c", ClientSecret: "s", RedirectURI: "r",
				AuthorizationEndpoint: "a", TokenEndpoint: "t",
			},
			true, "OIDC",
		},
		{
			"oauth",
			connectionExportModel{
				ID: "4", Type: "oauth", Name: "n", ClientID: "c", ClientSecret: "s", RedirectURI: "r",
				AuthorizationEndpoint: "a", TokenEndpoint: "t", UserInfoEndpoint: "u",
			},
			true, "OAUTH",
		},
	}
	for _, tc := range cases {
		idpDTO, senderDTO, err := connectionModelToDTO(tc.model)
		s.Require().NoError(err, tc.name)
		s.Require().NotNil(idpDTO, tc.name)
		s.Nil(senderDTO, tc.name)
		s.Equal(tc.model.ID, idpDTO.ID, tc.name)
		s.Equal(providers.IDPType(tc.wantType), idpDTO.Type, tc.name)
	}
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelToDTORoundTripsSMSVendors() {
	cases := []struct {
		name         string
		model        connectionExportModel
		wantProvider ncommon.NotificationProviderType
	}{
		{
			"twilio",
			connectionExportModel{
				ID: "s1", Type: "twilio", Name: "n", AccountSID: "AC00000000000000000000000000000000",
				AuthToken: "t", SenderID: "+1",
			},
			ncommon.NotificationProviderTypeTwilio,
		},
		{
			"vonage",
			connectionExportModel{
				ID: "s2", Type: "vonage", Name: "n", APIKey: "k", APISecret: "s", SenderID: "ThunderID",
			},
			ncommon.NotificationProviderTypeVonage,
		},
		{
			"sms-gateway",
			connectionExportModel{ID: "s3", Type: smsGatewayVendorName, Name: "n", URL: "https://x/send"},
			ncommon.NotificationProviderTypeCustom,
		},
	}
	for _, tc := range cases {
		idpDTO, senderDTO, err := connectionModelToDTO(tc.model)
		s.Require().NoError(err, tc.name)
		s.Nil(idpDTO, tc.name)
		s.Require().NotNil(senderDTO, tc.name)
		s.Equal(tc.model.ID, senderDTO.ID, tc.name)
		s.Equal(tc.wantProvider, senderDTO.Provider, tc.name)
	}
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelToDTOUnsupportedVendor() {
	_, _, err := connectionModelToDTO(connectionExportModel{Type: "unknown-vendor"})
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestAuthZENPDPConnectionExportModelRoundTrip() {
	doc := []byte(`
id: pdp-1
type: authzen-pdp
name: Production PDP
endpoint: http://localhost:3592/access/v1/evaluation
batchEndpoint: http://localhost:3592/access/v1/evaluations
authentication:
  type: api_key
  properties:
    X-API-Key: secret
subjectAttributeMappings:
  - subjectCategory: user
    entityType: TravelCustomer
    attributes:
      - attribute: accountStatus
        pdpAttribute: account_status
`)

	dto, err := parseToConnectionDTOWrapper(doc)
	s.Require().NoError(err)
	pdp, ok := dto.(*authzenpdp.AuthZENPDPConnection)
	s.Require().True(ok)
	s.Equal("pdp-1", pdp.ID)
	s.Equal("http://localhost:3592/access/v1/evaluation", pdp.Endpoint)
	s.Equal("http://localhost:3592/access/v1/evaluations", pdp.BatchEndpoint)
	exported, err := connectionModelFromAuthZENPDP(*pdp)
	s.Require().NoError(err)
	s.Equal("authzen-pdp", exported.Type)
	s.Require().NotNil(exported.Authentication)
	s.Equal(string(outboundauth.TypeAPIKey), exported.Authentication.Type)
	s.Equal(map[string]string{"X-Api-Key": "secret"}, exported.Authentication.Properties)
	s.Equal("pdp-1", connectionResourceID(pdp))

	roundTripped, err := connectionModelToAuthZENPDP(exported)
	s.Require().NoError(err)
	s.Require().NotNil(roundTripped)
	authenticationConfig, err := outboundauth.FromProperties(roundTripped.AuthenticationProperties)
	s.Require().NoError(err)
	s.Equal(map[string]string{"X-Api-Key": "secret"}, authenticationConfig.Properties)
}

func (s *DeclarativeResourceTestSuite) TestAuthZENPDPDeclarativeImportRejectsMaskedCredential() {
	_, err := connectionModelToAuthZENPDP(connectionExportModel{
		Type: authZENPDPVendorName,
		Authentication: &outboundauth.Authentication{
			Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "******"},
		},
	})
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestParseConnectionFromNodeIDPVendor() {
	doc := `
id: corp-google
type: google
name: Corp Google
clientId: abc
clientSecret: shh
redirectUri: https://app/cb
`
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte(doc), &node))
	resource, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	idpDTO, ok := resource.(*providers.IDPDTO)
	s.Require().True(ok)
	s.Equal("corp-google", idpDTO.ID)
	s.Equal(providers.IDPTypeGoogle, idpDTO.Type)
}

func (s *DeclarativeResourceTestSuite) TestParseConnectionFromNodeGoogleTokenExchange() {
	doc := `
id: corp-google
type: google
name: Corp Google
clientId: abc
clientSecret: shh
redirectUri: https://app/cb
issuer: https://accounts.google.com
jwksEndpoint: https://www.googleapis.com/oauth2/v3/certs
tokenExchangeEnabled: true
`
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte(doc), &node))
	resource, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	idpDTO, ok := resource.(*providers.IDPDTO)
	s.Require().True(ok)

	values, err := propertyValues(idpDTO.Properties)
	s.Require().NoError(err)
	s.Equal("https://accounts.google.com", values[idp.PropIssuer])
	s.Equal("https://www.googleapis.com/oauth2/v3/certs", values[idp.PropJwksEndpoint])
	s.Equal("true", values[idp.PropTokenExchangeEnabled])
}

func (s *DeclarativeResourceTestSuite) TestParseConnectionFromNodeSMSVendor() {
	doc := `
id: prod-sms
type: sms-gateway
name: Prod SMS
url: https://sms.example.com/send
httpMethod: POST
`
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte(doc), &node))
	resource, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	senderDTO, ok := resource.(*ncommon.NotificationSenderDTO)
	s.Require().True(ok)
	s.Equal("prod-sms", senderDTO.ID)
	s.Equal(ncommon.NotificationProviderTypeCustom, senderDTO.Provider)
}

func (s *DeclarativeResourceTestSuite) TestParseToConnectionDTOWrapperIDPVendor() {
	doc := []byte("id: corp-google\ntype: google\nname: Corp Google\nclientId: abc\n")
	dto, err := parseToConnectionDTOWrapper(doc)
	s.Require().NoError(err)
	idpDTO, ok := dto.(*providers.IDPDTO)
	s.Require().True(ok)
	s.Equal("corp-google", idpDTO.ID)
}

func (s *DeclarativeResourceTestSuite) TestParseToConnectionDTOWrapperSenderVendor() {
	doc := []byte("id: prod-sms\ntype: sms-gateway\nname: Prod SMS\nurl: https://sms.example.com/send\n")
	dto, err := parseToConnectionDTOWrapper(doc)
	s.Require().NoError(err)
	senderDTO, ok := dto.(*ncommon.NotificationSenderDTO)
	s.Require().True(ok)
	s.Equal("prod-sms", senderDTO.ID)
}

func (s *DeclarativeResourceTestSuite) TestParseToConnectionDTOWrapperInvalidYAML() {
	_, err := parseToConnectionDTOWrapper([]byte("id: [unterminated"))
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestParseToConnectionDTOWrapperUnsupportedVendor() {
	_, err := parseToConnectionDTOWrapper([]byte("id: bad\ntype: unknown-vendor\nname: Bad\n"))
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestConnectionResourceID() {
	s.Equal("idp-1", connectionResourceID(&providers.IDPDTO{ID: "idp-1"}))
	s.Equal("sender-1", connectionResourceID(&ncommon.NotificationSenderDTO{ID: "sender-1"}))
	s.Equal("", connectionResourceID("not-a-dto"))
}

func (s *DeclarativeResourceTestSuite) TestValidateConnectionDTOWrapper() {
	s.Error(validateConnectionDTOWrapper(&providers.IDPDTO{ID: "idp-1", Name: ""}, nil, nil, nil, nil))
	s.Error(validateConnectionDTOWrapper(&providers.IDPDTO{ID: "idp-1", Name: "Google"}, nil, nil, nil, nil),
		"a name-only IdP DTO must fail full validation (missing type and required properties)")
	s.NoError(validateConnectionDTOWrapper(&providers.IDPDTO{
		ID: "idp-1", Name: "Google", Type: providers.IDPTypeGoogle, OUID: "ou-1",
		Properties: []cmodels.Property{
			mustProperty(s.T(), idp.PropClientID, "client-id", false),
			mustProperty(s.T(), idp.PropClientSecret, "client-secret", true),
			mustProperty(s.T(), idp.PropRedirectURI, "https://app/cb", false),
		},
	}, nil, nil, nil, nil))
	s.Error(validateConnectionDTOWrapper(&ncommon.NotificationSenderDTO{ID: "s-1", Name: ""}, nil, nil, nil, nil))
	s.NoError(validateConnectionDTOWrapper(&ncommon.NotificationSenderDTO{ID: "s-1", Name: "Twilio", OUID: "ou-1"},
		nil, nil, nil, nil))
	s.NoError(validateConnectionDTOWrapper("not-a-dto", nil, nil, nil, nil))
}

// An email sender takes the API's validation path on load, so a provider that cannot deliver
// fails here rather than on every send.
func (s *DeclarativeResourceTestSuite) TestValidateConnectionDTOWrapperValidatesEmailSender() {
	parse := func(doc string) *ncommon.NotificationSenderDTO {
		dto, err := parseToConnectionDTOWrapper([]byte(doc))
		s.Require().NoError(err)
		sender, ok := dto.(*ncommon.NotificationSenderDTO)
		s.Require().True(ok)
		return sender
	}

	valid := "id: sm-1\ntype: email-smtp\nname: SMTP\nouId: ou-1\nhost: smtp.example.com\n" +
		"port: 587\nfromAddress: noreply@example.com\n"
	s.NoError(validateConnectionDTOWrapper(parse(valid), nil, nil, nil, nil))

	cases := map[string]string{
		"missing port": "id: sm-1\ntype: email-smtp\nname: SMTP\nhost: smtp.example.com\n" +
			"fromAddress: noreply@example.com\n",
		"invalid from address": "id: sm-1\ntype: email-smtp\nname: SMTP\nhost: smtp.example.com\nport: 587\n" +
			"fromAddress: not-an-address\n",
		"credentials without tls": "id: sm-1\ntype: email-smtp\nname: SMTP\nhost: smtp.example.com\nport: 587\n" +
			"fromAddress: noreply@example.com\ntls: none\nauthentication:\n  type: basic\n" +
			"  properties:\n    username: mailer\n    password: s3cret\n",
	}
	for name, doc := range cases {
		err := validateConnectionDTOWrapper(parse(doc), nil, nil, nil, nil)
		s.Require().Error(err, name)
		s.Contains(err.Error(), `connection resource "sm-1" is invalid`, name)
	}
}

func (s *DeclarativeResourceTestSuite) TestGetResourceRulesReturnsEmptyDefault() {
	rules := s.exporter.GetResourceRules()
	s.Require().NotNil(rules)
	s.Empty(rules.Variables)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByIDPropagatesNonNotFoundIDPError() {
	svcErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "IDP-5000"}
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "idp-1").
		Return((*providers.IDPDTO)(nil), svcErr)

	resource, name, gotErr := s.exporter.GetResourceByID(context.Background(), "idp-1")
	s.Nil(resource)
	s.Empty(name)
	s.Require().NotNil(gotErr)
	s.Equal("IDP-5000", gotErr.Code)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByIDPropagatesSenderError() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "tw-1").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)
	senderErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "MNS-5000"}
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return((*ncommon.NotificationSenderDTO)(nil), senderErr)

	resource, name, gotErr := s.exporter.GetResourceByID(context.Background(), "tw-1")
	s.Nil(resource)
	s.Empty(name)
	s.Require().NotNil(gotErr)
	s.Equal("MNS-5000", gotErr.Code)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceRulesForResourceSecretSelection() {
	cases := []struct {
		model connectionExportModel
		want  []string
	}{
		{connectionExportModel{Type: "google", ClientSecret: "s"}, []string{"ClientSecret"}},
		{connectionExportModel{Type: "google"}, nil}, // no secret set -> nothing to externalize
		{connectionExportModel{Type: "twilio"}, []string{"AuthToken"}},
		{connectionExportModel{Type: "vonage"}, []string{"APISecret"}},
		{connectionExportModel{Type: "authzen-pdp"}, nil},
		{connectionExportModel{Type: "authzen-pdp", Authentication: &outboundauth.Authentication{
			Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "secret"},
		}}, []string{"Authentication.Properties.token"}},
		{connectionExportModel{Type: "authzen-pdp", Authentication: &outboundauth.Authentication{
			Type: string(outboundauth.TypeAPIKey), Properties: map[string]string{"X-API-Key": "secret"},
		}}, []string{"Authentication.Properties.X-API-Key"}},
		{connectionExportModel{Type: smsGatewayVendorName}, nil},
		{connectionExportModel{Type: smsGatewayVendorName,
			Authentication: &outboundauth.Authentication{
				Type: string(outboundauth.TypeAPIKey), Properties: map[string]string{"X-API-Key": "secret"},
			}}, []string{"Authentication.Properties.X-API-Key"}},
	}
	for _, tc := range cases {
		rules := s.exporter.GetResourceRulesForResource(&tc.model)
		s.Require().NotNil(rules)
		// Every one of these is a credential, so it is named on the list that says so: a value under
		// Variables is readable back from wherever it is held, and one under SecretVariables is not.
		s.Equal(tc.want, rules.SecretVariables, tc.model.Type)
		s.Empty(rules.Variables, tc.model.Type)
	}
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByIDFallsBackToSender() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "tw-1").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(&ncommon.NotificationSenderDTO{
			ID: "tw-1", Name: "My Twilio", Type: ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeTwilio,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.TwilioPropKeyAccountSID, "AC00000000000000000000000000000000", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	resource, name, svcErr := s.exporter.GetResourceByID(context.Background(), "tw-1")
	s.Require().Nil(svcErr)
	s.Equal("My Twilio", name)
	model, ok := resource.(*connectionExportModel)
	s.Require().True(ok)
	s.Equal("twilio", model.Type)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDsFiltersUnregisteredVendors() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Type: providers.IDPTypeGoogle},
		{ID: "2", Type: providers.IDPType("SAML")}, // unregistered -> excluded
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSenders", mock.Anything).Return([]ncommon.NotificationSenderDTO{
		{ID: "s1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio},
		{ID: "s2", Type: ncommon.NotificationSenderTypeEmail, Provider: ncommon.NotificationProviderType("mailer")},
	}, (*tidcommon.ServiceError)(nil))

	ids, svcErr := s.exporter.GetAllResourceIDs(context.Background())
	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"1", "s1"}, ids)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDsExcludesDeclarativeAuthZENPDPConnections() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return([]idp.BasicIDPDTO{}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSenders", mock.Anything).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil))
	pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	pdpService.On("ListAuthZENPDPs", mock.Anything).Return([]authzenpdp.AuthZENPDPConnection{
		{ID: "mutable", Name: "Mutable PDP"},
		{ID: "declarative", Name: "Declarative PDP", IsReadOnly: true},
	}, (*tidcommon.ServiceError)(nil))
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif,
		pdpService, nil)

	ids, svcErr := exporter.GetAllResourceIDs(context.Background())
	s.Require().Nil(svcErr)
	s.Equal([]string{"mutable"}, ids)
}

func (s *DeclarativeResourceTestSuite) TestValidateResourceRejectsEmptyName() {
	logger := log.GetLogger()
	_, exportErr := s.exporter.ValidateResource(context.Background(),
		&connectionExportModel{ID: "1", Type: "google"}, "1", logger)
	s.Require().NotNil(exportErr)
}

func (s *DeclarativeResourceTestSuite) TestConnectionDeclarativeStoreDispatchesByType() {
	// idpStore.Create is gated on the identity-provider package's own store mode, so it must
	// resolve to composite/declarative for this dispatch test to observe the IDP write.
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		IdentityProvider: config.IdentityProviderConfig{Store: "composite"},
	}))
	s.T().Cleanup(config.ResetServerRuntime)

	store := &connectionDeclarativeStore{
		idpStore:    declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeIDP),
		senderStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeNotificationSender),
	}

	idpDTO := &providers.IDPDTO{ID: "idp-1", Type: providers.IDPTypeGoogle}
	s.Require().NoError(store.Create("idp-1", idpDTO))
	got, err := store.idpStore.Get("idp-1")
	s.Require().NoError(err)
	s.Equal(idpDTO, got)

	senderDTO := &ncommon.NotificationSenderDTO{ID: "sender-1", Provider: ncommon.NotificationProviderTypeTwilio}
	s.Require().NoError(store.Create("sender-1", senderDTO))
	got, err = store.senderStore.Get("sender-1")
	s.Require().NoError(err)
	s.Equal(senderDTO, got)

	s.Error(store.Create("bad", "not-a-dto"))
}

func (s *DeclarativeResourceTestSuite) TestConnectionDeclarativeStoreStoresAuthZENPDPEndpoints() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}))
	s.T().Cleanup(config.ResetServerRuntime)

	declarativeStore := &connectionDeclarativeStore{
		authZENPDPStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeAuthZENPDP),
	}

	dto := &authzenpdp.AuthZENPDPConnection{
		Name:          "PDP",
		Endpoint:      "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
	}

	s.Require().NoError(declarativeStore.Create("pdp-1", dto))
	got, err := declarativeStore.authZENPDPStore.Get("pdp-1")
	s.Require().NoError(err)
	s.Equal("pdp-1", got.(*authzenpdp.AuthZENPDPConnection).ID)
	s.Equal(dto, got)
}

func (s *DeclarativeResourceTestSuite) TestConnectionDeclarativeStoreSkipsAuthZENPDPWhenDisabled() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		IdentityProvider: config.IdentityProviderConfig{Store: "composite"},
	}))
	s.T().Cleanup(config.ResetServerRuntime)

	declarativeStore := &connectionDeclarativeStore{
		authZENPDPStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeAuthZENPDP),
	}
	dto := &authzenpdp.AuthZENPDPConnection{
		Name:          "PDP",
		Endpoint:      "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
	}

	s.Require().NoError(declarativeStore.Create("pdp-1", dto))
	_, err := declarativeStore.authZENPDPStore.Get("pdp-1")
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestConnectionDeclarativeStoreSkipsAuthZENPDPInMutableMode() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
		AuthZENPDP:           config.AuthZENPDPConfig{Store: "mutable"},
	}))
	s.T().Cleanup(config.ResetServerRuntime)

	store := &connectionDeclarativeStore{
		authZENPDPStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeAuthZENPDP),
	}
	s.Require().NoError(store.Create("pdp-1", &authzenpdp.AuthZENPDPConnection{Name: "PDP"}))
	_, err := store.authZENPDPStore.Get("pdp-1")
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestConnectionDeclarativeStoreLoadsAuthZENPDPInCompositeMode() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		AuthZENPDP: config.AuthZENPDPConfig{Store: "composite"},
	}))
	s.T().Cleanup(config.ResetServerRuntime)

	store := &connectionDeclarativeStore{
		authZENPDPStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeAuthZENPDP),
	}
	s.Require().NoError(store.Create("pdp-1", &authzenpdp.AuthZENPDPConnection{Name: "PDP"}))
	got, err := store.authZENPDPStore.Get("pdp-1")
	s.Require().NoError(err)
	s.Equal("pdp-1", got.(*authzenpdp.AuthZENPDPConnection).ID)
}

// TestConnectionDeclarativeStoreSkipsIDPWhenIDPStoreModeIsMutable verifies that IdP-typed
// documents are silently skipped (not an error) when the identity-provider package's own
// per-service store mode resolves to mutable, even though the connection package's file
// loader is otherwise enabled. Sender-typed documents are unaffected.
func (s *DeclarativeResourceTestSuite) TestConnectionDeclarativeStoreSkipsIDPWhenIDPStoreModeIsMutable() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		IdentityProvider: config.IdentityProviderConfig{Store: "mutable"},
	}))
	s.T().Cleanup(config.ResetServerRuntime)

	store := &connectionDeclarativeStore{
		idpStore:    declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeIDP),
		senderStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeNotificationSender),
	}

	idpDTO := &providers.IDPDTO{ID: "idp-1", Type: providers.IDPTypeGoogle}
	s.Require().NoError(store.Create("idp-1", idpDTO))
	_, err := store.idpStore.Get("idp-1")
	s.Error(err, "IDP declarative resource should not be stored when idp store mode is mutable")

	senderDTO := &ncommon.NotificationSenderDTO{ID: "sender-1", Provider: ncommon.NotificationProviderTypeTwilio}
	s.Require().NoError(store.Create("sender-1", senderDTO))
	got, err := store.senderStore.Get("sender-1")
	s.Require().NoError(err)
	s.Equal(senderDTO, got)
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelFromSenderDTOSMTP() {
	dto := ncommon.NotificationSenderDTO{
		ID: "sm-1", Name: "Corp SMTP", Type: ncommon.NotificationSenderTypeEmail,
		Provider: ncommon.NotificationProviderTypeSMTP,
		Properties: []cmodels.Property{
			mustProperty(s.T(), ncommon.SMTPPropKeyHost, "smtp.example.com", false),
			mustProperty(s.T(), ncommon.SMTPPropKeyPort, "587", false),
			mustProperty(s.T(), "authentication_type", string(outboundauth.TypeBasic), false),
			mustProperty(s.T(), "authentication_username", "mailer", false),
			mustProperty(s.T(), "authentication_password", "s3cret", true),
			mustProperty(s.T(), ncommon.SMTPPropKeyFromAddress, "noreply@example.com", false),
			mustProperty(s.T(), ncommon.SMTPPropKeyFromName, "Acme Support", false),
			mustProperty(s.T(), ncommon.SMTPPropKeyTLS, string(ncommon.TLSModeSTARTTLS), false),
		},
	}

	model, err := connectionModelFromSenderDTO(dto)
	s.Require().NoError(err)
	s.Equal(emailSMTPVendorName, model.Type)
	s.Equal("smtp.example.com", model.Host)
	s.Equal(587, model.Port)
	s.Equal("noreply@example.com", model.FromAddress)
	s.Equal("Acme Support", model.FromName)
	s.Equal(string(ncommon.TLSModeSTARTTLS), model.TLS)

	s.Require().NotNil(model.Authentication)
	s.Equal(string(outboundauth.TypeBasic), model.Authentication.Type)
	s.Equal("mailer", model.Authentication.Properties[outboundauth.FieldBasicUsername])
	// The export parameterizer needs the real value to externalize it to the .env file.
	s.Equal("s3cret", model.Authentication.Properties[outboundauth.FieldBasicPassword])
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelToDTORoundTripsSMTP() {
	model := connectionExportModel{
		ID: "sm-1", Type: emailSMTPVendorName, Name: "Corp SMTP", Host: "smtp.example.com", Port: 587,
		FromAddress: "noreply@example.com", FromName: "Acme Support", TLS: string(ncommon.TLSModeSTARTTLS),
		Authentication: &outboundauth.Authentication{
			Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{
				outboundauth.FieldBasicUsername: "mailer",
				outboundauth.FieldBasicPassword: "s3cret",
			},
		},
	}

	idpDTO, senderDTO, err := connectionModelToDTO(model)
	s.Require().NoError(err)
	s.Nil(idpDTO)
	s.Require().NotNil(senderDTO)
	s.Equal("sm-1", senderDTO.ID)
	s.Equal(ncommon.NotificationSenderTypeEmail, senderDTO.Type)
	s.Equal(ncommon.NotificationProviderTypeSMTP, senderDTO.Provider)

	roundTripped, err := connectionModelFromSenderDTO(*senderDTO)
	s.Require().NoError(err)
	s.Equal(model.Host, roundTripped.Host)
	s.Equal(model.Port, roundTripped.Port)
	s.Equal(model.FromAddress, roundTripped.FromAddress)
	s.Equal(model.FromName, roundTripped.FromName)
	s.Equal(model.TLS, roundTripped.TLS)
	s.Require().NotNil(roundTripped.Authentication)
	s.Equal(model.Authentication.Type, roundTripped.Authentication.Type)
	s.Equal(model.Authentication.Properties, roundTripped.Authentication.Properties)
}

func (s *DeclarativeResourceTestSuite) TestParseConnectionFromNodeSMTPVendor() {
	doc := `
id: prod-smtp
type: email-smtp
name: Prod SMTP
host: smtp.example.com
port: 587
fromAddress: noreply@example.com
tls: starttls
authentication:
  type: basic
  properties:
    username: mailer
    password: s3cret
`
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte(doc), &node))
	resource, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	senderDTO, ok := resource.(*ncommon.NotificationSenderDTO)
	s.Require().True(ok)
	s.Equal("prod-smtp", senderDTO.ID)
	s.Equal(ncommon.NotificationSenderTypeEmail, senderDTO.Type)
	s.Equal(ncommon.NotificationProviderTypeSMTP, senderDTO.Provider)
}

// A credential field of the configured authentication method is the secret an exported
// connection must externalize, so it never lands in the rendered YAML. The path is derived from
// the registered method, which is what keeps a new method from needing a change here.
func (s *DeclarativeResourceTestSuite) TestGetResourceRulesExternalizesAuthenticationSecrets() {
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil, nil)

	basicAuth := func(properties map[string]string) *outboundauth.Authentication {
		return &outboundauth.Authentication{
			Type: string(outboundauth.TypeBasic), Properties: properties,
		}
	}

	rules := exporter.GetResourceRulesForResource(&connectionExportModel{
		Type: emailSMTPVendorName,
		Authentication: basicAuth(map[string]string{
			outboundauth.FieldBasicUsername: "mailer",
			outboundauth.FieldBasicPassword: "s3cret",
		}),
	})
	// Only the credential field is externalized, as a secret; the username stays in the document.
	s.Equal([]string{"Authentication.Properties.password"}, rules.SecretVariables)
	s.Empty(rules.Variables)

	// Nothing to externalize when the method carries no credential value.
	rules = exporter.GetResourceRulesForResource(&connectionExportModel{
		Type:           emailSMTPVendorName,
		Authentication: basicAuth(map[string]string{outboundauth.FieldBasicUsername: "mailer"}),
	})
	s.Empty(rules.SecretVariables)

	// Nor when the connection authenticates with nothing at all.
	rules = exporter.GetResourceRulesForResource(&connectionExportModel{Type: emailSMTPVendorName})
	s.Empty(rules.SecretVariables)
}

// yamlPathOf names the YAML field a struct path of connectionExportModel reaches: each struct
// segment by its yaml tag, and the segments under a map by their key.
func yamlPathOf(structPath string) string {
	parts := strings.Split(structPath, ".")
	current := reflect.TypeOf(connectionExportModel{})
	named := make([]string, 0, len(parts))
	for i, part := range parts {
		if current.Kind() == reflect.Pointer {
			current = current.Elem()
		}
		if current.Kind() == reflect.Map {
			named = append(named, parts[i:]...)
			break
		}
		field, ok := current.FieldByName(part)
		if !ok {
			named = append(named, part)
			continue
		}
		named = append(named, strings.Split(field.Tag.Get("yaml"), ",")[0])
		current = field.Type
	}
	return strings.Join(named, ".")
}

// Every secret a connection of any vendor exports a reference for is one of the fields the exporter
// says may hold a secret, so an import can put each one back where it came from and nowhere else.
func (s *DeclarativeResourceTestSuite) TestSecretFieldsCoverEveryVendorsSecrets() {
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil, nil)
	fields := exporter.SecretFields()
	s.True(secretAllowedAt(fields, "authentication.properties."+outboundauth.FieldBasicPassword))
	s.False(secretAllowedAt(fields, "authentication.properties."+outboundauth.FieldBasicUsername),
		"a secret in a field shown in clear is refused")
	s.False(secretAllowedAt(fields, "name"), "a secret outside a credential field is refused")

	for _, model := range []*connectionExportModel{
		{Type: "google", ClientSecret: "s"},
		{Type: "twilio", AuthToken: "t"},
		{Type: "vonage", APISecret: "a"},
		{Type: smsGatewayVendorName, Authentication: &outboundauth.Authentication{
			Type:       string(outboundauth.TypeAPIKey),
			Properties: map[string]string{"X-Api-Key": "k"},
		}},
		{Type: emailSMTPVendorName, Authentication: &outboundauth.Authentication{
			Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{
				outboundauth.FieldBasicUsername: "mailer", outboundauth.FieldBasicPassword: "p",
			},
		}},
	} {
		rules := exporter.GetResourceRulesForResource(model)
		s.Require().NotEmpty(rules.SecretVariables, model.Type)
		for _, path := range rules.SecretVariables {
			s.True(secretAllowedAt(fields, yamlPathOf(path)), "%s: %s", model.Type, path)
		}
	}
}

// secretAllowedAt reports whether an import accepts a secret reference at a dotted field path, judged the way
// the secret resolver judges an imported document.
func secretAllowedAt(fields []string, path string) bool {
	parts := strings.Split(path, ".")
	doc := map[string]interface{}{}
	current := doc
	for _, part := range parts[:len(parts)-1] {
		next := map[string]interface{}{}
		current[part] = next
		current = next
	}
	current[parts[len(parts)-1]] = "sec:TEST_SECRET"
	encoded, err := yaml.Marshal(doc)
	if err != nil {
		return false
	}
	var node yaml.Node
	if err := yaml.Unmarshal(encoded, &node); err != nil {
		return false
	}
	return len(secretresolver.MisplacedSecrets(&node, fields)) == 0
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelCarriesOU() {
	tests := []struct {
		name    string
		export  func() (connectionExportModel, error)
		restore func(connectionExportModel) (ouID, ouHandle string, err error)
	}{
		{
			name: "identity provider",
			export: func() (connectionExportModel, error) {
				return connectionModelFromIDPDTO(providers.IDPDTO{
					ID: "google-1", Name: "My Google", Type: providers.IDPTypeGoogle, OUID: "ou-1"})
			},
			restore: func(model connectionExportModel) (string, string, error) {
				idpDTO, senderDTO, err := connectionModelToDTO(model)
				s.Nil(senderDTO)
				return idpDTO.OUID, idpDTO.OUHandle, err
			},
		},
		{
			name: "notification sender",
			export: func() (connectionExportModel, error) {
				return connectionModelFromSenderDTO(ncommon.NotificationSenderDTO{
					ID: "s-1", Name: "Twilio", Type: ncommon.NotificationSenderTypeMessage,
					Provider: ncommon.NotificationProviderTypeTwilio, OUID: "ou-1",
				})
			},
			restore: func(model connectionExportModel) (string, string, error) {
				idpDTO, senderDTO, err := connectionModelToDTO(model)
				s.Nil(idpDTO)
				return senderDTO.OUID, senderDTO.OUHandle, err
			},
		},
		{
			name: "AuthZEN PDP",
			export: func() (connectionExportModel, error) {
				return connectionModelFromAuthZENPDP(authzenpdp.AuthZENPDPConnection{
					ID: "pdp-1", Name: "PDP", OUID: "ou-1", Endpoint: "https://pdp.example.com/evaluation",
				})
			},
			restore: func(model connectionExportModel) (string, string, error) {
				pdp, err := connectionModelToAuthZENPDP(model)
				return pdp.OUID, pdp.OUHandle, err
			},
		},
		{
			name: "HTTP",
			export: func() (connectionExportModel, error) {
				return connectionModelFromHTTP(httpconnection.HTTPConnection{
					ID: "http-1", Name: "Risk", OUID: "ou-1", URL: "https://risk.example.com/evaluate",
				})
			},
			restore: func(model connectionExportModel) (string, string, error) {
				connection, err := connectionModelToHTTP(model)
				return connection.OUID, connection.OUHandle, err
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			model, err := tc.export()
			s.Require().NoError(err)
			s.Equal("ou-1", model.OUID)

			model.OUHandle = "acme"
			ouID, ouHandle, err := tc.restore(model)

			s.Require().NoError(err)
			s.Equal("ou-1", ouID)
			s.Equal("acme", ouHandle)
		})
	}
}

// httpConnectionWithBasicAuth builds an HTTP connection through httpToDTO, so its password is encrypted.
func (s *DeclarativeResourceTestSuite) httpConnectionWithBasicAuth() httpconnection.HTTPConnection {
	dto, err := httpToDTO(httpConnectionRequest{
		Name: "Risk Engine", Description: "Risk", URL: "https://risk.example.com/evaluate", TimeoutMS: 5000,
		Headers: []httpconnection.Header{{Name: "X-Tenant", Value: "acme"}},
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{
				outboundauth.FieldBasicUsername: "svc-user",
				outboundauth.FieldBasicPassword: "s3cret",
			}},
	})
	s.Require().NoError(err)
	dto.ID = "http-1"
	return *dto
}

func (s *DeclarativeResourceTestSuite) TestConnectionModelFromHTTPUnmasksSecret() {
	connection := s.httpConnectionWithBasicAuth()
	s.Require().NotContains(fmt.Sprint(connection.AuthProperties), "s3cret", "the stored password is encrypted")

	model, err := connectionModelFromHTTP(connection)

	s.Require().NoError(err)
	s.Equal(connectionExportModel{
		ID: "http-1", Type: httpVendorName, Name: "Risk Engine", Description: "Risk",
		URL: "https://risk.example.com/evaluate", TimeoutMS: 5000,
		Headers: []httpconnection.Header{{Name: "X-Tenant", Value: "acme"}},
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{
				outboundauth.FieldBasicUsername: "svc-user",
				outboundauth.FieldBasicPassword: "s3cret",
			}},
	}, model)
	rules := s.exporter.GetResourceRulesForResource(&model)
	s.Equal([]string{"Authentication.Properties.password"}, rules.SecretVariables)
}

func (s *DeclarativeResourceTestSuite) TestHTTPConnectionDocumentRoundTrips() {
	doc := `
id: http-1
type: http
name: Risk Engine
url: https://risk.example.com/evaluate
timeoutMs: 5000
headers:
  - name: X-Tenant
    value: acme
authentication:
  type: basic
  properties:
    username: svc-user
    password: s3cret
`
	parsed, err := parseToConnectionDTOWrapper([]byte(doc))
	s.Require().NoError(err)
	connection, ok := parsed.(*httpconnection.HTTPConnection)
	s.Require().True(ok)
	s.Equal("http-1", connectionResourceID(connection))
	s.Equal("https://risk.example.com/evaluate", connection.URL)
	s.Equal(5000, connection.TimeoutMS)
	s.Equal([]httpconnection.Header{{Name: "X-Tenant", Value: "acme"}}, connection.Headers)
	s.NotContains(fmt.Sprint(connection.AuthProperties), "s3cret")
	authConfig, err := outboundauth.FromProperties(connection.AuthProperties)
	s.Require().NoError(err)
	s.Equal("s3cret", authConfig.Get(outboundauth.FieldBasicPassword))

	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte(doc), &node))
	fromNode, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	nodeConnection, ok := fromNode.(*httpconnection.HTTPConnection)
	s.Require().True(ok)
	s.Equal("http-1", nodeConnection.ID)

	var other yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte("id: h\ntype: http\nname: H\nurl: https://h.example.com\n"+
		"authentication:\n  type: kerberos\n"), &other))
	_, err = ParseConnectionFromNode(other.Content[0])
	s.Error(err)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDsListsMutableHTTPConnections() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return([]idp.BasicIDPDTO{{ID: "idp-1", Type: providers.IDPTypeGoogle}}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSenders", mock.Anything).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil))
	httpService := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	httpService.On("ListHTTPConnections", mock.Anything).Return([]httpconnection.HTTPConnection{
		{ID: "mutable", Name: "Mutable", OUID: "ou-1"},
		{ID: "declarative", Name: "Declarative", OUID: "ou-1", IsReadOnly: true},
	}, (*tidcommon.ServiceError)(nil))

	ids, svcErr := newConnectionExporter(s.mockIDP, s.mockNotif, nil, httpService).
		GetAllResourceIDs(context.Background())

	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"idp-1", "mutable"}, ids, "a declared HTTP connection is not exported")
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByIDFallsBackToHTTP() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, mock.Anything).
		Return(nil, &idp.ErrorIDPNotFound)
	s.mockNotif.On("GetSender", mock.Anything, mock.Anything).
		Return(nil, &notification.ErrorSenderNotFound)
	httpService := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	connection := s.httpConnectionWithBasicAuth()
	httpService.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(&connection, (*tidcommon.ServiceError)(nil))
	httpService.On("GetHTTPConnection", mock.Anything, "missing").
		Return(nil, &httpconnection.ErrorNotFound)
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil, httpService)

	resource, name, svcErr := exporter.GetResourceByID(context.Background(), "http-1")
	s.Require().Nil(svcErr)
	s.Equal("Risk Engine", name)
	s.Equal(httpVendorName, resource.(*connectionExportModel).Type)

	_, _, svcErr = exporter.GetResourceByID(context.Background(), "missing")
	s.Require().NotNil(svcErr)
}

func (s *DeclarativeResourceTestSuite) TestGetAllResourceIDsReportsAnHTTPListFailure() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return([]idp.BasicIDPDTO{}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSenders", mock.Anything).
		Return([]ncommon.NotificationSenderDTO{}, (*tidcommon.ServiceError)(nil))
	httpService := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	httpService.On("ListHTTPConnections", mock.Anything).
		Return(nil, &httpconnection.ErrorResultLimitExceededInCompositeMode)
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil, httpService)

	ids, svcErr := exporter.GetAllResourceIDs(context.Background())

	s.Nil(ids)
	s.Require().NotNil(svcErr)
	s.Equal(httpconnection.ErrorResultLimitExceededInCompositeMode.Code, svcErr.Code)
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByIDReportsHTTPFailures() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, mock.Anything).
		Return(nil, &idp.ErrorIDPNotFound)
	s.mockNotif.On("GetSender", mock.Anything, mock.Anything).
		Return(nil, &notification.ErrorSenderNotFound)
	undecryptable, err := cmodels.DeserializePropertiesFromJSON(
		`[{"name":"authentication_type","value":"basic"},` +
			`{"name":"authentication_password","value":"not-a-ciphertext","isSecret":true}]`)
	s.Require().NoError(err)
	httpService := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	httpService.On("GetHTTPConnection", mock.Anything, "unreadable").
		Return(nil, &tidcommon.InternalServerError)
	httpService.On("GetHTTPConnection", mock.Anything, "undecryptable").
		Return(&httpconnection.HTTPConnection{ID: "undecryptable", Name: "U", AuthProperties: undecryptable},
			(*tidcommon.ServiceError)(nil))
	exporter := newConnectionExporter(s.mockIDP, s.mockNotif, nil, httpService)

	for _, id := range []string{"unreadable", "undecryptable"} {
		resource, _, svcErr := exporter.GetResourceByID(context.Background(), id)
		s.Nil(resource, id)
		s.Require().NotNil(svcErr, id)
		s.Equal(tidcommon.InternalServerError.Code, svcErr.Code, id)
	}
}

func (s *DeclarativeResourceTestSuite) TestValidateConnectionDTOWrapperValidatesHTTPConnections() {
	valid := &httpconnection.HTTPConnection{ID: "h", Name: "H", URL: "https://h.example.com/x", OUHandle: "acme"}
	invalid := &httpconnection.HTTPConnection{ID: "i", Name: "I", URL: "http://h.example.com/x", OUID: "ou-1"}
	unowned := &httpconnection.HTTPConnection{ID: "u", Name: "U", URL: "https://h.example.com/x",
		OUHandle: "missing"}
	httpService := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	for _, connection := range []*httpconnection.HTTPConnection{valid, unowned} {
		httpService.On("ValidateHTTPConnection", mock.Anything, connection).Return((*tidcommon.ServiceError)(nil))
	}
	httpService.On("ValidateHTTPConnection", mock.Anything, invalid).Return(&httpconnection.ErrorInvalidURL)
	httpService.On("ResolveConnectionOU", mock.Anything, valid).
		Run(func(mock.Arguments) { valid.OUID = "ou-acme" }).Return((*tidcommon.ServiceError)(nil))
	httpService.On("ResolveConnectionOU", mock.Anything, unowned).Return(&httpconnection.ErrorInvalidOU)

	s.NoError(validateConnectionDTOWrapper(valid, nil, nil, nil, httpService))
	s.Equal("ou-acme", valid.OUID)
	err := validateConnectionDTOWrapper(invalid, nil, nil, nil, httpService)
	s.Require().Error(err)
	s.Contains(err.Error(), `connection resource "i" is invalid`)
	s.ErrorContains(validateConnectionDTOWrapper(unowned, nil, nil, nil, httpService),
		`organization unit "missing" of connection resource "u" could not be resolved`)
}

func (s *DeclarativeResourceTestSuite) TestConnectionDeclarativeStoreLoadsHTTPByStoreMode() {
	for mode, loads := range map[string]bool{"mutable": false, "composite": true, "declarative": true} {
		s.Run(mode, func() {
			config.ResetServerRuntime()
			s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
				HTTPConnection: config.HTTPConnectionConfig{Store: mode},
			}))
			s.T().Cleanup(config.ResetServerRuntime)
			store := &connectionDeclarativeStore{
				httpConnectionStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeHTTPConnection),
			}

			s.Require().NoError(store.Create("http-1", &httpconnection.HTTPConnection{Name: "H"}))

			got, err := store.httpConnectionStore.Get("http-1")
			if !loads {
				s.Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal("http-1", got.(*httpconnection.HTTPConnection).ID)
		})
	}
}

func (s *DeclarativeResourceTestSuite) TestConnectionDocumentOUHandle() {
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte(
		"id: idp-1\ntype: google\nname: G\nouHandle: acme\nclientId: c\nredirectUri: https://app/cb\n"), &node))

	resource, err := ParseConnectionFromNode(&node)
	s.Require().NoError(err)
	dto, ok := resource.(*providers.IDPDTO)
	s.Require().True(ok)
	s.Equal("acme", dto.OUHandle)
	s.Empty(dto.OUID)

	exported, err := connectionModelFromIDPDTO(providers.IDPDTO{
		ID: "idp-1", Name: "G", Type: providers.IDPTypeGoogle, OUID: "ou-acme", OUHandle: "acme",
	})
	s.Require().NoError(err)
	s.Equal("ou-acme", exported.OUID)
	s.Empty(exported.OUHandle)
}

func (s *DeclarativeResourceTestSuite) TestValidateConnectionDTOOwner() {
	const resolvedOUID = "ou-acme"
	newSender := func(ouID, ouHandle string) *ncommon.NotificationSenderDTO {
		return &ncommon.NotificationSenderDTO{ID: "s-1", Name: "Twilio", OUID: ouID, OUHandle: ouHandle}
	}
	newPDP := func(ouID, ouHandle string) *authzenpdp.AuthZENPDPConnection {
		return &authzenpdp.AuthZENPDPConnection{ID: "pdp-1", Name: "PDP", Endpoint: "https://pdp.example.com/e",
			OUID: ouID, OUHandle: ouHandle}
	}

	newIDP := func(ouID, ouHandle string) *providers.IDPDTO {
		return &providers.IDPDTO{ID: "idp-1", Name: "Google", Type: providers.IDPTypeGoogle, OUID: ouID,
			OUHandle: ouHandle, Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropClientID, "client-id", false),
				mustProperty(s.T(), idp.PropClientSecret, "client-secret", true),
				mustProperty(s.T(), idp.PropRedirectURI, "https://app/cb", false),
			}}
	}

	s.Run("an identity provider resolves its owner through the identity provider service", func() {
		dto := newIDP("", "acme")
		s.mockIDP.On("ResolveIdentityProviderOU", mock.Anything, dto).
			Run(func(mock.Arguments) { dto.OUID = resolvedOUID }).Return((*tidcommon.ServiceError)(nil)).Once()
		s.mockIDP.On("ApplySchemaAwareDefaults", mock.Anything, dto).Return().Once()
		s.NoError(validateConnectionDTOWrapper(dto, s.mockIDP, nil, nil, nil))
		s.Equal(resolvedOUID, dto.OUID)
	})
	s.Run("an identity provider fails when its owner cannot be resolved", func() {
		dto := newIDP("", "missing")
		s.mockIDP.On("ResolveIdentityProviderOU", mock.Anything, dto).Return(&idp.ErrorInvalidIDPOU).Once()
		err := validateConnectionDTOWrapper(dto, s.mockIDP, nil, nil, nil)
		s.ErrorContains(err, `organization unit "missing" of connection resource "idp-1"`)
	})
	s.Run("an identity provider fails without an owner", func() {
		s.ErrorContains(validateConnectionDTOWrapper(newIDP("", ""), nil, nil, nil, nil), "needs an organization unit")
	})
	s.Run("fails without an owner", func() {
		s.ErrorContains(validateConnectionDTOWrapper(newSender("", ""), nil, nil, nil, nil),
			"needs an organization unit")
		s.ErrorContains(validateConnectionDTOWrapper(newPDP("", ""), nil, nil, nil, nil),
			"needs an organization unit")
	})
	s.Run("fails for a handle when no service can resolve it", func() {
		s.ErrorContains(validateConnectionDTOWrapper(newSender("", "acme"), nil, nil, nil, nil),
			"needs an organization unit")
		s.ErrorContains(validateConnectionDTOWrapper(newPDP("", "acme"), nil, nil, nil, nil),
			"needs an organization unit")
	})
	s.Run("keeps an id given", func() {
		s.NoError(validateConnectionDTOWrapper(newSender("ou-1", ""), nil, nil, nil, nil))
		s.NoError(validateConnectionDTOWrapper(newPDP("ou-1", ""), nil, nil, nil, nil))
	})
	s.Run("resolves the owner through the service of the kind", func() {
		sender := newSender("", "acme")
		s.mockNotif.On("ResolveSenderOU", mock.Anything, sender).
			Run(func(mock.Arguments) { sender.OUID = resolvedOUID }).Return((*tidcommon.ServiceError)(nil)).Once()
		s.NoError(validateConnectionDTOWrapper(sender, nil, s.mockNotif, nil, nil))
		s.Equal(resolvedOUID, sender.OUID)

		pdp := newPDP("", "acme")
		pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
		pdpService.On("ResolveConnectionOU", mock.Anything, pdp).
			Run(func(mock.Arguments) { pdp.OUID = resolvedOUID }).Return((*tidcommon.ServiceError)(nil)).Once()
		s.NoError(validateConnectionDTOWrapper(pdp, nil, nil, pdpService, nil))
		s.Equal(resolvedOUID, pdp.OUID)
	})
	s.Run("fails when the service cannot resolve the owner", func() {
		sender := newSender("", "missing")
		s.mockNotif.On("ResolveSenderOU", mock.Anything, sender).Return(&authzenpdp.ErrorInvalidOU).Once()
		err := validateConnectionDTOWrapper(sender, nil, s.mockNotif, nil, nil)
		s.ErrorContains(err, `organization unit "missing" of connection resource "s-1" could not be resolved`)
	})
	s.Run("fails when the AuthZEN PDP service cannot resolve the owner", func() {
		pdp := newPDP("", "missing")
		pdpService := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
		pdpService.On("ResolveConnectionOU", mock.Anything, pdp).Return(&authzenpdp.ErrorInvalidOU).Once()
		err := validateConnectionDTOWrapper(pdp, nil, nil, pdpService, nil)
		s.ErrorContains(err, `organization unit "missing" of connection resource`)
	})
	s.Run("fails when the service cannot find the ouId given", func() {
		sender := newSender("ou-missing", "")
		s.mockNotif.On("ResolveSenderOU", mock.Anything, sender).Return(&authzenpdp.ErrorInvalidOU).Once()
		err := validateConnectionDTOWrapper(sender, nil, s.mockNotif, nil, nil)
		s.ErrorContains(err, `organization unit of connection resource "s-1" could not be resolved`)
	})
}

// apiKeyConnectionExporter returns an exporter holding one connection of the given vendor with the
// API-key headers, under ID c-1.
func (s *DeclarativeResourceTestSuite) apiKeyConnectionExporter(vendor string,
	headers map[string]string) *connectionExporter {
	s.T().Helper()
	authProps, err := outboundauth.ToProperties(outboundauth.Config{Type: outboundauth.TypeAPIKey,
		Properties: headers})
	s.Require().NoError(err)
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "c-1").Return(nil, &idp.ErrorIDPNotFound)
	var pdpService authzenpdp.AuthZENPDPServiceInterface
	httpService := httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	switch vendor {
	case smsGatewayVendorName:
		s.mockNotif.On("GetSender", mock.Anything, "c-1").Return(&ncommon.NotificationSenderDTO{
			ID: "c-1", Name: "Risk Engine", Type: ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeCustom,
			Properties: append([]cmodels.Property{
				mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
			}, authProps...),
		}, (*tidcommon.ServiceError)(nil))
	case authZENPDPVendorName:
		s.mockNotif.On("GetSender", mock.Anything, "c-1").Return(nil, &notification.ErrorSenderNotFound)
		pdpMock := authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
		pdpMock.On("GetAuthZENPDP", mock.Anything, "c-1").Return(&authzenpdp.AuthZENPDPConnection{
			ID: "c-1", Name: "Risk Engine", Endpoint: "https://pdp.example.com/access/v1/evaluation",
			AuthenticationProperties: authProps,
		}, (*tidcommon.ServiceError)(nil))
		pdpService = pdpMock
	default:
		s.mockNotif.On("GetSender", mock.Anything, "c-1").Return(nil, &notification.ErrorSenderNotFound)
		httpService.On("GetHTTPConnection", mock.Anything, "c-1").Return(&httpconnection.HTTPConnection{
			ID: "c-1", Name: "Risk Engine", URL: "https://risk.example.com/evaluate", AuthProperties: authProps,
		}, (*tidcommon.ServiceError)(nil)).Maybe()
	}
	return newConnectionExporter(s.mockIDP, s.mockNotif, pdpService, httpService)
}

// exportConnection exports c-1 through the template-style export, as the export API does.
func exportConnection(exporter *connectionExporter) (*export.ExportResponse, *tidcommon.ServiceError) {
	return export.Initialize(http.NewServeMux(), []declarativeresource.ResourceExporter{exporter},
		export.TemplatePlaceholders).ExportResources(context.Background(), &export.ExportRequest{
		Connections: []string{"c-1"}, Options: &export.ExportOptions{Format: "yaml"},
	})
}

func (s *DeclarativeResourceTestSuite) TestAPIKeyHeadersTheExportCannotNameAreRefused() {
	cases := map[string]struct {
		headers map[string]string
		reason  string
	}{
		"a dotted header": {map[string]string{"X.Tenant.Key": "dotted-s3cret"},
			"the API-key header 'X.Tenant.Key' contains a dot"},
		"headers deriving one parameter": {
			map[string]string{"X-Region_Key": "first-s3cret", "X_Region-Key": "second-s3cret"},
			"the API-key headers 'X-Region_Key' and 'X_Region-Key' map to the same parameter " +
				"CONNECTION_RISK_ENGINE_X_REGION_KEY"},
	}
	for _, vendor := range []string{httpVendorName, smsGatewayVendorName, authZENPDPVendorName} {
		for name, tc := range cases {
			s.Run(vendor+"/"+name, func() {
				s.SetupTest()
				exporter := s.apiKeyConnectionExporter(vendor, tc.headers)
				resource, _, svcErr := exporter.GetResourceByID(context.Background(), "c-1")
				s.Require().Nil(svcErr)

				_, exportErr := exporter.ValidateResource(context.Background(), resource, "c-1", log.GetLogger())

				s.Require().NotNil(exportErr)
				s.Equal("CONN_VALIDATION_ERROR", exportErr.Code)
				s.Equal("connection 'Risk Engine' cannot be exported: "+tc.reason, exportErr.Error)
				for _, value := range tc.headers {
					s.NotContains(exportErr.Error, value)
				}
				result, svcErr := exportConnection(exporter)
				s.Require().NotNil(svcErr, "the connection was exported")
				s.Nil(result)
			})
		}
	}
}

func (s *DeclarativeResourceTestSuite) TestAPIKeyHeadersExportAsParameters() {
	for _, vendor := range []string{httpVendorName, smsGatewayVendorName, authZENPDPVendorName} {
		s.Run(vendor, func() {
			s.SetupTest()

			result, svcErr := exportConnection(s.apiKeyConnectionExporter(vendor,
				map[string]string{"X-Tenant-Key": "tenant-s3cret", "X-Region-Key": "region-s3cret"}))

			s.Require().Nil(svcErr)
			s.Require().Len(result.Files, 1)
			content := result.Files[0].Content
			s.Contains(content, "X-Tenant-Key: {{.CONNECTION_RISK_ENGINE_X_TENANT_KEY}}")
			s.Contains(content, "X-Region-Key: {{.CONNECTION_RISK_ENGINE_X_REGION_KEY}}")
			s.NotContains(content, "tenant-s3cret")
			s.NotContains(content, "region-s3cret")
			s.Require().NotNil(result.EnvFile)
			s.Contains(result.EnvFile.Content, "CONNECTION_RISK_ENGINE_X_TENANT_KEY=tenant-s3cret")
			s.Contains(result.EnvFile.Content, "CONNECTION_RISK_ENGINE_X_REGION_KEY=region-s3cret")
		})
	}
}
