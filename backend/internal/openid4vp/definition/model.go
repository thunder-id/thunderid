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

// Package definition implements management (CRUD, persistence, and the REST API)
// of OpenID4VP presentation definitions. The verifier engine in the parent
// openid4vp package reads these definitions on demand via the Store interface.
package definition

// DefaultCredentialFormat is the credential format a presentation definition
// requests when none is specified.
const DefaultCredentialFormat = "dc+sd-jwt" //nolint:gosec

// PresentationDefinitionDTO is the managed representation of an OpenID4VP
// presentation definition. Trusted issuers and the verifier identity are
// engine-level configuration shared by every definition (global trust), so they
// are not part of the definition.
type PresentationDefinitionDTO struct {
	ID              string   `json:"id" yaml:"id"`
	Handle          string   `json:"handle" yaml:"handle"`
	OUID            string   `json:"ouId" yaml:"ouId,omitempty"`
	OUHandle        string   `json:"ouHandle,omitempty" yaml:"ouHandle,omitempty"`
	DisplayName     string   `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	VCT             string   `json:"vct" yaml:"vct"`
	Format          string   `json:"format,omitempty" yaml:"format,omitempty"`
	RequestedClaims []string `json:"requestedClaims,omitempty" yaml:"requestedClaims,omitempty"`
	MandatoryClaims []string `json:"mandatoryClaims,omitempty" yaml:"mandatoryClaims,omitempty"`
	OptionalClaims  []string `json:"optionalClaims,omitempty" yaml:"optionalClaims,omitempty"`
	// ClaimValues constrains specific claims (by dotted path) to an allowed set
	// of values, emitted as DCQL "values" and enforced at verification.
	ClaimValues map[string][]string `json:"claimValues,omitempty" yaml:"claimValues,omitempty"`
	// EnforceTrustedIssuer overrides the engine default issuer-trust enforcement
	// for this definition. Nil inherits the engine default.
	EnforceTrustedIssuer *bool `json:"enforceTrustedIssuer,omitempty" yaml:"enforceTrustedIssuer,omitempty"`
	// TrustedAuthorities restricts the acceptable trust anchors (by name).
	// Empty accepts any configured anchor.
	TrustedAuthorities []string `json:"trustedAuthorities,omitempty" yaml:"trustedAuthorities,omitempty"`
}

// presentationDefinitionRequest is the API request body for create/update.
type presentationDefinitionRequest struct {
	Handle               string              `json:"handle"`
	OUID                 string              `json:"ouId"`
	OUHandle             string              `json:"ouHandle"`
	DisplayName          string              `json:"displayName"`
	VCT                  string              `json:"vct"`
	Format               string              `json:"format"`
	RequestedClaims      []string            `json:"requestedClaims"`
	MandatoryClaims      []string            `json:"mandatoryClaims"`
	OptionalClaims       []string            `json:"optionalClaims"`
	ClaimValues          map[string][]string `json:"claimValues"`
	EnforceTrustedIssuer *bool               `json:"enforceTrustedIssuer"`
	TrustedAuthorities   []string            `json:"trustedAuthorities"`
}

// presentationDefinitionResponse is the API response body.
type presentationDefinitionResponse struct {
	ID                   string              `json:"id"`
	Handle               string              `json:"handle"`
	OUID                 string              `json:"ouId"`
	OUHandle             string              `json:"ouHandle,omitempty"`
	DisplayName          string              `json:"displayName,omitempty"`
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
	return presentationDefinitionResponse(dto)
}

// PresentationDefinitionList is the minimal projection returned by the list
// endpoint. It contains only the fields the management UI renders in the table.
type PresentationDefinitionList struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	OUID        string `json:"ouId"`
	OUHandle    string `json:"ouHandle,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	VCT         string `json:"vct"`
	Format      string `json:"format"`
}

// toSummary projects a full DTO to a list summary.
func toSummary(dto PresentationDefinitionDTO) PresentationDefinitionList {
	return PresentationDefinitionList{
		ID:          dto.ID,
		Handle:      dto.Handle,
		OUID:        dto.OUID,
		OUHandle:    dto.OUHandle,
		DisplayName: dto.DisplayName,
		VCT:         dto.VCT,
		Format:      dto.Format,
	}
}
