// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// Store persists external AuthZEN PDP connections.
type Store interface {
	Create(ctx context.Context, connection AuthZENPDPConnection) error
	List(ctx context.Context) ([]AuthZENPDPConnection, error)
	Get(ctx context.Context, id string) (*AuthZENPDPConnection, error)
	Update(ctx context.Context, id string, connection AuthZENPDPConnection) error
	Delete(ctx context.Context, id string) error
}

type authZENPDPStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// NewStore creates a store for external AuthZEN PDP connections.
func NewStore() Store {
	return &authZENPDPStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// Create persists an external AuthZEN PDP connection.
func (s *authZENPDPStore) Create(ctx context.Context, connection AuthZENPDPConnection) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	subjectProperties, mappings, err := encodeAuthZENPDPProperties(connection)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryCreateAuthZENPDPConnection,
		connection.ID, s.deploymentID, connection.Name, connection.Description, connection.Endpoint,
		subjectProperties, mappings)
	return err
}

// List retrieves all external AuthZEN PDP connections for the deployment.
func (s *authZENPDPStore) List(ctx context.Context) ([]AuthZENPDPConnection, error) {
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
		connection := buildAuthZENPDPConnection(row)
		connections = append(connections, connection)
	}
	return connections, nil
}

// Get retrieves an external AuthZEN PDP connection by ID.
func (s *authZENPDPStore) Get(ctx context.Context, id string) (*AuthZENPDPConnection, error) {
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
	connection := buildAuthZENPDPConnection(rows[0])
	return &connection, nil
}

// Update replaces an external AuthZEN PDP connection by ID.
func (s *authZENPDPStore) Update(ctx context.Context, id string, connection AuthZENPDPConnection) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	subjectProperties, mappings, err := encodeAuthZENPDPProperties(connection)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryUpdateAuthZENPDPConnection,
		connection.Name, connection.Description, connection.Endpoint, subjectProperties, mappings, id, s.deploymentID)
	return err
}

// Delete removes an external AuthZEN PDP connection by ID.
func (s *authZENPDPStore) Delete(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.ExecuteContext(ctx, queryDeleteAuthZENPDPConnection, id, s.deploymentID)
	return err
}

func encodeAuthZENPDPProperties(connection AuthZENPDPConnection) (string, string, error) {
	subjectPropertyList := connection.SubjectProperties
	if subjectPropertyList == nil {
		subjectPropertyList = []string{}
	}
	stored := authZENPDPStoredSubjectProperties{
		Properties:    subjectPropertyList,
		Mappings:      connection.SubjectAttributeMappings,
		BatchEndpoint: strings.TrimSpace(connection.BatchEndpoint),
		TimeoutMS:     connection.TimeoutMS,
		RetryCount:    connection.RetryCount,
		FailOpen:      connection.FailOpen,
	}
	subjectProperties, err := json.Marshal(stored)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode subject properties: %w", err)
	}
	mappings, err := json.Marshal(connection.SubjectPropertyMappings)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode subject property mappings: %w", err)
	}
	return string(subjectProperties), string(mappings), nil
}

type authZENPDPStoredSubjectProperties struct {
	Properties    []string                  `json:"properties"`
	Mappings      []SubjectAttributeMapping `json:"mappings,omitempty"`
	BatchEndpoint string                    `json:"batchEndpoint,omitempty"`
	TimeoutMS     int                       `json:"timeoutMs,omitempty"`
	RetryCount    int                       `json:"retryCount,omitempty"`
	FailOpen      bool                      `json:"failOpen,omitempty"`
}

func buildAuthZENPDPConnection(row map[string]interface{}) AuthZENPDPConnection {
	connection := AuthZENPDPConnection{
		ID:          stringValue(row["id"]),
		Name:        stringValue(row["name"]),
		Description: stringValue(row["description"]),
		Endpoint:    stringValue(row["endpoint"]),
	}
	if raw := stringValue(row["subject_properties"]); raw != "" {
		decodeSubjectProperties(raw, &connection)
	}
	if raw := stringValue(row["subject_property_mappings"]); raw != "" {
		_ = json.Unmarshal([]byte(raw), &connection.SubjectPropertyMappings)
	}
	if connection.TimeoutMS <= 0 {
		connection.TimeoutMS = DefaultTimeoutMS
	}
	if connection.RetryCount <= 0 {
		connection.RetryCount = DefaultRetryCount
	}
	return connection
}

func decodeSubjectProperties(raw string, connection *AuthZENPDPConnection) {
	var stored authZENPDPStoredSubjectProperties
	if err := json.Unmarshal([]byte(raw), &stored); err == nil && strings.HasPrefix(strings.TrimSpace(raw), "{") {
		connection.SubjectProperties = stored.Properties
		connection.SubjectAttributeMappings = stored.Mappings
		connection.BatchEndpoint = strings.TrimSpace(stored.BatchEndpoint)
		connection.TimeoutMS = stored.TimeoutMS
		connection.RetryCount = stored.RetryCount
		connection.FailOpen = stored.FailOpen
		return
	}

	var legacy []string
	if err := json.Unmarshal([]byte(raw), &legacy); err == nil {
		connection.SubjectProperties = legacy
		return
	}

	connection.SubjectProperties = splitSubjectProperties(raw)
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
