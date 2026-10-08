// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

type AuthenticationTestSuite struct {
	suite.Suite
}

func TestAuthenticationTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticationTestSuite))
}

func (s *AuthenticationTestSuite) SetupSuite() {
	setUpCryptoProvider(s.T())
}

func basicAuthentication() *Authentication {
	return &Authentication{
		Type: string(TypeBasic),
		Properties: map[string]string{
			FieldBasicUsername: "mailer",
			FieldBasicPassword: "s3cret",
		},
	}
}

// An omitted block reaches the server as a nil object.
func (s *AuthenticationTestSuite) TestAuthenticationConfigTreatsNilAsNone() {
	var auth *Authentication

	cfg := auth.Config()
	s.Equal(TypeNone, cfg.Type)
	s.False(cfg.Enabled())
}

func (s *AuthenticationTestSuite) TestAuthenticationConfigNormalizesType() {
	auth := basicAuthentication()
	auth.Type = "  BASIC "

	cfg := auth.Config()
	s.Equal(TypeBasic, cfg.Type)
	s.Equal("mailer", cfg.Get(FieldBasicUsername))
	s.Equal("s3cret", cfg.Get(FieldBasicPassword))
}

// An unrecognized type is kept as given so Validate rejects it instead of it becoming "none".
func (s *AuthenticationTestSuite) TestAuthenticationConfigPassesUnknownTypeThrough() {
	cfg := (&Authentication{Type: "bearer"}).Config()
	s.Equal(Type("bearer"), cfg.Type)

	err := Validate(cfg, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unsupported authentication type")
}

// The configuration owns its properties, so a caller changing the wire object afterwards cannot
// alter what is validated and stored.
func (s *AuthenticationTestSuite) TestAuthenticationConfigCopiesProperties() {
	auth := basicAuthentication()

	cfg := auth.Config()
	auth.Properties[FieldBasicPassword] = "changed"

	s.Equal("s3cret", cfg.Get(FieldBasicPassword))
}

func (s *AuthenticationTestSuite) TestAuthenticationFromValuesReadsMaskedSecrets() {
	auth := AuthenticationFromValues(map[string]string{
		propertyKeyType:                    string(TypeBasic),
		propertyKey(FieldBasicUsername):    "mailer",
		propertyKey(FieldBasicPassword):    "******",
		"host":                             "smtp.example.com",
		propertyKey("somethingUndeclared"): "ignored",
	})

	s.Equal(string(TypeBasic), auth.Type)
	s.Equal(map[string]string{
		FieldBasicUsername: "mailer",
		FieldBasicPassword: "******",
	}, auth.Properties)
}

// A sender configured before outbound authentication existed has no stored type, and reads back
// as authenticating with nothing so a client always has a concrete type to show.
func (s *AuthenticationTestSuite) TestAuthenticationFromValuesDefaultsToNone() {
	auth := AuthenticationFromValues(map[string]string{"host": "smtp.example.com"})

	s.Equal(string(TypeNone), auth.Type)
	s.Empty(auth.Properties)
}

func (s *AuthenticationTestSuite) TestAuthenticationRoundTripThroughProperties() {
	props, err := ToProperties(basicAuthentication().Config())
	s.Require().NoError(err)

	auth := AuthenticationFromValues(propertyMap(s.T(), props))
	s.Equal(*basicAuthentication(), auth)
}

// The REST API, declarative documents and export output all carry this struct, so its field
// names are part of each of those contracts.
func (s *AuthenticationTestSuite) TestAuthenticationWireFieldNames() {
	encodedJSON, err := json.Marshal(basicAuthentication())
	s.Require().NoError(err)
	s.JSONEq(`{"type":"basic","properties":{"username":"mailer","password":"s3cret"}}`,
		string(encodedJSON))

	encodedYAML, err := yaml.Marshal(basicAuthentication())
	s.Require().NoError(err)
	s.YAMLEq("type: basic\nproperties:\n  username: mailer\n  password: s3cret\n", string(encodedYAML))

	var decoded Authentication
	s.Require().NoError(yaml.Unmarshal(encodedYAML, &decoded))
	s.Equal(*basicAuthentication(), decoded)
}

// A method with no fields omits the property bag on the wire rather than sending null.
func (s *AuthenticationTestSuite) TestAuthenticationOmitsEmptyProperties() {
	auth := &Authentication{Type: string(TypeNone)}

	encodedJSON, err := json.Marshal(auth)
	s.Require().NoError(err)
	s.JSONEq(`{"type":"none"}`, string(encodedJSON))

	encodedYAML, err := yaml.Marshal(auth)
	s.Require().NoError(err)
	s.YAMLEq("type: none\n", string(encodedYAML))
}
