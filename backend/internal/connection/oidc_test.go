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

type OIDCTestSuite struct {
	suite.Suite
	handler *handler
	mockIDP *idpmock.IDPServiceInterfaceMock
}

func TestOIDCSuite(t *testing.T) {
	suite.Run(t, new(OIDCTestSuite))
}

func (s *OIDCTestSuite) SetupTest() {
	s.handler, s.mockIDP, _ = newConnectionTestHandler(s.T())
}

func (s *OIDCTestSuite) TestToIDPDTOMapsEndpointsAndTokenExchange() {
	dto, err := oidcToIDPDTO(oidcConnectionRequest{
		Name: "Okta", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		AuthorizationEndpoint: "https://okta/auth", TokenEndpoint: "https://okta/token",
		Issuer: "https://okta", TokenExchangeEnabled: boolPtr(true),
		TrustedTokenAudience: "okta-client-id",
	})
	s.Require().NoError(err)
	s.Equal(providers.IDPTypeOIDC, dto.Type)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("https://okta/auth", values[idp.PropAuthorizationEndpoint])
	s.Equal("https://okta", values[idp.PropIssuer])
	s.Equal("true", values[idp.PropTokenExchangeEnabled])
	s.Equal("okta-client-id", values[idp.PropTrustedTokenAudience])
}

func (s *OIDCTestSuite) TestToIDPDTOEnablesTokenExchangeWithTrustedAudience() {
	dto, err := oidcToIDPDTO(oidcConnectionRequest{
		Name: "Okta", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		AuthorizationEndpoint: "https://okta/auth", TokenEndpoint: "https://okta/token",
		Issuer: "https://okta", TrustedTokenAudience: "okta-client-id",
	})
	s.Require().NoError(err)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("true", values[idp.PropTokenExchangeEnabled])
	s.Equal("okta-client-id", values[idp.PropTrustedTokenAudience])
}

func (s *OIDCTestSuite) TestToIDPDTOMapsIDJagEnabled() {
	dto, err := oidcToIDPDTO(oidcConnectionRequest{
		Name: "Okta", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		AuthorizationEndpoint: "https://okta/auth", TokenEndpoint: "https://okta/token",
		Issuer: "https://okta", JwksEndpoint: "https://okta/keys",
		IDJagEnabled: boolPtr(true),
	})
	s.Require().NoError(err)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("true", values[idp.PropIDJagEnabled])
}

func (s *OIDCTestSuite) TestAttributeConfigurationRoundTrips() {
	attrCfg := &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "Person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{
			{
				UserType: "Person",
				Attributes: []providers.AttributeMapping{
					{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
				},
			},
		},
	}

	dto, err := oidcToIDPDTO(oidcConnectionRequest{
		Name: "Okta", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		AuthorizationEndpoint: "https://okta/auth", TokenEndpoint: "https://okta/token",
		AttributeConfiguration: attrCfg,
	})
	s.Require().NoError(err)
	s.Equal(attrCfg, dto.AttributeConfiguration)

	resp, err := oidcFromIDPDTO(*dto)
	s.Require().NoError(err)
	s.Equal(attrCfg, resp.AttributeConfiguration)
}

func (s *OIDCTestSuite) TestGetParsesTokenExchangeAndMasks() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "o-1").
		Return(&providers.IDPDTO{
			ID:   "o-1",
			Name: "Okta",
			Type: providers.IDPTypeOIDC,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
				mustProperty(s.T(), idp.PropTokenExchangeEnabled, "true", false),
				mustProperty(s.T(), idp.PropTrustedTokenAudience, "okta-client-id", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/oidc/o-1", nil)
	req.SetPathValue("id", "o-1")
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeOIDC, oidcFromIDPDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp oidcConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal(maskedSecretValue, resp.ClientSecret)
	s.Require().NotNil(resp.TokenExchangeEnabled)
	s.True(*resp.TokenExchangeEnabled)
	s.Equal("okta-client-id", resp.TrustedTokenAudience)
}

func (s *OIDCTestSuite) TestGetParsesIDJagEnabled() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "o-2").
		Return(&providers.IDPDTO{
			ID:   "o-2",
			Name: "External",
			Type: providers.IDPTypeOIDC,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropIDJagEnabled, "true", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/oidc/o-2", nil)
	req.SetPathValue("id", "o-2")
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeOIDC, oidcFromIDPDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp oidcConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Require().NotNil(resp.IDJagEnabled)
	s.True(*resp.IDJagEnabled)
}

func (s *OIDCTestSuite) TestUpdateAndDelete() {
	existing := &providers.IDPDTO{ID: "o-1", Type: providers.IDPTypeOIDC}
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "o-1").
		Return(existing, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("UpdateIdentityProvider", mock.Anything, "o-1", mock.Anything).
		Return(existing, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("DeleteIdentityProvider", mock.Anything, "o-1").Return((*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(oidcConnectionRequest{
		Name: "Okta", ClientID: "c", ClientSecret: maskedSecretValue, RedirectURI: "https://app/cb",
		AuthorizationEndpoint: "https://okta/auth", TokenEndpoint: "https://okta/token",
	})
	putReq := httptest.NewRequest(http.MethodPut, "/connections/oidc/o-1", bytes.NewReader(body))
	putReq.SetPathValue("id", "o-1")
	putRR := httptest.NewRecorder()
	updateHandler(s.handler, providers.IDPTypeOIDC, oidcToIDPDTO, oidcFromIDPDTO)(putRR, putReq)
	s.Equal(http.StatusOK, putRR.Code)

	delReq := httptest.NewRequest(http.MethodDelete, "/connections/oidc/o-1", nil)
	delReq.SetPathValue("id", "o-1")
	delRR := httptest.NewRecorder()
	s.handler.deleteInstance(providers.IDPTypeOIDC)(delRR, delReq)
	s.Equal(http.StatusNoContent, delRR.Code)
}
