// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"errors"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type fileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedStore creates an immutable store for declarative AuthZEN PDP connections.
func newFileBasedStore() authZENPDPStoreInterface {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeAuthZENPDP),
	}
}

// CreateAuthZENPDP stores a declarative AuthZEN PDP connection in memory.
func (s *fileBasedStore) CreateAuthZENPDP(_ context.Context, connection AuthZENPDPConnection) error {
	return s.Create(connection.ID, &connection)
}

// Create implements declarativeresource.Storer for AuthZEN PDP connection loading.
func (s *fileBasedStore) Create(id string, data interface{}) error {
	connection, ok := data.(*AuthZENPDPConnection)
	if !ok {
		return errors.New("invalid AuthZEN PDP connection data")
	}
	return s.GenericFileBasedStore.Create(id, connection)
}

// ListAuthZENPDPs returns every declarative AuthZEN PDP connection.
func (s *fileBasedStore) ListAuthZENPDPs(context.Context) ([]AuthZENPDPConnection, error) {
	items, err := s.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}
	result := make([]AuthZENPDPConnection, 0, len(items))
	for _, item := range items {
		if connection, ok := item.Data.(*AuthZENPDPConnection); ok {
			connectionCopy := *connection
			connectionCopy.IsReadOnly = true
			result = append(result, connectionCopy)
		}
	}
	return result, nil
}

// CountAuthZENPDPs returns the number of declarative connections.
func (s *fileBasedStore) CountAuthZENPDPs(context.Context) (int, error) {
	return s.GenericFileBasedStore.Count()
}

// GetAuthZENPDP retrieves a declarative AuthZEN PDP connection by ID.
func (s *fileBasedStore) GetAuthZENPDP(_ context.Context, id string) (*AuthZENPDPConnection, error) {
	item, err := s.GenericFileBasedStore.Get(id)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, nil
		}
		return nil, err
	}
	connection, ok := item.(*AuthZENPDPConnection)
	if !ok {
		return nil, errors.New("invalid AuthZEN PDP connection data")
	}
	connectionCopy := *connection
	connectionCopy.IsReadOnly = true
	return &connectionCopy, nil
}

// GetAuthZENPDPByName retrieves a declarative AuthZEN PDP connection by name.
func (s *fileBasedStore) GetAuthZENPDPByName(_ context.Context, name string) (*AuthZENPDPConnection, error) {
	item, err := s.GenericFileBasedStore.GetByField(name, func(data interface{}) string {
		return data.(*AuthZENPDPConnection).Name
	})
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, nil
		}
		return nil, err
	}
	connection, ok := item.(*AuthZENPDPConnection)
	if !ok {
		return nil, errors.New("invalid AuthZEN PDP connection data")
	}
	connectionCopy := *connection
	connectionCopy.IsReadOnly = true
	return &connectionCopy, nil
}

// UpdateAuthZENPDP rejects updates because declarative connections are immutable.
func (s *fileBasedStore) UpdateAuthZENPDP(context.Context, string, AuthZENPDPConnection) error {
	return ErrAuthZENPDPIsImmutable
}

// DeleteAuthZENPDP rejects deletes because declarative connections are immutable.
func (s *fileBasedStore) DeleteAuthZENPDP(context.Context, string) error {
	return ErrAuthZENPDPIsImmutable
}
