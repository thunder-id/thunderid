// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// storeInterface is what the service needs of persistence.
//
// Secrets are read and written as ciphertext here: encryption belongs to the service,
// because the store's job is rows and the key is not its business.
// variableStoreInterface is the variables half of persistence.
type variableStoreInterface interface {
	GetVariable(ctx context.Context, name string) (*Variable, error)
	ListVariables(ctx context.Context, q listQuery) ([]Variable, int, error)
	// InsertVariable stores a variable and reports whether it did. False means the name was taken.
	InsertVariable(ctx context.Context, v Variable) (bool, error)
	UpsertVariable(ctx context.Context, v Variable) (created bool, err error)
	// DeleteVariable removes a variable and reports whether one was stored under the name.
	DeleteVariable(ctx context.Context, name string) (bool, error)
	// DeleteVariables removes the named variables and returns the names of those it removed. A
	// positive unchangedFor removes only those last set longer ago than it, by the database's clock.
	DeleteVariables(ctx context.Context, names []string, unchangedFor time.Duration) ([]string, error)
}

// secretStoreInterface is the secrets half of persistence.
//
// The value it takes is already encrypted: the service encrypts before it reaches here, so nothing
// below this line ever holds a credential in the clear.
type secretStoreInterface interface {
	GetSecret(ctx context.Context, name string) (*Secret, error)
	// GetSecretValue returns a secret's ciphertext and whether one is stored under the name.
	GetSecretValue(ctx context.Context, name string) (string, bool, error)
	ListSecrets(ctx context.Context, q listQuery) ([]Secret, int, error)
	// InsertSecret stores a secret and reports whether it did. False means the name was taken.
	InsertSecret(ctx context.Context, name, encrypted, description string) (bool, error)
	UpsertSecret(ctx context.Context, name, encrypted, description string) (created bool, err error)
	// DeleteSecret removes a secret and reports whether one was stored under the name.
	DeleteSecret(ctx context.Context, name string) (bool, error)
	// DeleteSecrets removes the named secrets and returns the names of those it removed. A positive
	// unchangedFor removes only those last set longer ago than it, by the database's clock.
	DeleteSecrets(ctx context.Context, names []string, unchangedFor time.Duration) ([]string, error)
}

// storeInterface is both halves, which is what the service holds.
type storeInterface interface {
	variableStoreInterface
	secretStoreInterface
}

type store struct {
	dbProvider provider.DBProviderInterface
}

func newStore() storeInterface {
	return &store{dbProvider: provider.GetDBProvider()}
}

// scope is the deployment this request acts for.
func (s *store) scope(ctx context.Context) string {
	return deployment.Resolve(ctx)
}

func (s *store) client() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient, nil
}

func (s *store) GetVariable(ctx context.Context, name string) (*Variable, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	rows, err := dbClient.QueryContext(ctx, queryGetVariable, name, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read variable: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	v := variableFromRow(rows[0])
	return &v, nil
}

func (s *store) InsertVariable(ctx context.Context, v Variable) (bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return false, err
	}
	inserted, err := dbClient.ExecuteContext(ctx, queryInsertVariable,
		v.Name, v.Value, v.Description, s.scope(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to insert variable: %w", err)
	}
	return inserted > 0, nil
}

// UpsertVariable creates the variable when the name is free and replaces it when it is not. It
// reports which of the two happened, decided by the insert rather than by a prior read.
// upsertAttempts bounds how many times a create-or-replace retries.
//
// Each attempt inserts if the name is free and updates if it is taken. Both do nothing only when the
// row was deleted between them, which is another writer's doing; repeating resolves it. The bound is
// there so a name being deleted in a tight loop cannot hold a request open forever, and running out
// is reported as a failure rather than as a write that did not happen.
const upsertAttempts = 3

func (s *store) UpsertVariable(ctx context.Context, v Variable) (bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return false, err
	}

	for attempt := 0; attempt < upsertAttempts; attempt++ {
		inserted, err := dbClient.ExecuteContext(ctx, queryUpsertVariable,
			v.Name, v.Value, v.Description, s.scope(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to write variable: %w", err)
		}
		if inserted > 0 {
			return true, nil
		}

		// The name was taken when the insert ran, so replace what is there. If that matches nothing,
		// the row was deleted in between and neither statement stored the value: go round again
		// rather than report a write that did not happen.
		updated, err := dbClient.ExecuteContext(ctx, queryUpdateVariable,
			v.Name, v.Value, v.Description, s.scope(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to write variable: %w", err)
		}
		if updated > 0 {
			return false, nil
		}
	}
	return false, fmt.Errorf("failed to write variable %q: it was deleted during each attempt", v.Name)
}

func (s *store) DeleteVariable(ctx context.Context, name string) (bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return false, err
	}
	deleted, err := dbClient.ExecuteContext(ctx, queryDeleteVariable, name, s.scope(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to delete variable: %w", err)
	}
	return deleted > 0, nil
}

func (s *store) GetSecret(ctx context.Context, name string) (*Secret, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	rows, err := dbClient.QueryContext(ctx, querySecretExists, name, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	sec := secretFromRow(rows[0])
	return &sec, nil
}

func (s *store) GetSecretValue(ctx context.Context, name string) (string, bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return "", false, err
	}
	rows, err := dbClient.QueryContext(ctx, querySecretValue, name, s.scope(ctx))
	if err != nil {
		return "", false, fmt.Errorf("failed to read secret value: %w", err)
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return rowString(rows[0], "value"), true, nil
}

func (s *store) InsertSecret(ctx context.Context, name, encrypted, description string) (bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return false, err
	}
	inserted, err := dbClient.ExecuteContext(ctx, queryInsertSecret,
		name, encrypted, description, s.scope(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to insert secret: %w", err)
	}
	return inserted > 0, nil
}

// UpsertSecret stores the secret when the name is free and rotates it when it is not, reporting
// which happened. The encrypted value arrives ready to store; this never sees a plaintext one.
func (s *store) UpsertSecret(ctx context.Context, name, encrypted, description string) (bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return false, err
	}

	for attempt := 0; attempt < upsertAttempts; attempt++ {
		inserted, err := dbClient.ExecuteContext(ctx, queryUpsertSecret,
			name, encrypted, description, s.scope(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to write secret: %w", err)
		}
		if inserted > 0 {
			return true, nil
		}

		updated, err := dbClient.ExecuteContext(ctx, queryUpdateSecret,
			name, encrypted, description, s.scope(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to write secret: %w", err)
		}
		if updated > 0 {
			return false, nil
		}
	}
	return false, fmt.Errorf("failed to write secret %q: it was deleted during each attempt", name)
}

func (s *store) DeleteSecret(ctx context.Context, name string) (bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return false, err
	}
	deleted, err := dbClient.ExecuteContext(ctx, queryDeleteSecret, name, s.scope(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to delete secret: %w", err)
	}
	return deleted > 0, nil
}

// DeleteVariables removes the named variables in one statement and returns those it removed.
func (s *store) DeleteVariables(ctx context.Context, names []string, unchangedFor time.Duration) ([]string, error) {
	return s.deleteMany(ctx, "variables", queryDeleteVariables, names, unchangedFor)
}

// DeleteSecrets removes the named secrets in one statement and returns those it removed.
func (s *store) DeleteSecrets(ctx context.Context, names []string, unchangedFor time.Duration) ([]string, error) {
	return s.deleteMany(ctx, "secrets", queryDeleteSecrets, names, unchangedFor)
}

// deleteMany runs a delete of many names, given as one JSON array parameter the database expands, and
// reads back the names the statement removed. The age a value must have is checked by the same
// statement that removes it, against the database's own clock, so a value last set more recently than
// that when the statement runs is never removed.
func (s *store) deleteMany(ctx context.Context, collection string, query dbmodel.DBQuery,
	names []string, unchangedFor time.Duration) ([]string, error) {
	if len(names) == 0 {
		return []string{}, nil
	}
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(names)
	if err != nil {
		return nil, fmt.Errorf("failed to delete %s: %w", collection, err)
	}
	rows, err := dbClient.QueryContext(ctx, query, string(encoded), s.scope(ctx), ageBound(unchangedFor))
	if err != nil {
		return nil, fmt.Errorf("failed to delete %s: %w", collection, err)
	}
	deleted := make([]string, 0, len(rows))
	for _, row := range rows {
		deleted = append(deleted, rowString(row, "name"))
	}
	return deleted, nil
}

// ageBound is the age parameter of a delete of many names: NULL for none, which removes a value
// whenever it was set, or the age in whole seconds, rounded up so a value is never removed younger.
func ageBound(unchangedFor time.Duration) interface{} {
	if unchangedFor <= 0 {
		return nil
	}
	return int64((unchangedFor + time.Second - 1) / time.Second)
}

// ListVariables returns one page of variables and the total the query matched.
func (s *store) ListVariables(ctx context.Context, q listQuery) ([]Variable, int, error) {
	return listPage(ctx, s, "variables", queryCountVariables, queryListVariables, q, variableFromRow)
}

// ListSecrets returns one page of secret names and the total the query matched. No value is read.
func (s *store) ListSecrets(ctx context.Context, q listQuery) ([]Secret, int, error) {
	return listPage(ctx, s, "secrets", queryCountSecrets, queryListSecrets, q, secretFromRow)
}

// listPage counts what the query matches and reads one page of it, both in the database.
//
// The narrowing, the order and the page are all in the statements, so a names= lookup or an early
// page reads the rows it returns rather than the whole collection. The statements are fixed: every
// optional part of the query is always in them and is switched off by its parameter, so nothing is
// built from the request. A names= lookup goes in as one JSON array parameter, which the database
// expands, and a prefix goes in escaped so a name holding a LIKE wildcard matches only itself.
//
// The count runs first, so a page past the end is answered without reading the page at all.
func listPage[T any](
	ctx context.Context,
	s *store,
	collection string,
	countQuery dbmodel.DBQuery,
	pageQuery dbmodel.DBQuery,
	q listQuery,
	fromRow func(map[string]interface{}) T,
) ([]T, int, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, 0, err
	}
	args, err := listArgs(s.scope(ctx), q)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list %s: %w", collection, err)
	}

	countRows, err := dbClient.QueryContext(ctx, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count %s: %w", collection, err)
	}
	total := 0
	if len(countRows) > 0 {
		total, err = rowInt(countRows[0], "total")
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count %s: %w", collection, err)
		}
	}
	if total == 0 || q.offset >= total {
		return []T{}, total, nil
	}

	rows, err := dbClient.QueryContext(ctx, pageQuery, append(args, q.limit, q.offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list %s: %w", collection, err)
	}
	page := make([]T, 0, len(rows))
	for _, row := range rows {
		page = append(page, fromRow(row))
	}
	return page, total, nil
}

// listArgs are the parameters the list and count statements share, in their order. An absent part
// of the query is passed as the value that switches its predicate off: NULL for names and an empty string for the
// exact name and the prefix.
func listArgs(deploymentID string, q listQuery) ([]interface{}, error) {
	var names interface{}
	if len(q.names) > 0 {
		encoded, err := json.Marshal(q.names)
		if err != nil {
			return nil, err
		}
		names = string(encoded)
	}
	pattern := ""
	if q.namePrefix != "" {
		pattern = likePrefix(q.namePrefix)
	}
	return []interface{}{deploymentID, names, q.nameEquals, q.namePrefix, pattern}, nil
}

// likeEscaper escapes what LIKE would read as other than itself. The backslash goes first, or it
// would escape the escapes; the statements declare it with an ESCAPE clause.
var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

// likePrefix turns a prefix into a LIKE pattern matching any name that starts with it, and only
// those: a name may hold an underscore, which unescaped would match any character.
func likePrefix(prefix string) string {
	return likeEscaper.Replace(prefix) + "%"
}

// rowInt reads a count column, which the drivers hand back as different integer types.
func rowInt(row map[string]interface{}, column string) (int, error) {
	value, ok := row[column]
	if !ok {
		value = row[strings.ToUpper(column)]
	}
	switch typed := value.(type) {
	case int64:
		return int(typed), nil
	case int:
		return typed, nil
	case int32:
		return int(typed), nil
	case float64:
		return int(typed), nil
	case []byte:
		return strconv.Atoi(string(typed))
	case string:
		return strconv.Atoi(typed)
	default:
		return 0, fmt.Errorf("unexpected %s value %v", column, value)
	}
}

func variableFromRow(row map[string]interface{}) Variable {
	return Variable{
		Name:        rowString(row, "name"),
		Value:       rowString(row, "value"),
		Description: rowString(row, "description"),
		CreatedAt:   rowString(row, "created_at"),
		UpdatedAt:   rowString(row, "updated_at"),
	}
}

func secretFromRow(row map[string]interface{}) Secret {
	return Secret{
		Name:        rowString(row, "name"),
		Exists:      true,
		Description: rowString(row, "description"),
		CreatedAt:   rowString(row, "created_at"),
		UpdatedAt:   rowString(row, "updated_at"),
	}
}

// rowString reads a column as text, tolerating a driver that hands back bytes and a column name in
// either case.
func rowString(row map[string]interface{}, column string) string {
	value, ok := row[column]
	if !ok {
		value = row[strings.ToUpper(column)]
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case nil:
		return ""
	case time.Time:
		// A timestamp column arrives as a time on postgres and as text on sqlite. Both are given out
		// in one form, so a caller comparing them does not have to know which store answered.
		return typed.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
