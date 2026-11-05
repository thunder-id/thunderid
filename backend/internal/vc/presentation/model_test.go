// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
		TrustedAuthorities: []TrustedAuthority{
			{Name: "root-a", Certificate: "pem-a"}, {Name: "root-b", Certificate: "pem-b"},
		},
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
	assert.Equal(t, []string{"root-a", "root-b"}, resp.TrustedAuthorities)
}

func TestToResponseOmitsEmptyTrustedAuthorities(t *testing.T) {
	assert.Nil(t, toResponse(PresentationDefinitionDTO{ID: "def-1"}).TrustedAuthorities)
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
