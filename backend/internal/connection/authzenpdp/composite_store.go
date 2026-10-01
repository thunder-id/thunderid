// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"errors"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// ErrAuthZENPDPIsImmutable is returned when attempting to modify a declarative AuthZEN PDP connection.
var ErrAuthZENPDPIsImmutable = errors.New("AuthZEN PDP connection is immutable")

// ErrAuthZENPDPResultLimitExceeded reports that a composite list exceeds its record cap.
var ErrAuthZENPDPResultLimitExceeded = errors.New("AuthZEN PDP result limit exceeded in composite mode")

// compositeStore reads declarative and mutable connections, while writing only to the mutable store.
type compositeStore struct{ fileStore, dbStore authZENPDPStoreInterface }

// newCompositeStore creates a store that reads from the database before the declarative store.
func newCompositeStore(fileStore, dbStore authZENPDPStoreInterface) authZENPDPStoreInterface {
	return &compositeStore{fileStore: fileStore, dbStore: dbStore}
}

// CreateAuthZENPDP writes mutable connections to the database store.
func (s *compositeStore) CreateAuthZENPDP(ctx context.Context, connection AuthZENPDPConnection) error {
	return s.dbStore.CreateAuthZENPDP(ctx, connection)
}

// UpdateAuthZENPDP rejects declarative connections and updates mutable connections.
func (s *compositeStore) UpdateAuthZENPDP(ctx context.Context, id string, connection AuthZENPDPConnection) error {
	return declarativeresource.CompositeUpdateHelper(
		connection,
		func(AuthZENPDPConnection) string { return id },
		func(id string) (bool, error) {
			found, err := s.fileStore.GetAuthZENPDP(ctx, id)
			return found != nil, err
		},
		func(connection AuthZENPDPConnection) error {
			return s.dbStore.UpdateAuthZENPDP(ctx, id, connection)
		},
		ErrAuthZENPDPIsImmutable,
	)
}

// DeleteAuthZENPDP rejects declarative connections and deletes mutable connections.
func (s *compositeStore) DeleteAuthZENPDP(ctx context.Context, id string) error {
	return declarativeresource.CompositeDeleteHelper(
		id,
		func(id string) (bool, error) {
			found, err := s.fileStore.GetAuthZENPDP(ctx, id)
			return found != nil, err
		},
		func(id string) error { return s.dbStore.DeleteAuthZENPDP(ctx, id) },
		ErrAuthZENPDPIsImmutable,
	)
}

// GetAuthZENPDP retrieves a connection by ID, preferring the database store.
func (s *compositeStore) GetAuthZENPDP(ctx context.Context, id string) (*AuthZENPDPConnection, error) {
	connection, err := s.dbStore.GetAuthZENPDP(ctx, id)
	if err != nil {
		return nil, err
	}
	if connection != nil {
		return connection, nil
	}
	return s.fileStore.GetAuthZENPDP(ctx, id)
}

// GetAuthZENPDPByName retrieves a connection by name, preferring the database store.
func (s *compositeStore) GetAuthZENPDPByName(ctx context.Context, name string) (*AuthZENPDPConnection, error) {
	connection, err := s.dbStore.GetAuthZENPDPByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if connection != nil {
		return connection, nil
	}
	return s.fileStore.GetAuthZENPDPByName(ctx, name)
}

// ListAuthZENPDPs merges database and declarative connections, preferring database entries by ID.
func (s *compositeStore) ListAuthZENPDPs(ctx context.Context) ([]AuthZENPDPConnection, error) {
	dbCount, err := s.dbStore.CountAuthZENPDPs(ctx)
	if err != nil {
		return nil, err
	}
	fileCount, err := s.fileStore.CountAuthZENPDPs(ctx)
	if err != nil {
		return nil, err
	}
	connections, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return dbCount, nil },
		func() (int, error) { return fileCount, nil },
		func(int) ([]AuthZENPDPConnection, error) { return s.dbStore.ListAuthZENPDPs(ctx) },
		func(int) ([]AuthZENPDPConnection, error) { return s.fileStore.ListAuthZENPDPs(ctx) },
		mergeAuthZENPDPConnections,
		dbCount+fileCount,
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, ErrAuthZENPDPResultLimitExceeded
	}
	return connections, nil
}

// CountAuthZENPDPs returns the combined count from both stores.
func (s *compositeStore) CountAuthZENPDPs(ctx context.Context) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return s.dbStore.CountAuthZENPDPs(ctx) },
		func() (int, error) { return s.fileStore.CountAuthZENPDPs(ctx) },
	)
}

// mergeAuthZENPDPConnections combines mutable and declarative connections by ID.
func mergeAuthZENPDPConnections(db, file []AuthZENPDPConnection) []AuthZENPDPConnection {
	result := make([]AuthZENPDPConnection, 0, len(db)+len(file))
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
