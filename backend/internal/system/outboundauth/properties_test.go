// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

type PropertiesTestSuite struct {
	suite.Suite
}

func TestPropertiesTestSuite(t *testing.T) {
	suite.Run(t, new(PropertiesTestSuite))
}

func (s *PropertiesTestSuite) SetupSuite() {
	setUpCryptoProvider(s.T())
}

func (s *PropertiesTestSuite) TestOwnsPropertyKey() {
	s.Equal("authentication_type", PropertyKeyType)
	s.True(OwnsPropertyKey(PropertyKeyType))
	s.True(OwnsPropertyKey(propertyKey(FieldBasicPassword)))
	s.False(OwnsPropertyKey("host"))
	s.False(OwnsPropertyKey("username"))

	// A transport property that merely starts with "auth" is not claimed by the prefix.
	s.False(OwnsPropertyKey("auth_token"))
}

func (s *PropertiesTestSuite) TestToPropertiesBasic() {
	props, err := ToProperties(basicConfig())
	s.Require().NoError(err)

	values := propertyMap(s.T(), props)
	s.Equal(string(TypeBasic), values[PropertyKeyType])
	s.Equal("mailer", values[propertyKey(FieldBasicUsername)])
	s.Equal("s3cret", values[propertyKey(FieldBasicPassword)])

	byName := make(map[string]*cmodels.Property, len(props))
	for i := range props {
		byName[props[i].GetName()] = &props[i]
	}
	s.True(byName[propertyKey(FieldBasicPassword)].IsSecret())
	s.False(byName[propertyKey(FieldBasicUsername)].IsSecret())
	s.False(byName[PropertyKeyType].IsSecret())
}

func (s *PropertiesTestSuite) TestToPropertiesAlwaysWritesTypeIncludingNone() {
	props, err := ToProperties(Config{Type: TypeNone})
	s.Require().NoError(err)
	s.Require().Len(props, 1)
	s.Equal(PropertyKeyType, props[0].GetName())

	// A zero Config is treated as none rather than rejected, so an omitted block is storable.
	props, err = ToProperties(Config{})
	s.Require().NoError(err)
	s.Require().Len(props, 1)
	values := propertyMap(s.T(), props)
	s.Equal(string(TypeNone), values[PropertyKeyType])
}

func (s *PropertiesTestSuite) TestToPropertiesSkipsBlankAndDropsUndeclaredFields() {
	cfg := Config{
		Type: TypeBasic,
		Properties: map[string]string{
			FieldBasicUsername: "   ",
			FieldBasicPassword: "s3cret",
			// A caller must not be able to smuggle an unencrypted key into the bag.
			"clientSecret": "should-be-dropped",
		},
	}

	props, err := ToProperties(cfg)
	s.Require().NoError(err)

	values := propertyMap(s.T(), props)
	s.NotContains(values, propertyKey(FieldBasicUsername))
	s.NotContains(values, propertyKey("clientSecret"))
	s.Equal("s3cret", values[propertyKey(FieldBasicPassword)])
}

func (s *PropertiesTestSuite) TestToPropertiesRejectsUnknownType() {
	_, err := ToProperties(Config{Type: Type("digest")})
	s.Require().Error(err)
	s.Contains(err.Error(), "unsupported authentication type")
	s.Require().ErrorIs(err, errUnsupportedType)
	s.Contains(err.Error(), "digest")
}

func (s *PropertiesTestSuite) TestRoundTripThroughProperties() {
	props, err := ToProperties(basicConfig())
	s.Require().NoError(err)

	cfg, err := FromProperties(props)
	s.Require().NoError(err)
	s.Equal(TypeBasic, cfg.Type)
	s.Equal("mailer", cfg.Get(FieldBasicUsername))
	s.Equal("s3cret", cfg.Get(FieldBasicPassword))
}

func (s *PropertiesTestSuite) TestFromPropertiesIgnoresForeignKeys() {
	// A transport's own property bag must not be read as an authentication configuration.
	accountSID, err := cmodels.NewProperty("account_sid", "AC123", false)
	s.Require().NoError(err)
	authToken, err := cmodels.NewProperty("auth_token", "transport-secret", true)
	s.Require().NoError(err)
	senderID, err := cmodels.NewProperty("sender_id", "+15550100", false)
	s.Require().NoError(err)

	cfg, err := FromProperties([]cmodels.Property{*accountSID, *authToken, *senderID})
	s.Require().NoError(err)
	s.Equal(TypeNone, cfg.Type)
	s.Empty(cfg.Properties)
}

func (s *PropertiesTestSuite) TestUpdatePropertiesRetainsOnlyForSameTarget() {
	stored, err := ToProperties(basicConfig())
	s.Require().NoError(err)
	request := &Authentication{Type: string(TypeBasic), Properties: map[string]string{
		FieldBasicUsername: "mailer", FieldBasicPassword: "******",
	}}
	updated, err := UpdateProperties(request, stored, []string{"smtp.example.com"},
		[]string{"smtp.example.com"}, []Type{TypeNone, TypeBasic}, nil, nil)
	s.Require().NoError(err)
	config, err := FromProperties(updated)
	s.Require().NoError(err)
	s.Equal("s3cret", config.Get(FieldBasicPassword))

	_, err = UpdateProperties(request, stored, []string{"smtp.example.com"},
		[]string{"other.example.com"}, []Type{TypeNone, TypeBasic}, nil, nil)
	s.Require().ErrorIs(err, ErrAuthenticationRequiredForTargetChange)
	_, err = UpdateProperties(nil, stored, []string{"smtp.example.com"},
		[]string{"other.example.com"}, []Type{TypeNone, TypeBasic}, nil, nil)
	s.Require().ErrorIs(err, ErrAuthenticationRequiredForTargetChange)
}

func (s *PropertiesTestSuite) TestUpdatePropertiesDoesNotRetainSecretForChangedIdentity() {
	stored, err := ToProperties(basicConfig())
	s.Require().NoError(err)
	request := &Authentication{Type: string(TypeBasic), Properties: map[string]string{
		FieldBasicUsername: "other", FieldBasicPassword: "******",
	}}
	_, err = UpdateProperties(request, stored, []string{"smtp.example.com"},
		[]string{"smtp.example.com"}, []Type{TypeNone, TypeBasic}, nil, nil)
	s.Require().Error(err)
}

func (s *PropertiesTestSuite) TestAuthenticationFromPropertiesMasksSecrets() {
	stored, err := ToProperties(basicConfig())
	s.Require().NoError(err)
	authentication, err := AuthenticationFromProperties(stored)
	s.Require().NoError(err)
	s.Equal(string(TypeBasic), authentication.Type)
	s.Equal("mailer", authentication.Properties[FieldBasicUsername])
	s.Equal("******", authentication.Properties[FieldBasicPassword])
}

func (s *PropertiesTestSuite) TestFromValuesSurfacesUnknownStoredType() {
	cfg := FromValues(map[string]string{PropertyKeyType: "digest"})
	s.Equal(Type("digest"), cfg.Type)

	// Surfacing it verbatim lets Validate reject it descriptively instead of silently
	// downgrading a sender to unauthenticated.
	err := Validate(cfg, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unsupported authentication type")
}

func (s *PropertiesTestSuite) TestFromValuesReadsMaskedSecrets() {
	cfg := FromValues(map[string]string{
		PropertyKeyType:                    string(TypeBasic),
		propertyKey(FieldBasicUsername):    "mailer",
		propertyKey(FieldBasicPassword):    "******",
		propertyKey("somethingUndeclared"): "ignored",
	})

	s.Equal(TypeBasic, cfg.Type)
	s.Equal("mailer", cfg.Get(FieldBasicUsername))
	s.Equal("******", cfg.Get(FieldBasicPassword))
	s.Len(cfg.Properties, 2)
}
