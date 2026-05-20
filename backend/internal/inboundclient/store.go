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

package inboundclient

import (
	"context"
	"encoding/json"
	"fmt"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// inboundClientJSONBlob is the internal structure for marshaling/unmarshaling the
// PROPERTIES column.
type inboundClientJSONBlob struct {
	Assertion        *inboundmodel.AssertionConfig    `json:"assertion,omitempty"`
	LoginConsent     *inboundmodel.LoginConsentConfig `json:"loginConsent,omitempty"`
	AllowedUserTypes []string                         `json:"allowedUserTypes,omitempty"`
	Properties       map[string]interface{}           `json:"properties,omitempty"`
}

// inboundClientStoreInterface defines persistence operations for inbound clients.
// All operations are keyed by entity ID so the same store serves applications, agents, and
// any future principal category. OAuth methods accept typed OAuthProfile; the store
// handles JSON marshaling internally so callers never need to know the wire format.
type inboundClientStoreInterface interface {
	CreateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error
	CreateOAuthProfile(ctx context.Context, entityID string, oauthProfile *inboundmodel.OAuthProfile) error
	GetInboundClientByEntityID(ctx context.Context, entityID string) (*inboundmodel.InboundClient, error)
	GetOAuthProfileByEntityID(ctx context.Context, entityID string) (*inboundmodel.OAuthProfile, error)
	GetInboundClientList(ctx context.Context, limit int) ([]inboundmodel.InboundClient, error)
	GetTotalInboundClientCount(ctx context.Context) (int, error)
	UpdateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error
	UpdateOAuthProfile(ctx context.Context, entityID string, oauthProfile *inboundmodel.OAuthProfile) error
	DeleteInboundClient(ctx context.Context, entityID string) error
	DeleteOAuthProfile(ctx context.Context, entityID string) error
	InboundClientExists(ctx context.Context, entityID string) (bool, error)
	// IsDeclarative reports whether the inbound client with the given entity ID is sourced
	// from a declarative (YAML) resource and therefore immutable. DB-backed stores always
	// return false; file-based stores return true when the inbound client exists in their
	// in-memory set.
	IsDeclarative(ctx context.Context, entityID string) bool
}

// store implements inboundClientStoreInterface using the configured config database.
type store struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// getDBProvider is a package-level indirection to allow test override.
var getDBProvider = provider.GetDBProvider

// newStore returns a database-backed inbound client store along with its transactioner.
func newStore() (inboundClientStoreInterface, transaction.Transactioner, error) {
	dbProvider := getDBProvider()
	client, err := dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, nil, err
	}

	transactioner, err := dbProvider.GetConfigDBTransactioner()
	if err != nil {
		return nil, nil, err
	}

	deploymentID := config.GetServerRuntime().Config.Server.Identifier
	if _, err := client.QueryContext(context.Background(), queryGetInboundClientCount, deploymentID); err != nil {
		return nil, nil, fmt.Errorf("failed to verify inbound client table: %w", err)
	}

	return &store{
		dbProvider:   dbProvider,
		deploymentID: deploymentID,
	}, transactioner, nil
}

// marshalInboundClient marshals the JSON fields of an InboundClient and returns the prepared
// values ready for a SQL statement.
func marshalInboundClient(c inboundmodel.InboundClient) (
	propertiesBytes interface{},
	isRegistrationEnabledStr string,
	isRecoveryEnabledStr string,
	recoveryFlowID, registrationFlowID, themeID, layoutID interface{},
	err error,
) {
	blob := inboundClientJSONBlob{
		Assertion:        c.Assertion,
		LoginConsent:     c.LoginConsent,
		AllowedUserTypes: c.AllowedUserTypes,
		Properties:       c.Properties,
	}
	propertiesBytes, err = marshalNullableJSON(blob)
	if err != nil {
		return nil, "", "", nil, nil, nil, nil, fmt.Errorf("failed to marshal properties: %w", err)
	}

	isRegistrationEnabledStr = utils.BoolToNumString(c.IsRegistrationFlowEnabled)
	isRecoveryEnabledStr = utils.BoolToNumString(c.IsRecoveryFlowEnabled)

	if c.RecoveryFlowID != "" {
		recoveryFlowID = c.RecoveryFlowID
	}
	if c.RegistrationFlowID != "" {
		registrationFlowID = c.RegistrationFlowID
	}
	if c.ThemeID != "" {
		themeID = c.ThemeID
	}
	if c.LayoutID != "" {
		layoutID = c.LayoutID
	}

	return propertiesBytes, isRegistrationEnabledStr, isRecoveryEnabledStr, recoveryFlowID,
		registrationFlowID, themeID, layoutID, nil
}

// CreateInboundClient creates a new inbound client entry.
func (st *store) CreateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	propsBytes, isRegEnabledStr, isRecoveryEnabledStr, recoveryFlowID,
		registrationFlowID, themeID, layoutID, marshalErr := marshalInboundClient(client)
	if marshalErr != nil {
		return marshalErr
	}

	_, err = dbClient.ExecuteContext(ctx, queryCreateInboundClient,
		client.ID, client.AuthFlowID, registrationFlowID, isRegEnabledStr,
		recoveryFlowID, isRecoveryEnabledStr, themeID, layoutID, propsBytes, st.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to insert inbound client: %w", err)
	}
	return nil
}

// CreateOAuthProfile creates a new OAuth inbound profile entry. The typed profile is
// marshaled to JSON internally.
func (st *store) CreateOAuthProfile(ctx context.Context, entityID string,
	oauthProfile *inboundmodel.OAuthProfile) error {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	profileJSON, err := marshalOAuthProfile(oauthProfile)
	if err != nil {
		return err
	}

	_, err = dbClient.ExecuteContext(ctx, queryCreateOAuthProfile, entityID, profileJSON, st.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to insert OAuth profile: %w", err)
	}
	return nil
}

// GetInboundClientByEntityID retrieves an inbound client by entity ID.
func (st *store) GetInboundClientByEntityID(ctx context.Context, entityID string) (*inboundmodel.InboundClient, error) {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetInboundClientByEntityID, entityID, st.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return nil, ErrInboundClientNotFound
	}
	return buildInboundClientFromRow(results[0])
}

// GetOAuthProfileByEntityID retrieves an OAuth profile by entity ID.
func (st *store) GetOAuthProfileByEntityID(ctx context.Context, entityID string) (*inboundmodel.OAuthProfile, error) {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetOAuthProfileByEntityID, entityID, st.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return nil, ErrInboundClientNotFound
	}
	return buildOAuthProfileFromRow(results[0])
}

// GetInboundClientList retrieves all inbound clients.
func (st *store) GetInboundClientList(ctx context.Context, limit int) ([]inboundmodel.InboundClient, error) {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetInboundClientList, st.deploymentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	clients := make([]inboundmodel.InboundClient, 0, len(results))
	for _, row := range results {
		c, err := buildInboundClientFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build inbound client from result row: %w", err)
		}
		clients = append(clients, *c)
	}
	return clients, nil
}

// GetTotalInboundClientCount retrieves the total count of inbound clients.
func (st *store) GetTotalInboundClientCount(ctx context.Context) (int, error) {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetInboundClientCount, st.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) > 0 {
		if total, ok := results[0]["total"].(int64); ok {
			return int(total), nil
		}
		return 0, fmt.Errorf("failed to parse total count")
	}
	return 0, nil
}

// UpdateInboundClient updates an inbound client.
func (st *store) UpdateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	propsBytes, isRegEnabledStr, isRecoveryEnabledStr, recoveryFlowID,
		registrationFlowID, themeID, layoutID, marshalErr := marshalInboundClient(client)
	if marshalErr != nil {
		return marshalErr
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryUpdateInboundClientByEntityID,
		client.ID, client.AuthFlowID, registrationFlowID, isRegEnabledStr,
		recoveryFlowID, isRecoveryEnabledStr, themeID, layoutID, propsBytes, st.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update inbound client: %w", err)
	}
	if rowsAffected == 0 {
		return ErrInboundClientNotFound
	}
	return nil
}

// UpdateOAuthProfile updates an OAuth profile for an entity. The typed profile is marshaled
// to JSON internally.
func (st *store) UpdateOAuthProfile(ctx context.Context, entityID string,
	oauthProfile *inboundmodel.OAuthProfile) error {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	profileJSON, err := marshalOAuthProfile(oauthProfile)
	if err != nil {
		return err
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryUpdateOAuthProfileByEntityID,
		entityID, profileJSON, st.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update OAuth profile: %w", err)
	}
	if rowsAffected == 0 {
		return ErrInboundClientNotFound
	}
	return nil
}

// marshalOAuthProfile serializes an OAuthProfile to the OAUTH_CONFIG JSON format.
// Returns nil bytes for nil input.
func marshalOAuthProfile(p *inboundmodel.OAuthProfile) (json.RawMessage, error) {
	if p == nil {
		return nil, nil
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OAuth profile JSON: %w", err)
	}
	return data, nil
}

// DeleteInboundClient deletes an inbound client by entity ID. Cascades to OAuth profile.
func (st *store) DeleteInboundClient(ctx context.Context, entityID string) error {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteInboundClientByEntityID, entityID, st.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete inbound client: %w", err)
	}
	return nil
}

// DeleteOAuthProfile deletes an OAuth profile by entity ID.
func (st *store) DeleteOAuthProfile(ctx context.Context, entityID string) error {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteOAuthProfileByEntityID, entityID, st.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete OAuth profile: %w", err)
	}
	return nil
}

// InboundClientExists checks if an inbound client exists by entity ID.
func (st *store) InboundClientExists(ctx context.Context, entityID string) (bool, error) {
	dbClient, err := st.dbProvider.GetConfigDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryCheckInboundClientExistsByEntityID, entityID, st.deploymentID)
	if err != nil {
		return false, fmt.Errorf("failed to execute existence check query: %w", err)
	}

	return parseBoolFromCount(results)
}

// IsDeclarative returns false for the DB-backed store since all DB-backed inbound clients are mutable.
func (st *store) IsDeclarative(_ context.Context, _ string) bool {
	return false
}

// --- Helper functions ---

// buildInboundClientFromRow constructs an InboundClient from a database result row.
func buildInboundClientFromRow(row map[string]interface{}) (*inboundmodel.InboundClient, error) {
	entityID, ok := row["entity_id"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse entity_id as string")
	}

	authFlowID := parseStringColumn(row, "auth_flow_id")
	regFlowID := parseStringColumn(row, "registration_flow_id")
	recoveryFlowID := parseStringColumn(row, "recovery_flow_id")
	themeID := parseStringColumn(row, "theme_id")
	layoutID := parseStringColumn(row, "layout_id")

	isRegistrationFlowEnabled := false
	if val := parseStringOrBytesColumn(row, "is_registration_flow_enabled"); val != "" {
		isRegistrationFlowEnabled = utils.NumStringToBool(val)
	}

	isRecoveryFlowEnabled := false
	if val := parseStringOrBytesColumn(row, "is_recovery_flow_enabled"); val != "" {
		isRecoveryFlowEnabled = utils.NumStringToBool(val)
	}

	client := &inboundmodel.InboundClient{
		ID:                        entityID,
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        regFlowID,
		IsRegistrationFlowEnabled: isRegistrationFlowEnabled,
		RecoveryFlowID:            recoveryFlowID,
		IsRecoveryFlowEnabled:     isRecoveryFlowEnabled,
		ThemeID:                   themeID,
		LayoutID:                  layoutID,
	}

	if blobStr := parseJSONColumnString(row, "properties"); blobStr != "" {
		var blob inboundClientJSONBlob
		if err := json.Unmarshal([]byte(blobStr), &blob); err != nil {
			log.GetLogger().Debug("Failed to unmarshal properties", log.Error(err))
		} else {
			client.Assertion = blob.Assertion
			client.LoginConsent = blob.LoginConsent
			client.AllowedUserTypes = blob.AllowedUserTypes
			client.Properties = blob.Properties
		}
	}

	return client, nil
}

// buildOAuthProfileFromRow constructs an OAuthProfile from a database result row.
// Returns nil when the row has no oauth_config payload.
func buildOAuthProfileFromRow(row map[string]interface{}) (*inboundmodel.OAuthProfile, error) {
	profileStr := parseJSONColumnString(row, "oauth_config")
	if profileStr == "" {
		return nil, nil
	}
	var p inboundmodel.OAuthProfile
	if err := json.Unmarshal([]byte(profileStr), &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OAuth profile JSON: %w", err)
	}
	return &p, nil
}

// marshalNullableJSON marshals a value to JSON, returning nil for nil/empty input.
func marshalNullableJSON(v interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(data) == "null" {
		return nil, nil
	}
	return data, nil
}

// parseStringColumn safely extracts a string from a result row, returning "" for nil.
func parseStringColumn(row map[string]interface{}, key string) string {
	if row[key] == nil {
		return ""
	}
	if s, ok := row[key].(string); ok {
		return s
	}
	return ""
}

// parseStringOrBytesColumn handles columns that may come as string or []byte.
func parseStringOrBytesColumn(row map[string]interface{}, key string) string {
	if row[key] == nil {
		return ""
	}
	switch v := row[key].(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// parseJSONColumnString extracts a JSON column value as a string.
func parseJSONColumnString(row map[string]interface{}, column string) string {
	val, exists := row[column]
	if !exists || val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// parseBoolFromCount parses a boolean from a COUNT(*) query result.
func parseBoolFromCount(results []map[string]interface{}) (bool, error) {
	if len(results) == 0 {
		return false, nil
	}
	count, ok := results[0]["count"].(int64)
	if !ok {
		return false, fmt.Errorf("failed to parse count from query result")
	}
	return count > 0, nil
}
