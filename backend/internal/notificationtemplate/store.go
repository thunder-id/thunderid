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

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// notificationTemplateStoreInterface defines the persistence operations for notification templates.
// All methods take a context so they participate in a transaction started by the service (the DB
// client picks up a transaction from the context automatically).
type notificationTemplateStoreInterface interface {
	CreateTemplate(ctx context.Context, template templateDAO) error
	GetTemplate(ctx context.Context, channel ChannelType, id string) (templateDAO, error)
	GetTemplateByHandle(ctx context.Context, channel ChannelType, handle string) (templateDAO, error)
	ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) ([]templateDAO, error)
	CountTemplates(ctx context.Context, channel ChannelType) (int, error)
	UpdateTemplate(ctx context.Context, template templateDAO) error
	DeleteTemplate(ctx context.Context, channel ChannelType, id string) error
	IsHandleExists(ctx context.Context, channel ChannelType, handle string) (bool, error)
}

// notificationTemplateStore is the config-DB backed implementation.
type notificationTemplateStore struct {
	dbProvider provider.DBProviderInterface
}

// newNotificationTemplateStore creates a new config-DB backed store.
func newNotificationTemplateStore() notificationTemplateStoreInterface {
	return &notificationTemplateStore{
		dbProvider: provider.GetDBProvider(),
	}
}

// scope returns the deployment id this request acts for. The id is put on the context at the
// edge, so a request scopes by what it names; a context that never passed through the edge,
// such as a start-up task or a background job, falls back to the configured identifier.
func (s *notificationTemplateStore) scope(ctx context.Context) string {
	return deployment.Resolve(ctx)
}

// CreateTemplate inserts a template row.
func (s *notificationTemplateStore) CreateTemplate(ctx context.Context, t templateDAO) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	contentJSON, err := json.Marshal(t.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal template content: %w", err)
	}

	designJSON, err := designColumnValue(t)
	if err != nil {
		return err
	}

	if _, err := dbClient.ExecuteContext(ctx, queryCreateTemplate,
		t.ID, string(t.Channel), t.Handle, t.DisplayName, t.Description, string(contentJSON),
		designJSON, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to execute create query: %w", err)
	}

	return nil
}

// GetTemplate retrieves a template by channel and id.
func (s *notificationTemplateStore) GetTemplate(ctx context.Context, channel ChannelType, id string) (
	templateDAO, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return templateDAO{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetTemplateByID, id, string(channel), s.scope(ctx))
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

// GetTemplateByHandle retrieves a template by channel and handle (the runtime lookup key; handles are
// immutable and unique per channel).
func (s *notificationTemplateStore) GetTemplateByHandle(ctx context.Context, channel ChannelType, handle string) (
	templateDAO, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return templateDAO{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetTemplateByHandle, handle, string(channel), s.scope(ctx))
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

// ListTemplates retrieves a page of templates of a channel.
func (s *notificationTemplateStore) ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) (
	[]templateDAO, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryListTemplates, string(channel), limit, offset, s.scope(ctx))
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

// CountTemplates returns the number of templates in a channel, for pagination totals.
func (s *notificationTemplateStore) CountTemplates(ctx context.Context, channel ChannelType) (int, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return 0, err
	}

	results, err := dbClient.QueryContext(ctx, queryCountTemplates, string(channel), s.scope(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	return parseCountResult(results)
}

// UpdateTemplate updates a template's mutable fields, including its design.
func (s *notificationTemplateStore) UpdateTemplate(ctx context.Context, t templateDAO) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	contentJSON, err := json.Marshal(t.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal template content: %w", err)
	}

	designJSON, err := designColumnValue(t)
	if err != nil {
		return err
	}

	if _, err := dbClient.ExecuteContext(ctx, queryUpdateTemplate,
		t.DisplayName, t.Description, string(contentJSON), designJSON,
		t.ID, string(t.Channel), s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to execute update query: %w", err)
	}

	return nil
}

// DeleteTemplate deletes a template row by channel and id.
func (s *notificationTemplateStore) DeleteTemplate(ctx context.Context, channel ChannelType, id string) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	if _, err := dbClient.ExecuteContext(ctx, queryDeleteTemplate, id, string(channel), s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to execute delete query: %w", err)
	}

	return nil
}

// IsHandleExists checks whether a template in the channel already uses the given handle. The handle is
// immutable, so this is only ever a create-time pre-check.
func (s *notificationTemplateStore) IsHandleExists(ctx context.Context, channel ChannelType, handle string) (
	bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.QueryContext(ctx, queryCheckHandleExists, string(channel), handle, s.scope(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to check template handle: %w", err)
	}

	count, err := parseCountResult(results)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// designColumnValue returns the DESIGN column value to persist for a template: the design marshaled to
// a JSON string when present, or an empty JSON object for a channel without a design. The column is
// NOT NULL, so a value is always written.
func designColumnValue(t templateDAO) (string, error) {
	if t.Design == nil {
		return "{}", nil
	}
	b, err := json.Marshal(t.Design)
	if err != nil {
		return "", fmt.Errorf("failed to marshal template design: %w", err)
	}
	return string(b), nil
}

// getConfigDBClient retrieves the config database client.
func (s *notificationTemplateStore) getConfigDBClient() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get config database client: %w", err)
	}
	return dbClient, nil
}

// buildTemplateFromRow maps a DB result row to a templateDAO. The CONTENT column is a JSON document;
// the DESIGN column holds an empty JSON object ({}) when the template carries no design.
func buildTemplateFromRow(row map[string]interface{}) (templateDAO, error) {
	id, ok := row["id"].(string)
	if !ok {
		return templateDAO{}, fmt.Errorf("id not found or invalid type")
	}

	channel, ok := row["channel"].(string)
	if !ok {
		return templateDAO{}, fmt.Errorf("channel not found or invalid type")
	}

	handle, ok := row["handle"].(string)
	if !ok {
		return templateDAO{}, fmt.Errorf("handle not found or invalid type")
	}

	displayName, ok := row["display_name"].(string)
	if !ok {
		return templateDAO{}, fmt.Errorf("display_name not found or invalid type")
	}

	dao := templateDAO{
		ID:          id,
		Channel:     ChannelType(channel),
		Handle:      handle,
		DisplayName: displayName,
		Description: stringOrEmpty(row["description"]),
	}

	if contentStr := jsonColumnToString(row["content"]); contentStr != "" {
		if err := json.Unmarshal([]byte(contentStr), &dao.Content); err != nil {
			return templateDAO{}, fmt.Errorf("failed to unmarshal template content: %w", err)
		}
	}

	if designStr := jsonColumnToString(row["design"]); designStr != "" {
		var design TemplateDesign
		if err := json.Unmarshal([]byte(designStr), &design); err != nil {
			return templateDAO{}, fmt.Errorf("failed to unmarshal template design: %w", err)
		}
		// An empty JSON object ({}) is how a channel without a design is stored; treat it as absent.
		if design != (TemplateDesign{}) {
			dao.Design = &design
		}
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
		return 0, fmt.Errorf("count field 'total' not found in result")
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
