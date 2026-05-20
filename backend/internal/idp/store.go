/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package idp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

var getDBProvider = provider.GetDBProvider

// idpStoreInterface defines the interface for identity provider store operations.
type idpStoreInterface interface {
	CreateIdentityProvider(ctx context.Context, idp IDPDTO) error
	GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, error)
	GetIdentityProviderListCount(ctx context.Context) (int, error)
	GetIdentityProvider(ctx context.Context, idpID string) (*IDPDTO, error)
	GetIdentityProviderByName(ctx context.Context, idpName string) (*IDPDTO, error)
	GetIdentityProviderByIssuer(ctx context.Context, issuer string) (*IDPDTO, error)
	UpdateIdentityProvider(ctx context.Context, idp *IDPDTO) error
	DeleteIdentityProvider(ctx context.Context, idpID string) error
}

// idpStore is the default implementation of IDPStoreInterface.
type idpStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newIDPStore creates a new instance of IDPStore.
func newIDPStore() (idpStoreInterface, transaction.Transactioner, error) {
	dbProvider := getDBProvider()
	client, err := dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, nil, err
	}
	transactioner, err := client.GetTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &idpStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}, transactioner, nil
}

// CreateIdentityProvider handles the IdP creation in the database.
func (s *idpStore) CreateIdentityProvider(ctx context.Context, idp IDPDTO) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	var propertiesJSON string
	if len(idp.Properties) > 0 {
		propertiesJSON, err = cmodels.SerializePropertiesToJSONArray(idp.Properties)
		if err != nil {
			return fmt.Errorf("failed to serialize properties to JSON: %w", err)
		}
	}

	_, err = dbClient.ExecuteContext(ctx,
		queryCreateIdentityProvider, idp.ID, idp.Name, idp.Description, idp.Type, propertiesJSON, s.deploymentID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// GetIdentityProviderList retrieves a list of IdPs from the database.
func (s *idpStore) GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetIdentityProviderList, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	idpList := make([]BasicIDPDTO, 0)
	for _, row := range results {
		idp, err := buildIDPFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build idp from result row: %w", err)
		}
		idpList = append(idpList, *idp)
	}

	return idpList, nil
}

// GetIdentityProviderListCount retrieves the total count of identity providers.
func (s *idpStore) GetIdentityProviderListCount(ctx context.Context) (int, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetIdentityProviderListCount, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 || len(results[0]) == 0 {
		return 0, nil
	}

	countVal, ok := results[0]["count"]
	if !ok {
		return 0, fmt.Errorf("count field not found in result")
	}

	switch v := countVal.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected count type: %T", countVal)
	}
}

// GetIdentityProvider retrieves a specific idp by its ID from the database.
func (s *idpStore) GetIdentityProvider(ctx context.Context, id string) (*IDPDTO, error) {
	return s.getIDP(ctx, queryGetIdentityProviderByID, id)
}

// GetIdentityProviderByName retrieves a specific idp by its name from the database.
func (s *idpStore) GetIdentityProviderByName(ctx context.Context, name string) (*IDPDTO, error) {
	return s.getIDP(ctx, queryGetIdentityProviderByName, name)
}

// GetIdentityProviderByIssuer retrieves a specific idp by its issuer property from the database.
func (s *idpStore) GetIdentityProviderByIssuer(ctx context.Context, issuer string) (*IDPDTO, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	// For Postgres the $1 placeholder expects a JSON fragment; for SQLite it expects the raw string value.
	// Build the JSON fragment safely via json.Marshal to avoid injection.
	type issuerEntry struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	pgParam, err := json.Marshal([]issuerEntry{{Name: "issuer", Value: issuer}})
	if err != nil {
		return nil, fmt.Errorf("failed to build issuer query parameter: %w", err)
	}

	// Select the right argument for the dialect. The DBClient picks the correct query string
	// internally, but we must supply the matching arg for that query's $1 placeholder.
	var param string
	if config.GetServerRuntime().Config.Database.Config.Type == "postgres" {
		param = string(pgParam)
	} else {
		param = issuer
	}

	results, err := dbClient.QueryContext(ctx, queryGetIdentityProviderByIssuer, param, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return nil, ErrIDPNotFound
	}

	row := results[0]

	basicIDP, err := buildIDPFromResultRow(row)
	if err != nil {
		return nil, fmt.Errorf("failed to build idp from result row: %w", err)
	}

	var properties []cmodels.Property
	var propertiesJSON string

	switch v := row["properties"].(type) {
	case string:
		propertiesJSON = v
	case []byte:
		propertiesJSON = string(v)
	}

	if propertiesJSON != "" {
		properties, err = cmodels.DeserializePropertiesFromJSON(propertiesJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize properties from JSON: %w", err)
		}
	}

	return &IDPDTO{
		ID:          basicIDP.ID,
		Name:        basicIDP.Name,
		Description: basicIDP.Description,
		Type:        basicIDP.Type,
		Properties:  properties,
	}, nil
}

// getIDP retrieves an IDP based on the provided query and identifier.
func (s *idpStore) getIDP(ctx context.Context, query dbmodel.DBQuery, identifier string) (*IDPDTO, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, query, identifier, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return nil, ErrIDPNotFound
	}
	if len(results) != 1 {
		return nil, fmt.Errorf("unexpected number of results: %d", len(results))
	}

	row := results[0]

	basicIDP, err := buildIDPFromResultRow(row)
	if err != nil {
		return nil, fmt.Errorf("failed to build idp from result row: %w", err)
	}

	var properties []cmodels.Property
	var propertiesJSON string

	// Handle both string and []byte types for properties
	switch v := row["properties"].(type) {
	case string:
		propertiesJSON = v
	case []byte:
		propertiesJSON = string(v)
	}

	if propertiesJSON != "" {
		var err error
		properties, err = cmodels.DeserializePropertiesFromJSON(propertiesJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize properties from JSON: %w", err)
		}
	}

	idp := &IDPDTO{
		ID:          basicIDP.ID,
		Name:        basicIDP.Name,
		Description: basicIDP.Description,
		Type:        basicIDP.Type,
		Properties:  properties,
	}

	return idp, nil
}

// UpdateIdentityProvider updates the idp in the database.
func (s *idpStore) UpdateIdentityProvider(ctx context.Context, idp *IDPDTO) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	var propertiesJSON string
	if len(idp.Properties) > 0 {
		propertiesJSON, err = cmodels.SerializePropertiesToJSONArray(idp.Properties)
		if err != nil {
			return fmt.Errorf("failed to serialize properties to JSON: %w", err)
		}
	}

	// Update the IDP in the database
	_, err = dbClient.ExecuteContext(ctx, queryUpdateIdentityProviderByID, idp.ID, idp.Name,
		idp.Description, idp.Type, propertiesJSON, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// DeleteIdentityProvider deletes the idp from the database.
func (s *idpStore) DeleteIdentityProvider(ctx context.Context, id string) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPStore"))

	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryDeleteIdentityProviderByID, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	if rowsAffected == 0 {
		logger.Debug("idp not found with id: " + id)
	}

	return nil
}

func buildIDPFromResultRow(row map[string]interface{}) (*BasicIDPDTO, error) {
	idpID, ok := row["id"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse id as string")
	}

	idpName, ok := row["name"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse name as string")
	}

	idpDescription, ok := row["description"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse description as string")
	}

	idpType, ok := row["type"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse type as string")
	}

	idp := BasicIDPDTO{
		ID:          idpID,
		Name:        idpName,
		Description: idpDescription,
		Type:        IDPType(idpType),
	}

	return &idp, nil
}
