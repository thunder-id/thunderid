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

type OAuthTestSuite struct {
	suite.Suite
	handler *handler
	mockIDP *idpmock.IDPServiceInterfaceMock
}

func TestOAuthSuite(t *testing.T) {
	suite.Run(t, new(OAuthTestSuite))
}

func (s *OAuthTestSuite) SetupTest() {
	s.handler, s.mockIDP, _ = newConnectionTestHandler(s.T())
}

func (s *OAuthTestSuite) TestToIDPDTOMapsEndpoints() {
	dto, err := oauthToIDPDTO(oauthConnectionRequest{
		Name: "Acme OAuth2", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		AuthorizationEndpoint: "https://acme/auth", TokenEndpoint: "https://acme/token",
		UserInfoEndpoint: "https://acme/userinfo", Scopes: []string{"read:user", "email"},
	})
	s.Require().NoError(err)
	s.Equal(providers.IDPTypeOAuth, dto.Type)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("https://acme/auth", values[idp.PropAuthorizationEndpoint])
	s.Equal("https://acme/token", values[idp.PropTokenEndpoint])
	s.Equal("https://acme/userinfo", values[idp.PropUserInfoEndpoint])
	s.Equal("read:user,email", values[idp.PropScopes])
}

func (s *OAuthTestSuite) TestAttributeConfigurationRoundTrips() {
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

	dto, err := oauthToIDPDTO(oauthConnectionRequest{
		Name: "Acme OAuth2", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		AuthorizationEndpoint: "https://acme/auth", TokenEndpoint: "https://acme/token",
		UserInfoEndpoint: "https://acme/userinfo", AttributeConfiguration: attrCfg,
	})
	s.Require().NoError(err)
	s.Equal(attrCfg, dto.AttributeConfiguration)

	resp, err := oauthFromIDPDTO(*dto)
	s.Require().NoError(err)
	s.Equal(attrCfg, resp.AttributeConfiguration)
}

func (s *OAuthTestSuite) TestGetMasksSecret() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "o-1").
		Return(&providers.IDPDTO{
			ID:   "o-1",
			Name: "Acme OAuth2",
			Type: providers.IDPTypeOAuth,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
				mustProperty(s.T(), idp.PropUserInfoEndpoint, "https://acme/userinfo", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/oauth/o-1", nil)
	req.SetPathValue("id", "o-1")
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeOAuth, oauthFromIDPDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp oauthConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal(maskedSecretValue, resp.ClientSecret)
	s.Equal("https://acme/userinfo", resp.UserInfoEndpoint)
}

func (s *OAuthTestSuite) TestUpdateAndDelete() {
	existing := &providers.IDPDTO{ID: "o-1", Type: providers.IDPTypeOAuth}
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "o-1").
		Return(existing, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("UpdateIdentityProvider", mock.Anything, "o-1", mock.Anything).
		Return(existing, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("DeleteIdentityProvider", mock.Anything, "o-1").Return((*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(oauthConnectionRequest{
		Name: "Acme OAuth2", ClientID: "c", ClientSecret: maskedSecretValue, RedirectURI: "https://app/cb",
		AuthorizationEndpoint: "https://acme/auth", TokenEndpoint: "https://acme/token",
		UserInfoEndpoint: "https://acme/userinfo",
	})
	putReq := httptest.NewRequest(http.MethodPut, "/connections/oauth/o-1", bytes.NewReader(body))
	putReq.SetPathValue("id", "o-1")
	putRR := httptest.NewRecorder()
	updateHandler(s.handler, providers.IDPTypeOAuth, oauthToIDPDTO, oauthFromIDPDTO)(putRR, putReq)
	s.Equal(http.StatusOK, putRR.Code)

	delReq := httptest.NewRequest(http.MethodDelete, "/connections/oauth/o-1", nil)
	delReq.SetPathValue("id", "o-1")
	delRR := httptest.NewRecorder()
	s.handler.deleteInstance(providers.IDPTypeOAuth)(delRR, delReq)
	s.Equal(http.StatusNoContent, delRR.Code)
}
