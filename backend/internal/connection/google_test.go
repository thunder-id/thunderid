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

type GoogleTestSuite struct {
	suite.Suite
	handler *handler
	mockIDP *idpmock.IDPServiceInterfaceMock
}

func TestGoogleSuite(t *testing.T) {
	suite.Run(t, new(GoogleTestSuite))
}

func (s *GoogleTestSuite) SetupTest() {
	s.handler, s.mockIDP, _ = newConnectionTestHandler(s.T())
}

func (s *GoogleTestSuite) TestToIDPDTOMapsFields() {
	dto, err := googleToIDPDTO(googleConnectionRequest{
		Name: "My Google", Description: "Login with Google", ClientID: "c", ClientSecret: "s",
		RedirectURI: "https://app/cb", Scopes: []string{"openid", "email"},
	})
	s.Require().NoError(err)
	s.Equal(providers.IDPTypeGoogle, dto.Type)
	s.Equal("Login with Google", dto.Description)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("c", values[idp.PropClientID])
	s.Equal(maskedSecretValue, values[idp.PropClientSecret]) // secret is encrypted/masked
	s.Equal("openid,email", values[idp.PropScopes])
}

func (s *GoogleTestSuite) TestToIDPDTOMapsTokenExchange() {
	dto, err := googleToIDPDTO(googleConnectionRequest{
		Name: "Google", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		Issuer: "https://accounts.google.com", JwksEndpoint: "https://www.googleapis.com/oauth2/v3/certs",
		TokenExchangeEnabled: boolPtr(true),
	})
	s.Require().NoError(err)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("https://accounts.google.com", values[idp.PropIssuer])
	s.Equal("https://www.googleapis.com/oauth2/v3/certs", values[idp.PropJwksEndpoint])
	s.Equal("true", values[idp.PropTokenExchangeEnabled])
}

func (s *GoogleTestSuite) TestGetParsesTokenExchange() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{
			ID:   "g-1",
			Name: "Google",
			Type: providers.IDPTypeGoogle,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
				mustProperty(s.T(), idp.PropIssuer, "https://accounts.google.com", false),
				mustProperty(s.T(), idp.PropJwksEndpoint, "https://www.googleapis.com/oauth2/v3/certs", false),
				mustProperty(s.T(), idp.PropTokenExchangeEnabled, "true", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/google/g-1", nil)
	req.SetPathValue("id", "g-1")
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeGoogle, googleFromIDPDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp googleConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal(maskedSecretValue, resp.ClientSecret)
	s.Equal("https://accounts.google.com", resp.Issuer)
	s.Equal("https://www.googleapis.com/oauth2/v3/certs", resp.JwksEndpoint)
	s.Require().NotNil(resp.TokenExchangeEnabled)
	s.True(*resp.TokenExchangeEnabled)
}

func (s *GoogleTestSuite) TestAttributeConfigurationRoundTrips() {
	attrCfg := &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "Person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{
			{
				UserType: "Person",
				Attributes: []providers.AttributeMapping{
					{ExternalAttribute: "email", LocalAttribute: "email"},
				},
			},
		},
	}

	dto, err := googleToIDPDTO(googleConnectionRequest{
		Name: "Google", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		AttributeConfiguration: attrCfg,
	})
	s.Require().NoError(err)
	s.Equal(attrCfg, dto.AttributeConfiguration)

	resp, err := googleFromIDPDTO(*dto)
	s.Require().NoError(err)
	s.Equal(attrCfg, resp.AttributeConfiguration)
}

func (s *GoogleTestSuite) TestCreateMasksSecret() {
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return(&providers.IDPDTO{
			ID:   "g-1",
			Name: "My Google",
			Type: providers.IDPTypeGoogle,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropClientID, "client-123", false),
				mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
			},
		}, (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(googleConnectionRequest{
		Name: "My Google", ClientID: "client-123", ClientSecret: "s3cret", RedirectURI: "https://app/cb",
	})
	req := httptest.NewRequest(http.MethodPost, "/connections/google", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	createHandler(s.handler, googleToIDPDTO, googleFromIDPDTO)(rr, req)

	s.Equal(http.StatusCreated, rr.Code)
	var resp googleConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("g-1", resp.ID)
	s.Equal("client-123", resp.ClientID)
	s.Equal(maskedSecretValue, resp.ClientSecret)
}

func (s *GoogleTestSuite) TestGetRoundTrip() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{
			ID:          "g-1",
			Name:        "My Google",
			Description: "Login with Google",
			Type:        providers.IDPTypeGoogle,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropClientID, "client-123", false),
				mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
				mustProperty(s.T(), idp.PropRedirectURI, "https://app/cb", false),
				mustProperty(s.T(), idp.PropScopes, "openid,email", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/google/g-1", nil)
	req.SetPathValue("id", "g-1")
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeGoogle, googleFromIDPDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp googleConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("client-123", resp.ClientID)
	s.Equal("Login with Google", resp.Description)
	s.Equal(maskedSecretValue, resp.ClientSecret)
	s.Equal("https://app/cb", resp.RedirectURI)
	s.Equal([]string{"openid", "email"}, resp.Scopes)
}
