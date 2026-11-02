// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeStore reads declarative and mutable connections, while writing only to the mutable store.
type compositeStore struct {
	fileStore httpConnectionStoreInterface
	dbStore   httpConnectionStoreInterface
}

// newCompositeStore creates a store that reads from the database before the declarative store.
func newCompositeStore(fileStore httpConnectionStoreInterface,
	dbStore httpConnectionStoreInterface) httpConnectionStoreInterface {
	return &compositeStore{fileStore: fileStore, dbStore: dbStore}
}

// CreateHTTPConnection writes a mutable connection to the database store. An ID a declarative
// connection has is refused, since the database entry would shadow it on every read.
func (s *compositeStore) CreateHTTPConnection(ctx context.Context, connection HTTPConnection) error {
	declared, err := s.fileStore.GetHTTPConnection(ctx, connection.ID)
	if err != nil {
		return err
	}
	if declared != nil {
		return errIDConflict
	}
	if err := s.checkDeclarativeName(ctx, connection); err != nil {
		return err
	}
	return s.dbStore.CreateHTTPConnection(ctx, connection)
}

// UpdateHTTPConnection rejects declarative connections and updates mutable connections.
func (s *compositeStore) UpdateHTTPConnection(ctx context.Context, connection HTTPConnection) error {
	return declarativeresource.CompositeUpdateHelper(
		connection,
		func(connection HTTPConnection) string { return connection.ID },
		s.isDeclarative(ctx),
		func(connection HTTPConnection) error {
			if err := s.checkDeclarativeName(ctx, connection); err != nil {
				return err
			}
			return s.dbStore.UpdateHTTPConnection(ctx, connection)
		},
		errHTTPConnectionIsImmutable,
	)
}

// DeleteHTTPConnection rejects declarative connections and deletes mutable connections.
func (s *compositeStore) DeleteHTTPConnection(ctx context.Context, id string) error {
	return declarativeresource.CompositeDeleteHelper(
		id,
		s.isDeclarative(ctx),
		func(id string) error { return s.dbStore.DeleteHTTPConnection(ctx, id) },
		errHTTPConnectionIsImmutable,
	)
}

// GetHTTPConnection retrieves a connection by ID, preferring the database store.
func (s *compositeStore) GetHTTPConnection(ctx context.Context, id string) (*HTTPConnection, error) {
	connection, err := s.dbStore.GetHTTPConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if connection != nil {
		return connection, nil
	}
	return s.fileStore.GetHTTPConnection(ctx, id)
}

// GetHTTPConnectionByName retrieves a connection by name, preferring the database store.
func (s *compositeStore) GetHTTPConnectionByName(ctx context.Context, name string) (*HTTPConnection, error) {
	connection, err := s.dbStore.GetHTTPConnectionByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if connection != nil {
		return connection, nil
	}
	return s.fileStore.GetHTTPConnectionByName(ctx, name)
}

// ListHTTPConnections merges database and declarative connections, preferring database entries by ID.
func (s *compositeStore) ListHTTPConnections(ctx context.Context) ([]HTTPConnection, error) {
	dbCount, err := s.dbStore.CountHTTPConnections(ctx)
	if err != nil {
		return nil, err
	}
	fileCount, err := s.fileStore.CountHTTPConnections(ctx)
	if err != nil {
		return nil, err
	}
	connections, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return dbCount, nil },
		func() (int, error) { return fileCount, nil },
		func(int) ([]HTTPConnection, error) { return s.dbStore.ListHTTPConnections(ctx) },
		func(int) ([]HTTPConnection, error) { return s.fileStore.ListHTTPConnections(ctx) },
		mergeHTTPConnections,
		dbCount+fileCount,
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}
	return connections, nil
}

// CountHTTPConnections returns the combined count from both stores.
func (s *compositeStore) CountHTTPConnections(ctx context.Context) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return s.dbStore.CountHTTPConnections(ctx) },
		func() (int, error) { return s.fileStore.CountHTTPConnections(ctx) },
	)
}

// isDeclarative returns a check of whether an ID belongs to a declarative connection.
func (s *compositeStore) isDeclarative(ctx context.Context) func(string) (bool, error) {
	return func(id string) (bool, error) {
		found, err := s.fileStore.GetHTTPConnection(ctx, id)
		return found != nil, err
	}
}

// checkDeclarativeName returns errNameConflict when a declarative connection already has the name.
// The database's unique index covers only the mutable connections.
func (s *compositeStore) checkDeclarativeName(ctx context.Context, connection HTTPConnection) error {
	declared, err := s.fileStore.GetHTTPConnectionByName(ctx, connection.Name)
	if err != nil {
		return err
	}
	if declared != nil && declared.ID != connection.ID {
		return errNameConflict
	}
	return nil
}

// mergeHTTPConnections combines mutable and declarative connections by ID.
func mergeHTTPConnections(db, file []HTTPConnection) []HTTPConnection {
	result := make([]HTTPConnection, 0, len(db)+len(file))
	seen := make(map[string]struct{}, len(db)+len(file))
	for _, connection := range db {
		if _, exists := seen[connection.ID]; exists {
			continue
		}
		seen[connection.ID] = struct{}{}
		connection.IsReadOnly = false
		result = append(result, connection)
	}
	for _, connection := range file {
		if _, exists := seen[connection.ID]; exists {
			continue
		}
		seen[connection.ID] = struct{}{}
		connection.IsReadOnly = true
		result = append(result, connection)
	}
	return result
}
