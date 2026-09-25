// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// notificationTemplateStoreInterface defines the persistence operations for notification templates.
// All methods take a context so they participate in a transaction started by the service (the DB
// client picks up a transaction from the context automatically).
type notificationTemplateStoreInterface interface {
	CreateTemplate(ctx context.Context, template templateDAO) error
	GetTemplate(ctx context.Context, channel, id string) (templateDAO, error)
	ListTemplates(ctx context.Context, channel string) ([]templateDAO, error)
	UpdateTemplate(ctx context.Context, template templateDAO) error
	DeleteTemplate(ctx context.Context, channel, id string) error
	IsNameExists(ctx context.Context, channel, name, excludeID string) (bool, error)
}

// notificationTemplateStore is the config-DB backed implementation.
type notificationTemplateStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newNotificationTemplateStore creates a new config-DB backed store.
func newNotificationTemplateStore() notificationTemplateStoreInterface {
	return &notificationTemplateStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// CreateTemplate inserts a template row and, when a design is present, its companion design row.
// Atomicity across the two tables is provided by the transaction the service wraps this call in.
func (s *notificationTemplateStore) CreateTemplate(ctx context.Context, t templateDAO) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	contentJSON, err := json.Marshal(t.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal template content: %w", err)
	}

	if _, err := dbClient.ExecuteContext(ctx, queryCreateTemplate,
		t.ID, t.Channel, t.Name, t.Description, contentJSON, s.deploymentID); err != nil {
		return fmt.Errorf("failed to execute create query: %w", err)
	}

	return s.writeDesign(ctx, dbClient, t)
}

// GetTemplate retrieves a template and its design by channel and id.
func (s *notificationTemplateStore) GetTemplate(ctx context.Context, channel, id string) (templateDAO, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return templateDAO{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetTemplateByID, id, channel, s.deploymentID)
	if err != nil {
		return templateDAO{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return templateDAO{}, errTemplateNotFound
	}
	if len(results) != 1 {
		return templateDAO{}, fmt.Errorf("unexpected number of results: %d", len(results))
	}

	return buildTemplateFromRow(results[0])
}

// ListTemplates retrieves all templates of a channel with their designs.
func (s *notificationTemplateStore) ListTemplates(ctx context.Context, channel string) ([]templateDAO, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryListTemplates, channel, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute list query: %w", err)
	}

	templates := make([]templateDAO, 0, len(results))
	for _, row := range results {
		t, err := buildTemplateFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build template from row: %w", err)
		}
		templates = append(templates, t)
	}

	return templates, nil
}

// UpdateTemplate updates a template's mutable fields and reconciles its design row. Atomicity is
// provided by the transaction the service wraps this call in.
func (s *notificationTemplateStore) UpdateTemplate(ctx context.Context, t templateDAO) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	contentJSON, err := json.Marshal(t.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal template content: %w", err)
	}

	if _, err := dbClient.ExecuteContext(ctx, queryUpdateTemplate,
		t.Name, t.Description, contentJSON, t.ID, t.Channel, s.deploymentID); err != nil {
		return fmt.Errorf("failed to execute update query: %w", err)
	}

	// Reconcile the design row: drop any existing one, then write the new one if present.
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteTemplateDesign, t.ID, s.deploymentID); err != nil {
		return fmt.Errorf("failed to clear template design: %w", err)
	}

	return s.writeDesign(ctx, dbClient, t)
}

// DeleteTemplate deletes a template and its design row. Atomicity is provided by the transaction the
// service wraps this call in.
// DeleteTemplate deletes a template row. The companion design row is removed automatically by the
// NOTIFICATION_TEMPLATE_DESIGN foreign key's ON DELETE CASCADE (foreign keys are enforced on both
// Postgres and SQLite), so a single statement is atomic and no explicit design delete is needed.
func (s *notificationTemplateStore) DeleteTemplate(ctx context.Context, channel, id string) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	if _, err := dbClient.ExecuteContext(ctx, queryDeleteTemplate, id, channel, s.deploymentID); err != nil {
		return fmt.Errorf("failed to execute delete query: %w", err)
	}

	return nil
}

// IsNameExists checks whether another template in the channel already uses the given name. excludeID
// lets an update ignore the template being updated; pass "" for a create.
func (s *notificationTemplateStore) IsNameExists(ctx context.Context, channel, name, excludeID string) (bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.QueryContext(ctx, queryCheckNameExists, channel, name, s.deploymentID, excludeID)
	if err != nil {
		return false, fmt.Errorf("failed to check template name: %w", err)
	}

	count, err := parseCountResult(results)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// writeDesign inserts the companion design row when the template carries a design. It assumes any
// prior row was already removed (create has none; update deletes first).
func (s *notificationTemplateStore) writeDesign(ctx context.Context,
	dbClient provider.DBClientInterface, t templateDAO) error {
	if t.Design == nil || t.Design.ColorScheme == "" {
		return nil
	}
	if _, err := dbClient.ExecuteContext(ctx, queryUpsertTemplateDesign,
		t.ID, t.Design.ColorScheme, s.deploymentID); err != nil {
		return fmt.Errorf("failed to write template design: %w", err)
	}
	return nil
}

// getConfigDBClient retrieves the config database client.
func (s *notificationTemplateStore) getConfigDBClient() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get config database client: %w", err)
	}
	return dbClient, nil
}

// buildTemplateFromRow maps a joined DB result row to a templateDAO. The CONTENT column is a JSON
// document; the COLOR_SCHEME column comes from the LEFT-joined design table and is NULL when absent.
func buildTemplateFromRow(row map[string]interface{}) (templateDAO, error) {
	id, ok := row["id"].(string)
	if !ok {
		return templateDAO{}, fmt.Errorf("id not found or invalid type")
	}

	channel, ok := row["channel"].(string)
	if !ok {
		return templateDAO{}, fmt.Errorf("channel not found or invalid type")
	}

	name, ok := row["name"].(string)
	if !ok {
		return templateDAO{}, fmt.Errorf("name not found or invalid type")
	}

	dao := templateDAO{
		ID:          id,
		Channel:     channel,
		Name:        name,
		Description: stringOrEmpty(row["description"]),
	}

	if contentStr := jsonColumnToString(row["content"]); contentStr != "" {
		if err := json.Unmarshal([]byte(contentStr), &dao.Content); err != nil {
			return templateDAO{}, fmt.Errorf("failed to unmarshal template content: %w", err)
		}
	}

	if colorScheme := stringOrEmpty(row["color_scheme"]); colorScheme != "" {
		dao.Design = &TemplateDesign{ColorScheme: colorScheme}
	}

	return dao, nil
}

// isUniqueViolation reports whether err is a UNIQUE-constraint violation, across both supported
// drivers: PostgreSQL (lib/pq, SQLSTATE 23505) and SQLite (modernc, whose message contains
// "UNIQUE constraint failed"). Store errors are wrapped with %w, so errors.As unwraps the pq error.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key")
}

// stringOrEmpty returns the string value of a (possibly NULL) column or "".
func stringOrEmpty(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// jsonColumnToString returns a JSON column value as a string, tolerating both the string (SQLite) and
// []byte (PostgreSQL) representations.
func jsonColumnToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return ""
	}
}

// parseCountResult extracts a COUNT(*) value from a result set.
func parseCountResult(results []map[string]interface{}) (int, error) {
	if len(results) == 0 {
		return 0, fmt.Errorf("no results returned from count query")
	}

	countVal, exists := results[0]["total"]
	if !exists {
		countVal, exists = results[0]["count"]
		if !exists {
			return 0, fmt.Errorf("count field not found in result (tried 'total' and 'count')")
		}
	}

	switch v := countVal.(type) {
	case int64:
		return int(v), nil
	case int:
		return v, nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected type for count: %T", countVal)
	}
}
