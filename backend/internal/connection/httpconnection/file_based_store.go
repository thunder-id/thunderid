// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"errors"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// fileStoreInterface is the file store as the composite store reads it, with the name lookup the
// database store leaves to its unique index.
type fileStoreInterface interface {
	httpConnectionStoreInterface
	GetHTTPConnectionByName(ctx context.Context, name string) (*HTTPConnection, error)
}

// fileBasedStore holds the HTTP connections loaded from declarative files. The connection loader fills
// it at startup; at runtime it is read-only.
type fileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedStore creates an immutable store for declarative HTTP connections.
func newFileBasedStore() fileStoreInterface {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeHTTPConnection),
	}
}

// CreateHTTPConnection rejects runtime creates because declarative connections are immutable.
func (s *fileBasedStore) CreateHTTPConnection(context.Context, HTTPConnection) error {
	return errHTTPConnectionIsImmutable
}

// ListHTTPConnections returns every declarative HTTP connection.
func (s *fileBasedStore) ListHTTPConnections(context.Context) ([]HTTPConnection, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	result := make([]HTTPConnection, 0, len(items))
	for _, item := range items {
		if connection, ok := item.Data.(*HTTPConnection); ok {
			connectionCopy := *connection
			connectionCopy.IsReadOnly = true
			result = append(result, connectionCopy)
		}
	}
	return result, nil
}

// CountHTTPConnections returns the number of declarative HTTP connections.
func (s *fileBasedStore) CountHTTPConnections(context.Context) (int, error) {
	return s.Count()
}

// GetHTTPConnection retrieves a declarative HTTP connection by ID, or nil when it does not exist.
func (s *fileBasedStore) GetHTTPConnection(_ context.Context, id string) (*HTTPConnection, error) {
	item, err := s.Get(id)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, nil
		}
		return nil, err
	}
	connection, ok := item.(*HTTPConnection)
	if !ok {
		return nil, errInvalidDeclarativeData
	}
	connectionCopy := *connection
	connectionCopy.IsReadOnly = true
	return &connectionCopy, nil
}

// GetHTTPConnectionByName retrieves a declarative HTTP connection by name, or nil when none has it.
func (s *fileBasedStore) GetHTTPConnectionByName(_ context.Context, name string) (*HTTPConnection, error) {
	item, err := s.GetByField(name, func(data interface{}) string {
		connection, ok := data.(*HTTPConnection)
		if !ok {
			return ""
		}
		return connection.Name
	})
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, nil
		}
		return nil, err
	}
	connection, ok := item.(*HTTPConnection)
	if !ok {
		return nil, errInvalidDeclarativeData
	}
	connectionCopy := *connection
	connectionCopy.IsReadOnly = true
	return &connectionCopy, nil
}

// UpdateHTTPConnection rejects updates because declarative connections are immutable.
func (s *fileBasedStore) UpdateHTTPConnection(context.Context, HTTPConnection) error {
	return errHTTPConnectionIsImmutable
}

// DeleteHTTPConnection rejects deletes because declarative connections are immutable.
func (s *fileBasedStore) DeleteHTTPConnection(context.Context, string) error {
	return errHTTPConnectionIsImmutable
}
