// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// Store persists AuthZEN PDP connections.
type Store interface {
	CreateAuthZENPDP(ctx context.Context, connection AuthZENPDPConnection) error
	ListAuthZENPDPs(ctx context.Context) ([]AuthZENPDPConnection, error)
	GetAuthZENPDP(ctx context.Context, id string) (*AuthZENPDPConnection, error)
	GetAuthZENPDPByName(ctx context.Context, name string) (*AuthZENPDPConnection, error)
	UpdateAuthZENPDP(ctx context.Context, id string, connection AuthZENPDPConnection) error
	DeleteAuthZENPDP(ctx context.Context, id string) error
}

type authZENPDPStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// NewStore creates a store for AuthZEN PDP connections.
func NewStore(deploymentID string) Store {
	return &authZENPDPStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: deploymentID,
	}
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
		connection.ID, connection.Name, connection.Description, properties, s.deploymentID)
	return err
}

// ListAuthZENPDPs retrieves all AuthZEN PDP connections for the deployment.
func (s *authZENPDPStore) ListAuthZENPDPs(ctx context.Context) ([]AuthZENPDPConnection, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryListAuthZENPDPConnections, s.deploymentID)
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

// GetAuthZENPDP retrieves an AuthZEN PDP connection by ID.
func (s *authZENPDPStore) GetAuthZENPDP(ctx context.Context, id string) (*AuthZENPDPConnection, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryGetAuthZENPDPConnectionByID, id, s.deploymentID)
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
	rows, err := dbClient.QueryContext(ctx, queryGetAuthZENPDPConnectionByName, name, s.deploymentID)
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
		connection.Name, connection.Description, properties, id, s.deploymentID)
	return err
}

// DeleteAuthZENPDP removes an AuthZEN PDP connection by ID.
func (s *authZENPDPStore) DeleteAuthZENPDP(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.ExecuteContext(ctx, queryDeleteAuthZENPDPConnection, id, s.deploymentID)
	return err
}

// authZENPDPProperties holds the connection settings stored as JSON.
type authZENPDPProperties struct {
	Endpoint                 string                    `json:"endpoint"`
	BatchEndpoint            string                    `json:"batchEndpoint"`
	TimeoutMS                int                       `json:"timeoutMs"`
	RetryCount               int                       `json:"retryCount"`
	SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
}

func encodeAuthZENPDPProperties(connection AuthZENPDPConnection) (string, error) {
	data, err := json.Marshal(authZENPDPProperties{
		Endpoint: connection.Endpoint, BatchEndpoint: connection.BatchEndpoint,
		TimeoutMS: connection.TimeoutMS, RetryCount: connection.RetryCount,
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode AuthZEN PDP properties: %w", err)
	}
	return string(data), nil
}

func (s *authZENPDPStore) buildAuthZENPDPConnection(row map[string]interface{}) (AuthZENPDPConnection, error) {
	properties := authZENPDPProperties{}
	if err := json.Unmarshal([]byte(stringValue(row["properties"])), &properties); err != nil {
		return AuthZENPDPConnection{}, fmt.Errorf("failed to decode AuthZEN PDP properties: %w", err)
	}
	return AuthZENPDPConnection{
		ID: stringValue(row["id"]), Name: stringValue(row["name"]), Description: stringValue(row["description"]),
		Endpoint: properties.Endpoint, BatchEndpoint: properties.BatchEndpoint,
		TimeoutMS: properties.TimeoutMS, RetryCount: properties.RetryCount,
		SubjectAttributeMappings: properties.SubjectAttributeMappings,
	}, nil
}

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
