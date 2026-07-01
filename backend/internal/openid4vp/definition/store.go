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

package definition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// ErrNotFound is the store-level not-found sentinel.
var ErrNotFound = errors.New("openid4vp: presentation definition not found")

// claimsBlob is the JSON shape of the CLAIMS column.
type claimsBlob struct {
	Requested []string            `json:"requested,omitempty"`
	Mandatory []string            `json:"mandatory,omitempty"`
	Optional  []string            `json:"optional,omitempty"`
	Values    map[string][]string `json:"values,omitempty"`
}

// definitionStoreInterface persists managed presentation definitions in configdb.
type definitionStoreInterface interface {
	CreatePresentationDefinition(ctx context.Context, dto PresentationDefinitionDTO) error
	GetPresentationDefinitionByID(ctx context.Context, id string) (*PresentationDefinitionDTO, error)
	GetPresentationDefinitionByHandle(ctx context.Context, handle string) (*PresentationDefinitionDTO, error)
	ListPresentationDefinitions(ctx context.Context) ([]PresentationDefinitionDTO, error)
	ListPresentationDefinitionSummaries(ctx context.Context) ([]PresentationDefinitionList, error)
	UpdatePresentationDefinition(ctx context.Context, dto PresentationDefinitionDTO) error
	DeletePresentationDefinition(ctx context.Context, id string) error
	IsPresentationDefinitionDeclarative(ctx context.Context, id string) (bool, error)
}

type definitionStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newDefinitionStore returns a configdb-backed presentation-definition store.
func newDefinitionStore() definitionStoreInterface {
	return &definitionStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// CreatePresentationDefinition inserts a new presentation definition into the config database.
func (s *definitionStore) CreatePresentationDefinition(ctx context.Context, dto PresentationDefinitionDTO) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	claimsJSON, err := marshalClaims(dto)
	if err != nil {
		return err
	}
	authoritiesJSON, err := marshalTrustedAuthorities(dto.TrustedAuthorities)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryCreateDefinition,
		dto.ID, dto.Handle, dto.OUID, dto.DisplayName, dto.VCT, dto.Format, claimsJSON,
		nullableBool(dto.EnforceTrustedIssuer), authoritiesJSON, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to create presentation definition: %w", err)
	}
	return nil
}

// GetPresentationDefinitionByID returns the presentation definition matching the given ID.
func (s *definitionStore) GetPresentationDefinitionByID(
	ctx context.Context, id string,
) (*PresentationDefinitionDTO, error) {
	return s.getOne(ctx, queryGetDefinitionByID, id)
}

// GetPresentationDefinitionByHandle returns the presentation definition matching the given handle.
func (s *definitionStore) GetPresentationDefinitionByHandle(
	ctx context.Context, handle string,
) (*PresentationDefinitionDTO, error) {
	return s.getOne(ctx, queryGetDefinitionByHandle, handle)
}

// getOne runs the given single-row query with the identifier and returns the resulting definition, or ErrNotFound.
func (s *definitionStore) getOne(
	ctx context.Context, query dbmodel.DBQuery, identifier string,
) (*PresentationDefinitionDTO, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, query, identifier, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query presentation definition: %w", err)
	}
	if len(results) == 0 {
		return nil, ErrNotFound
	}
	return buildDefinitionDTOFromRow(results[0])
}

// ListPresentationDefinitions returns all presentation definitions stored in the config database.
func (s *definitionStore) ListPresentationDefinitions(ctx context.Context) ([]PresentationDefinitionDTO, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryListDefinitions, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list presentation definitions: %w", err)
	}
	defs := make([]PresentationDefinitionDTO, 0, len(results))
	for _, row := range results {
		dto, err := buildDefinitionDTOFromRow(row)
		if err != nil {
			return nil, err
		}
		defs = append(defs, *dto)
	}
	return defs, nil
}

// ListPresentationDefinitionSummaries returns summary rows for all presentation definitions in the config database.
func (s *definitionStore) ListPresentationDefinitionSummaries(
	ctx context.Context,
) ([]PresentationDefinitionList, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryListDefinitionSummaries, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list presentation definition summaries: %w", err)
	}
	summaries := make([]PresentationDefinitionList, 0, len(results))
	for _, row := range results {
		summaries = append(summaries, PresentationDefinitionList{
			ID:          columnString(row["id"]),
			Handle:      columnString(row["handle"]),
			OUID:        columnString(row["ou_id"]),
			DisplayName: columnString(row["display_name"]),
			VCT:         columnString(row["vct"]),
			Format:      columnString(row["format"]),
		})
	}
	return summaries, nil
}

// UpdatePresentationDefinition persists changes to an existing presentation definition in the config database.
func (s *definitionStore) UpdatePresentationDefinition(ctx context.Context, dto PresentationDefinitionDTO) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	claimsJSON, err := marshalClaims(dto)
	if err != nil {
		return err
	}
	authoritiesJSON, err := marshalTrustedAuthorities(dto.TrustedAuthorities)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryUpdateDefinition,
		dto.ID, dto.Handle, dto.OUID, dto.DisplayName, dto.VCT, dto.Format, claimsJSON,
		nullableBool(dto.EnforceTrustedIssuer), authoritiesJSON, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update presentation definition: %w", err)
	}
	return nil
}

// DeletePresentationDefinition removes the presentation definition with the given ID from the config database.
func (s *definitionStore) DeletePresentationDefinition(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteDefinition, id, s.deploymentID); err != nil {
		return fmt.Errorf("failed to delete presentation definition: %w", err)
	}
	return nil
}

// IsDeclarative reports whether the presentation definition is file-based.
// The database store only holds mutable resources, so this always returns false.
func (s *definitionStore) IsPresentationDefinitionDeclarative(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// marshalClaims serializes the claim sets to the CLAIMS JSON column.
func marshalClaims(dto PresentationDefinitionDTO) (string, error) {
	claimsBytes, err := json.Marshal(claimsBlob{
		Requested: dto.RequestedClaims,
		Mandatory: dto.MandatoryClaims,
		Optional:  dto.OptionalClaims,
		Values:    dto.ClaimValues,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %w", err)
	}
	return string(claimsBytes), nil
}

// marshalTrustedAuthorities serializes the trusted-authority names to a JSON
// array, returning nil (SQL NULL) when none are configured.
func marshalTrustedAuthorities(names []string) (interface{}, error) {
	if len(names) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(names)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trusted authorities: %w", err)
	}
	return string(b), nil
}

// nullableBool maps an optional bool to its SQL value, preserving NULL (inherit).
func nullableBool(v *bool) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

// buildDefinitionDTOFromRow reconstructs a DTO from a result row.
func buildDefinitionDTOFromRow(row map[string]interface{}) (*PresentationDefinitionDTO, error) {
	dto := &PresentationDefinitionDTO{
		ID:          columnString(row["id"]),
		Handle:      columnString(row["handle"]),
		OUID:        columnString(row["ou_id"]),
		DisplayName: columnString(row["display_name"]),
		VCT:         columnString(row["vct"]),
		Format:      columnString(row["format"]),
	}
	if claimsBytes := columnBytes(row["claims"]); len(claimsBytes) > 0 {
		var cb claimsBlob
		if err := json.Unmarshal(claimsBytes, &cb); err != nil {
			return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
		}
		dto.RequestedClaims = cb.Requested
		dto.MandatoryClaims = cb.Mandatory
		dto.OptionalClaims = cb.Optional
		dto.ClaimValues = cb.Values
	}
	dto.EnforceTrustedIssuer = columnNullableBool(row["enforce_trusted_issuer"])
	if authoritiesBytes := columnBytes(row["trusted_authorities"]); len(authoritiesBytes) > 0 {
		if err := json.Unmarshal(authoritiesBytes, &dto.TrustedAuthorities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal trusted authorities: %w", err)
		}
	}
	return dto, nil
}

// columnNullableBool coerces a result-row value to an optional bool, tolerating
// the bool/int64/[]byte representations returned by the supported drivers.
// NULL (nil) yields a nil pointer (inherit the engine default).
func columnNullableBool(v interface{}) *bool {
	switch t := v.(type) {
	case nil:
		return nil
	case bool:
		return &t
	case int64:
		b := t != 0
		return &b
	default:
		return nil
	}
}

// columnString coerces a result-row value to a string, tolerating string/[]byte.
func columnString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

// columnBytes coerces a result-row value to bytes, tolerating []byte/string.
func columnBytes(v interface{}) []byte {
	switch t := v.(type) {
	case []byte:
		return t
	case string:
		return []byte(t)
	default:
		return nil
	}
}
