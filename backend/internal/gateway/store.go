// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
)

// storeInterface is the persistence a gateway service needs.
type storeInterface interface {
	List(ctx context.Context) ([]Gateway, error)
	GetByID(ctx context.Context, id string) (*Gateway, error)
	GetByName(ctx context.Context, name string) (*Gateway, error)
	Count(ctx context.Context) (int, error)
	Create(ctx context.Context, gw *Gateway) error
	Update(ctx context.Context, gw *Gateway) error
	Delete(ctx context.Context, id string) error
}

type store struct {
	dbProvider   dbprovider.DBProviderInterface
	deploymentID string
}

func newStore() storeInterface {
	return &store{
		dbProvider:   dbprovider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

func (s *store) List(ctx context.Context) ([]Gateway, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryListGateways, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list the gateways: %w", err)
	}
	gateways := make([]Gateway, 0, len(rows))
	for _, row := range rows {
		gw, err := gatewayFromRow(row)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, *gw)
	}
	return gateways, nil
}

func (s *store) GetByID(ctx context.Context, id string) (*Gateway, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryGetGatewayByID, id, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the gateway: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return gatewayFromRow(rows[0])
}

func (s *store) GetByName(ctx context.Context, name string) (*Gateway, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryGetGatewayByName, name, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up the gateway name: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return gatewayFromRow(rows[0])
}

func (s *store) Count(ctx context.Context) (int, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryCountGateways, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to count the gateways: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	switch total := rows[0]["total"].(type) {
	case int64:
		return int(total), nil
	case int:
		return total, nil
	default:
		return 0, fmt.Errorf("the gateway count came back as %T", rows[0]["total"])
	}
}

func (s *store) Create(ctx context.Context, gw *Gateway) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.ExecuteContext(ctx, queryInsertGateway,
		gw.ID, gw.Name, gw.BaseURL, gw.ClientID, gw.ClientSecret, gw.Scope, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to register the gateway: %w", err)
	}
	return nil
}

func (s *store) Update(ctx context.Context, gw *Gateway) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.ExecuteContext(ctx, queryUpdateGateway,
		gw.ID, gw.Name, gw.BaseURL, gw.ClientID, gw.ClientSecret, gw.Scope, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update the gateway: %w", err)
	}
	return nil
}

func (s *store) Delete(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteGateway, id, s.deploymentID); err != nil {
		return fmt.Errorf("failed to remove the gateway: %w", err)
	}
	return nil
}

// gatewayFromRow reads one row. The stored secret stays as it is here; decrypting it is the
// service's job, and only when an apply actually needs it.
func gatewayFromRow(row map[string]interface{}) (*Gateway, error) {
	gw := &Gateway{
		ID:           asString(row["id"]),
		Name:         asString(row["name"]),
		BaseURL:      asString(row["base_url"]),
		ClientID:     asString(row["client_id"]),
		ClientSecret: asString(row["client_secret"]),
		Scope:        asString(row["scope"]),
	}
	if gw.ID == "" {
		return nil, fmt.Errorf("a gateway row carries no id")
	}
	gw.CreatedAt = asTime(row["created_at"])
	gw.UpdatedAt = asTime(row["updated_at"])
	return gw, nil
}

func asString(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return ""
	}
}

func asTime(v interface{}) time.Time {
	switch value := v.(type) {
	case time.Time:
		return value
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}
