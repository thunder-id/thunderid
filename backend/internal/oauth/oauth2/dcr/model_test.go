// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

// DCRModelTestSuite is the test suite for DCR model marshaling/unmarshaling.
type DCRModelTestSuite struct {
	suite.Suite
}

func TestDCRModelTestSuite(t *testing.T) {
	suite.Run(t, new(DCRModelTestSuite))
}

func (s *DCRModelTestSuite) TestUnmarshalJSON_BasicFields() {
	input := `{
		"client_name": "My App",
		"redirect_uris": ["https://example.com/cb"],
		"scope": "openid profile"
	}`
	var req DCRRegistrationRequest
	s.Require().NoError(json.Unmarshal([]byte(input), &req))

	s.Equal("My App", req.ClientName)
	s.Equal([]string{"https://example.com/cb"}, req.RedirectURIs)
	s.Equal("openid profile", req.Scope)
	s.Nil(req.LocalizedClientName)
}

// An update request carries the registration metadata plus the two fields only an update has. The
// embedded registration request defines its own UnmarshalJSON, which an alias of the outer type
// would promote and let consume the whole object, leaving client_id and client_secret empty while
// every other field decoded correctly. Decoding has to fill both levels in one pass.
func (s *DCRModelTestSuite) TestUnmarshalJSON_UpdateRequestFillsBothLevels() {
	input := `{
		"client_id": "abc123",
		"client_secret": "s3cr3t",
		"client_name": "My App",
		"client_name#fr": "Mon Application",
		"redirect_uris": ["https://example.com/cb"]
	}`
	var req DCRUpdateRequest
	s.Require().NoError(json.Unmarshal([]byte(input), &req))

	// The update-only fields, which live on the outer type.
	s.Equal("abc123", req.ClientID)
	s.Equal("s3cr3t", req.ClientSecret)
	// The embedded registration metadata, including the language tagged variants.
	s.Equal("My App", req.ClientName)
	s.Equal([]string{"https://example.com/cb"}, req.RedirectURIs)
	s.Equal(map[string]string{"fr": "Mon Application"}, req.LocalizedClientName)
}

// Registration has no client_id or client_secret of its own: the server issues both. A body that
// carries them is decoded as ordinary metadata with those fields ignored.
func (s *DCRModelTestSuite) TestUnmarshalJSON_RegistrationIgnoresClientCredentials() {
	input := `{
		"client_id": "should-be-ignored",
		"client_secret": "should-be-ignored",
		"client_name": "My App",
		"redirect_uris": ["https://example.com/cb"]
	}`
	var req DCRRegistrationRequest
	s.Require().NoError(json.Unmarshal([]byte(input), &req))

	s.Equal("My App", req.ClientName)
	s.Equal([]string{"https://example.com/cb"}, req.RedirectURIs)
}

func (s *DCRModelTestSuite) TestUnmarshalJSON_LocalizedFields() {
	input := `{
		"client_name": "My App",
		"client_name#fr": "Mon Application",
		"client_name#de": "Meine Anwendung",
		"logo_uri": "https://example.com/logo.png",
		"logo_uri#fr": "https://example.com/fr/logo.png",
		"tos_uri#fr": "https://example.com/fr/tos",
		"policy_uri#fr": "https://example.com/fr/policy"
	}`
	var req DCRRegistrationRequest
	s.Require().NoError(json.Unmarshal([]byte(input), &req))

	s.Equal("My App", req.ClientName)
	s.Equal("Mon Application", req.LocalizedClientName["fr"])
	s.Equal("Meine Anwendung", req.LocalizedClientName["de"])
	s.Equal("https://example.com/fr/logo.png", req.LocalizedLogoURI["fr"])
	s.Equal("https://example.com/fr/tos", req.LocalizedTosURI["fr"])
	s.Equal("https://example.com/fr/policy", req.LocalizedPolicyURI["fr"])
}

func (s *DCRModelTestSuite) TestUnmarshalJSON_InvalidBCP47Tag() {
	input := `{"client_name#not!!valid": "Bad"}`
	var req DCRRegistrationRequest
	err := json.Unmarshal([]byte(input), &req)

	s.Error(err)
	s.Contains(err.Error(), "invalid BCP 47 language tag")
}

func (s *DCRModelTestSuite) TestUnmarshalJSON_InvalidJSON() {
	input := `{"client_name": invalid json}`
	var req DCRRegistrationRequest
	s.Error(json.Unmarshal([]byte(input), &req))
}

func (s *DCRModelTestSuite) TestUnmarshalJSON_NonStringLocalizedValue() {
	input := `{"client_name#fr": 42}`
	var req DCRRegistrationRequest
	s.Require().NoError(json.Unmarshal([]byte(input), &req))
	s.Nil(req.LocalizedClientName)
}

func (s *DCRModelTestSuite) TestMarshalJSON_NoLocalizedFields() {
	resp := DCRRegistrationResponse{
		ClientID:   "client-123",
		ClientName: "My App",
	}
	data, err := json.Marshal(resp)
	s.Require().NoError(err)

	var m map[string]interface{}
	s.Require().NoError(json.Unmarshal(data, &m))
	s.Equal("client-123", m["client_id"])
	s.Equal("My App", m["client_name"])

	for key := range m {
		s.NotContains(key, "#")
	}
}

func (s *DCRModelTestSuite) TestMarshalJSON_WithLocalizedFields() {
	resp := DCRRegistrationResponse{
		ClientID:   "client-123",
		ClientName: "My App",
		LocalizedClientName: map[string]string{
			"fr": "Mon Application",
			"de": "Meine Anwendung",
		},
		LocalizedLogoURI: map[string]string{
			"fr": "https://example.com/fr/logo.png",
		},
		LocalizedTosURI: map[string]string{
			"fr": "https://example.com/fr/tos",
		},
		LocalizedPolicyURI: map[string]string{
			"fr": "https://example.com/fr/policy",
		},
	}
	data, err := json.Marshal(resp)
	s.Require().NoError(err)

	var m map[string]interface{}
	s.Require().NoError(json.Unmarshal(data, &m))
	s.Equal("Mon Application", m["client_name#fr"])
	s.Equal("Meine Anwendung", m["client_name#de"])
	s.Equal("https://example.com/fr/logo.png", m["logo_uri#fr"])
	s.Equal("https://example.com/fr/tos", m["tos_uri#fr"])
	s.Equal("https://example.com/fr/policy", m["policy_uri#fr"])
	s.Equal("My App", m["client_name"])
}

func (s *DCRModelTestSuite) TestErrInvalidBCP47Tag_Error() {
	err := &errInvalidBCP47Tag{key: "client_name#not!!valid"}
	s.Equal(`invalid BCP 47 language tag in field "client_name#not!!valid"`, err.Error())
}

func (s *DCRModelTestSuite) TestUnmarshalJSON_DuplicateTagsNormalized() {
	input := `{
		"client_name#FR": "Première valeur",
		"client_name#fr": "Deuxième valeur"
	}`
	var req DCRRegistrationRequest
	s.Require().NoError(json.Unmarshal([]byte(input), &req))

	s.Require().NotNil(req.LocalizedClientName)
	s.Len(req.LocalizedClientName, 1, "duplicate normalised tags must collapse to a single entry")
	s.Contains(req.LocalizedClientName, "fr")
}

func (s *DCRModelTestSuite) TestUnmarshalJSON_MaxLocalizedVariants() {
	langs := []string{
		"aa", "ab", "ae", "af", "ak", "am", "an", "ar", "as", "av",
		"ay", "az", "ba", "be", "bg", "bh", "bi", "bm", "bn", "bo", "br",
	}
	s.Require().Greater(len(langs), maxLocalizedVariantsPerField, "test data must exceed the limit")

	input := `{"client_name": "Base"`
	for _, lang := range langs {
		input += `, "client_name#` + lang + `": "Value ` + lang + `"`
	}
	input += `}`

	var req DCRRegistrationRequest
	err := json.Unmarshal([]byte(input), &req)

	s.Error(err)
	s.Contains(err.Error(), "exceeds the maximum")
}
