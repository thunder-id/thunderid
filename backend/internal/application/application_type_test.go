// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type ApplicationTypeTestSuite struct {
	suite.Suite
}

func TestApplicationTypeTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationTypeTestSuite))
}

// TestToInboundClientPersistsType verifies the application type is packed into the inbound client
// properties for persistence.
func (s *ApplicationTypeTestSuite) TestToInboundClientPersistsType() {
	dto := &model.ApplicationProcessedDTO{ID: "app-1", Type: model.ApplicationTypeMobile}

	dao := toInboundClient(dto)

	s.Equal("mobile", dao.Properties[propType])
}

// TestToInboundClientOmitsEmptyType verifies an unset type is not written to properties.
func (s *ApplicationTypeTestSuite) TestToInboundClientOmitsEmptyType() {
	dto := &model.ApplicationProcessedDTO{ID: "app-1"}

	dao := toInboundClient(dto)

	_, ok := dao.Properties[propType]
	s.False(ok)
}

// TestToProcessedDTOReadsType verifies a persisted type is read back onto the DTO.
func (s *ApplicationTypeTestSuite) TestToProcessedDTOReadsType() {
	dao := &inboundmodel.InboundClient{
		ID:         "app-1",
		Properties: map[string]interface{}{propType: "browser"},
	}

	dto := toProcessedDTO(nil, dao, nil)

	s.Equal(model.ApplicationTypeBrowser, dto.Type)
}

// TestToProcessedDTOEmptyWhenTypeAbsent verifies applications without a stored type resolve to an
// empty type (no implicit default is applied).
func (s *ApplicationTypeTestSuite) TestToProcessedDTOEmptyWhenTypeAbsent() {
	withProps := toProcessedDTO(nil, &inboundmodel.InboundClient{
		ID:         "app-1",
		Properties: map[string]interface{}{},
	}, nil)
	s.Equal(model.ApplicationType(""), withProps.Type)

	nilProps := toProcessedDTO(nil, &inboundmodel.InboundClient{ID: "app-2"}, nil)
	s.Equal(model.ApplicationType(""), nilProps.Type)
}

// TestBuildBasicApplicationResponseType verifies the list-view response reads the stored type and
// leaves it empty when absent (no implicit default).
func (s *ApplicationTypeTestSuite) TestBuildBasicApplicationResponseType() {
	withType := buildBasicApplicationResponse(inboundmodel.InboundClient{
		ID:         "app-1",
		Properties: map[string]interface{}{propType: "m2m"},
	}, nil)
	s.Equal(model.ApplicationTypeM2M, withType.Type)

	absent := buildBasicApplicationResponse(inboundmodel.InboundClient{ID: "app-2"}, nil)
	s.Equal(model.ApplicationType(""), absent.Type)
}

// TestFlowSecretIneligibleByType verifies browser, mobile, and m2m apps are never issued a Flow
// Secret, decided by their type alone regardless of OAuth config shape.
func (s *ApplicationTypeTestSuite) TestFlowSecretIneligibleByType() {
	embedded := &providers.InboundAuthConfigWithSecret{
		Type: providers.OAuthInboundAuthType,
		OAuthConfig: &providers.OAuthConfigWithSecret{
			GrantTypes: []providers.GrantType{
				providers.GrantTypeClientCredentials,
				providers.GrantTypeTokenExchange,
			},
		},
	}
	for _, appType := range []model.ApplicationType{
		model.ApplicationTypeBrowser,
		model.ApplicationTypeMobile,
		model.ApplicationTypeM2M,
	} {
		s.False(isFlowSecretEligible(appType, nil), "type %q should not be eligible", appType)
		s.False(isFlowSecretEligible(appType, embedded), "type %q should not be eligible", appType)
	}
}

// TestFullStackCustomAndMCPFlowSecretEligibility verifies full-stack, custom, and mcp apps derive
// eligibility from the OAuth config shape: an app with no OAuth configuration (embedded), or a
// confidential, non-redirect client, is eligible; a redirect or m2m-shaped client is not.
func (s *ApplicationTypeTestSuite) TestFullStackCustomAndMCPFlowSecretEligibility() {
	embedded := &providers.InboundAuthConfigWithSecret{
		Type: providers.OAuthInboundAuthType,
		OAuthConfig: &providers.OAuthConfigWithSecret{
			GrantTypes: []providers.GrantType{
				providers.GrantTypeClientCredentials,
				providers.GrantTypeTokenExchange,
			},
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
		},
	}
	redirect := &providers.InboundAuthConfigWithSecret{
		Type: providers.OAuthInboundAuthType,
		OAuthConfig: &providers.OAuthConfigWithSecret{
			GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode},
		},
	}
	m2mShaped := &providers.InboundAuthConfigWithSecret{
		Type: providers.OAuthInboundAuthType,
		OAuthConfig: &providers.OAuthConfigWithSecret{
			GrantTypes: []providers.GrantType{providers.GrantTypeClientCredentials},
		},
	}

	for _, appType := range []model.ApplicationType{
		model.ApplicationTypeFullStack,
		model.ApplicationTypeCustom,
		model.ApplicationTypeMCP,
	} {
		// Embedded app with no OAuth config, and confidential non-redirect app, are eligible.
		s.True(isFlowSecretEligible(appType, nil), "type %q embedded should be eligible", appType)
		s.True(isFlowSecretEligible(appType, embedded), "type %q embedded should be eligible", appType)
		// Redirect and m2m-shaped apps are not eligible.
		s.False(isFlowSecretEligible(appType, redirect), "type %q redirect should not be eligible", appType)
		s.False(isFlowSecretEligible(appType, m2mShaped), "type %q m2m-shaped should not be eligible", appType)
	}
}

// processedDTOWithGrants builds a processed application DTO with one OAuth inbound config.
func processedDTOWithGrants(
	grantTypes []providers.GrantType, clientAttributes []string,
) *model.ApplicationProcessedDTO {
	var token *providers.OAuthTokenConfig
	if clientAttributes != nil {
		token = &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				ClientConfig: &providers.AccessTokenSubConfig{Attributes: clientAttributes},
			},
		}
	}
	return &model.ApplicationProcessedDTO{
		InboundAuthConfig: []inboundmodel.InboundAuthConfigProcessed{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthClient{
					GrantTypes: grantTypes,
					Token:      token,
				},
			},
		},
	}
}

// clientAttributesOf reads back the client-token attribute selection, or nil when unset.
func clientAttributesOf(dto *model.ApplicationProcessedDTO) []string {
	cfg := dto.InboundAuthConfig[0].OAuthConfig.Token
	if cfg == nil || cfg.AccessToken == nil || cfg.AccessToken.ClientConfig == nil {
		return nil
	}
	return cfg.AccessToken.ClientConfig.Attributes
}

// Creation selects sub_type whenever the client_credentials grant is present, alone or not.
func (s *ApplicationTypeTestSuite) TestSeedClientSubTypeAttributeForClientCredentialsGrants() {
	for _, grantTypes := range [][]providers.GrantType{
		{providers.GrantTypeClientCredentials},
		{providers.GrantTypeAuthorizationCode, providers.GrantTypeClientCredentials},
	} {
		dto := processedDTOWithGrants(grantTypes, nil)

		seedClientSubTypeAttribute(dto)

		s.Equal([]string{"sub_type"}, clientAttributesOf(dto), "grants %v should receive the claim", grantTypes)
	}
}

// An application that never receives a token for itself gets no selection.
func (s *ApplicationTypeTestSuite) TestSeedClientSubTypeAttributeSkipsNonClientGrants() {
	dto := processedDTOWithGrants([]providers.GrantType{providers.GrantTypeAuthorizationCode}, nil)

	seedClientSubTypeAttribute(dto)

	s.Nil(clientAttributesOf(dto), "an application with no client_credentials grant must not be seeded")
}

// Seeding adds to the caller's selection rather than replacing it.
func (s *ApplicationTypeTestSuite) TestSeedClientSubTypeAttributeAppendsToCallerSelection() {
	dto := processedDTOWithGrants([]providers.GrantType{providers.GrantTypeClientCredentials}, []string{"ouId"})

	seedClientSubTypeAttribute(dto)

	s.Equal([]string{"ouId", "sub_type"}, clientAttributesOf(dto))
}
