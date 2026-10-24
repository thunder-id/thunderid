// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ConstantsTestSuite struct {
	suite.Suite
}

func TestConstantsSuite(t *testing.T) {
	suite.Run(t, new(ConstantsTestSuite))
}

func (suite *ConstantsTestSuite) TestNodeVariant_String() {
	assert.Equal(suite.T(), "LOGIN_OPTIONS", NodeVariantLoginOptions.String())
	assert.Equal(suite.T(), "CUSTOM", NodeVariant("CUSTOM").String())
}

func (suite *ConstantsTestSuite) TestGrantType_IsValid() {
	valid := []GrantType{
		GrantTypeAuthorizationCode,
		GrantTypeClientCredentials,
		GrantTypeRefreshToken,
		GrantTypeTokenExchange,
		GrantTypeCIBA,
	}
	for _, gt := range valid {
		assert.True(suite.T(), gt.IsValid(), "expected %q to be valid", gt)
	}
	assert.False(suite.T(), GrantType("implicit").IsValid())
	assert.False(suite.T(), GrantType("").IsValid())
}

func (suite *ConstantsTestSuite) TestGrantType_IssuesRefreshToken() {
	assert.True(suite.T(), GrantTypeAuthorizationCode.IssuesRefreshToken())
	assert.True(suite.T(), GrantTypeCIBA.IssuesRefreshToken())
	assert.False(suite.T(), GrantTypeClientCredentials.IssuesRefreshToken())
	assert.False(suite.T(), GrantTypeRefreshToken.IssuesRefreshToken())
	assert.False(suite.T(), GrantTypeTokenExchange.IssuesRefreshToken())
	assert.False(suite.T(), GrantTypeJWTBearer.IssuesRefreshToken())
}

func (suite *ConstantsTestSuite) TestAnyIssuesRefreshToken() {
	assert.True(suite.T(), AnyIssuesRefreshToken([]string{"client_credentials", "authorization_code"}))
	assert.True(suite.T(), AnyIssuesRefreshToken([]string{string(GrantTypeCIBA)}))
	assert.False(suite.T(), AnyIssuesRefreshToken([]string{"client_credentials", "refresh_token"}))
	assert.False(suite.T(), AnyIssuesRefreshToken([]string{}))
}

func (suite *ConstantsTestSuite) TestResponseType_IsValid() {
	assert.True(suite.T(), ResponseTypeCode.IsValid())
	assert.False(suite.T(), ResponseTypeIDToken.IsValid())
	assert.False(suite.T(), ResponseType("token").IsValid())
	assert.False(suite.T(), ResponseType("").IsValid())
}

func (suite *ConstantsTestSuite) TestTokenEndpointAuthMethod_IsValid() {
	valid := []TokenEndpointAuthMethod{
		TokenEndpointAuthMethodClientSecretBasic,
		TokenEndpointAuthMethodClientSecretPost,
		TokenEndpointAuthMethodPrivateKeyJWT,
		TokenEndpointAuthMethodNone,
	}
	for _, m := range valid {
		assert.True(suite.T(), m.IsValid(), "expected %q to be valid", m)
	}
	assert.False(suite.T(), TokenEndpointAuthMethod("tls_client_auth").IsValid())
	assert.False(suite.T(), TokenEndpointAuthMethod("").IsValid())
}

func (suite *ConstantsTestSuite) TestEntityCategory_String() {
	assert.Equal(suite.T(), "user", EntityCategoryUser.String())
	assert.Equal(suite.T(), "app", EntityCategoryApp.String())
	assert.Equal(suite.T(), "agent", EntityCategoryAgent.String())
}

func (suite *ConstantsTestSuite) TestEntityState_String() {
	assert.Equal(suite.T(), "ACTIVE", EntityStateActive.String())
}

func (suite *ConstantsTestSuite) TestResourceServerType_IsValid() {
	assert.True(suite.T(), ResourceServerTypeAPI.IsValid())
	assert.True(suite.T(), ResourceServerTypeMCP.IsValid())
	assert.True(suite.T(), ResourceServerTypeCustom.IsValid())
	assert.False(suite.T(), ResourceServerType("UNKNOWN").IsValid())
	assert.False(suite.T(), ResourceServerType("").IsValid())
}

// The selection inputs are rendering hints the client fills from a managed resource, so the engine
// has to recognize them before a flow can declare one.
func (suite *ConstantsTestSuite) TestValidInputTypes_AcceptsTheSelectionInputs() {
	for _, inputType := range []string{
		InputTypeUserSelect, InputTypeOUSelect, InputTypeAuthFlowSelect, InputTypeUserTypeSelect,
	} {
		assert.True(suite.T(), ValidInputTypes[inputType], inputType)
	}
	assert.Equal(suite.T(), "AUTH_FLOW_SELECT", InputTypeAuthFlowSelect)
	assert.Equal(suite.T(), "USER_TYPE_SELECT", InputTypeUserTypeSelect)
	assert.False(suite.T(), ValidInputTypes["FLOW_SELECT"])
}
