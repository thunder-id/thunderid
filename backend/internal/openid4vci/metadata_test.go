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

package openid4vci

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/openid4vci/credential"
)

type MetadataTestSuite struct {
	suite.Suite
}

func TestMetadataTestSuite(t *testing.T) {
	suite.Run(t, new(MetadataTestSuite))
}

func (s *MetadataTestSuite) TestBuildMetadataFull() {
	cfg := serviceConfig{
		CredentialIssuer:     testIssuer,
		BaseURL:              "https://issuer.example",
		AuthorizationServers: []string{"https://as.example"},
		BatchSize:            5,
	}
	creds := []credential.CredentialConfigurationDTO{
		{
			Handle: "eudi-pid",
			VCT:    "urn:eudi:pid:1",
			Format: "",
			Claims: []credential.ClaimMapping{
				{Name: "given_name", DisplayName: "Given Name"},
				{Name: "no_display"},
			},
			Display: &credential.CredentialDisplay{Name: "PID", Locale: "en", LogoURI: "https://logo"},
		},
	}

	md := buildMetadata(cfg, creds)
	s.Equal(testIssuer, md["credential_issuer"])
	s.Equal("https://issuer.example"+credentialPath, md["credential_endpoint"])
	s.Equal("https://issuer.example"+noncePath, md["nonce_endpoint"])
	s.Equal([]string{"https://as.example"}, md["authorization_servers"])
	s.Equal(map[string]interface{}{"batch_size": 5}, md["batch_credential_issuance"])

	configs := md["credential_configurations_supported"].(map[string]interface{})
	entry := configs["eudi-pid"].(map[string]interface{})
	s.Equal(credential.DefaultCredentialFormat, entry["format"])
	s.Equal("eudi-pid", entry["scope"])
	s.Equal("urn:eudi:pid:1", entry["vct"])
	s.NotNil(entry["display"])
	s.NotNil(entry["claims"])
}

func (s *MetadataTestSuite) TestBuildMetadataMinimal() {
	cfg := serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://i", BatchSize: 1}
	creds := []credential.CredentialConfigurationDTO{
		{Handle: "h", VCT: "v", Format: "dc+sd-jwt"},
	}
	md := buildMetadata(cfg, creds)
	_, hasAuth := md["authorization_servers"]
	s.False(hasAuth)
	_, hasBatch := md["batch_credential_issuance"]
	s.False(hasBatch)

	configs := md["credential_configurations_supported"].(map[string]interface{})
	entry := configs["h"].(map[string]interface{})
	_, hasDisplay := entry["display"]
	s.False(hasDisplay)
	_, hasClaims := entry["claims"]
	s.False(hasClaims)
}

func (s *MetadataTestSuite) TestCredentialClaims() {
	out := credentialClaims([]credential.ClaimMapping{
		{Name: "given_name", DisplayName: "Given Name"},
		{Name: "skip"},
	})
	s.Require().NotNil(out)
	s.Contains(out, "given_name")
	s.NotContains(out, "skip")

	s.Nil(credentialClaims([]credential.ClaimMapping{{Name: "skip"}}))
	s.Nil(credentialClaims(nil))
}

func (s *MetadataTestSuite) TestCredentialDisplay() {
	s.Nil(credentialDisplay(nil))
	s.Nil(credentialDisplay(&credential.CredentialDisplay{}))

	out := credentialDisplay(&credential.CredentialDisplay{Name: "PID", Locale: "en", LogoURI: "https://logo"})
	s.Require().Len(out, 1)
	entry := out[0].(map[string]interface{})
	s.Equal("PID", entry["name"])
	s.Equal("en", entry["locale"])
	s.Equal(map[string]interface{}{"uri": "https://logo"}, entry["logo"])
}
