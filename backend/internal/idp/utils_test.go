/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package idp

import (
	"context"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type IDPUtilsTestSuite struct {
	suite.Suite
	logger *log.Logger
}

func TestIDPUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(IDPUtilsTestSuite))
}

func (s *IDPUtilsTestSuite) SetupTest() {
	s.logger = log.GetLogger()
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_OAuth_AllRequired() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "http://idp/auth", false)
	prop5, _ := cmodels.NewProperty(PropTokenEndpoint, "http://idp/token", false)
	prop6, _ := cmodels.NewProperty(PropUserInfoEndpoint, "http://idp/userinfo", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3, *prop4, *prop5, *prop6}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)
	s.Len(result, 6)
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_OAuth_WithOptional() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "http://idp/auth", false)
	prop5, _ := cmodels.NewProperty(PropTokenEndpoint, "http://idp/token", false)
	prop6, _ := cmodels.NewProperty(PropUserInfoEndpoint, "http://idp/userinfo", false)
	prop7, _ := cmodels.NewProperty(PropScopes, "profile,email", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3, *prop4, *prop5, *prop6, *prop7}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)
	s.Len(result, 7)
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_OAuth_MissingRequired() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)

	properties := []cmodels.Property{*prop1, *prop2}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "required property")
	s.Contains(err.ErrorDescription.DefaultValue, "missing")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_OIDC_AllRequired() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "http://idp/auth", false)
	prop5, _ := cmodels.NewProperty(PropTokenEndpoint, "http://idp/token", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3, *prop4, *prop5}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOIDC, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)
	s.GreaterOrEqual(len(result), 6)

	hasOpenIDScope := false
	for _, prop := range result {
		if prop.GetName() == PropScopes {
			value, _ := prop.GetValue()
			s.Contains(value, "openid")
			hasOpenIDScope = true
		}
	}
	s.True(hasOpenIDScope, "OIDC should have openid scope added")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_OIDC_WithExistingScopes() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "http://idp/auth", false)
	prop5, _ := cmodels.NewProperty(PropTokenEndpoint, "http://idp/token", false)
	prop6, _ := cmodels.NewProperty(PropScopes, "profile,email", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3, *prop4, *prop5, *prop6}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOIDC, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)

	for _, prop := range result {
		if prop.GetName() == PropScopes {
			value, _ := prop.GetValue()
			s.Contains(value, "openid")
			s.Contains(value, "profile")
			s.Contains(value, "email")
		}
	}
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_OIDC_ScopesAlreadyHasOpenID() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "http://idp/auth", false)
	prop5, _ := cmodels.NewProperty(PropTokenEndpoint, "http://idp/token", false)
	prop6, _ := cmodels.NewProperty(PropScopes, "openid,profile,email", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3, *prop4, *prop5, *prop6}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOIDC, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)

	for _, prop := range result {
		if prop.GetName() == PropScopes {
			value, _ := prop.GetValue()
			s.Contains(value, "openid")
		}
	}
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_Google_WithDefaults() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeGoogle, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)
	s.GreaterOrEqual(len(result), 7)

	foundProperties := make(map[string]string)
	for _, prop := range result {
		value, _ := prop.GetValue()
		foundProperties[prop.GetName()] = value
	}

	s.Equal(googleAuthorizationEndpoint, foundProperties[PropAuthorizationEndpoint])
	s.Equal(googleTokenEndpoint, foundProperties[PropTokenEndpoint])
	s.Equal(googleUserInfoEndpoint, foundProperties[PropUserInfoEndpoint])
	s.Equal(googleJwksEndpoint, foundProperties[PropJwksEndpoint])
	s.Contains(foundProperties[PropScopes], "openid")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_Google_WithCustomEndpoints() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "http://custom/auth", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3, *prop4}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeGoogle, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)

	foundProperties := make(map[string]string)
	for _, prop := range result {
		value, _ := prop.GetValue()
		foundProperties[prop.GetName()] = value
	}

	s.Equal("http://custom/auth", foundProperties[PropAuthorizationEndpoint])
	s.Equal(googleTokenEndpoint, foundProperties[PropTokenEndpoint])
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_GitHub_WithDefaults() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeGitHub, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)
	s.GreaterOrEqual(len(result), 6)

	foundProperties := make(map[string]string)
	for _, prop := range result {
		value, _ := prop.GetValue()
		foundProperties[prop.GetName()] = value
	}

	s.Equal(gitHubAuthorizationEndpoint, foundProperties[PropAuthorizationEndpoint])
	s.Equal(gitHubTokenEndpoint, foundProperties[PropTokenEndpoint])
	s.Equal(gitHubUserInfoEndpoint, foundProperties[PropUserInfoEndpoint])
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_GitHub_WithCustomEndpoints() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropTokenEndpoint, "http://custom/token", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3, *prop4}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeGitHub, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)

	foundProperties := make(map[string]string)
	for _, prop := range result {
		value, _ := prop.GetValue()
		foundProperties[prop.GetName()] = value
	}

	s.Equal("http://custom/token", foundProperties[PropTokenEndpoint])
	s.Equal(gitHubAuthorizationEndpoint, foundProperties[PropAuthorizationEndpoint])
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_EmptyPropertyName() {
	prop1, _ := cmodels.NewProperty("", "test-value", false)

	properties := []cmodels.Property{*prop1}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "property names cannot be empty")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_EmptyPropertyValue() {
	prop1, _ := cmodels.NewProperty(PropClientID, "", false)

	properties := []cmodels.Property{*prop1}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "value cannot be empty")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_UnsupportedProperty() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty("unsupported_prop", "value", false)

	properties := []cmodels.Property{*prop1, *prop2}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorUnsupportedIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "unsupported_prop")
	s.Contains(err.ErrorDescription.DefaultValue, "not supported")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_InvalidIDPType() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)

	properties := []cmodels.Property{*prop1}

	result, err := validateIDPProperties(context.Background(), providers.IDPType("INVALID"), properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestPropertyMapToSlice() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)

	propertyMap := map[string]cmodels.Property{
		PropClientID:     *prop1,
		PropClientSecret: *prop2,
	}

	result := propertyMapToSlice(propertyMap)

	s.NotNil(result)
	s.Len(result, 2)

	names := make([]string, 0)
	for _, prop := range result {
		names = append(names, prop.GetName())
	}
	s.Contains(names, PropClientID)
	s.Contains(names, PropClientSecret)
}

func (s *IDPUtilsTestSuite) TestEnsureOpenIDScope_NoExistingScopes() {
	propertyMap := make(map[string]cmodels.Property)

	err := ensureOpenIDScope(context.Background(), propertyMap, s.logger)

	s.Nil(err)
	s.Contains(propertyMap, PropScopes)

	scopesProp := propertyMap[PropScopes]
	value, _ := scopesProp.GetValue()
	s.Equal("openid", value)
}

func (s *IDPUtilsTestSuite) TestEnsureOpenIDScope_WithExistingScopes() {
	prop, _ := cmodels.NewProperty(PropScopes, "profile,email", false)
	propertyMap := map[string]cmodels.Property{
		PropScopes: *prop,
	}

	err := ensureOpenIDScope(context.Background(), propertyMap, s.logger)

	s.Nil(err)

	scopesProp := propertyMap[PropScopes]
	value, _ := scopesProp.GetValue()
	s.Contains(value, "openid")
	s.Contains(value, "profile")
	s.Contains(value, "email")
}

func (s *IDPUtilsTestSuite) TestEnsureOpenIDScope_AlreadyHasOpenID() {
	prop, _ := cmodels.NewProperty(PropScopes, "openid,profile", false)
	propertyMap := map[string]cmodels.Property{
		PropScopes: *prop,
	}

	err := ensureOpenIDScope(context.Background(), propertyMap, s.logger)

	s.Nil(err)

	scopesProp := propertyMap[PropScopes]
	value, _ := scopesProp.GetValue()
	s.Contains(value, "openid")
	s.Contains(value, "profile")
}

func (s *IDPUtilsTestSuite) TestEnsureOpenIDScope_EmptyScopesValue() {
	prop, _ := cmodels.NewProperty(PropScopes, "", false)
	propertyMap := map[string]cmodels.Property{
		PropScopes: *prop,
	}

	err := ensureOpenIDScope(context.Background(), propertyMap, s.logger)

	s.Nil(err)

	scopesProp := propertyMap[PropScopes]
	value, _ := scopesProp.GetValue()
	s.Equal("openid", value)
}

func (s *IDPUtilsTestSuite) TestValidateIDP_ValidOAuth() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "http://idp/auth", false)
	prop5, _ := cmodels.NewProperty(PropTokenEndpoint, "http://idp/token", false)
	prop6, _ := cmodels.NewProperty(PropUserInfoEndpoint, "http://idp/userinfo", false)

	idp := &providers.IDPDTO{
		Name:       "Test OAuth IDP",
		Type:       providers.IDPTypeOAuth,
		Properties: []cmodels.Property{*prop1, *prop2, *prop3, *prop4, *prop5, *prop6},
	}

	err := validateIDP(context.Background(), idp, s.logger)

	s.Nil(err)
	s.NotNil(idp.Properties)
}

func (s *IDPUtilsTestSuite) TestValidateIDP_NilIDP() {
	err := validateIDP(context.Background(), nil, s.logger)

	s.NotNil(err)
	s.Equal(ErrorIDPNil.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestValidateIDP_EmptyName() {
	idp := &providers.IDPDTO{
		Name: "",
		Type: providers.IDPTypeOAuth,
	}

	err := validateIDP(context.Background(), idp, s.logger)

	s.NotNil(err)
	s.Equal(ErrorInvalidIDPName.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestValidateIDP_EmptyType() {
	idp := &providers.IDPDTO{
		Name: "Test IDP",
		Type: "",
	}

	err := validateIDP(context.Background(), idp, s.logger)

	s.NotNil(err)
	s.Equal(ErrorInvalidIDPType.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestValidateIDP_InvalidType() {
	idp := &providers.IDPDTO{
		Name: "Test IDP",
		Type: "INVALID",
	}

	err := validateIDP(context.Background(), idp, s.logger)

	s.NotNil(err)
	s.Equal(ErrorInvalidIDPType.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestValidateIDP_WithWhitespaceName() {
	prop1, _ := cmodels.NewProperty(PropClientID, "test-client", false)
	prop2, _ := cmodels.NewProperty(PropClientSecret, "test-secret", false)
	prop3, _ := cmodels.NewProperty(PropRedirectURI, "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "http://idp/auth", false)
	prop5, _ := cmodels.NewProperty(PropTokenEndpoint, "http://idp/token", false)
	prop6, _ := cmodels.NewProperty(PropUserInfoEndpoint, "http://idp/userinfo", false)

	idp := &providers.IDPDTO{
		Name:       "   ",
		Type:       providers.IDPTypeOAuth,
		Properties: []cmodels.Property{*prop1, *prop2, *prop3, *prop4, *prop5, *prop6},
	}

	err := validateIDP(context.Background(), idp, s.logger)

	s.NotNil(err)
	s.Equal(ErrorInvalidIDPName.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestValidateIDP_WithWhitespaceType() {
	idp := &providers.IDPDTO{
		Name: "Test IDP",
		Type: "   ",
	}

	err := validateIDP(context.Background(), idp, s.logger)

	s.NotNil(err)
	s.Equal(ErrorInvalidIDPType.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_WithWhitespacePropertyName() {
	prop1, _ := cmodels.NewProperty("   ", "test-value", false)

	properties := []cmodels.Property{*prop1}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "property names cannot be empty")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_WithWhitespacePropertyValue() {
	prop1, _ := cmodels.NewProperty(PropClientID, "   ", false)

	properties := []cmodels.Property{*prop1}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "value cannot be empty")
}

func (s *IDPUtilsTestSuite) TestCreateAndAppendProperty_Success() {
	propertyMap := make(map[string]cmodels.Property)

	err := createAndAppendProperty(context.Background(), propertyMap, "test_prop", "test_value", false, s.logger)

	s.Nil(err)
	s.Contains(propertyMap, "test_prop")

	prop := propertyMap["test_prop"]
	value, _ := prop.GetValue()
	s.Equal("test_value", value)
	s.False(prop.IsSecret())
}

func (s *IDPUtilsTestSuite) TestCreateAndAppendProperty_OverwriteExisting() {
	prop1, _ := cmodels.NewProperty("test_prop", "old_value", false)
	propertyMap := map[string]cmodels.Property{
		"test_prop": *prop1,
	}

	err := createAndAppendProperty(context.Background(), propertyMap, "test_prop", "new_value", false, s.logger)

	s.Nil(err)
	s.Contains(propertyMap, "test_prop")

	prop := propertyMap["test_prop"]
	value, _ := prop.GetValue()
	s.Equal("new_value", value)
}

func (s *IDPUtilsTestSuite) TestEnsureOpenIDScope_WithWhitespaceScopes() {
	prop, _ := cmodels.NewProperty(PropScopes, "   ", false)
	propertyMap := map[string]cmodels.Property{
		PropScopes: *prop,
	}

	err := ensureOpenIDScope(context.Background(), propertyMap, s.logger)

	s.Nil(err)

	scopesProp := propertyMap[PropScopes]
	value, _ := scopesProp.GetValue()
	s.Equal("openid", value)
}

func (s *IDPUtilsTestSuite) TestEnsureOpenIDScope_CommaSeparatedScopes() {
	prop, _ := cmodels.NewProperty(PropScopes, "profile,email,address", false)
	propertyMap := map[string]cmodels.Property{
		PropScopes: *prop,
	}

	err := ensureOpenIDScope(context.Background(), propertyMap, s.logger)

	s.Nil(err)

	scopesProp := propertyMap[PropScopes]
	value, _ := scopesProp.GetValue()
	// Should have openid added
	s.Contains(value, "openid")
	s.Contains(value, "profile")
	s.Contains(value, "email")
	s.Contains(value, "address")
	// Verify comma separation
	s.NotContains(value, " ")
}

func (s *IDPUtilsTestSuite) TestEnsureOpenIDScope_WithEmptyStringInScopes() {
	prop, _ := cmodels.NewProperty(PropScopes, "profile,,email,,", false)
	propertyMap := map[string]cmodels.Property{
		PropScopes: *prop,
	}

	err := ensureOpenIDScope(context.Background(), propertyMap, s.logger)
	s.Nil(err)

	scopesProp := propertyMap[PropScopes]
	value, _ := scopesProp.GetValue()
	// Should have openid added and empty strings filtered out
	s.Contains(value, "openid")
	s.Contains(value, "profile")
	s.Contains(value, "email")
	// Should not have consecutive commas
	s.NotContains(value, ",,")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_TokenExchangeOnly_OIDC_Succeeds() {
	// OIDC IDP with only the token-exchange required props and token_exchange_enabled=true should succeed.
	// client_id is no longer required for token exchange; issuer and jwks_endpoint are sufficient.
	prop1, _ := cmodels.NewProperty(PropIssuer, "https://api.asgardeo.io/t/myorg/oauth2/token", false)
	prop2, _ := cmodels.NewProperty(PropJwksEndpoint, "https://api.asgardeo.io/t/myorg/oauth2/jwks", false)
	prop3, _ := cmodels.NewProperty(PropTokenExchangeEnabled, "true", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOIDC, properties, s.logger)

	s.Nil(err)
	s.NotNil(result)
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_TokenExchangeEnabled_MissingIssuer_Fails() {
	// OIDC IDP with token_exchange_enabled=true but missing issuer should fail.
	prop1, _ := cmodels.NewProperty(PropClientID, "your_client_id", false)
	prop2, _ := cmodels.NewProperty(PropJwksEndpoint, "https://api.asgardeo.io/t/myorg/oauth2/jwks", false)
	prop3, _ := cmodels.NewProperty(PropTokenExchangeEnabled, "true", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOIDC, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "required property")
	s.Contains(err.ErrorDescription.DefaultValue, PropIssuer)
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_TokenExchangeEnabled_MissingJWKS_Fails() {
	// OIDC IDP with token_exchange_enabled=true but missing jwks_endpoint should fail.
	prop1, _ := cmodels.NewProperty(PropClientID, "your_client_id", false)
	prop2, _ := cmodels.NewProperty(PropIssuer, "https://api.asgardeo.io/t/myorg/oauth2/token", false)
	prop3, _ := cmodels.NewProperty(PropTokenExchangeEnabled, "true", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOIDC, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "required property")
	s.Contains(err.ErrorDescription.DefaultValue, PropJwksEndpoint)
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_OIDCWithoutTokenExchange_StillRequiresRedirectProps() {
	// OIDC IDP without token_exchange_enabled must still require all 5 redirect-flow props.
	prop1, _ := cmodels.NewProperty(PropClientID, "your_client_id", false)
	prop2, _ := cmodels.NewProperty(PropIssuer, "https://api.asgardeo.io/t/myorg/oauth2/token", false)
	prop3, _ := cmodels.NewProperty(PropJwksEndpoint, "https://api.asgardeo.io/t/myorg/oauth2/jwks", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOIDC, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "required property")
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_OIDCWithoutTokenExchange_MissingClientSecret_Fails() {
	// OIDC IDP without token_exchange_enabled must fail when client_secret is missing.
	prop1, _ := cmodels.NewProperty(PropClientID, "your_client_id", false)
	prop2, _ := cmodels.NewProperty(PropRedirectURI, "https://thunder.example.com/callback", false)
	prop3, _ := cmodels.NewProperty(PropAuthorizationEndpoint, "https://api.asgardeo.io/t/myorg/oauth2/authorize",
		false)
	prop4, _ := cmodels.NewProperty(PropTokenEndpoint, "https://api.asgardeo.io/t/myorg/oauth2/token", false)

	properties := []cmodels.Property{*prop1, *prop2, *prop3, *prop4}

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOIDC, properties, s.logger)

	s.NotNil(err)
	s.Nil(result)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "required property")
	s.Contains(err.ErrorDescription.DefaultValue, PropClientSecret)
}

func (s *IDPUtilsTestSuite) TestValidateIDP_PropertyValidationFailure() {
	prop, _ := cmodels.NewProperty("", "value", false)
	idp := &providers.IDPDTO{
		Name:       "Test IDP",
		Type:       providers.IDPTypeOAuth,
		Properties: []cmodels.Property{*prop},
	}

	err := validateIDP(context.Background(), idp, s.logger)

	s.NotNil(err)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestValidateIDPProperties_SecretPropertyValueUnreadable() {
	// Initialize the server runtime with the test crypto key; the secret
	// property below holds a value that is not valid ciphertext, so reading
	// it fails on decryption.
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
	})
	defer config.ResetServerRuntime()

	properties, dErr := cmodels.DeserializePropertiesFromJSONObject(
		`{"client_secret":{"value":"not-valid-ciphertext","isSecret":true}}`)
	s.NoError(dErr)

	result, err := validateIDPProperties(context.Background(), providers.IDPTypeOAuth, properties, s.logger)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPProperty.Code, err.Code)
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_NilMappings_NoOp() {
	attrs := map[string]interface{}{"email": "user@example.com"}
	result := ApplyAttributeMappings(attrs, nil)
	s.Equal(attrs, result)
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_EmptyMappings_NoOp() {
	attrs := map[string]interface{}{"email": "user@example.com"}
	result := ApplyAttributeMappings(attrs, []providers.AttributeMapping{})
	s.Equal(attrs, result)
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_RenamesMappedClaim() {
	attrs := map[string]interface{}{
		"http://schemas.example.com/emailaddress": "user@example.com",
	}
	result := ApplyAttributeMappings(attrs, []providers.AttributeMapping{
		{ExternalAttribute: "http://schemas.example.com/emailaddress", LocalAttribute: "email"},
	})
	s.Equal("user@example.com", result["email"])
	_, present := result["http://schemas.example.com/emailaddress"]
	s.False(present)
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_OneSourceToMultipleTargets() {
	attrs := map[string]interface{}{"email": "user@example.com"}
	result := ApplyAttributeMappings(attrs, []providers.AttributeMapping{
		{ExternalAttribute: "email", LocalAttribute: "email"},
		{ExternalAttribute: "email", LocalAttribute: "contactEmail"},
	})
	s.Equal("user@example.com", result["email"])
	s.Equal("user@example.com", result["contactEmail"])
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_UnmappedPassesThrough() {
	attrs := map[string]interface{}{
		"given_name": "Jane",
		"department": "engineering",
	}
	result := ApplyAttributeMappings(
		attrs, []providers.AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}})
	s.Equal("Jane", result["firstName"])
	s.Equal("engineering", result["department"])
	_, present := result["given_name"]
	s.False(present)
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_MappedValueOverridesCollision() {
	attrs := map[string]interface{}{
		"given_name": "Jane",
		"firstName":  "stale",
	}
	result := ApplyAttributeMappings(
		attrs, []providers.AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}})
	s.Equal("Jane", result["firstName"])
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_MissingExternalClaimIgnored() {
	attrs := map[string]interface{}{"email": "user@example.com"}
	result := ApplyAttributeMappings(
		attrs, []providers.AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}})
	s.Equal("user@example.com", result["email"])
	_, present := result["firstName"]
	s.False(present)
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_NestedSourcePath() {
	attrs := map[string]interface{}{
		"address": map[string]interface{}{
			"email": "user@example.com",
		},
		"keep": "value",
	}
	result := ApplyAttributeMappings(
		attrs, []providers.AttributeMapping{{ExternalAttribute: "address.email", LocalAttribute: "email"}})
	s.Equal("user@example.com", result["email"])
	// The containing nested object is read non-destructively and passes through.
	s.Equal(attrs["address"], result["address"])
	s.Equal("value", result["keep"])
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_DottedKeyMatchedLiterallyBeforePath() {
	// A URI-style claim name containing dots must be matched literally, not split into a path.
	attrs := map[string]interface{}{
		"http://schemas.example.com/emailaddress": "user@example.com",
	}
	result := ApplyAttributeMappings(attrs, []providers.AttributeMapping{
		{ExternalAttribute: "http://schemas.example.com/emailaddress", LocalAttribute: "email"},
	})
	s.Equal("user@example.com", result["email"])
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_SubPreservedWhenMappedToMultipleTargets() {
	attrs := map[string]interface{}{"sub": "user-123"}
	result := ApplyAttributeMappings(attrs, []providers.AttributeMapping{
		{ExternalAttribute: "sub", LocalAttribute: "username"},
		{ExternalAttribute: "sub", LocalAttribute: "email"},
	})
	s.Equal("user-123", result["sub"])
	s.Equal("user-123", result["username"])
	s.Equal("user-123", result["email"])
}

func (s *IDPUtilsTestSuite) TestApplyAttributeMappings_SubPreservedAlongsideOtherRenamedSources() {
	attrs := map[string]interface{}{
		"sub":        "user-123",
		"given_name": "Jane",
	}
	result := ApplyAttributeMappings(attrs, []providers.AttributeMapping{
		{ExternalAttribute: "sub", LocalAttribute: "picture"},
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
	})
	s.Equal("user-123", result["sub"])
	s.Equal("user-123", result["picture"])
	s.Equal("Jane", result["firstName"])
	// Non-sub sources are still consumed by the mapping.
	_, present := result["given_name"]
	s.False(present)
}

func (s *IDPUtilsTestSuite) TestGetAttributeMappings_NilIDP() {
	s.Nil(GetAttributeMappings(nil))
}

func (s *IDPUtilsTestSuite) TestGetAttributeMappings_NilAttributeConfiguration() {
	s.Nil(GetAttributeMappings(&providers.IDPDTO{}))
}

func (s *IDPUtilsTestSuite) TestGetAttributeMappings_ReturnsMappings() {
	mappings := []providers.AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}}
	idpDTO := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution:        &providers.UserTypeResolution{Default: "person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{UserType: "person", Attributes: mappings}},
	}}
	s.Equal(mappings, GetAttributeMappings(idpDTO))
}

func (s *IDPUtilsTestSuite) TestGetMappedUserType_NilIDP() {
	s.Equal("", GetMappedUserType(nil))
}

func (s *IDPUtilsTestSuite) TestGetMappedUserType_NilAttributeConfiguration() {
	s.Equal("", GetMappedUserType(&providers.IDPDTO{}))
}

func (s *IDPUtilsTestSuite) TestGetMappedUserType_ReturnsEntityType() {
	s.Equal("person", GetMappedUserType(&providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "person"},
	}}))
}
