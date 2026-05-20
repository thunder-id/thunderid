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

package ou

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

const storeLoggerComponentName = "OrganizationUnitStore"

// organizationUnitStoreInterface defines the interface for organization unit store operations.
type organizationUnitStoreInterface interface {
	GetOrganizationUnitListCount(ctx context.Context, f *filter.FilterGroup) (int, error)
	GetOrganizationUnitList(
		ctx context.Context, limit, offset int, f *filter.FilterGroup,
	) ([]OrganizationUnitBasic, error)
	GetOrganizationUnitsByIDs(ctx context.Context, ids []string) ([]OrganizationUnitBasic, error)
	CreateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error
	GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, error)
	GetOrganizationUnitByHandle(ctx context.Context, handle string, parent *string) (OrganizationUnit, error)
	GetOrganizationUnitByPath(ctx context.Context, handles []string) (OrganizationUnit, error)
	IsOrganizationUnitExists(ctx context.Context, id string) (bool, error)
	IsOrganizationUnitDeclarative(ctx context.Context, id string) bool
	CheckOrganizationUnitNameConflict(ctx context.Context, name string, parent *string) (bool, error)
	CheckOrganizationUnitHandleConflict(ctx context.Context, handle string, parent *string) (bool, error)
	UpdateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error
	DeleteOrganizationUnit(ctx context.Context, id string) error
	GetOrganizationUnitChildrenCount(ctx context.Context, id string, f *filter.FilterGroup) (int, error)
	GetOrganizationUnitChildrenList(
		ctx context.Context, id string, limit, offset int, f *filter.FilterGroup,
	) ([]OrganizationUnitBasic, error)
}

var getDBProvider = provider.GetDBProvider

// organizationUnitStore is the default implementation of organizationUnitStoreInterface.
type organizationUnitStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newOrganizationUnitStore creates a new instance of organizationUnitStore.
func newOrganizationUnitStore() (organizationUnitStoreInterface, transaction.Transactioner, error) {
	dbProvider := getDBProvider()
	transactioner, err := dbProvider.GetUserDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &organizationUnitStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}, transactioner, nil
}

// GetOrganizationUnitListCount retrieves the total count of organization units.
func (s *organizationUnitStore) GetOrganizationUnitListCount(
	ctx context.Context, f *filter.FilterGroup,
) (int, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	query, filterArgs, err := buildRootOUCountQuery(f)
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}
	args := append([]interface{}{s.deploymentID}, filterArgs...)

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
	ctx context.Context, limit, offset int, f *filter.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, filterArgs, err := buildRootOUListQuery(f)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query: %w", err)
	}
	args := append([]interface{}{limit, offset, s.deploymentID}, filterArgs...)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	ous := make([]OrganizationUnitBasic, 0, len(results))
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
) ([]OrganizationUnitBasic, error) {
	if len(ids) == 0 {
		return []OrganizationUnitBasic{}, nil
	}

	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query := buildGetOrganizationUnitsByIDsQuery(ids)
	args := make([]interface{}, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, s.deploymentID)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	ous := make([]OrganizationUnitBasic, 0, len(results))
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
func (s *organizationUnitStore) CreateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
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
		ou.ThemeID,
		ou.LayoutID,
		string(ouMetadataBytes),
		s.deploymentID,
		ou.CreatedAt,
		ou.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// GetOrganizationUnit retrieves an organization unit by its id.
func (s *organizationUnitStore) GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetOrganizationUnitByID, id, s.deploymentID)
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	ou, err := buildOrganizationUnitFromResultRow(results[0])
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to build organization unit: %w", err)
	}

	return ou, nil
}

// GetOrganizationUnitByHandle retrieves an organization unit by handle and parent.
// When parent is nil, only root organization units are considered.
func (s *organizationUnitStore) GetOrganizationUnitByHandle(
	ctx context.Context, handle string, parent *string,
) (OrganizationUnit, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to get database client: %w", err)
	}

	var results []map[string]interface{}
	if parent == nil {
		results, err = dbClient.QueryContext(ctx, queryGetRootOrganizationUnitByHandle, handle, s.deploymentID)
	} else {
		results, err = dbClient.QueryContext(ctx, queryGetOrganizationUnitByHandle, handle, *parent, s.deploymentID)
	}
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to execute query for handle %s: %w", handle, err)
	}

	if len(results) == 0 {
		return OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	ou, err := buildOrganizationUnitFromResultRow(results[0])
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to build organization unit for handle %s: %w", handle, err)
	}

	return ou, nil
}

// GetOrganizationUnitByPath retrieves an organization unit by its hierarchical handle path.
func (s *organizationUnitStore) GetOrganizationUnitByPath(
	ctx context.Context, handlePath []string,
) (OrganizationUnit, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, storeLoggerComponentName))

	if len(handlePath) == 0 {
		return OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to get database client: %w", err)
	}

	var currentOU OrganizationUnit
	var parentID *string
	var fullPath string

	for i, handle := range handlePath {
		fullPath = fullPath + "/" + handle
		currentOU, err = s.getOrganizationUnitByHandleWithClient(ctx, dbClient, handle, parentID)
		if err != nil {
			if !errors.Is(err, ErrOrganizationUnitNotFound) {
				return OrganizationUnit{}, err
			}
			logger.Debug("Organization unit not found in path",
				log.String("handle", handle),
				log.Int("pathIndex", i),
				log.String("fullPath", fullPath))
			return OrganizationUnit{}, ErrOrganizationUnitNotFound
		}

		parentID = &currentOU.ID
	}

	return currentOU, nil
}

func (s *organizationUnitStore) getOrganizationUnitByHandleWithClient(
	ctx context.Context, dbClient provider.DBClientInterface, handle string, parent *string,
) (OrganizationUnit, error) {
	var results []map[string]interface{}
	var err error

	if parent == nil {
		results, err = dbClient.QueryContext(ctx, queryGetRootOrganizationUnitByHandle, handle, s.deploymentID)
	} else {
		results, err = dbClient.QueryContext(ctx, queryGetOrganizationUnitByHandle, handle, *parent, s.deploymentID)
	}
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to execute query for handle %s: %w", handle, err)
	}

	if len(results) == 0 {
		return OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	ou, err := buildOrganizationUnitFromResultRow(results[0])
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to build organization unit for handle %s: %w", handle, err)
	}

	return ou, nil
}

// IsOrganizationUnitExists checks if an organization unit exists by ID.
func (s *organizationUnitStore) IsOrganizationUnitExists(ctx context.Context, id string) (bool, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryCheckOrganizationUnitExists, id, s.deploymentID)
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
func (s *organizationUnitStore) UpdateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
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
		ou.ThemeID,
		ou.LayoutID,
		string(ouMetadataBytes),
		ou.UpdatedAt,
		s.deploymentID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// DeleteOrganizationUnit deletes an organization unit.
func (s *organizationUnitStore) DeleteOrganizationUnit(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteOrganizationUnit, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// GetOrganizationUnitChildrenCount retrieves the total count of child organization units for a given parent ID.
func (s *organizationUnitStore) GetOrganizationUnitChildrenCount(
	ctx context.Context, parentID string, f *filter.FilterGroup,
) (int, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	query, filterArgs, err := buildChildrenOUCountQuery(f)
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}
	args := append([]interface{}{parentID, s.deploymentID}, filterArgs...)

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
	parentID string, limit, offset int, f *filter.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, filterArgs, err := buildChildrenOUListQuery(f)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query: %w", err)
	}
	args := append([]interface{}{parentID, limit, offset, s.deploymentID}, filterArgs...)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	childOUs := make([]OrganizationUnitBasic, 0, len(results))
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
		s.deploymentID,
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
		s.deploymentID,
	)
}

// buildOrganizationUnitBasicFromResultRow constructs a OrganizationUnitBasic from a database result row.
func buildOrganizationUnitBasicFromResultRow(
	row map[string]interface{},
) (OrganizationUnitBasic, error) {
	ouID, ok := row["ou_id"].(string)
	if !ok {
		return OrganizationUnitBasic{}, fmt.Errorf("ou_id is not a string")
	}

	name, ok := row["name"].(string)
	if !ok {
		return OrganizationUnitBasic{}, fmt.Errorf("name is not a string")
	}

	handle, ok := row["handle"].(string)
	if !ok {
		return OrganizationUnitBasic{}, fmt.Errorf("handle is not a string")
	}

	description := ""
	if desc, ok := row["description"]; ok && desc != nil {
		if descStr, ok := desc.(string); ok {
			description = descStr
		}
	}

	ouMetadataData, err := parseOUMetadata(row)
	if err != nil {
		return OrganizationUnitBasic{}, fmt.Errorf("failed to parse OU Metadata: %w", err)
	}

	logoURL, err := extractStringFromOUMetadata(ouMetadataData, "logo_url")
	if err != nil {
		return OrganizationUnitBasic{}, err
	}

	createdAt, err := parseTimeField(row["created_at"], "created_at")
	if err != nil {
		return OrganizationUnitBasic{}, fmt.Errorf("failed to parse created_at: %w", err)
	}

	updatedAt, err := parseTimeField(row["updated_at"], "updated_at")
	if err != nil {
		return OrganizationUnitBasic{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return OrganizationUnitBasic{
		ID:          ouID,
		Handle:      handle,
		Name:        name,
		Description: description,
		LogoURL:     logoURL,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// buildOrganizationUnitFromResultRow constructs a OrganizationUnit from a database result row.
func buildOrganizationUnitFromResultRow(
	row map[string]interface{},
) (OrganizationUnit, error) {
	ou, err := buildOrganizationUnitBasicFromResultRow(row)
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to build organization unit: %w", err)
	}

	var parentID *string
	if parent, ok := row["parent_id"]; ok && parent != nil {
		if parentStr, ok := parent.(string); ok {
			parentID = &parentStr
		}
	}

	themeID := ""
	if v, ok := row["theme_id"]; ok && v != nil {
		if s, ok := v.(string); ok {
			themeID = s
		}
	}

	layoutID := ""
	if v, ok := row["layout_id"]; ok && v != nil {
		if s, ok := v.(string); ok {
			layoutID = s
		}
	}

	// Extract OU Metadata data
	ouMetadataData, err := parseOUMetadata(row)
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to parse OU Metadata: %w", err)
	}

	// Extract fields from OU Metadata
	logoURL, err := extractStringFromOUMetadata(ouMetadataData, "logo_url")
	if err != nil {
		return OrganizationUnit{}, err
	}

	tosURI, err := extractStringFromOUMetadata(ouMetadataData, "tos_uri")
	if err != nil {
		return OrganizationUnit{}, err
	}

	policyURI, err := extractStringFromOUMetadata(ouMetadataData, "policy_uri")
	if err != nil {
		return OrganizationUnit{}, err
	}

	cookiePolicyURI, err := extractStringFromOUMetadata(ouMetadataData, "cookie_policy_uri")
	if err != nil {
		return OrganizationUnit{}, err
	}

	createdAt, err := parseTimeField(row["created_at"], "created_at")
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to parse created_at: %w", err)
	}

	updatedAt, err := parseTimeField(row["updated_at"], "updated_at")
	if err != nil {
		return OrganizationUnit{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return OrganizationUnit{
		ID:              ou.ID,
		Handle:          ou.Handle,
		Name:            ou.Name,
		Description:     ou.Description,
		Parent:          parentID,
		ThemeID:         themeID,
		LayoutID:        layoutID,
		LogoURL:         logoURL,
		TosURI:          tosURI,
		PolicyURI:       policyURI,
		CookiePolicyURI: cookiePolicyURI,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
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
	dbClient, err := s.dbProvider.GetUserDBClient()
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
func getOUMetadataDataBytes(ou *OrganizationUnit) ([]byte, error) {
	jsonData := map[string]interface{}{
		"logo_url":          ou.LogoURL,
		"tos_uri":           ou.TosURI,
		"policy_uri":        ou.PolicyURI,
		"cookie_policy_uri": ou.CookiePolicyURI,
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
