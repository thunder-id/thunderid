// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
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

type GitHubTestSuite struct {
	suite.Suite
	handler *handler
	mockIDP *idpmock.IDPServiceInterfaceMock
}

func TestGitHubSuite(t *testing.T) {
	suite.Run(t, new(GitHubTestSuite))
}

func (s *GitHubTestSuite) SetupTest() {
	s.handler, s.mockIDP, _ = newConnectionTestHandler(s.T())
}

func (s *GitHubTestSuite) TestToIDPDTOSetsGitHubType() {
	dto, err := githubToIDPDTO(githubConnectionRequest{
		Name: "My GitHub", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
	})
	s.Require().NoError(err)
	s.Equal(providers.IDPTypeGitHub, dto.Type)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("c", values[idp.PropClientID])
	s.Equal("https://app/cb", values[idp.PropRedirectURI])
}

func (s *GitHubTestSuite) TestAttributeConfigurationRoundTrips() {
	attrCfg := &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "Person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{
			{
				UserType: "Person",
				Attributes: []providers.AttributeMapping{
					{ExternalAttribute: "login", LocalAttribute: "username"},
				},
			},
		},
	}

	dto, err := githubToIDPDTO(githubConnectionRequest{
		Name: "GitHub", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
		AttributeConfiguration: attrCfg,
	})
	s.Require().NoError(err)
	s.Equal(attrCfg, dto.AttributeConfiguration)

	resp, err := githubFromIDPDTO(*dto)
	s.Require().NoError(err)
	s.Equal(attrCfg, resp.AttributeConfiguration)
}

func (s *GitHubTestSuite) TestGetMasksSecret() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "gh-1").
		Return(&providers.IDPDTO{
			ID:   "gh-1",
			Name: "My GitHub",
			Type: providers.IDPTypeGitHub,
			Properties: []cmodels.Property{
				mustProperty(s.T(), idp.PropClientID, "client-123", false),
				mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/github/gh-1", nil)
	req.SetPathValue("id", "gh-1")
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeGitHub, githubFromIDPDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.Contains(rr.Body.String(), maskedSecretValue)
	s.Contains(rr.Body.String(), `"type":"github"`)
}

func (s *GitHubTestSuite) TestOUCarriedThroughMapping() {
	dto, err := githubToIDPDTO(githubConnectionRequest{
		Name: "n", OUID: "ou-1", ClientID: "c", RedirectURI: "https://app/cb",
	})
	s.Require().NoError(err)
	s.Equal("ou-1", dto.OUID)

	resp, err := githubFromIDPDTO(*dto)
	s.Require().NoError(err)
	s.Equal("ou-1", resp.OUID)
}
