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
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// credentialStoreInterface persists managed credential configurations in configdb.
type credentialStoreInterface interface {
	CreateCredentialConfiguration(ctx context.Context, dto CredentialConfigurationDTO) error
	GetCredentialConfigurationByID(ctx context.Context, id string) (*CredentialConfigurationDTO, error)
	GetCredentialConfigurationByHandle(ctx context.Context, handle string) (*CredentialConfigurationDTO, error)
	ListCredentialConfigurations(ctx context.Context) ([]CredentialConfigurationDTO, error)
	ListCredentialConfigurationSummaries(ctx context.Context) ([]CredentialConfigurationList, error)
	UpdateCredentialConfiguration(ctx context.Context, dto CredentialConfigurationDTO) error
	DeleteCredentialConfiguration(ctx context.Context, id string) error
	IsCredentialConfigurationDeclarative(ctx context.Context, id string) (bool, error)
}

type credentialStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newCredentialStore returns a configdb-backed credential-configuration store.
func newCredentialStore() credentialStoreInterface {
	return &credentialStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// CreateCredentialConfiguration persists a new credential configuration in the database.
func (s *credentialStore) CreateCredentialConfiguration(ctx context.Context, dto CredentialConfigurationDTO) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	claimsJSON, displayJSON, err := marshalConfiguration(dto)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryCreateConfiguration,
		dto.ID, dto.Handle, dto.OUID, dto.Name, dto.Description, dto.Format, dto.VCT, claimsJSON, displayJSON,
		nullableInt(dto.ValiditySeconds), s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to create credential configuration: %w", err)
	}
	return nil
}

// GetCredentialConfigurationByID retrieves a credential configuration from the database by its ID.
func (s *credentialStore) GetCredentialConfigurationByID(
	ctx context.Context, id string,
) (*CredentialConfigurationDTO, error) {
	return s.getOne(ctx, queryGetConfigurationByID, id)
}

// GetCredentialConfigurationByHandle retrieves a credential configuration from the database by its handle.
func (s *credentialStore) GetCredentialConfigurationByHandle(
	ctx context.Context, handle string,
) (*CredentialConfigurationDTO, error) {
	return s.getOne(ctx, queryGetConfigurationByHandle, handle)
}

// getOne executes the given query with the identifier and returns a single credential configuration, or ErrNotFound.
func (s *credentialStore) getOne(
	ctx context.Context, query dbmodel.DBQuery, identifier string,
) (*CredentialConfigurationDTO, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, query, identifier, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query credential configuration: %w", err)
	}
	if len(results) == 0 {
		return nil, ErrNotFound
	}
	return buildConfigurationDTOFromRow(results[0])
}

// ListCredentialConfigurations retrieves all credential configurations from the database.
func (s *credentialStore) ListCredentialConfigurations(ctx context.Context) ([]CredentialConfigurationDTO, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryListConfigurations, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credential configurations: %w", err)
	}
	configs := make([]CredentialConfigurationDTO, 0, len(results))
	for _, row := range results {
		dto, err := buildConfigurationDTOFromRow(row)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *dto)
	}
	return configs, nil
}

// ListCredentialConfigurationSummaries retrieves summary views of all credential configurations from the database.
func (s *credentialStore) ListCredentialConfigurationSummaries(
	ctx context.Context,
) ([]CredentialConfigurationList, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryListConfigurationSummaries, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credential configuration summaries: %w", err)
	}
	summaries := make([]CredentialConfigurationList, 0, len(results))
	for _, row := range results {
		summary := CredentialConfigurationList{
			ID:     columnString(row["id"]),
			Handle: columnString(row["handle"]),
			OUID:   columnString(row["ou_id"]),
			Name:   columnString(row["name"]),
			Format: columnString(row["format"]),
			VCT:    columnString(row["vct"]),
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// UpdateCredentialConfiguration persists changes to an existing credential configuration in the database.
func (s *credentialStore) UpdateCredentialConfiguration(ctx context.Context, dto CredentialConfigurationDTO) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	claimsJSON, displayJSON, err := marshalConfiguration(dto)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryUpdateConfiguration,
		dto.ID, dto.Handle, dto.OUID, dto.Name, dto.Description, dto.Format, dto.VCT, claimsJSON, displayJSON,
		nullableInt(dto.ValiditySeconds), s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update credential configuration: %w", err)
	}
	return nil
}

// DeleteCredentialConfiguration removes a credential configuration from the database by ID.
func (s *credentialStore) DeleteCredentialConfiguration(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteConfiguration, id, s.deploymentID); err != nil {
		return fmt.Errorf("failed to delete credential configuration: %w", err)
	}
	return nil
}

// IsDeclarative reports whether the credential configuration is file-based.
// The database store only holds mutable resources, so this always returns false.
func (s *credentialStore) IsCredentialConfigurationDeclarative(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// marshalConfiguration serializes the claims and display to JSON columns.
func marshalConfiguration(dto CredentialConfigurationDTO) (claims, display interface{}, err error) {
	if len(dto.Claims) > 0 {
		b, mErr := json.Marshal(dto.Claims)
		if mErr != nil {
			return nil, nil, fmt.Errorf("failed to marshal claims: %w", mErr)
		}
		claims = string(b)
	}
	if dto.Display != nil {
		b, mErr := json.Marshal(dto.Display)
		if mErr != nil {
			return nil, nil, fmt.Errorf("failed to marshal display: %w", mErr)
		}
		display = string(b)
	}
	return claims, display, nil
}

// nullableInt maps an optional int to its SQL value, preserving NULL.
func nullableInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

// buildConfigurationDTOFromRow reconstructs a DTO from a result row.
func buildConfigurationDTOFromRow(row map[string]interface{}) (*CredentialConfigurationDTO, error) {
	dto := &CredentialConfigurationDTO{
		ID:          columnString(row["id"]),
		Handle:      columnString(row["handle"]),
		OUID:        columnString(row["ou_id"]),
		Name:        columnString(row["name"]),
		Description: columnString(row["description"]),
		Format:      columnString(row["format"]),
		VCT:         columnString(row["vct"]),
	}
	if claimsBytes := columnBytes(row["claims"]); len(claimsBytes) > 0 {
		if err := json.Unmarshal(claimsBytes, &dto.Claims); err != nil {
			return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
		}
	}
	if displayBytes := columnBytes(row["display"]); len(displayBytes) > 0 {
		var d CredentialDisplay
		if err := json.Unmarshal(displayBytes, &d); err != nil {
			return nil, fmt.Errorf("failed to unmarshal display: %w", err)
		}
		dto.Display = &d
	}
	dto.ValiditySeconds = columnNullableInt(row["validity_seconds"])
	return dto, nil
}

// columnNullableInt coerces a result-row value to an optional int, tolerating the
// int64/[]byte/string representations returned by the supported drivers.
func columnNullableInt(v interface{}) *int {
	switch t := v.(type) {
	case nil:
		return nil
	case int64:
		i := int(t)
		return &i
	case int:
		return &t
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
