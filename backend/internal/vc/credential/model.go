// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package credential implements management (CRUD, persistence, and the REST API)
// of OpenID4VCI credential configurations. The issuer engine in the parent
// openid4vci package reads these configurations on demand via the credentialStoreInterface.
package credential

// DefaultCredentialFormat is the credential format assumed when none is specified.
const DefaultCredentialFormat = "dc+sd-jwt" //nolint:gosec

// ClaimMapping is one selectively disclosable claim: the attribute name (also the
// user-profile lookup key) and its human-readable display name shown in wallets.
type ClaimMapping struct {
	Name        string `json:"name" yaml:"name"`
	DisplayName string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
}

// CredentialDisplay holds wallet-facing display metadata with no admin-facing
// equivalent; name and description come from the configuration's own fields.
type CredentialDisplay struct {
	Locale  string `json:"locale,omitempty" yaml:"locale,omitempty"`
	LogoURI string `json:"logoUri,omitempty" yaml:"logoUri,omitempty"`
}

// CredentialConfigurationDTO is the managed representation of an OpenID4VCI
// credential configuration. The handle is the credential_configuration_id and the
// OAuth scope. Engine-level settings (signing key, default validity) are shared by
// every configuration and live in server config, not here.
type CredentialConfigurationDTO struct {
	ID              string             `json:"id" yaml:"id"`
	Handle          string             `json:"handle" yaml:"handle"`
	OUID            string             `json:"ouId" yaml:"ouId,omitempty"`
	OUHandle        string             `json:"ouHandle,omitempty" yaml:"ouHandle,omitempty"`
	Name            string             `json:"name,omitempty" yaml:"name,omitempty"`
	Description     string             `json:"description,omitempty" yaml:"description,omitempty"`
	Format          string             `json:"format,omitempty" yaml:"format,omitempty"`
	VCT             string             `json:"vct" yaml:"vct"`
	Claims          []ClaimMapping     `json:"claims,omitempty" yaml:"claims,omitempty"`
	Display         *CredentialDisplay `json:"display,omitempty" yaml:"display,omitempty"`
	ValiditySeconds *int               `json:"validitySeconds,omitempty" yaml:"validitySeconds,omitempty"`
}

// credentialConfigurationRequest is the API request body for create/update.
type credentialConfigurationRequest struct {
	Handle          string             `json:"handle"`
	OUID            string             `json:"ouId"`
	OUHandle        string             `json:"ouHandle"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Format          string             `json:"format"`
	VCT             string             `json:"vct"`
	Claims          []ClaimMapping     `json:"claims"`
	Display         *CredentialDisplay `json:"display"`
	ValiditySeconds *int               `json:"validitySeconds"`
}

// credentialConfigurationResponse is the API response body.
type credentialConfigurationResponse struct {
	ID              string             `json:"id"`
	Handle          string             `json:"handle"`
	OUID            string             `json:"ouId"`
	OUHandle        string             `json:"ouHandle,omitempty"`
	Name            string             `json:"name,omitempty"`
	Description     string             `json:"description,omitempty"`
	Format          string             `json:"format"`
	VCT             string             `json:"vct"`
	Claims          []ClaimMapping     `json:"claims,omitempty"`
	Display         *CredentialDisplay `json:"display,omitempty"`
	ValiditySeconds *int               `json:"validitySeconds,omitempty"`
}

// toResponse converts a DTO to its API response shape.
func toResponse(dto CredentialConfigurationDTO) credentialConfigurationResponse {
	return credentialConfigurationResponse(dto)
}

// CredentialConfigurationList is the minimal projection returned by the list
// endpoint. It contains only the fields the management UI renders in the table.
type CredentialConfigurationList struct {
	ID         string `json:"id"`
	Handle     string `json:"handle"`
	OUID       string `json:"ouId"`
	OUHandle   string `json:"ouHandle,omitempty"`
	Format     string `json:"format"`
	VCT        string `json:"vct"`
	Name       string `json:"name,omitempty"`
	IsReadOnly bool   `json:"isReadOnly"`
}

// toConfigSummary projects a full DTO to a list summary.
func toConfigSummary(dto CredentialConfigurationDTO) CredentialConfigurationList {
	return CredentialConfigurationList{
		ID:       dto.ID,
		Handle:   dto.Handle,
		OUID:     dto.OUID,
		OUHandle: dto.OUHandle,
		Format:   dto.Format,
		VCT:      dto.VCT,
		Name:     dto.Name,
	}
}
