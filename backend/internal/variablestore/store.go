// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"fmt"
	"sort"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// storeInterface is what the service needs of persistence.
//
// Secrets are read and written as ciphertext here: sealing and unsealing belong to the service,
// because the store's job is rows and the key is not its business.
type storeInterface interface {
	GetVariable(ctx context.Context, name string) (*Variable, error)
	ListVariables(ctx context.Context, q listQuery) ([]Variable, int, error)
	// InsertVariable stores a variable and reports whether it did. False means the name was taken.
	InsertVariable(ctx context.Context, v Variable) (bool, error)
	UpsertVariable(ctx context.Context, v Variable) (created bool, err error)
	DeleteVariable(ctx context.Context, name string) error

	GetSecret(ctx context.Context, name string) (*Secret, error)
	ListSecrets(ctx context.Context, q listQuery) ([]Secret, int, error)
	// InsertSecret stores a secret and reports whether it did. False means the name was taken.
	InsertSecret(ctx context.Context, name, sealed, description string) (bool, error)
	UpsertSecret(ctx context.Context, name, sealed, description string) (created bool, err error)
	DeleteSecret(ctx context.Context, name string) error
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

func (s *store) DeleteVariable(ctx context.Context, name string) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteVariable, name, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to delete variable: %w", err)
	}
	return nil
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

func (s *store) InsertSecret(ctx context.Context, name, sealed, description string) (bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return false, err
	}
	inserted, err := dbClient.ExecuteContext(ctx, queryInsertSecret,
		name, sealed, description, s.scope(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to insert secret: %w", err)
	}
	return inserted > 0, nil
}

// UpsertSecret stores the secret when the name is free and rotates it when it is not, reporting
// which happened. The sealed value arrives ready to store; this never sees a plaintext one.
func (s *store) UpsertSecret(ctx context.Context, name, sealed, description string) (bool, error) {
	dbClient, err := s.client()
	if err != nil {
		return false, err
	}

	for attempt := 0; attempt < upsertAttempts; attempt++ {
		inserted, err := dbClient.ExecuteContext(ctx, queryUpsertSecret,
			name, sealed, description, s.scope(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to write secret: %w", err)
		}
		if inserted > 0 {
			return true, nil
		}

		updated, err := dbClient.ExecuteContext(ctx, queryUpdateSecret,
			name, sealed, description, s.scope(ctx))
		if err != nil {
			return false, fmt.Errorf("failed to write secret: %w", err)
		}
		if updated > 0 {
			return false, nil
		}
	}
	return false, fmt.Errorf("failed to write secret %q: it was deleted during each attempt", name)
}

func (s *store) DeleteSecret(ctx context.Context, name string) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteSecret, name, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}

// ListVariables returns one page of variables and the total the query matched.
func (s *store) ListVariables(ctx context.Context, q listQuery) ([]Variable, int, error) {
	return listPage(ctx, s, queryListVariables, q, variableFromRow, func(v Variable) string { return v.Name })
}

// ListSecrets returns one page of secret names and the total the query matched. No value is read.
func (s *store) ListSecrets(ctx context.Context, q listQuery) ([]Secret, int, error) {
	return listPage(ctx, s, queryListSecrets, q, secretFromRow, func(sec Secret) string { return sec.Name })
}

// listPage reads a collection, narrows it to what the query asked for, and returns one page of it
// with the total that matched.
//
// The page is taken after filtering rather than in SQL, because a names= lookup and a prefix match
// are not expressible as one indexable predicate without building the statement by string. The store
// is bounded by what a deployment can hold, so reading the collection and slicing it is honest here;
// if that stops being true it becomes a query, not a bigger loop.
func listPage[T any](
	ctx context.Context,
	s *store,
	query dbmodel.DBQuery,
	q listQuery,
	fromRow func(map[string]interface{}) T,
	nameOf func(T) string,
) ([]T, int, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, 0, err
	}
	rows, err := dbClient.QueryContext(ctx, query, s.scope(ctx))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list %s: %w", collectionName(query), err)
	}

	all := make([]T, 0, len(rows))
	for _, row := range rows {
		item := fromRow(row)
		if matches(nameOf(item), q) {
			all = append(all, item)
		}
	}
	sort.Slice(all, func(i, j int) bool { return nameOf(all[i]) < nameOf(all[j]) })

	total := len(all)
	start, end := pageBounds(total, q)
	return all[start:end], total, nil
}

// collectionName names the collection a list query reads, for an error message.
func collectionName(query dbmodel.DBQuery) string {
	if query.ID == queryListSecrets.ID {
		return "secrets"
	}
	return "variables"
}

// matches reports whether a name survives the query's narrowing.
func matches(name string, q listQuery) bool {
	if len(q.names) > 0 {
		found := false
		for _, want := range q.names {
			if want == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q.nameEquals != "" && name != q.nameEquals {
		return false
	}
	if q.namePrefix != "" && !strings.HasPrefix(name, q.namePrefix) {
		return false
	}
	return true
}

// pageBounds clamps offset and limit to what the slice actually holds, so an offset past the end is
// an empty page rather than a panic.
func pageBounds(total int, q listQuery) (int, int) {
	start := q.offset
	if start > total {
		start = total
	}
	end := start + q.limit
	if end > total {
		end = total
	}
	return start, end
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
	default:
		return fmt.Sprintf("%v", typed)
	}
}
