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

package credential

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) TestToResponse() {
	validity := 3600
	dto := CredentialConfigurationDTO{
		ID:       "cfg-1",
		Handle:   "eudi-pid",
		OUID:     "ou-1",
		OUHandle: "default",
		Format:   DefaultCredentialFormat,
		VCT:      "urn:eudi:pid:de:1",
		Claims:   []ClaimMapping{{Name: "given_name", DisplayName: "Given Name"}},
		Display: &CredentialDisplay{
			Locale: "en-US", LogoURI: "https://example.com/logo.png",
		},
		ValiditySeconds: &validity,
	}

	resp := toResponse(dto)
	s.Equal("cfg-1", resp.ID)
	s.Equal("eudi-pid", resp.Handle)
	s.Equal("ou-1", resp.OUID)
	s.Equal("default", resp.OUHandle)
	s.Equal(DefaultCredentialFormat, resp.Format)
	s.Equal("urn:eudi:pid:de:1", resp.VCT)
	s.Require().Len(resp.Claims, 1)
	s.Require().NotNil(resp.Display)
	s.Equal("en-US", resp.Display.Locale)
	s.Require().NotNil(resp.ValiditySeconds)
	s.Equal(3600, *resp.ValiditySeconds)
}

func (s *ModelTestSuite) TestToConfigSummaryWithDisplay() {
	dto := CredentialConfigurationDTO{
		ID:       "cfg-1",
		Handle:   "eudi-pid",
		OUID:     "ou-1",
		OUHandle: "default",
		Name:     "EUDI PID",
		Format:   DefaultCredentialFormat,
		VCT:      "urn:eudi:pid:de:1",
	}

	summary := toConfigSummary(dto)
	s.Equal("cfg-1", summary.ID)
	s.Equal("eudi-pid", summary.Handle)
	s.Equal("ou-1", summary.OUID)
	s.Equal("default", summary.OUHandle)
	s.Equal(DefaultCredentialFormat, summary.Format)
	s.Equal("urn:eudi:pid:de:1", summary.VCT)
	s.Equal("EUDI PID", summary.Name)
}

func (s *ModelTestSuite) TestToConfigSummaryWithoutName() {
	summary := toConfigSummary(CredentialConfigurationDTO{ID: "cfg-2", Handle: "h", VCT: "v"})
	s.Equal("cfg-2", summary.ID)
	s.Empty(summary.Name)
}
