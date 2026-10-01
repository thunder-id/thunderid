// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// authZENPDPStoreInterface defines AuthZEN PDP connection persistence operations.
type authZENPDPStoreInterface interface {
	CreateAuthZENPDP(ctx context.Context, connection AuthZENPDPConnection) error
	ListAuthZENPDPs(ctx context.Context) ([]AuthZENPDPConnection, error)
	CountAuthZENPDPs(ctx context.Context) (int, error)
	GetAuthZENPDP(ctx context.Context, id string) (*AuthZENPDPConnection, error)
	GetAuthZENPDPByName(ctx context.Context, name string) (*AuthZENPDPConnection, error)
	UpdateAuthZENPDP(ctx context.Context, id string, connection AuthZENPDPConnection) error
	DeleteAuthZENPDP(ctx context.Context, id string) error
}

type authZENPDPStore struct {
	dbProvider provider.DBProviderInterface
}

// newAuthZENPDPStore creates the mutable AuthZEN PDP connection store.
func newAuthZENPDPStore() authZENPDPStoreInterface {
	return &authZENPDPStore{
		dbProvider: provider.GetDBProvider(),
	}
}

// scope resolves the deployment scope from the request context.
func (s *authZENPDPStore) scope(ctx context.Context) string {
	return deployment.Resolve(ctx)
}

// CreateAuthZENPDP persists an AuthZEN PDP connection.
func (s *authZENPDPStore) CreateAuthZENPDP(ctx context.Context, connection AuthZENPDPConnection) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	properties, err := encodeAuthZENPDPProperties(connection)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryCreateAuthZENPDPConnection,
		connection.ID, connection.Name, connection.Description, authZENPDPConnectionType, properties, s.scope(ctx))
	return err
}

// ListAuthZENPDPs retrieves all AuthZEN PDP connections for the deployment.
func (s *authZENPDPStore) ListAuthZENPDPs(ctx context.Context) ([]AuthZENPDPConnection, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryListAuthZENPDPConnections, authZENPDPConnectionType, s.scope(ctx))
	if err != nil {
		return nil, err
	}
	connections := make([]AuthZENPDPConnection, 0, len(rows))
	for _, row := range rows {
		connection, err := s.buildAuthZENPDPConnection(row)
		if err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	return connections, nil
}

// CountAuthZENPDPs returns the number of connections for the deployment.
func (s *authZENPDPStore) CountAuthZENPDPs(ctx context.Context) (int, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryCountAuthZENPDPConnections, authZENPDPConnectionType, s.scope(ctx))
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("AuthZEN PDP count query returned no rows")
	}
	switch count := rows[0]["count"].(type) {
	case int:
		return count, nil
	case int64:
		return int(count), nil
	default:
		return 0, fmt.Errorf("unexpected AuthZEN PDP count type: %T", rows[0]["count"])
	}
}

// GetAuthZENPDP retrieves an AuthZEN PDP connection by ID.
func (s *authZENPDPStore) GetAuthZENPDP(ctx context.Context, id string) (*AuthZENPDPConnection, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryGetAuthZENPDPConnectionByID,
		id, authZENPDPConnectionType, s.scope(ctx))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	connection, err := s.buildAuthZENPDPConnection(rows[0])
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

// GetAuthZENPDPByName retrieves an AuthZEN PDP connection by name.
func (s *authZENPDPStore) GetAuthZENPDPByName(
	ctx context.Context, name string,
) (*AuthZENPDPConnection, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryGetAuthZENPDPConnectionByName,
		name, authZENPDPConnectionType, s.scope(ctx))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	connection, err := s.buildAuthZENPDPConnection(rows[0])
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

// UpdateAuthZENPDP replaces an AuthZEN PDP connection by ID.
func (s *authZENPDPStore) UpdateAuthZENPDP(
	ctx context.Context, id string, connection AuthZENPDPConnection,
) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	properties, err := encodeAuthZENPDPProperties(connection)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryUpdateAuthZENPDPConnection,
		connection.Name, connection.Description, properties, id, authZENPDPConnectionType, s.scope(ctx))
	return err
}

// DeleteAuthZENPDP removes an AuthZEN PDP connection by ID.
func (s *authZENPDPStore) DeleteAuthZENPDP(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.ExecuteContext(ctx, queryDeleteAuthZENPDPConnection, id, authZENPDPConnectionType, s.scope(ctx))
	return err
}

// authZENPDPProperties holds the connection settings stored as JSON.
type authZENPDPProperties struct {
	Endpoint                 string                    `json:"endpoint"`
	BatchEndpoint            string                    `json:"batchEndpoint"`
	TimeoutMS                int                       `json:"timeoutMs"`
	RetryCount               int                       `json:"retryCount"`
	SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
	AuthenticationScheme     string                    `json:"authenticationScheme,omitempty"`
	AuthenticationProperties json.RawMessage           `json:"authenticationProperties,omitempty"`
}

// encodeAuthZENPDPProperties serializes connection-specific settings for storage.
func encodeAuthZENPDPProperties(connection AuthZENPDPConnection) (string, error) {
	authenticationProperties, err := cmodels.SerializePropertiesToJSONArray(connection.AuthenticationProperties)
	if err != nil {
		return "", fmt.Errorf("failed to encode AuthZEN PDP authentication properties: %w", err)
	}
	data, err := json.Marshal(authZENPDPProperties{
		Endpoint: connection.Endpoint, BatchEndpoint: connection.BatchEndpoint,
		TimeoutMS: connection.TimeoutMS, RetryCount: connection.RetryCount,
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
		AuthenticationScheme:     connection.AuthenticationScheme,
		AuthenticationProperties: json.RawMessage(authenticationProperties),
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode AuthZEN PDP properties: %w", err)
	}
	return string(data), nil
}

// buildAuthZENPDPConnection builds a connection from a database row and its JSON properties.
func (s *authZENPDPStore) buildAuthZENPDPConnection(row map[string]interface{}) (AuthZENPDPConnection, error) {
	properties := authZENPDPProperties{}
	if err := json.Unmarshal([]byte(stringValue(row["properties"])), &properties); err != nil {
		return AuthZENPDPConnection{}, fmt.Errorf("failed to decode AuthZEN PDP properties: %w", err)
	}
	var authenticationProperties []cmodels.Property
	if len(properties.AuthenticationProperties) > 0 {
		decodedProperties, err := cmodels.DeserializePropertiesFromJSON(string(properties.AuthenticationProperties))
		if err != nil {
			return AuthZENPDPConnection{}, fmt.Errorf("failed to decode AuthZEN PDP authentication properties: %w", err)
		}
		authenticationProperties = decodedProperties
	}
	return AuthZENPDPConnection{
		ID: stringValue(row["id"]), Name: stringValue(row["name"]), Description: stringValue(row["description"]),
		Endpoint: properties.Endpoint, BatchEndpoint: properties.BatchEndpoint,
		TimeoutMS: properties.TimeoutMS, RetryCount: properties.RetryCount,
		SubjectAttributeMappings: properties.SubjectAttributeMappings,
		AuthenticationScheme:     properties.AuthenticationScheme,
		AuthenticationProperties: authenticationProperties,
	}, nil
}

// stringValue converts database string or byte values into a string.
func stringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}
