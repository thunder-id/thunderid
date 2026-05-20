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

package notification

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

var (
	serializePropertiesToJSONArray = cmodels.SerializePropertiesToJSONArray
	deserializePropertiesFromJSON  = cmodels.DeserializePropertiesFromJSON
	getDBProvider                  = provider.GetDBProvider
)

// notificationStoreInterface defines the interface for notification sender storage operations.
type notificationStoreInterface interface {
	createSender(ctx context.Context, sender common.NotificationSenderDTO) error
	listSenders(ctx context.Context) ([]common.NotificationSenderDTO, error)
	getSenderByID(ctx context.Context, id string) (*common.NotificationSenderDTO, error)
	getSenderByName(ctx context.Context, name string) (*common.NotificationSenderDTO, error)
	updateSender(ctx context.Context, id string, sender common.NotificationSenderDTO) error
	deleteSender(ctx context.Context, id string) error
}

// notificationStore is the implementation of notificationStoreInterface.
type notificationStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newNotificationStore returns a new instance of notificationStoreInterface.
func newNotificationStore() (notificationStoreInterface, transaction.Transactioner, error) {
	dbProvider := getDBProvider()
	client, err := dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get database client: %w", err)
	}
	tx, err := client.GetTransactioner()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get transactioner: %w", err)
	}
	store := &notificationStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
	return store, tx, nil
}

// createSender creates a new notification sender.
func (s *notificationStore) createSender(ctx context.Context, sender common.NotificationSenderDTO) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	// Serialize properties to JSON
	var propertiesJSON string
	if len(sender.Properties) > 0 {
		propertiesJSON, err = serializePropertiesToJSONArray(sender.Properties)
		if err != nil {
			return fmt.Errorf("failed to serialize properties to JSON: %w", err)
		}
	}

	_, err = dbClient.ExecuteContext(ctx, queryCreateNotificationSender, sender.Name, sender.ID,
		sender.Description, string(sender.Type), string(sender.Provider), propertiesJSON, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// listSenders retrieves all notification senders
func (s *notificationStore) listSenders(ctx context.Context) ([]common.NotificationSenderDTO, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetAllNotificationSenders, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	senders := make([]common.NotificationSenderDTO, 0, len(results))
	for _, row := range results {
		sender, err := s.buildSenderFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build sender from result row: %w", err)
		}

		var propertiesJSON string
		// Handle both string and []byte types for properties
		switch v := row["properties"].(type) {
		case string:
			propertiesJSON = v
		case []byte:
			propertiesJSON = string(v)
		}

		if propertiesJSON != "" {
			properties, err := deserializePropertiesFromJSON(propertiesJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize properties from JSON: %w", err)
			}
			sender.Properties = properties
		}

		senders = append(senders, *sender)
	}

	return senders, nil
}

// getSenderByID retrieves a notification sender by ID.
func (s *notificationStore) getSenderByID(ctx context.Context, id string) (*common.NotificationSenderDTO, error) {
	return s.getSender(ctx, queryGetNotificationSenderByID, id)
}

// getSenderByName retrieves a notification sender by name
func (s *notificationStore) getSenderByName(ctx context.Context, name string) (*common.NotificationSenderDTO, error) {
	return s.getSender(ctx, queryGetNotificationSenderByName, name)
}

// getSender retrieves a notification sender by a specific identifier (ID or name).
func (s *notificationStore) getSender(ctx context.Context, query dbmodel.DBQuery,
	identifier string) (*common.NotificationSenderDTO, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationStore"))

	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, query, identifier, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		logger.Debug("Notification sender not found", log.String("identifier", identifier))
		return nil, nil
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("multiple senders found for identifier: %s", identifier)
	}

	sender, err := s.buildSenderFromResultRow(results[0])
	if err != nil {
		return nil, fmt.Errorf("failed to build sender from result row: %w", err)
	}

	var propertiesJSON string
	// Handle both string and []byte types for properties
	switch v := results[0]["properties"].(type) {
	case string:
		propertiesJSON = v
	case []byte:
		propertiesJSON = string(v)
	}

	if propertiesJSON != "" {
		properties, err := deserializePropertiesFromJSON(propertiesJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize properties from JSON: %w", err)
		}
		sender.Properties = properties
	}

	return sender, nil
}

// updateSender updates an existing notification sender.
func (s *notificationStore) updateSender(ctx context.Context, id string, sender common.NotificationSenderDTO) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	// Serialize properties to JSON
	var propertiesJSON string
	if len(sender.Properties) > 0 {
		propertiesJSON, err = serializePropertiesToJSONArray(sender.Properties)
		if err != nil {
			return fmt.Errorf("failed to serialize properties to JSON: %w", err)
		}
	}

	_, err = dbClient.ExecuteContext(ctx, queryUpdateNotificationSender, sender.Name, sender.Description,
		string(sender.Provider), propertiesJSON, id, string(sender.Type), s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// deleteSender deletes a notification sender.
func (s *notificationStore) deleteSender(ctx context.Context, id string) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationStore"))

	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryDeleteNotificationSender, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute delete query: %w", err)
	}
	if rowsAffected == 0 {
		logger.Debug("No sender found to delete", log.String("id", id))
	}

	return nil
}

// buildSenderFromResultRow constructs a NotificationSenderDTO from a database result row.
func (s *notificationStore) buildSenderFromResultRow(
	row map[string]interface{}) (*common.NotificationSenderDTO, error) {
	senderID, ok := row["id"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse id as string")
	}

	name, ok := row["name"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse name as string")
	}

	description, ok := row["description"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse description as string")
	}

	_type, ok := row["type"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse type as string")
	}

	provider, ok := row["provider"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse provider as string")
	}

	sender := &common.NotificationSenderDTO{
		ID:          senderID,
		Name:        name,
		Description: description,
		Type:        common.NotificationSenderType(_type),
		Provider:    common.MessageProviderType(provider),
		Properties:  []cmodels.Property{},
	}

	return sender, nil
}
