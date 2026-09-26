// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// reversingCryptoProvider is a minimal kmprovider.ConfigCryptoProvider test double. It reverses
// the bytes so an encrypted value is visibly different from its plaintext, which lets a test
// assert that a secret was stored encrypted rather than verbatim.
type reversingCryptoProvider struct{}

func (p *reversingCryptoProvider) Encrypt(_ context.Context, content []byte) ([]byte, error) {
	return reverse(content), nil
}

func (p *reversingCryptoProvider) Decrypt(_ context.Context, content []byte) ([]byte, error) {
	return reverse(content), nil
}

func reverse(content []byte) []byte {
	out := make([]byte, len(content))
	for i, b := range content {
		out[len(content)-1-i] = b
	}
	return out
}

type OutboundAuthTestSuite struct {
	suite.Suite
}

func TestOutboundAuthTestSuite(t *testing.T) {
	suite.Run(t, new(OutboundAuthTestSuite))
}

func (s *OutboundAuthTestSuite) SetupSuite() {
	cmodels.SetConfigCryptoProvider(&reversingCryptoProvider{})
}

func basicConfig() Config {
	return Config{
		Type: TypeBasic,
		Properties: map[string]string{
			FieldBasicUsername: "mailer",
			FieldBasicPassword: "s3cret",
		},
	}
}

func allTypes() []Type {
	return []Type{TypeNone, TypeBasic}
}

// propertyMap resolves a property slice to a name/value map for assertions.
func (s *OutboundAuthTestSuite) propertyMap(props []cmodels.Property) map[string]string {
	values := make(map[string]string, len(props))
	for i := range props {
		value, err := props[i].GetValue()
		s.Require().NoError(err)
		values[props[i].GetName()] = value
	}
	return values
}

func (s *OutboundAuthTestSuite) TestParseType() {
	cases := []struct {
		value    string
		expected Type
		ok       bool
	}{
		{"basic", TypeBasic, true},
		{"BASIC", TypeBasic, true},
		{"  Basic  ", TypeBasic, true},
		{"none", TypeNone, true},
		{"", TypeNone, true},
		{"   ", TypeNone, true},
		{"bearer", "", false},
		{"oauth2", "", false},
	}

	for _, tc := range cases {
		parsed, ok := ParseType(tc.value)
		s.Equal(tc.ok, ok, "value %q", tc.value)
		s.Equal(tc.expected, parsed, "value %q", tc.value)
	}
}

// ParseType reads the registry rather than a list of its own, so a method added to the table is
// parsable without a second edit.
func (s *OutboundAuthTestSuite) TestParseTypeFollowsRegistry() {
	const apiKey Type = "api_key"

	_, ok := ParseType(string(apiKey))
	s.False(ok)

	s.withTemporaryMethod(Method{Type: apiKey, DisplayName: "API key"}, func() {
		parsed, ok := ParseType("  API_KEY  ")
		s.True(ok)
		s.Equal(apiKey, parsed)
	})
}

// The registry decides which values are encrypted, so a caller changing a returned method must
// not change the registry itself.
func (s *OutboundAuthTestSuite) TestGetMethodReturnsACopyOfFields() {
	method, ok := GetMethod(TypeBasic)
	s.Require().True(ok)
	for i := range method.Fields {
		method.Fields[i].Credential = false
	}

	password, ok := getField(TypeBasic, FieldBasicPassword)
	s.Require().True(ok)
	s.True(password.Credential)
}

// The table is hand-maintained and nothing validates it at startup, by design. A malformed entry
// would otherwise surface only as a misbehaving API, so these invariants are its only guard.
func (s *OutboundAuthTestSuite) TestRegisteredMethodsAreWellFormed() {
	seenTypes := make(map[Type]struct{}, len(registeredMethods))

	for _, method := range registeredMethods {
		s.NotEmpty(method.Type, "every method must declare a type")
		s.NotContains(seenTypes, method.Type, "method %q is registered twice", method.Type)
		seenTypes[method.Type] = struct{}{}
		s.NotEmpty(method.DisplayName, "method %q must have a display name", method.Type)

		seenFields := make(map[string]struct{}, len(method.Fields))
		for _, field := range method.Fields {
			s.NotEmpty(field.Name, "method %q declares a field with no name", method.Type)
			s.NotContains(seenFields, field.Name,
				"method %q declares field %q twice", method.Type, field.Name)
			seenFields[field.Name] = struct{}{}

			s.Equal(FieldTypeString, field.Type,
				"method %q field %q: only string fields are supported today", method.Type, field.Name)
			s.NotEmpty(field.DisplayName,
				"method %q field %q must have a display name", method.Type, field.Name)

			// Validate compiles this per request, so an invalid pattern would fail every call
			// rather than the build.
			if field.Regex != "" {
				_, err := regexp.Compile(field.Regex)
				s.NoError(err, "method %q field %q has an uncompilable regex", method.Type, field.Name)
			}
		}
	}
}

func (s *OutboundAuthTestSuite) TestConfigEnabled() {
	s.False(Config{}.Enabled())
	s.False(Config{Type: TypeNone}.Enabled())
	s.True(basicConfig().Enabled())
}

func (s *OutboundAuthTestSuite) TestGetMethod() {
	method, ok := GetMethod(TypeBasic)
	s.True(ok)
	s.Equal(TypeBasic, method.Type)
	s.Require().Len(method.Fields, 2)

	// Render order is part of the contract: a credential form shows the username first.
	s.Equal(FieldBasicUsername, method.Fields[0].Name)
	s.Equal(FieldBasicPassword, method.Fields[1].Name)
	s.False(method.Fields[0].Credential)
	s.True(method.Fields[1].Credential)

	_, ok = GetMethod(Type("bearer"))
	s.False(ok)
}

func (s *OutboundAuthTestSuite) TestGetMethodsPreservesCallerOrderAndSkipsUnknown() {
	result := GetMethods([]Type{TypeBasic, Type("bearer"), TypeNone})
	s.Require().Len(result, 2)
	s.Equal(TypeBasic, result[0].Type)
	s.Equal(TypeNone, result[1].Type)
}

// The API declares the fields of a method as an array. A nil slice would serialize to null and
// break a client that iterates it.
func (s *OutboundAuthTestSuite) TestGetMethodsServesFieldlessMethodAsEmptyArray() {
	result := GetMethods([]Type{TypeNone})
	s.Require().Len(result, 1)
	s.NotNil(result[0].Fields)
	s.Empty(result[0].Fields)

	encoded, err := json.Marshal(result[0])
	s.Require().NoError(err)
	s.Contains(string(encoded), `"properties":[]`)
}

func (s *OutboundAuthTestSuite) TestOwnsPropertyKey() {
	s.True(OwnsPropertyKey(PropertyKeyType))
	s.True(OwnsPropertyKey(PropertyKey(FieldBasicPassword)))
	s.False(OwnsPropertyKey("host"))
	s.False(OwnsPropertyKey("username"))

	// The Twilio sender stores a transport secret called auth_token. A shorter prefix would
	// have claimed it; this asserts the chosen prefix does not.
	s.False(OwnsPropertyKey("auth_token"))
}

func (s *OutboundAuthTestSuite) TestToPropertiesBasic() {
	props, err := ToProperties(basicConfig())
	s.Require().NoError(err)

	values := s.propertyMap(props)
	s.Equal(string(TypeBasic), values[PropertyKeyType])
	s.Equal("mailer", values[PropertyKey(FieldBasicUsername)])
	s.Equal("s3cret", values[PropertyKey(FieldBasicPassword)])

	byName := make(map[string]*cmodels.Property, len(props))
	for i := range props {
		byName[props[i].GetName()] = &props[i]
	}
	s.True(byName[PropertyKey(FieldBasicPassword)].IsSecret())
	s.False(byName[PropertyKey(FieldBasicUsername)].IsSecret())
	s.False(byName[PropertyKeyType].IsSecret())
}

func (s *OutboundAuthTestSuite) TestToPropertiesAlwaysWritesTypeIncludingNone() {
	props, err := ToProperties(Config{Type: TypeNone})
	s.Require().NoError(err)
	s.Require().Len(props, 1)
	s.Equal(PropertyKeyType, props[0].GetName())

	// A zero Config is treated as none rather than rejected, so an omitted block is storable.
	props, err = ToProperties(Config{})
	s.Require().NoError(err)
	s.Require().Len(props, 1)
	values := s.propertyMap(props)
	s.Equal(string(TypeNone), values[PropertyKeyType])
}

func (s *OutboundAuthTestSuite) TestToPropertiesSkipsBlankAndDropsUndeclaredFields() {
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

	values := s.propertyMap(props)
	s.NotContains(values, PropertyKey(FieldBasicUsername))
	s.NotContains(values, PropertyKey("clientSecret"))
	s.Equal("s3cret", values[PropertyKey(FieldBasicPassword)])
}

func (s *OutboundAuthTestSuite) TestToPropertiesRejectsUnknownType() {
	_, err := ToProperties(Config{Type: Type("bearer")})
	s.Require().Error(err)
	s.Contains(err.Error(), "unsupported authentication type")
	// A transport maps this onto a 400 rather than a 500, so the sentinel has to stay matchable
	// through the wrapping that names the offending type.
	s.Require().ErrorIs(err, ErrUnsupportedType)
	s.Contains(err.Error(), "bearer")
}

func (s *OutboundAuthTestSuite) TestRoundTripThroughProperties() {
	props, err := ToProperties(basicConfig())
	s.Require().NoError(err)

	cfg, err := FromProperties(props)
	s.Require().NoError(err)
	s.Equal(TypeBasic, cfg.Type)
	s.Equal("mailer", cfg.Get(FieldBasicUsername))
	s.Equal("s3cret", cfg.Get(FieldBasicPassword))
}

func (s *OutboundAuthTestSuite) TestFromPropertiesIgnoresForeignKeys() {
	// A Twilio property bag must not be read as an authentication configuration.
	accountSID, err := cmodels.NewProperty("account_sid", "AC123", false)
	s.Require().NoError(err)
	authToken, err := cmodels.NewProperty("auth_token", "twilio-secret", true)
	s.Require().NoError(err)
	senderID, err := cmodels.NewProperty("sender_id", "+15550100", false)
	s.Require().NoError(err)

	cfg, err := FromProperties([]cmodels.Property{*accountSID, *authToken, *senderID})
	s.Require().NoError(err)
	s.Equal(TypeNone, cfg.Type)
	s.Empty(cfg.Properties)
}

func (s *OutboundAuthTestSuite) TestFromValuesSurfacesUnknownStoredType() {
	cfg := FromValues(map[string]string{PropertyKeyType: "bearer"})
	s.Equal(Type("bearer"), cfg.Type)

	// Surfacing it verbatim lets Validate reject it descriptively instead of silently
	// downgrading a sender to unauthenticated.
	err := Validate(cfg, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unsupported authentication type")
}

func (s *OutboundAuthTestSuite) TestFromValuesReadsMaskedSecrets() {
	cfg := FromValues(map[string]string{
		PropertyKeyType:                    string(TypeBasic),
		PropertyKey(FieldBasicUsername):    "mailer",
		PropertyKey(FieldBasicPassword):    "******",
		PropertyKey("somethingUndeclared"): "ignored",
	})

	s.Equal(TypeBasic, cfg.Type)
	s.Equal("mailer", cfg.Get(FieldBasicUsername))
	s.Equal("******", cfg.Get(FieldBasicPassword))
	s.Len(cfg.Properties, 2)
}

func (s *OutboundAuthTestSuite) TestValidateAcceptsValidConfigurations() {
	s.NoError(Validate(basicConfig(), allTypes()))
	s.NoError(Validate(Config{Type: TypeNone}, allTypes()))
	s.NoError(Validate(Config{}, allTypes()))
}

func (s *OutboundAuthTestSuite) TestValidateRejectsUnknownType() {
	err := Validate(Config{Type: Type("bearer")}, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unsupported authentication type")
}

func (s *OutboundAuthTestSuite) TestValidateRejectsTypeOutsideSupportedSet() {
	err := Validate(basicConfig(), []Type{TypeNone})
	s.Require().Error(err)
	s.Contains(err.Error(), "not supported by this provider")
}

func (s *OutboundAuthTestSuite) TestValidateRejectsMissingAndBlankRequiredFields() {
	missing := Config{Type: TypeBasic, Properties: map[string]string{FieldBasicUsername: "mailer"}}
	err := Validate(missing, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), FieldBasicPassword)

	blank := Config{Type: TypeBasic, Properties: map[string]string{
		FieldBasicUsername: "   ",
		FieldBasicPassword: "s3cret",
	}}
	err = Validate(blank, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), FieldBasicUsername)
}

func (s *OutboundAuthTestSuite) TestValidateRejectsUndeclaredField() {
	cfg := basicConfig()
	cfg.Properties["clientSecret"] = "nope"

	err := Validate(cfg, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unknown authentication property")
}

func (s *OutboundAuthTestSuite) TestValidateRejectsPropertiesOnNone() {
	cfg := Config{Type: TypeNone, Properties: map[string]string{FieldBasicUsername: "mailer"}}
	err := Validate(cfg, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unknown authentication property")
}

// The remaining field features have no built-in method using them yet, so they are exercised
// against a temporarily registered method. This is what a future method relies on.
func (s *OutboundAuthTestSuite) withTemporaryMethod(method Method, body func()) {
	original := registeredMethods
	registeredMethods = append(append([]Method{}, registeredMethods...), method)
	defer func() { registeredMethods = original }()
	body()
}

func (s *OutboundAuthTestSuite) TestValidateEnforcesEnum() {
	const apiKey Type = "api_key"
	s.withTemporaryMethod(Method{
		Type:        apiKey,
		DisplayName: "API key",
		Fields: []Field{
			{Name: "in", Type: FieldTypeString, Required: true, Enum: []string{"header", "query"},
				DisplayName: "Placement"},
		},
	}, func() {
		supported := []Type{apiKey}
		s.NoError(Validate(Config{Type: apiKey, Properties: map[string]string{"in": "header"}}, supported))

		err := Validate(Config{Type: apiKey, Properties: map[string]string{"in": "body"}}, supported)
		s.Require().Error(err)
		s.Contains(err.Error(), "must be one of")
	})
}

func (s *OutboundAuthTestSuite) TestValidateEnforcesRegex() {
	const signed Type = "signed"
	s.withTemporaryMethod(Method{
		Type:        signed,
		DisplayName: "Signed",
		Fields: []Field{
			{Name: "region", Type: FieldTypeString, Required: true, Regex: `^[a-z]{2}-[a-z]+-\d$`,
				DisplayName: "Region"},
		},
	}, func() {
		supported := []Type{signed}
		s.NoError(Validate(Config{Type: signed, Properties: map[string]string{"region": "us-east-1"}},
			supported))

		err := Validate(Config{Type: signed, Properties: map[string]string{"region": "nowhere"}},
			supported)
		s.Require().Error(err)
		s.Contains(err.Error(), "invalid format")
	})
}

func (s *OutboundAuthTestSuite) TestValidateRunsMethodHook() {
	const dual Type = "dual"
	hookErr := errors.New("exactly one of clientSecret or privateKey is required")
	s.withTemporaryMethod(Method{
		Type:        dual,
		DisplayName: "Dual",
		Fields: []Field{
			{Name: "clientSecret", Type: FieldTypeString, Credential: true, DisplayName: "Secret"},
			{Name: "privateKey", Type: FieldTypeString, Credential: true, DisplayName: "Key"},
		},
		Validate: func(cfg Config) error {
			if (cfg.Get("clientSecret") == "") == (cfg.Get("privateKey") == "") {
				return hookErr
			}
			return nil
		},
	}, func() {
		supported := []Type{dual}
		s.NoError(Validate(Config{Type: dual, Properties: map[string]string{"clientSecret": "a"}},
			supported))

		err := Validate(Config{Type: dual, Properties: map[string]string{
			"clientSecret": "a", "privateKey": "b",
		}}, supported)
		s.Require().ErrorIs(err, hookErr)
	})
}
