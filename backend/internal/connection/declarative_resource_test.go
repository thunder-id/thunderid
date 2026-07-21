/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
	initConfigWithTestCryptoKey()
	s.T().Cleanup(config.ResetServerRuntime)
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.exporter = NewConnectionExporterForTest(s.mockIDP, s.mockNotif)
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
		Provider: ncommon.MessageProviderTypeTwilio,
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
		Provider: ncommon.MessageProviderTypeCustom,
		Properties: []cmodels.Property{
			mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
		},
	}
	model, err := connectionModelFromSenderDTO(dto)
	s.Require().NoError(err)
	s.Equal(smsGatewayVendorName, model.Type)
	s.Equal("https://sms.example.com/send", model.URL)
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
		wantProvider ncommon.MessageProviderType
	}{
		{
			"twilio",
			connectionExportModel{
				ID: "s1", Type: "twilio", Name: "n", AccountSID: "AC00000000000000000000000000000000",
				AuthToken: "t", SenderID: "+1",
			},
			ncommon.MessageProviderTypeTwilio,
		},
		{
			"vonage",
			connectionExportModel{
				ID: "s2", Type: "vonage", Name: "n", APIKey: "k", APISecret: "s", SenderID: "ThunderID",
			},
			ncommon.MessageProviderTypeVonage,
		},
		{
			"sms-gateway",
			connectionExportModel{ID: "s3", Type: smsGatewayVendorName, Name: "n", URL: "https://x/send"},
			ncommon.MessageProviderTypeCustom,
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
	idpDTO, senderDTO, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	s.Require().NotNil(idpDTO)
	s.Nil(senderDTO)
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
	idpDTO, senderDTO, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	s.Require().NotNil(idpDTO)
	s.Nil(senderDTO)

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
	idpDTO, senderDTO, err := ParseConnectionFromNode(node.Content[0])
	s.Require().NoError(err)
	s.Nil(idpDTO)
	s.Require().NotNil(senderDTO)
	s.Equal("prod-sms", senderDTO.ID)
	s.Equal(ncommon.MessageProviderTypeCustom, senderDTO.Provider)
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
	s.Error(validateConnectionDTOWrapper(&providers.IDPDTO{ID: "idp-1", Name: ""}))
	s.Error(validateConnectionDTOWrapper(&providers.IDPDTO{ID: "idp-1", Name: "Google"}),
		"a name-only IdP DTO must fail full validation (missing type and required properties)")
	s.NoError(validateConnectionDTOWrapper(&providers.IDPDTO{
		ID: "idp-1", Name: "Google", Type: providers.IDPTypeGoogle,
		Properties: []cmodels.Property{
			mustProperty(s.T(), idp.PropClientID, "client-id", false),
			mustProperty(s.T(), idp.PropClientSecret, "client-secret", true),
			mustProperty(s.T(), idp.PropRedirectURI, "https://app/cb", false),
		},
	}))
	s.Error(validateConnectionDTOWrapper(&ncommon.NotificationSenderDTO{ID: "s-1", Name: ""}))
	s.NoError(validateConnectionDTOWrapper(&ncommon.NotificationSenderDTO{ID: "s-1", Name: "Twilio"}))
	s.NoError(validateConnectionDTOWrapper("not-a-dto"))
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
		{connectionExportModel{Type: smsGatewayVendorName}, nil},
	}
	for _, tc := range cases {
		rules := s.exporter.GetResourceRulesForResource(&tc.model)
		s.Require().NotNil(rules)
		s.Equal(tc.want, rules.Variables, tc.model.Type)
	}
}

func (s *DeclarativeResourceTestSuite) TestGetResourceByIDFallsBackToSender() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "tw-1").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(&ncommon.NotificationSenderDTO{
			ID: "tw-1", Name: "My Twilio", Type: ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.MessageProviderTypeTwilio,
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
		{ID: "s1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio},
		{ID: "s2", Type: ncommon.NotificationSenderTypeEmail, Provider: ncommon.MessageProviderType("mailer")},
	}, (*tidcommon.ServiceError)(nil))

	ids, svcErr := s.exporter.GetAllResourceIDs(context.Background())
	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"1", "s1"}, ids)
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

	senderDTO := &ncommon.NotificationSenderDTO{ID: "sender-1", Provider: ncommon.MessageProviderTypeTwilio}
	s.Require().NoError(store.Create("sender-1", senderDTO))
	got, err = store.senderStore.Get("sender-1")
	s.Require().NoError(err)
	s.Equal(senderDTO, got)

	s.Error(store.Create("bad", "not-a-dto"))
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

	senderDTO := &ncommon.NotificationSenderDTO{ID: "sender-1", Provider: ncommon.MessageProviderTypeTwilio}
	s.Require().NoError(store.Create("sender-1", senderDTO))
	got, err := store.senderStore.Get("sender-1")
	s.Require().NoError(err)
	s.Equal(senderDTO, got)
}
