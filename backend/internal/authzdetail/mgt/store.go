// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// typeStoreInterface defines persistence for authorization details types.
type typeStoreInterface interface {
	CreateType(ctx context.Context, t providers.AuthorizationDetailType) error
	GetTypeByID(ctx context.Context, id string) (providers.AuthorizationDetailType, error)
	GetTypeByValue(ctx context.Context, typ string) (providers.AuthorizationDetailType, error)
	ListTypes(ctx context.Context) ([]providers.AuthorizationDetailType, error)
	CountTypes(ctx context.Context) (int, error)
	ListTypePage(ctx context.Context, limit, offset int) ([]providers.AuthorizationDetailType, error)
	UpdateType(ctx context.Context, t providers.AuthorizationDetailType, bumpVersion bool) error
	DeleteType(ctx context.Context, id string) error
	IsTypeDeclarative(ctx context.Context, id string) (bool, error)
}

// typeStore implements typeStoreInterface against the config database.
type typeStore struct {
	dbProvider provider.DBProviderInterface
}

// newTypeStore creates a new typeStore.
func newTypeStore() typeStoreInterface {
	return &typeStore{dbProvider: provider.GetDBProvider()}
}

// CreateType inserts an authorization details type.
func (s *typeStore) CreateType(ctx context.Context, t providers.AuthorizationDetailType) error {
	props, err := buildTypeProperties(t)
	if err != nil {
		return err
	}
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		if _, err := dbClient.ExecuteContext(ctx, queryCreateType,
			t.ID, t.Type, t.Name, resolveNullableString(t.Description), t.Version,
			[]byte(t.Schema), props, deployment.Resolve(ctx)); err != nil {
			return fmt.Errorf("failed to create authorization details type: %w", err)
		}
		return nil
	})
}

// GetTypeByID retrieves an authorization details type by id.
func (s *typeStore) GetTypeByID(ctx context.Context, id string) (providers.AuthorizationDetailType, error) {
	return s.getOne(func(dbClient provider.DBClientInterface) ([]map[string]interface{}, error) {
		return dbClient.QueryContext(ctx, queryGetTypeByID, id, deployment.Resolve(ctx))
	})
}

// GetTypeByValue retrieves an authorization details type by its type value.
func (s *typeStore) GetTypeByValue(ctx context.Context, typ string) (providers.AuthorizationDetailType, error) {
	return s.getOne(func(dbClient provider.DBClientInterface) ([]map[string]interface{}, error) {
		return dbClient.QueryContext(ctx, queryGetTypeByValue, typ, deployment.Resolve(ctx))
	})
}

// getOne runs a single-row lookup and builds the type from its row.
func (s *typeStore) getOne(
	query func(provider.DBClientInterface) ([]map[string]interface{}, error),
) (providers.AuthorizationDetailType, error) {
	var t providers.AuthorizationDetailType
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := query(dbClient)
		if err != nil {
			return fmt.Errorf("failed to get authorization details type: %w", err)
		}
		if len(results) == 0 {
			return errTypeNotFound
		}
		t, err = buildTypeFromRow(results[0])
		return err
	})
	return t, err
}

// ListTypes lists the authorization details types of the deployment.
func (s *typeStore) ListTypes(ctx context.Context) ([]providers.AuthorizationDetailType, error) {
	var types []providers.AuthorizationDetailType
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryListTypes, deployment.Resolve(ctx))
		if err != nil {
			return fmt.Errorf("failed to list authorization details types: %w", err)
		}
		types = make([]providers.AuthorizationDetailType, 0, len(results))
		for _, row := range results {
			t, err := buildTypeFromRow(row)
			if err != nil {
				return err
			}
			types = append(types, t)
		}
		return nil
	})
	return types, err
}

// CountTypes counts the authorization details types of the deployment.
func (s *typeStore) CountTypes(ctx context.Context) (int, error) {
	var count int
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryCountTypes, deployment.Resolve(ctx))
		if err != nil {
			return fmt.Errorf("failed to count authorization details types: %w", err)
		}
		count, err = parseCount(results)
		return err
	})
	return count, err
}

// ListTypePage lists one page of the authorization details types of the deployment, ordered by type value.
func (s *typeStore) ListTypePage(ctx context.Context, limit, offset int) ([]providers.AuthorizationDetailType,
	error) {
	var types []providers.AuthorizationDetailType
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryListTypePage, limit, offset, deployment.Resolve(ctx))
		if err != nil {
			return fmt.Errorf("failed to list authorization details types: %w", err)
		}
		types = make([]providers.AuthorizationDetailType, 0, len(results))
		for _, row := range results {
			t, err := buildTypeFromRow(row)
			if err != nil {
				return err
			}
			types = append(types, t)
		}
		return nil
	})
	return types, err
}

// UpdateType updates an authorization details type that is still at t.Version, incrementing its version when
// bumpVersion is set. It returns errTypeNotFound when no type has the id at that version.
func (s *typeStore) UpdateType(ctx context.Context, t providers.AuthorizationDetailType, bumpVersion bool) error {
	props, err := buildTypeProperties(t)
	if err != nil {
		return err
	}
	increment := 0
	if bumpVersion {
		increment = 1
	}
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		rows, err := dbClient.ExecuteContext(ctx, queryUpdateType,
			t.ID, t.Name, resolveNullableString(t.Description), []byte(t.Schema), props,
			increment, t.Version, deployment.Resolve(ctx))
		if err != nil {
			return fmt.Errorf("failed to update authorization details type: %w", err)
		}
		if rows == 0 {
			return errTypeNotFound
		}
		return nil
	})
}

// DeleteType deletes an authorization details type.
func (s *typeStore) DeleteType(ctx context.Context, id string) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		if _, err := dbClient.ExecuteContext(ctx, queryDeleteType, id, deployment.Resolve(ctx)); err != nil {
			return fmt.Errorf("failed to delete authorization details type: %w", err)
		}
		return nil
	})
}

// IsTypeDeclarative reports false: types in the database are never declarative.
func (s *typeStore) IsTypeDeclarative(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// parseCount reads the count a COUNT query returns.
func parseCount(results []map[string]interface{}) (int, error) {
	if len(results) == 0 {
		return 0, errors.New("no count result returned")
	}
	switch v := results[0]["total"].(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected count result type %T", v)
	}
}

// withDBClient executes a function with a config DB client.
func (s *typeStore) withDBClient(fn func(provider.DBClientInterface) error) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get config DB client: %w", err)
	}
	return fn(dbClient)
}

// buildTypeProperties builds the PROPERTIES JSON of a type.
func buildTypeProperties(t providers.AuthorizationDetailType) ([]byte, error) {
	var processor *storedProcessor
	if t.Processor != nil {
		processor = &storedProcessor{Endpoint: t.Processor.Endpoint}
		if len(t.Processor.AuthenticationProperties) > 0 {
			authentication, err := cmodels.SerializePropertiesToJSONArray(t.Processor.AuthenticationProperties)
			if err != nil {
				return nil, fmt.Errorf("failed to encode processor authentication: %w", err)
			}
			processor.AuthenticationProperties = json.RawMessage(authentication)
		}
	}
	props, err := json.Marshal(typeProperties{
		ConsentTemplate: t.ConsentTemplate,
		Processor:       processor,
		Reuse:           &t.Reuse,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode authorization details type properties: %w", err)
	}
	return props, nil
}

// buildTypeFromRow builds a type from a database row.
func buildTypeFromRow(row map[string]interface{}) (providers.AuthorizationDetailType, error) {
	t := providers.AuthorizationDetailType{}
	t.ID, _ = row["id"].(string)
	t.Type, _ = row["type"].(string)
	t.Name, _ = row["name"].(string)
	t.Description, _ = row["description"].(string)

	version, _ := sysutils.ToInt64(row["version"])
	t.Version = int(version)

	t.Schema = json.RawMessage(columnBytes(row["schema_definition"]))

	if raw := columnBytes(row["properties"]); len(raw) > 0 {
		var props typeProperties
		if err := json.Unmarshal(raw, &props); err != nil {
			return providers.AuthorizationDetailType{},
				fmt.Errorf("failed to decode authorization details type properties: %w",
					err)
		}
		t.ConsentTemplate = props.ConsentTemplate
		if props.Processor != nil {
			t.Processor = &providers.AuthorizationDetailProcessor{Endpoint: props.Processor.Endpoint}
			if len(props.Processor.AuthenticationProperties) > 0 {
				authentication, err := cmodels.DeserializePropertiesFromJSON(
					string(props.Processor.AuthenticationProperties))
				if err != nil {
					return providers.AuthorizationDetailType{},
						fmt.Errorf("failed to decode processor authentication: %w", err)
				}
				t.Processor.AuthenticationProperties = authentication
			}
		}
		t.Reuse = props.Reuse != nil && *props.Reuse
	}
	return t, nil
}

// columnBytes returns a JSON column value as bytes, whichever form the driver returned it in.
func columnBytes(v interface{}) []byte {
	switch value := v.(type) {
	case string:
		return []byte(value)
	case []byte:
		return value
	default:
		return nil
	}
}

// resolveNullableString converts an empty string to nil for database storage.
func resolveNullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
