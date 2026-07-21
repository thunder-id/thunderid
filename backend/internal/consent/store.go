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

package consent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// consentStoreInterface defines the persistence operations for consent records.
type consentStoreInterface interface {
	CreateConsent(ctx context.Context, consent *Consent) error
	GetConsent(ctx context.Context, id string) (*Consent, error)
	UpdateConsent(ctx context.Context, consent *Consent) error
	SearchConsents(ctx context.Context, filters ConsentFilter) ([]*Consent, error)
}

// consentStore is the default database-backed implementation of consentStoreInterface.
type consentStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newConsentStore creates a new consentStore along with a transactioner that callers can use to
// wrap the multi-table create and update operations in a single database transaction. Consent
// records are long-lived persistent state, so they are persisted to the runtime persistent datasource.
func newConsentStore() (consentStoreInterface, transaction.Transactioner, error) {
	dbProvider := provider.GetDBProvider()

	transactioner, err := dbProvider.GetRuntimePersistentDBTransactioner()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get transactioner: %w", err)
	}

	return &consentStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}, transactioner, nil
}

// CreateConsent persists a consent record and its authorization records.
func (s *consentStore) CreateConsent(ctx context.Context, consent *Consent) error {
	dbClient, err := s.dbProvider.GetRuntimePersistentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	purposes, err := marshalPurposes(consent.Purposes)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = dbClient.ExecuteContext(
		ctx,
		QueryCreateConsent,
		consent.ID,
		consent.GroupID,
		string(consent.Status),
		unixToNullableTime(consent.ValidityTime),
		purposes,
		s.deploymentID,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create consent: %w", err)
	}

	return s.insertAuthorizations(ctx, dbClient, consent.ID, consent.Authorizations)
}

// GetConsent retrieves a consent record and its authorization records by id.
func (s *consentStore) GetConsent(ctx context.Context, id string) (*Consent, error) {
	dbClient, err := s.dbProvider.GetRuntimePersistentDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetConsentByID, id, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return nil, errConsentNotFound
	}

	consent, err := buildConsentFromResultRow(results[0])
	if err != nil {
		return nil, err
	}

	authorizationsByConsent, err := s.getAuthorizations(ctx, dbClient, []string{consent.ID})
	if err != nil {
		return nil, err
	}
	consent.Authorizations = authorizationsByConsent[consent.ID]

	return consent, nil
}

// UpdateConsent updates an existing consent record and replaces its authorization records.
func (s *consentStore) UpdateConsent(ctx context.Context, consent *Consent) error {
	dbClient, err := s.dbProvider.GetRuntimePersistentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	purposes, err := marshalPurposes(consent.Purposes)
	if err != nil {
		return err
	}

	rowsAffected, err := dbClient.ExecuteContext(
		ctx,
		QueryUpdateConsent,
		consent.ID,
		string(consent.Status),
		unixToNullableTime(consent.ValidityTime),
		purposes,
		time.Now().UTC(),
		s.deploymentID,
	)
	if err != nil {
		return fmt.Errorf("failed to update consent: %w", err)
	}

	if rowsAffected == 0 {
		return errConsentNotFound
	}

	if _, err := dbClient.ExecuteContext(
		ctx, QueryDeleteConsentAuthorizations, consent.ID, s.deploymentID); err != nil {
		return fmt.Errorf("failed to delete consent authorizations: %w", err)
	}

	return s.insertAuthorizations(ctx, dbClient, consent.ID, consent.Authorizations)
}

// SearchConsents retrieves consent records matching the given filters, each with its authorizations.
func (s *consentStore) SearchConsents(ctx context.Context, filters ConsentFilter) ([]*Consent, error) {
	dbClient, err := s.dbProvider.GetRuntimePersistentDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, args := buildSearchConsentsQuery(filters, s.deploymentID)
	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	consents := make([]*Consent, 0, len(results))
	consentIDs := make([]string, 0, len(results))
	for _, row := range results {
		consent, err := buildConsentFromResultRow(row)
		if err != nil {
			return nil, err
		}
		consents = append(consents, consent)
		consentIDs = append(consentIDs, consent.ID)
	}

	if len(consents) == 0 {
		return consents, nil
	}

	authorizationsByConsent, err := s.getAuthorizations(ctx, dbClient, consentIDs)
	if err != nil {
		return nil, err
	}
	for _, consent := range consents {
		consent.Authorizations = authorizationsByConsent[consent.ID]
	}

	return consents, nil
}

func (s *consentStore) insertAuthorizations(
	ctx context.Context, dbClient provider.DBClientInterface, consentID string,
	authorizations []ConsentAuthorization,
) error {
	if len(authorizations) == 0 {
		return nil
	}

	query, args := buildInsertConsentAuthorizationsQuery(consentID, authorizations, s.deploymentID)
	if _, err := dbClient.ExecuteContext(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to create consent authorizations: %w", err)
	}
	return nil
}

// getAuthorizations retrieves the authorization records for the given consent IDs in a single query,
// grouped by consent ID.
func (s *consentStore) getAuthorizations(
	ctx context.Context, dbClient provider.DBClientInterface, consentIDs []string,
) (map[string][]ConsentAuthorization, error) {
	query, args := buildGetConsentAuthorizationsQuery(consentIDs, s.deploymentID)
	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get consent authorizations: %w", err)
	}

	authorizationsByConsent := make(map[string][]ConsentAuthorization)
	for _, row := range results {
		consentID, err := parseStringColumn(row, "consent_id")
		if err != nil {
			return nil, err
		}
		authorization, err := buildAuthorizationFromResultRow(row)
		if err != nil {
			return nil, err
		}
		authorizationsByConsent[consentID] = append(authorizationsByConsent[consentID], authorization)
	}

	return authorizationsByConsent, nil
}

// buildConsentFromResultRow constructs a Consent (without authorizations) from a database result row.
func buildConsentFromResultRow(row map[string]interface{}) (*Consent, error) {
	id, err := parseStringColumn(row, "id")
	if err != nil {
		return nil, err
	}
	groupID, err := parseStringColumn(row, "group_id")
	if err != nil {
		return nil, err
	}
	status, err := parseStringColumn(row, "status")
	if err != nil {
		return nil, err
	}

	validityTime, err := parseUnixColumn(row, "validity_time")
	if err != nil {
		return nil, err
	}

	purposes, err := unmarshalPurposes(row["purposes"])
	if err != nil {
		return nil, err
	}

	return &Consent{
		ID:           id,
		GroupID:      groupID,
		Status:       ConsentStatus(status),
		ValidityTime: validityTime,
		Purposes:     purposes,
	}, nil
}

// buildAuthorizationFromResultRow constructs a ConsentAuthorization from a database result row.
func buildAuthorizationFromResultRow(row map[string]interface{}) (ConsentAuthorization, error) {
	id, err := parseStringColumn(row, "id")
	if err != nil {
		return ConsentAuthorization{}, err
	}
	userID, err := parseStringColumn(row, "user_id")
	if err != nil {
		return ConsentAuthorization{}, err
	}
	authorizationType, err := parseStringColumn(row, "type")
	if err != nil {
		return ConsentAuthorization{}, err
	}
	status, err := parseStringColumn(row, "status")
	if err != nil {
		return ConsentAuthorization{}, err
	}

	updatedTime, err := parseUnixColumn(row, "updated_time")
	if err != nil {
		return ConsentAuthorization{}, err
	}

	return ConsentAuthorization{
		ID:          id,
		UserID:      userID,
		Type:        ConsentAuthorizationType(authorizationType),
		Status:      ConsentAuthorizationStatus(status),
		UpdatedTime: updatedTime,
	}, nil
}

// marshalPurposes serializes the per-element approval decisions for storage in the PURPOSES column.
func marshalPurposes(purposes []ConsentPurposeItem) (string, error) {
	data, err := json.Marshal(purposes)
	if err != nil {
		return "", fmt.Errorf("failed to marshal consent purposes: %w", err)
	}
	return string(data), nil
}

// unmarshalPurposes deserializes the PURPOSES column value into the consent purpose items.
func unmarshalPurposes(value interface{}) ([]ConsentPurposeItem, error) {
	var raw string
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return nil, fmt.Errorf("failed to parse purposes as string")
	}

	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var purposes []ConsentPurposeItem
	if err := json.Unmarshal([]byte(raw), &purposes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal consent purposes: %w", err)
	}

	return purposes, nil
}

// parseStringColumn extracts a string column value from a database result row.
func parseStringColumn(row map[string]interface{}, column string) (string, error) {
	value, ok := row[column].(string)
	if !ok {
		return "", fmt.Errorf("failed to parse %s as string", column)
	}
	return value, nil
}

// parseUnixColumn extracts a nullable DATETIME column and returns it as a Unix timestamp.
// A NULL column value yields a zero timestamp.
func parseUnixColumn(row map[string]interface{}, column string) (int64, error) {
	value, exists := row[column]
	if !exists || value == nil {
		return 0, nil
	}

	t, err := sysutils.ParseDBTimeField(value, column)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

// unixToNullableTime converts a Unix timestamp into a value suitable for a nullable DATETIME column,
// returning nil for a zero timestamp so the column stores NULL.
func unixToNullableTime(sec int64) interface{} {
	if sec == 0 {
		return nil
	}
	return time.Unix(sec, 0).UTC()
}
