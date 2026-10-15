// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// httpConnectionStoreInterface defines HTTP connection persistence operations.
type httpConnectionStoreInterface interface {
	CreateHTTPConnection(ctx context.Context, connection HTTPConnection) error
	ListHTTPConnections(ctx context.Context) ([]HTTPConnection, error)
	GetHTTPConnection(ctx context.Context, id string) (*HTTPConnection, error)
	UpdateHTTPConnection(ctx context.Context, connection HTTPConnection) error
	DeleteHTTPConnection(ctx context.Context, id string) error
}

// httpConnectionStore persists HTTP connections in the configuration database.
type httpConnectionStore struct {
	dbProvider provider.DBProviderInterface
}

// newHTTPConnectionStore creates the mutable HTTP connection store.
func newHTTPConnectionStore() httpConnectionStoreInterface {
	return &httpConnectionStore{
		dbProvider: provider.GetDBProvider(),
	}
}

// CreateHTTPConnection persists an HTTP connection, returning errNameConflict when the name is taken.
func (s *httpConnectionStore) CreateHTTPConnection(ctx context.Context, connection HTTPConnection) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	properties, err := encodeHTTPConnectionProperties(connection)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryCreateHTTPConnection,
		connection.ID, connection.Name, connection.Description, properties, deployment.Resolve(ctx))
	return nameConflictOr(err)
}

// ListHTTPConnections retrieves all HTTP connections for the deployment.
func (s *httpConnectionStore) ListHTTPConnections(ctx context.Context) ([]HTTPConnection, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryListHTTPConnections, deployment.Resolve(ctx))
	if err != nil {
		return nil, err
	}
	connections := make([]HTTPConnection, 0, len(rows))
	for _, row := range rows {
		connection, err := buildHTTPConnectionFromRow(row)
		if err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	return connections, nil
}

// GetHTTPConnection retrieves an HTTP connection by ID, or nil when it does not exist.
func (s *httpConnectionStore) GetHTTPConnection(ctx context.Context, id string) (*HTTPConnection, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryGetHTTPConnectionByID, id, deployment.Resolve(ctx))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	connection, err := buildHTTPConnectionFromRow(rows[0])
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

// UpdateHTTPConnection replaces an HTTP connection, returning errConnectionNotFound when no row
// matches and errNameConflict when the new name is taken.
func (s *httpConnectionStore) UpdateHTTPConnection(ctx context.Context, connection HTTPConnection) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	properties, err := encodeHTTPConnectionProperties(connection)
	if err != nil {
		return err
	}
	affected, err := dbClient.ExecuteContext(ctx, queryUpdateHTTPConnection,
		connection.Name, connection.Description, properties, connection.ID, deployment.Resolve(ctx))
	if err != nil {
		return nameConflictOr(err)
	}
	if affected == 0 {
		return errConnectionNotFound
	}
	return nil
}

// DeleteHTTPConnection removes an HTTP connection, returning errConnectionNotFound when no row matches.
func (s *httpConnectionStore) DeleteHTTPConnection(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	affected, err := dbClient.ExecuteContext(ctx, queryDeleteHTTPConnection, id, deployment.Resolve(ctx))
	if err != nil {
		return err
	}
	if affected == 0 {
		return errConnectionNotFound
	}
	return nil
}

// nameConflictOr maps a refusal by the unique name index onto errNameConflict.
func nameConflictOr(err error) error {
	if provider.IsUniqueIndexViolation(err, nameIndexName, nameIndexColumns) {
		return fmt.Errorf("%w: %w", errNameConflict, err)
	}
	return err
}

// httpConnectionProperties holds the connection settings stored as JSON. Authentication holds the
// outbound authentication properties in the cmodels property array form, secrets encrypted.
type httpConnectionProperties struct {
	URL            string          `json:"url"`
	TimeoutMS      int             `json:"timeoutMs"`
	Headers        []Header        `json:"headers,omitempty"`
	Authentication json.RawMessage `json:"authentication,omitempty"`
}

// encodeHTTPConnectionProperties serializes connection settings and authentication for storage.
func encodeHTTPConnectionProperties(connection HTTPConnection) (string, error) {
	properties := httpConnectionProperties{
		URL:       connection.URL,
		TimeoutMS: connection.TimeoutMS,
		Headers:   connection.Headers,
	}
	if len(connection.AuthProperties) > 0 {
		serialized, err := cmodels.SerializePropertiesToJSONArray(connection.AuthProperties)
		if err != nil {
			return "", fmt.Errorf("failed to encode HTTP connection authentication: %w", err)
		}
		properties.Authentication = json.RawMessage(serialized)
	}
	data, err := json.Marshal(properties)
	if err != nil {
		return "", fmt.Errorf("failed to encode HTTP connection properties: %w", err)
	}
	return string(data), nil
}

// buildHTTPConnectionFromRow builds a connection from a database row and its JSON properties.
func buildHTTPConnectionFromRow(row map[string]interface{}) (HTTPConnection, error) {
	properties := httpConnectionProperties{}
	if err := json.Unmarshal([]byte(sysutils.ConvertInterfaceValueToString(row["properties"])),
		&properties); err != nil {
		return HTTPConnection{}, fmt.Errorf("failed to decode HTTP connection properties: %w", err)
	}
	authProperties, err := cmodels.DeserializePropertiesFromJSON(string(properties.Authentication))
	if err != nil {
		return HTTPConnection{}, fmt.Errorf("failed to decode HTTP connection authentication: %w", err)
	}
	if len(authProperties) == 0 {
		authProperties = nil
	}
	return HTTPConnection{
		ID:             sysutils.ConvertInterfaceValueToString(row["id"]),
		Name:           sysutils.ConvertInterfaceValueToString(row["name"]),
		Description:    sysutils.ConvertInterfaceValueToString(row["description"]),
		URL:            properties.URL,
		TimeoutMS:      properties.TimeoutMS,
		Headers:        properties.Headers,
		AuthProperties: authProperties,
	}, nil
}
