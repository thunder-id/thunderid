// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const storeLoggerComponentName = "OrganizationUnitStore"

// organizationUnitStoreInterface defines the interface for organization unit store operations.
type organizationUnitStoreInterface interface {
	GetOrganizationUnitListCount(ctx context.Context, f *tidcommon.FilterGroup) (int, error)
	GetOrganizationUnitList(
		ctx context.Context, limit, offset int, f *tidcommon.FilterGroup,
	) ([]providers.OrganizationUnitBasic, error)
	GetOrganizationUnitsByIDs(ctx context.Context, ids []string) ([]providers.OrganizationUnitBasic, error)
	CreateOrganizationUnit(ctx context.Context, ou providers.OrganizationUnit) error
	GetOrganizationUnit(ctx context.Context, id string) (providers.OrganizationUnit, error)
	GetOrganizationUnitByHandle(ctx context.Context, handle string, parent *string) (providers.OrganizationUnit, error)
	GetOrganizationUnitByPath(ctx context.Context, handles []string) (providers.OrganizationUnit, error)
	IsOrganizationUnitExists(ctx context.Context, id string) (bool, error)
	IsOrganizationUnitDeclarative(ctx context.Context, id string) bool
	CheckOrganizationUnitNameConflict(ctx context.Context, name string, parent *string) (bool, error)
	CheckOrganizationUnitHandleConflict(ctx context.Context, handle string, parent *string) (bool, error)
	UpdateOrganizationUnit(ctx context.Context, ou providers.OrganizationUnit) error
	DeleteOrganizationUnit(ctx context.Context, id string) error
	GetOrganizationUnitChildrenCount(ctx context.Context, id string, f *tidcommon.FilterGroup) (int, error)
	GetOrganizationUnitChildrenList(
		ctx context.Context, id string, limit, offset int, f *tidcommon.FilterGroup,
	) ([]providers.OrganizationUnitBasic, error)
}

var getDBProvider = provider.GetDBProvider

// organizationUnitStore is the default implementation of organizationUnitStoreInterface.
type organizationUnitStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newOrganizationUnitStore creates a new instance of organizationUnitStore.
func newOrganizationUnitStore() (organizationUnitStoreInterface, providers.Transactioner, error) {
	dbProvider := getDBProvider()
	transactioner, err := dbProvider.GetEntityDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &organizationUnitStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}, transactioner, nil
}

// scope returns the deployment id this request acts for, falling back to the configured
// identifier for a context that never passed through the edge.
func (s *organizationUnitStore) scope(ctx context.Context) string {
	return deployment.Resolve(ctx, s.deploymentID)
}

// GetOrganizationUnitListCount retrieves the total count of organization units.
func (s *organizationUnitStore) GetOrganizationUnitListCount(
	ctx context.Context, f *tidcommon.FilterGroup,
) (int, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	query, filterArgs, err := buildRootOUCountQuery(f)
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}
	args := append([]interface{}{s.scope(ctx)}, filterArgs...)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	var total int
	if len(results) > 0 {
		if count, ok := results[0]["total"].(int64); ok {
			total = int(count)
		} else {
			return 0, fmt.Errorf("unexpected type for total: %T", results[0]["total"])
		}
	}

	return total, nil
}

// GetOrganizationUnitList retrieves organization units with pagination.
func (s *organizationUnitStore) GetOrganizationUnitList(
	ctx context.Context, limit, offset int, f *tidcommon.FilterGroup,
) ([]providers.OrganizationUnitBasic, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, filterArgs, err := buildRootOUListQuery(f)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query: %w", err)
	}
	args := append([]interface{}{limit, offset, s.scope(ctx)}, filterArgs...)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	ous := make([]providers.OrganizationUnitBasic, 0, len(results))
	for _, row := range results {
		ou, err := buildOrganizationUnitBasicFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build organization unit basic: %w", err)
		}
		ous = append(ous, ou)
	}

	return ous, nil
}

// GetOrganizationUnitsByIDs retrieves organization units matching the given IDs.
func (s *organizationUnitStore) GetOrganizationUnitsByIDs(
	ctx context.Context, ids []string,
) ([]providers.OrganizationUnitBasic, error) {
	if len(ids) == 0 {
		return []providers.OrganizationUnitBasic{}, nil
	}

	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query := buildGetOrganizationUnitsByIDsQuery(ids)
	args := make([]interface{}, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, s.scope(ctx))

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	ous := make([]providers.OrganizationUnitBasic, 0, len(results))
	for _, row := range results {
		ou, err := buildOrganizationUnitBasicFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build organization unit basic: %w", err)
		}
		ous = append(ous, ou)
	}

	return ous, nil
}

// CreateOrganizationUnit creates a new organization unit in the database.
func (s *organizationUnitStore) CreateOrganizationUnit(ctx context.Context, ou providers.OrganizationUnit) error {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	// Serialize OU Metadata data
	ouMetadataBytes, err := getOUMetadataDataBytes(&ou)
	if err != nil {
		return fmt.Errorf("failed to serialize OU Metadata: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx,
		queryCreateOrganizationUnit,
		ou.ID,
		ou.Parent,
		ou.Handle,
		ou.Name,
		ou.Description,
		string(ouMetadataBytes),
		s.scope(ctx),
		ou.CreatedAt,
		ou.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// GetOrganizationUnit retrieves an organization unit by its id.
func (s *organizationUnitStore) GetOrganizationUnit(
	ctx context.Context,
	id string,
) (providers.OrganizationUnit, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetOrganizationUnitByID, id, s.scope(ctx))
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return providers.OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	ou, err := buildOrganizationUnitFromResultRow(results[0])
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to build organization unit: %w", err)
	}

	return ou, nil
}

// GetOrganizationUnitByHandle retrieves an organization unit by handle and parent.
// When parent is nil, only root organization units are considered.
func (s *organizationUnitStore) GetOrganizationUnitByHandle(
	ctx context.Context, handle string, parent *string,
) (providers.OrganizationUnit, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to get database client: %w", err)
	}

	var results []map[string]interface{}
	if parent == nil {
		results, err = dbClient.QueryContext(ctx, queryGetRootOrganizationUnitByHandle, handle, s.scope(ctx))
	} else {
		results, err = dbClient.QueryContext(ctx, queryGetOrganizationUnitByHandle, handle, *parent, s.scope(ctx))
	}
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to execute query for handle %s: %w", handle, err)
	}

	if len(results) == 0 {
		return providers.OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	ou, err := buildOrganizationUnitFromResultRow(results[0])
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf(
			"failed to build organization unit for handle %s: %w",
			handle,
			err,
		)
	}

	return ou, nil
}

// GetOrganizationUnitByPath retrieves an organization unit by its hierarchical handle path.
func (s *organizationUnitStore) GetOrganizationUnitByPath(
	ctx context.Context, handlePath []string,
) (providers.OrganizationUnit, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, storeLoggerComponentName))

	if len(handlePath) == 0 {
		return providers.OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to get database client: %w", err)
	}

	var currentOU providers.OrganizationUnit
	var parentID *string
	var fullPath string

	for i, handle := range handlePath {
		fullPath = fullPath + "/" + handle
		currentOU, err = s.getOrganizationUnitByHandleWithClient(ctx, dbClient, handle, parentID)
		if err != nil {
			if !errors.Is(err, ErrOrganizationUnitNotFound) {
				return providers.OrganizationUnit{}, err
			}
			logger.Debug(ctx, "Organization unit not found in path",
				log.String("handle", handle),
				log.Int("pathIndex", i),
				log.String("fullPath", fullPath))
			return providers.OrganizationUnit{}, ErrOrganizationUnitNotFound
		}

		parentID = &currentOU.ID
	}

	return currentOU, nil
}

func (s *organizationUnitStore) getOrganizationUnitByHandleWithClient(
	ctx context.Context, dbClient provider.DBClientInterface, handle string, parent *string,
) (providers.OrganizationUnit, error) {
	var results []map[string]interface{}
	var err error

	if parent == nil {
		results, err = dbClient.QueryContext(ctx, queryGetRootOrganizationUnitByHandle, handle, s.scope(ctx))
	} else {
		results, err = dbClient.QueryContext(ctx, queryGetOrganizationUnitByHandle, handle, *parent, s.scope(ctx))
	}
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to execute query for handle %s: %w", handle, err)
	}

	if len(results) == 0 {
		return providers.OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	ou, err := buildOrganizationUnitFromResultRow(results[0])
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf(
			"failed to build organization unit for handle %s: %w",
			handle,
			err,
		)
	}

	return ou, nil
}

// IsOrganizationUnitExists checks if an organization unit exists by ID.
func (s *organizationUnitStore) IsOrganizationUnitExists(ctx context.Context, id string) (bool, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryCheckOrganizationUnitExists, id, s.scope(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to execute existence check query: %w", err)
	}

	if len(results) == 0 {
		return false, nil
	}

	if countInterface, exists := results[0]["count"]; exists {
		if count, ok := countInterface.(int64); ok {
			return count > 0, nil
		}
	}

	return false, fmt.Errorf("failed to parse existence check result")
}

// IsOrganizationUnitDeclarative checks if an organization unit is immutable.
// Database store resources are always mutable, so this always returns false.
func (s *organizationUnitStore) IsOrganizationUnitDeclarative(ctx context.Context, id string) bool {
	return false
}

// UpdateOrganizationUnit updates an existing organization unit.
func (s *organizationUnitStore) UpdateOrganizationUnit(ctx context.Context, ou providers.OrganizationUnit) error {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	// Serialize OU Metadata data
	ouMetadataBytes, err := getOUMetadataDataBytes(&ou)
	if err != nil {
		return fmt.Errorf("failed to serialize OU Metadata: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx,
		queryUpdateOrganizationUnit,
		ou.ID,
		ou.Parent,
		ou.Handle,
		ou.Name,
		ou.Description,
		string(ouMetadataBytes),
		ou.UpdatedAt,
		s.scope(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// DeleteOrganizationUnit deletes an organization unit.
func (s *organizationUnitStore) DeleteOrganizationUnit(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteOrganizationUnit, id, s.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// GetOrganizationUnitChildrenCount retrieves the total count of child organization units for a given parent ID.
func (s *organizationUnitStore) GetOrganizationUnitChildrenCount(
	ctx context.Context, parentID string, f *tidcommon.FilterGroup,
) (int, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	query, filterArgs, err := buildChildrenOUCountQuery(f)
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}
	args := append([]interface{}{parentID, s.scope(ctx)}, filterArgs...)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	if len(results) == 0 {
		return 0, nil
	}

	if totalInterface, exists := results[0]["total"]; exists {
		if total, ok := totalInterface.(int64); ok {
			return int(total), nil
		}
	}

	return 0, fmt.Errorf("failed to parse count result")
}

// GetOrganizationUnitChildrenList retrieves a paginated list of child organization units for a given parent ID.
func (s *organizationUnitStore) GetOrganizationUnitChildrenList(ctx context.Context,
	parentID string, limit, offset int, f *tidcommon.FilterGroup,
) ([]providers.OrganizationUnitBasic, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, filterArgs, err := buildChildrenOUListQuery(f)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query: %w", err)
	}
	args := append([]interface{}{parentID, limit, offset, s.scope(ctx)}, filterArgs...)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	childOUs := make([]providers.OrganizationUnitBasic, 0, len(results))
	for _, row := range results {
		childOU, err := buildOrganizationUnitBasicFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build organization unit basic: %w", err)
		}
		childOUs = append(childOUs, childOU)
	}

	return childOUs, nil
}

// CheckOrganizationUnitNameConflict checks if an organization unit name conflicts under the same parent.
func (s *organizationUnitStore) CheckOrganizationUnitNameConflict(
	ctx context.Context, name string, parentID *string,
) (bool, error) {
	return s.checkConflict(ctx,
		queryCheckOrganizationUnitNameConflict,
		queryCheckOrganizationUnitNameConflictRoot,
		name,
		parentID,
		s.scope(ctx),
	)
}

// CheckOrganizationUnitHandleConflict checks if an organization unit handle conflicts under the same parent.
func (s *organizationUnitStore) CheckOrganizationUnitHandleConflict(
	ctx context.Context, handle string, parentID *string,
) (bool, error) {
	return s.checkConflict(ctx,
		queryCheckOrganizationUnitHandleConflict,
		queryCheckOrganizationUnitHandleConflictRoot,
		handle,
		parentID,
		s.scope(ctx),
	)
}

// buildOrganizationUnitBasicFromResultRow constructs a providers.OrganizationUnitBasic from a database result row.
func buildOrganizationUnitBasicFromResultRow(
	row map[string]interface{},
) (providers.OrganizationUnitBasic, error) {
	ouID, ok := row["ou_id"].(string)
	if !ok {
		return providers.OrganizationUnitBasic{}, fmt.Errorf("ou_id is not a string")
	}

	name, ok := row["name"].(string)
	if !ok {
		return providers.OrganizationUnitBasic{}, fmt.Errorf("name is not a string")
	}

	handle, ok := row["handle"].(string)
	if !ok {
		return providers.OrganizationUnitBasic{}, fmt.Errorf("handle is not a string")
	}

	description := ""
	if desc, ok := row["description"]; ok && desc != nil {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}

	ouMetadataData, err := parseOUMetadata(row)
	if err != nil {
		return providers.OrganizationUnitBasic{}, fmt.Errorf("failed to parse OU Metadata: %w", err)
	}

	logoURL, err := extractStringFromOUMetadata(ouMetadataData, "logo_url")
	if err != nil {
		return providers.OrganizationUnitBasic{}, err
	}

	createdAt, err := parseTimeField(row["created_at"], "created_at")
	if err != nil {
		return providers.OrganizationUnitBasic{}, fmt.Errorf("failed to parse created_at: %w", err)
	}

	updatedAt, err := parseTimeField(row["updated_at"], "updated_at")
	if err != nil {
		return providers.OrganizationUnitBasic{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return providers.OrganizationUnitBasic{
		ID:          ouID,
		Handle:      handle,
		Name:        name,
		Description: description,
		LogoURL:     logoURL,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// buildOrganizationUnitFromResultRow constructs a providers.OrganizationUnit from a database result row.
func buildOrganizationUnitFromResultRow(
	row map[string]interface{},
) (providers.OrganizationUnit, error) {
	ou, err := buildOrganizationUnitBasicFromResultRow(row)
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to build organization unit: %w", err)
	}

	var parentID *string
	if parent, ok := row["parent_id"]; ok && parent != nil {
		if parentStr, ok := parent.(string); ok {
			parentID = &parentStr
		}
	}

	// Extract OU Metadata data
	ouMetadataData, err := parseOUMetadata(row)
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to parse OU Metadata: %w", err)
	}

	// Extract fields from OU Metadata
	themeID, err := extractStringFromOUMetadata(ouMetadataData, "theme_id")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	layoutID, err := extractStringFromOUMetadata(ouMetadataData, "layout_id")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	authFlowID, err := extractStringFromOUMetadata(ouMetadataData, "auth_flow_id")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	registrationFlowID, err := extractStringFromOUMetadata(ouMetadataData, "registration_flow_id")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	isRegistrationFlowEnabled, err := extractBoolFromOUMetadata(ouMetadataData, "is_registration_flow_enabled")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	recoveryFlowID, err := extractStringFromOUMetadata(ouMetadataData, "recovery_flow_id")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	isRecoveryFlowEnabled, err := extractBoolFromOUMetadata(ouMetadataData, "is_recovery_flow_enabled")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	signOutFlowID, err := extractStringFromOUMetadata(ouMetadataData, "signout_flow_id")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	userOnboardingFlowID, err := extractStringFromOUMetadata(ouMetadataData, "user_onboarding_flow_id")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	logoURL, err := extractStringFromOUMetadata(ouMetadataData, "logo_url")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	tosURI, err := extractStringFromOUMetadata(ouMetadataData, "tos_uri")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	policyURI, err := extractStringFromOUMetadata(ouMetadataData, "policy_uri")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	cookiePolicyURI, err := extractStringFromOUMetadata(ouMetadataData, "cookie_policy_uri")
	if err != nil {
		return providers.OrganizationUnit{}, err
	}

	createdAt, err := parseTimeField(row["created_at"], "created_at")
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to parse created_at: %w", err)
	}

	updatedAt, err := parseTimeField(row["updated_at"], "updated_at")
	if err != nil {
		return providers.OrganizationUnit{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return providers.OrganizationUnit{
		ID:                        ou.ID,
		Handle:                    ou.Handle,
		Name:                      ou.Name,
		Description:               ou.Description,
		Parent:                    parentID,
		ThemeID:                   themeID,
		LayoutID:                  layoutID,
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        registrationFlowID,
		IsRegistrationFlowEnabled: isRegistrationFlowEnabled,
		RecoveryFlowID:            recoveryFlowID,
		IsRecoveryFlowEnabled:     isRecoveryFlowEnabled,
		SignOutFlowID:             signOutFlowID,
		UserOnboardingFlowID:      userOnboardingFlowID,
		LogoURL:                   logoURL,
		TosURI:                    tosURI,
		PolicyURI:                 policyURI,
		CookiePolicyURI:           cookiePolicyURI,
		CreatedAt:                 createdAt,
		UpdatedAt:                 updatedAt,
	}, nil
}

// parseTimeField parses a time field from the database result.
func parseTimeField(field interface{}, fieldName string) (time.Time, error) {
	const customTimeFormat = "2006-01-02 15:04:05.999999999"

	switch v := field.(type) {
	case string:
		trimmedTime := trimTimeString(v)
		parsedTime, err := time.Parse(customTimeFormat, trimmedTime)
		if err != nil {
			parsedTime, err = time.Parse(time.RFC3339, v)
			if err != nil {
				return time.Time{}, fmt.Errorf("error parsing %s: %w", fieldName, err)
			}
		}
		return parsedTime, nil
	case time.Time:
		return v, nil
	case nil:
		return time.Time{}, fmt.Errorf("%s is nil", fieldName)
	default:
		return time.Time{}, fmt.Errorf("unexpected type for %s: %T", fieldName, field)
	}
}

// trimTimeString trims extra information from a time string to match the expected format.
func trimTimeString(timeStr string) string {
	parts := strings.SplitN(timeStr, " ", 3)
	if len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}
	return timeStr
}

// checkConflict is a helper function to check for conflicts in organization unit attributes.
func (s *organizationUnitStore) checkConflict(ctx context.Context,
	queryWithParent, queryWithoutParent dbmodel.DBQuery,
	value string,
	parentID *string,
	extraArgs ...interface{},
) (bool, error) {
	dbClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}

	var results []map[string]interface{}

	if parentID != nil {
		args := append([]interface{}{value, *parentID}, extraArgs...)
		results, err = dbClient.QueryContext(ctx, queryWithParent, args...)
	} else {
		args := append([]interface{}{value}, extraArgs...)
		results, err = dbClient.QueryContext(ctx, queryWithoutParent, args...)
	}

	if err != nil {
		return false, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) > 0 {
		if count, ok := results[0]["count"].(int64); ok && count > 0 {
			return true, nil
		}
	}

	return false, nil
}

// getOUMetadataDataBytes constructs the JSON data bytes for the organization unit.
func getOUMetadataDataBytes(ou *providers.OrganizationUnit) ([]byte, error) {
	jsonData := map[string]interface{}{
		"theme_id":                     ou.ThemeID,
		"layout_id":                    ou.LayoutID,
		"auth_flow_id":                 ou.AuthFlowID,
		"registration_flow_id":         ou.RegistrationFlowID,
		"is_registration_flow_enabled": ou.IsRegistrationFlowEnabled,
		"recovery_flow_id":             ou.RecoveryFlowID,
		"is_recovery_flow_enabled":     ou.IsRecoveryFlowEnabled,
		"signout_flow_id":              ou.SignOutFlowID,
		"user_onboarding_flow_id":      ou.UserOnboardingFlowID,
		"logo_url":                     ou.LogoURL,
		"tos_uri":                      ou.TosURI,
		"policy_uri":                   ou.PolicyURI,
		"cookie_policy_uri":            ou.CookiePolicyURI,
	}

	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OU Metadata: %w", err)
	}

	return jsonBytes, nil
}

// parseOUMetadata parses the OU Metadata from the database result row.
func parseOUMetadata(row map[string]interface{}) (map[string]interface{}, error) {
	ouMetadataInterface, exists := row["metadata"]
	if !exists || ouMetadataInterface == nil {
		return map[string]interface{}{}, nil
	}

	var ouMetadataStr string
	switch v := ouMetadataInterface.(type) {
	case string:
		ouMetadataStr = v
	case []byte:
		ouMetadataStr = string(v)
	default:
		return nil, fmt.Errorf("failed to parse metadata as string or []byte, got type: %T", ouMetadataInterface)
	}

	if ouMetadataStr == "" {
		return map[string]interface{}{}, nil
	}

	var ouMetadataData map[string]interface{}
	if err := json.Unmarshal([]byte(ouMetadataStr), &ouMetadataData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OU Metadata: %w", err)
	}

	return ouMetadataData, nil
}

// extractStringFromOUMetadata extracts a string value from OU Metadata data,
// returns empty string if not found or invalid.
func extractStringFromOUMetadata(data map[string]interface{}, key string) (string, error) {
	if data[key] == nil {
		return "", nil
	}
	if str, ok := data[key].(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("failed to parse %s from OU Metadata", key)
}

// extractBoolFromOUMetadata extracts a boolean value from OU Metadata data,
// returns false if not found or invalid.
func extractBoolFromOUMetadata(data map[string]interface{}, key string) (bool, error) {
	if data[key] == nil {
		return false, nil
	}
	if b, ok := data[key].(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("failed to parse %s from OU Metadata", key)
}
