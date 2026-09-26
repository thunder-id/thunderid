// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type MappingTestSuite struct {
	suite.Suite
}

func TestMappingSuite(t *testing.T) {
	suite.Run(t, new(MappingTestSuite))
}

func (s *MappingTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
}

func (s *MappingTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *MappingTestSuite) TestAppendPropertySkipsEmpty() {
	props, err := appendProperty(nil, idp.PropClientID, "", false)
	s.NoError(err)
	s.Empty(props)

	props, err = appendProperty(props, idp.PropClientID, "client-1", false)
	s.NoError(err)
	s.Len(props, 1)
	s.False(props[0].IsSecret())
	v, err := props[0].GetValue()
	s.NoError(err)
	s.Equal("client-1", v)
}

func (s *MappingTestSuite) TestAppendPropertyEncryptsSecret() {
	props, err := appendProperty(nil, idp.PropClientSecret, "s3cret", true)
	s.NoError(err)
	s.Len(props, 1)
	s.True(props[0].IsSecret())
	v, err := props[0].GetValue()
	s.NoError(err)
	s.Equal("s3cret", v)
}

func (s *MappingTestSuite) TestPropertyValuesMasksSecret() {
	props := []cmodels.Property{
		mustProperty(s.T(), idp.PropClientID, "client-1", false),
		mustProperty(s.T(), idp.PropClientSecret, "s3cret", true),
	}
	values, err := propertyValues(props)
	s.NoError(err)
	s.Equal("client-1", values[idp.PropClientID])
	s.Equal(maskedSecretValue, values[idp.PropClientSecret])
}

func (s *MappingTestSuite) TestMergeStoredSecretsKeepsOmitted() {
	// Secret omitted from the request → carried over from the stored connection.
	existing := []cmodels.Property{mustProperty(s.T(), idp.PropClientSecret, "stored", true)}

	merged := mergeStoredSecrets(nil, existing)
	s.Len(merged, 1)
	v, err := merged[0].GetValue()
	s.NoError(err)
	s.Equal("stored", v)
}

func (s *MappingTestSuite) TestMergeStoredSecretsUsesProvidedValue() {
	// Secret present in the request → used verbatim; stored value not carried over.
	incoming := []cmodels.Property{mustProperty(s.T(), idp.PropClientSecret, "new", true)}
	existing := []cmodels.Property{mustProperty(s.T(), idp.PropClientSecret, "stored", true)}

	merged := mergeStoredSecrets(incoming, existing)
	s.Len(merged, 1)
	v, err := merged[0].GetValue()
	s.NoError(err)
	s.Equal("new", v)
}

func (s *MappingTestSuite) TestMergeStoredSecretsOnlyBackfillsSecrets() {
	// A non-secret property omitted from the request is NOT carried over.
	existing := []cmodels.Property{mustProperty(s.T(), idp.PropRedirectURI, "https://app/cb", false)}

	merged := mergeStoredSecrets(nil, existing)
	s.Empty(merged)
}

// authBag builds a property bag carrying an authentication method and, optionally, its stored
// password.
func (s *MappingTestSuite) authBag(authType outboundauth.Type, password string) []cmodels.Property {
	props := []cmodels.Property{
		mustProperty(s.T(), outboundauth.PropertyKeyType, string(authType), false),
	}
	if password != "" {
		props = append(props,
			mustProperty(s.T(), outboundauth.PropertyKey(outboundauth.FieldBasicPassword), password, true))
	}
	return props
}

// secretValue returns the value of the authentication password in a merged bag, and whether it
// is present at all.
func (s *MappingTestSuite) secretValue(props []cmodels.Property) (string, bool) {
	for i := range props {
		if props[i].GetName() != outboundauth.PropertyKey(outboundauth.FieldBasicPassword) {
			continue
		}
		value, err := props[i].GetValue()
		s.Require().NoError(err)
		return value, true
	}
	return "", false
}

// Omitting the password while keeping the same method is how an update says "keep the stored
// credential", so it must still be carried over.
func (s *MappingTestSuite) TestMergeStoredSecretsKeepsAuthSecretWhenTypeUnchanged() {
	incoming := s.authBag(outboundauth.TypeBasic, "")
	existing := s.authBag(outboundauth.TypeBasic, "stored")

	value, found := s.secretValue(mergeStoredSecrets(incoming, existing))
	s.True(found)
	s.Equal("stored", value)
}

// Turning authentication off must not leave the credential behind, or a later switch back to
// basic would silently reuse a password the administrator never re-entered.
func (s *MappingTestSuite) TestMergeStoredSecretsDropsAuthSecretWhenSwitchedToNone() {
	incoming := s.authBag(outboundauth.TypeNone, "")
	existing := s.authBag(outboundauth.TypeBasic, "stored")

	_, found := s.secretValue(mergeStoredSecrets(incoming, existing))
	s.False(found, "the previous method's credential must not survive being switched off")
}

// The same applies to switching between two authenticating methods.
func (s *MappingTestSuite) TestMergeStoredSecretsDropsAuthSecretWhenTypeChanges() {
	incoming := []cmodels.Property{
		mustProperty(s.T(), outboundauth.PropertyKeyType, "bearer", false),
	}
	existing := s.authBag(outboundauth.TypeBasic, "stored")

	_, found := s.secretValue(mergeStoredSecrets(incoming, existing))
	s.False(found)
}

// A changed transport key names a different credential target, so the stored credential is not
// carried over to it.
func (s *MappingTestSuite) TestMergeStoredSecretsDropsAuthSecretWhenTargetKeyChanges() {
	incoming := append(s.authBag(outboundauth.TypeBasic, ""),
		mustProperty(s.T(), "host", "smtp.attacker.example", false))
	existing := append(s.authBag(outboundauth.TypeBasic, "stored"),
		mustProperty(s.T(), "host", "smtp.example.com", false))

	_, found := s.secretValue(mergeStoredSecrets(incoming, existing, "host"))
	s.False(found)
}

// Adding a transport key the stored connection did not have is a change too.
func (s *MappingTestSuite) TestMergeStoredSecretsDropsAuthSecretWhenTargetKeyAdded() {
	incoming := append(s.authBag(outboundauth.TypeBasic, ""),
		mustProperty(s.T(), "port", "2525", false))
	existing := s.authBag(outboundauth.TypeBasic, "stored")

	_, found := s.secretValue(mergeStoredSecrets(incoming, existing, "port"))
	s.False(found)
}

// An unchanged transport key keeps the stored credential.
func (s *MappingTestSuite) TestMergeStoredSecretsKeepsAuthSecretWhenTargetUnchanged() {
	incoming := append(s.authBag(outboundauth.TypeBasic, ""),
		mustProperty(s.T(), "host", "smtp.example.com", false),
		mustProperty(s.T(), "from_address", "new@example.com", false))
	existing := append(s.authBag(outboundauth.TypeBasic, "stored"),
		mustProperty(s.T(), "host", "smtp.example.com", false),
		mustProperty(s.T(), "from_address", "old@example.com", false))

	value, found := s.secretValue(mergeStoredSecrets(incoming, existing, "host"))
	s.True(found, "a property outside the credential target must not drop the credential")
	s.Equal("stored", value)
}

// A changed non-secret authentication property, such as the username, presents the credential
// as someone else, so it is not carried over either.
func (s *MappingTestSuite) TestMergeStoredSecretsDropsAuthSecretWhenUsernameChanges() {
	usernameKey := outboundauth.PropertyKey(outboundauth.FieldBasicUsername)
	incoming := append(s.authBag(outboundauth.TypeBasic, ""),
		mustProperty(s.T(), usernameKey, "other", false))
	existing := append(s.authBag(outboundauth.TypeBasic, "stored"),
		mustProperty(s.T(), usernameKey, "mailer", false))

	_, found := s.secretValue(mergeStoredSecrets(incoming, existing))
	s.False(found)
}

// A vendor's own secret is not an outbound-authentication property, so it keeps the original
// carry-over behavior regardless of what the authentication block does.
func (s *MappingTestSuite) TestMergeStoredSecretsStillCarriesVendorSecrets() {
	incoming := s.authBag(outboundauth.TypeNone, "")
	existing := append(s.authBag(outboundauth.TypeBasic, "stored"),
		mustProperty(s.T(), "auth_token", "twilio-secret", true))

	merged := mergeStoredSecrets(incoming, existing)

	var carried string
	for i := range merged {
		if merged[i].GetName() == "auth_token" {
			value, err := merged[i].GetValue()
			s.Require().NoError(err)
			carried = value
		}
	}
	s.Equal("twilio-secret", carried)
}

func (s *MappingTestSuite) TestScopes() {
	s.Equal("openid,email", joinScopes([]string{"openid", "email"}))
	s.Equal([]string{"openid", "email"}, splitScopes("openid,email"))
	s.Nil(splitScopes(""))
}

func (s *MappingTestSuite) TestWriteServiceErrorStatusMapping() {
	cases := []struct {
		svcErr *tidcommon.ServiceError
		want   int
	}{
		{&idp.ErrorIDPNotFound, http.StatusNotFound},
		{&idp.ErrorIDPAlreadyExists, http.StatusConflict},
		{&idp.ErrorIDPHasBlockingDependencies, http.StatusConflict},
		{&idp.ErrorInvalidIDPID, http.StatusBadRequest},
		{&notification.ErrorSenderNotFound, http.StatusNotFound},
		{&notification.ErrorDuplicateSenderName, http.StatusConflict},
		{&notification.ErrorSenderHasBlockingDependencies, http.StatusConflict},
		{&notification.ErrorInvalidProvider, http.StatusBadRequest},
		{&authzenpdp.ErrorHasBlockingDependencies, http.StatusConflict},
		{&tidcommon.InternalServerError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		writeServiceError(context.Background(), rr, tc.svcErr)
		s.Equal(tc.want, rr.Code, tc.svcErr.Code)
	}
}

func (s *MappingTestSuite) TestWriteInvalidBody() {
	rr := httptest.NewRecorder()
	writeInvalidBody(context.Background(), rr)
	s.Equal(http.StatusBadRequest, rr.Code)
}
