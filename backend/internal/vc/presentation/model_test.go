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

package presentation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToResponse(t *testing.T) {
	enforce := true
	dto := PresentationDefinitionDTO{
		ID:                   "def-1",
		Handle:               "eudi-pid",
		OUID:                 "ou-1",
		OUHandle:             "default",
		Name:                 "EUDI PID",
		VCT:                  "urn:eudi:pid:1",
		Format:               DefaultCredentialFormat,
		RequestedClaims:      []string{"given_name"},
		MandatoryClaims:      []string{"family_name"},
		OptionalClaims:       []string{"birthdate"},
		ClaimValues:          map[string][]string{"address.country": {"DE"}},
		EnforceTrustedIssuer: &enforce,
		TrustedAuthorities:   []string{"root-a"},
	}

	resp := toResponse(dto)

	assert.Equal(t, dto.ID, resp.ID)
	assert.Equal(t, dto.Handle, resp.Handle)
	assert.Equal(t, dto.OUID, resp.OUID)
	assert.Equal(t, dto.OUHandle, resp.OUHandle)
	assert.Equal(t, dto.Name, resp.Name)
	assert.Equal(t, dto.VCT, resp.VCT)
	assert.Equal(t, dto.Format, resp.Format)
	assert.Equal(t, dto.RequestedClaims, resp.RequestedClaims)
	assert.Equal(t, dto.MandatoryClaims, resp.MandatoryClaims)
	assert.Equal(t, dto.OptionalClaims, resp.OptionalClaims)
	assert.Equal(t, dto.ClaimValues, resp.ClaimValues)
	assert.Equal(t, dto.EnforceTrustedIssuer, resp.EnforceTrustedIssuer)
	assert.Equal(t, dto.TrustedAuthorities, resp.TrustedAuthorities)
}

func TestToSummary(t *testing.T) {
	dto := PresentationDefinitionDTO{
		ID:              "def-1",
		Handle:          "eudi-pid",
		OUID:            "ou-1",
		OUHandle:        "default",
		Name:            "EUDI PID",
		VCT:             "urn:eudi:pid:1",
		Format:          DefaultCredentialFormat,
		MandatoryClaims: []string{"family_name"},
	}

	summary := toSummary(dto)

	assert.Equal(t, PresentationDefinitionList{
		ID:       "def-1",
		Handle:   "eudi-pid",
		OUID:     "ou-1",
		OUHandle: "default",
		Name:     "EUDI PID",
		VCT:      "urn:eudi:pid:1",
		Format:   DefaultCredentialFormat,
	}, summary)
}
