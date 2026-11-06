// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package presentation implements management (CRUD, persistence, and the REST API)
// of OpenID4VP presentation definitions. The verifier engine in the parent
// openid4vp package reads these definitions on demand via the Store interface.
package presentation

// DefaultCredentialFormat is the credential format a presentation definition
// requests when none is specified.
const DefaultCredentialFormat = "dc+sd-jwt" //nolint:gosec

// TrustedAuthority is a trusted issuer certificate registered on a presentation
// definition. Certificate holds the PEM-encoded X.509 certificate that roots the
// issuer's x5c chain.
type TrustedAuthority struct {
	Name        string `json:"name" yaml:"name"`
	Certificate string `json:"certificate,omitempty" yaml:"certificate,omitempty"`
}

// PresentationDefinitionDTO is the managed representation of an OpenID4VP
// presentation definition. The verifier identity is engine-level configuration
// shared by every definition, so it is not part of the definition.
type PresentationDefinitionDTO struct {
	ID              string   `json:"id" yaml:"id"`
	Handle          string   `json:"handle" yaml:"handle"`
	OUID            string   `json:"ouId" yaml:"ouId,omitempty"`
	OUHandle        string   `json:"ouHandle,omitempty" yaml:"ouHandle,omitempty"`
	Name            string   `json:"name,omitempty" yaml:"name,omitempty"`
	Description     string   `json:"description,omitempty" yaml:"description,omitempty"`
	VCT             string   `json:"vct" yaml:"vct"`
	Format          string   `json:"format,omitempty" yaml:"format,omitempty"`
	RequestedClaims []string `json:"requestedClaims,omitempty" yaml:"requestedClaims,omitempty"`
	MandatoryClaims []string `json:"mandatoryClaims,omitempty" yaml:"mandatoryClaims,omitempty"`
	OptionalClaims  []string `json:"optionalClaims,omitempty" yaml:"optionalClaims,omitempty"`
	// ClaimValues constrains specific claims (by dotted path) to an allowed set
	// of values, emitted as DCQL "values" and enforced at verification.
	ClaimValues map[string][]string `json:"claimValues,omitempty" yaml:"claimValues,omitempty"`
	// EnforceTrustedIssuer requires the issuer's x5c chain to verify against one
	// of TrustedAuthorities. Nil means not enforced.
	EnforceTrustedIssuer *bool `json:"enforceTrustedIssuer,omitempty" yaml:"enforceTrustedIssuer,omitempty"`
	// TrustedAuthorities holds the trusted issuer certificates of this definition.
	TrustedAuthorities []TrustedAuthority `json:"trustedAuthorities,omitempty" yaml:"trustedAuthorities,omitempty"`
}

// presentationDefinitionRequest is the API request body for create/update.
type presentationDefinitionRequest struct {
	Handle               string              `json:"handle"`
	OUID                 string              `json:"ouId" native:"required"`
	Name                 string              `json:"name"`
	Description          string              `json:"description"`
	VCT                  string              `json:"vct"`
	Format               string              `json:"format"`
	RequestedClaims      []string            `json:"requestedClaims"`
	MandatoryClaims      []string            `json:"mandatoryClaims"`
	OptionalClaims       []string            `json:"optionalClaims"`
	ClaimValues          map[string][]string `json:"claimValues"`
	EnforceTrustedIssuer *bool               `json:"enforceTrustedIssuer"`
	TrustedAuthorities   []TrustedAuthority  `json:"trustedAuthorities"`
}

// presentationDefinitionResponse is the API response body. Trusted authorities
// are returned by name only; their certificates are write-only.
type presentationDefinitionResponse struct {
	ID                   string              `json:"id"`
	Handle               string              `json:"handle"`
	OUID                 string              `json:"ouId"`
	OUHandle             string              `json:"ouHandle,omitempty"`
	Name                 string              `json:"name,omitempty"`
	Description          string              `json:"description,omitempty"`
	VCT                  string              `json:"vct"`
	Format               string              `json:"format"`
	RequestedClaims      []string            `json:"requestedClaims,omitempty"`
	MandatoryClaims      []string            `json:"mandatoryClaims,omitempty"`
	OptionalClaims       []string            `json:"optionalClaims,omitempty"`
	ClaimValues          map[string][]string `json:"claimValues,omitempty"`
	EnforceTrustedIssuer *bool               `json:"enforceTrustedIssuer,omitempty"`
	TrustedAuthorities   []string            `json:"trustedAuthorities,omitempty"`
}

// toResponse converts a DTO to its API response shape.
func toResponse(dto PresentationDefinitionDTO) presentationDefinitionResponse {
	return presentationDefinitionResponse{
		ID:                   dto.ID,
		Handle:               dto.Handle,
		OUID:                 dto.OUID,
		OUHandle:             dto.OUHandle,
		Name:                 dto.Name,
		Description:          dto.Description,
		VCT:                  dto.VCT,
		Format:               dto.Format,
		RequestedClaims:      dto.RequestedClaims,
		MandatoryClaims:      dto.MandatoryClaims,
		OptionalClaims:       dto.OptionalClaims,
		ClaimValues:          dto.ClaimValues,
		EnforceTrustedIssuer: dto.EnforceTrustedIssuer,
		TrustedAuthorities:   trustedAuthorityNames(dto.TrustedAuthorities),
	}
}

// trustedAuthorityNames returns the names of the given trusted authorities, or nil when there are none.
func trustedAuthorityNames(authorities []TrustedAuthority) []string {
	if len(authorities) == 0 {
		return nil
	}
	names := make([]string, 0, len(authorities))
	for _, a := range authorities {
		names = append(names, a.Name)
	}
	return names
}

// PresentationDefinitionList is the minimal projection returned by the list
// endpoint. It contains only the fields the management UI renders in the table.
type PresentationDefinitionList struct {
	ID       string `json:"id"`
	Handle   string `json:"handle"`
	OUID     string `json:"ouId"`
	OUHandle string `json:"ouHandle,omitempty"`
	Name     string `json:"name,omitempty"`
	VCT      string `json:"vct"`
	Format   string `json:"format"`
}

// toSummary projects a full DTO to a list summary.
func toSummary(dto PresentationDefinitionDTO) PresentationDefinitionList {
	return PresentationDefinitionList{
		ID:       dto.ID,
		Handle:   dto.Handle,
		OUID:     dto.OUID,
		OUHandle: dto.OUHandle,
		Name:     dto.Name,
		VCT:      dto.VCT,
		Format:   dto.Format,
	}
}
