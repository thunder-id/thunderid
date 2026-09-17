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

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type ESignetTestSuite struct {
	suite.Suite
	handler *handler
	mockIDP *idpmock.IDPServiceInterfaceMock
}

func TestESignetSuite(t *testing.T) {
	suite.Run(t, new(ESignetTestSuite))
}

func (s *ESignetTestSuite) SetupTest() {
	s.handler, s.mockIDP, _ = newConnectionTestHandler(s.T())
}

// testSigningKeyPEM stands in for the RSA private key. Nothing in these tests parses it; the
// executor tests cover parsing. It only has to survive encryption and decryption intact.
// #nosec G101 -- placeholder material shaped like a PEM key, not a credential.
const testSigningKeyPEM = "-----BEGIN RSA PRIVATE KEY-----\ntest-key-material\n-----END RSA PRIVATE KEY-----"

// testSigningKeySingleLine is the form testSigningKeyPEM is stored in: newlines written as the
// two-character sequence \n, so configuration export can carry the key in a one-line .env entry.
// Note the backquotes: unlike testSigningKeyPEM above, the \n here are literal backslash-n
// characters, not newline bytes. The two constants read alike but hold different bytes.
// #nosec G101 -- placeholder material shaped like a PEM key, not a credential.
const testSigningKeySingleLine = `-----BEGIN RSA PRIVATE KEY-----\ntest-key-material\n-----END RSA PRIVATE KEY-----`

// fullESignetRequest is a create payload with every property populated.
func fullESignetRequest() esignetConnectionRequest {
	return esignetConnectionRequest{
		Name:                  "MOSIP eSignet",
		Description:           "Sign in with eSignet",
		ClientID:              "thunderid-esignet",
		RedirectURI:           "https://app/gate/callback",
		AuthorizationEndpoint: "https://esignet.example.com/authorize",
		TokenEndpoint:         "https://esignet.example.com/v1/esignet/oauth/v2/token",
		UserInfoEndpoint:      "https://esignet.example.com/v1/esignet/oidc/userinfo",
		JwksEndpoint:          "https://esignet.example.com/v1/esignet/oauth/.well-known/jwks.json",
		SigningKey:            testSigningKeyPEM,
		SigningKeyID:          "W7PUmiG1rrSmsjDVcRQWA3mZPyPHVXqHELzgnMjrGrg",
		Scopes:                []string{"openid", "profile"},
		ACRValues:             "mosip:idp:acr:generated-code",
		UsernamePrefix:        "esignet-",
	}
}

func (s *ESignetTestSuite) TestToIDPDTOMapsFields() {
	dto, err := esignetToIDPDTO(fullESignetRequest())
	s.Require().NoError(err)
	s.Equal(providers.IDPTypeESignet, dto.Type)
	s.Equal("Sign in with eSignet", dto.Description)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("thunderid-esignet", values[idp.PropClientID])
	s.Equal("https://app/gate/callback", values[idp.PropRedirectURI])
	s.Equal("https://esignet.example.com/authorize", values[idp.PropAuthorizationEndpoint])
	s.Equal("https://esignet.example.com/v1/esignet/oauth/v2/token", values[idp.PropTokenEndpoint])
	s.Equal("https://esignet.example.com/v1/esignet/oidc/userinfo", values[idp.PropUserInfoEndpoint])
	s.Equal("https://esignet.example.com/v1/esignet/oauth/.well-known/jwks.json", values[idp.PropJwksEndpoint])
	s.Equal(maskedSecretValue, values[idp.PropSigningKey]) // encrypted and masked on read
	s.Equal("W7PUmiG1rrSmsjDVcRQWA3mZPyPHVXqHELzgnMjrGrg", values[idp.PropSigningKeyID])
	s.Equal("openid,profile", values[idp.PropScopes])
	s.Equal("mosip:idp:acr:generated-code", values[idp.PropACRValues])
	s.Equal("esignet-", values[idp.PropUsernamePrefix])
}

// The signing key authenticates ThunderID to eSignet, so it must be stored encrypted. Nothing
// else in this vendor is a secret: eSignet uses private_key_jwt and issues no client secret.
func (s *ESignetTestSuite) TestOnlyTheSigningKeyIsSecret() {
	dto, err := esignetToIDPDTO(fullESignetRequest())
	s.Require().NoError(err)

	secrets := make([]string, 0)
	for i := range dto.Properties {
		if dto.Properties[i].IsSecret() {
			secrets = append(secrets, dto.Properties[i].GetName())
		}
	}
	s.Equal([]string{idp.PropSigningKey}, secrets)
}

// The executor reads the key through GetPropertyValue, which decrypts. Only the API response
// layer masks it, so the round trip must yield the original PEM rather than the placeholder.
func (s *ESignetTestSuite) TestSigningKeyDecryptsForInternalConsumers() {
	dto, err := esignetToIDPDTO(fullESignetRequest())
	s.Require().NoError(err)

	s.Equal(testSigningKeySingleLine, idp.GetPropertyValue(dto.Properties, idp.PropSigningKey))
}

// A pasted multi-line PEM is collapsed to a single line before it is stored, so the value can
// travel through configuration export, the generated .env file and a re-import unchanged.
func (s *ESignetTestSuite) TestSigningKeyIsStoredOnASingleLine() {
	dto, err := esignetToIDPDTO(fullESignetRequest())
	s.Require().NoError(err)

	stored := idp.GetPropertyValue(dto.Properties, idp.PropSigningKey)
	s.Equal(testSigningKeySingleLine, stored)
	s.NotContains(stored, "\n")
	s.NotContains(stored, "\r")
}

// Collapsing is idempotent: a key that is already single-line passes through untouched, so a
// re-imported connection stores exactly what was exported.
func (s *ESignetTestSuite) TestAlreadySingleLineSigningKeyIsUnchanged() {
	req := fullESignetRequest()
	req.SigningKey = testSigningKeySingleLine

	dto, err := esignetToIDPDTO(req)
	s.Require().NoError(err)
	s.Equal(testSigningKeySingleLine, idp.GetPropertyValue(dto.Properties, idp.PropSigningKey))
}

// Windows line endings collapse to the same single-line form as Unix ones.
func (s *ESignetTestSuite) TestSigningKeyCRLFCollapses() {
	req := fullESignetRequest()
	req.SigningKey = "-----BEGIN RSA PRIVATE KEY-----\r\ntest-key-material\r\n-----END RSA PRIVATE KEY-----"

	dto, err := esignetToIDPDTO(req)
	s.Require().NoError(err)
	s.Equal(testSigningKeySingleLine, idp.GetPropertyValue(dto.Properties, idp.PropSigningKey))
}

func (s *ESignetTestSuite) TestToIDPDTOOmitsEmptyOptionalFields() {
	req := fullESignetRequest()
	req.Scopes = nil
	req.ACRValues = ""
	req.UsernamePrefix = ""

	dto, err := esignetToIDPDTO(req)
	s.Require().NoError(err)
	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.NotContains(values, idp.PropScopes)
	s.NotContains(values, idp.PropACRValues)
	s.NotContains(values, idp.PropUsernamePrefix)
}

func (s *ESignetTestSuite) TestRoundTrip() {
	dto, err := esignetToIDPDTO(fullESignetRequest())
	s.Require().NoError(err)
	dto.ID = "es-1"

	resp, err := esignetFromIDPDTO(*dto)
	s.Require().NoError(err)
	s.Equal("es-1", resp.ID)
	s.Equal("esignet", resp.Type)
	s.Equal("thunderid-esignet", resp.ClientID)
	s.Equal(maskedSecretValue, resp.SigningKey)
	s.Equal("W7PUmiG1rrSmsjDVcRQWA3mZPyPHVXqHELzgnMjrGrg", resp.SigningKeyID)
	s.Equal([]string{"openid", "profile"}, resp.Scopes)
	s.Equal("mosip:idp:acr:generated-code", resp.ACRValues)
	s.Equal("esignet-", resp.UsernamePrefix)
}

func (s *ESignetTestSuite) TestAttributeConfigurationRoundTrips() {
	attrCfg := &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "Citizen"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{
			{
				UserType:   "Citizen",
				Attributes: []providers.AttributeMapping{{ExternalAttribute: "name", LocalAttribute: "name"}},
			},
		},
	}
	req := fullESignetRequest()
	req.AttributeConfiguration = attrCfg

	dto, err := esignetToIDPDTO(req)
	s.Require().NoError(err)
	s.Equal(attrCfg, dto.AttributeConfiguration)

	resp, err := esignetFromIDPDTO(*dto)
	s.Require().NoError(err)
	s.Equal(attrCfg, resp.AttributeConfiguration)
}

func (s *ESignetTestSuite) TestCreate() {
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return(&providers.IDPDTO{
			ID:   "es-1",
			Name: "MOSIP eSignet",
			Type: providers.IDPTypeESignet,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropClientID, "thunderid-esignet", false),
				mustProperty(s.T(), idp.PropSigningKeyID, "kid-1", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(fullESignetRequest())
	req := httptest.NewRequest(http.MethodPost, "/connections/esignet", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	createHandler(s.handler, esignetToIDPDTO, esignetFromIDPDTO)(rr, req)

	s.Equal(http.StatusCreated, rr.Code)
	var resp esignetConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("es-1", resp.ID)
	s.Equal("thunderid-esignet", resp.ClientID)
	s.Equal("kid-1", resp.SigningKeyID)
}

func (s *ESignetTestSuite) TestGetMasksTheSigningKey() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "es-1").
		Return(&providers.IDPDTO{
			ID:   "es-1",
			Name: "MOSIP eSignet",
			Type: providers.IDPTypeESignet,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropSigningKey, testSigningKeyPEM, true),
				mustProperty(s.T(), idp.PropSigningKeyID, "kid-1", false),
				mustProperty(s.T(), idp.PropScopes, "openid,profile", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/esignet/es-1", nil)
	req.SetPathValue("id", "es-1")
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeESignet, esignetFromIDPDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp esignetConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal(maskedSecretValue, resp.SigningKey)
	s.Equal("kid-1", resp.SigningKeyID)
	s.Equal([]string{"openid", "profile"}, resp.Scopes)
}
