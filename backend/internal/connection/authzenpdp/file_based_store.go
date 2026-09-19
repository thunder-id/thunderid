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

func newFileBasedStore() Store {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeAuthZENPDP),
	}
}

func (s *fileBasedStore) CreateAuthZENPDP(ctx context.Context, connection AuthZENPDPConnection) error {
	return s.Create(connection.ID, &connection)
}

func (s *fileBasedStore) Create(id string, data interface{}) error {
	connection, ok := data.(*AuthZENPDPConnection)
	if !ok {
		return errors.New("invalid AuthZEN PDP connection data")
	}
	return s.GenericFileBasedStore.Create(id, connection)
}

func (s *fileBasedStore) ListAuthZENPDPs(context.Context) ([]AuthZENPDPConnection, error) {
	items, err := s.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}
	result := make([]AuthZENPDPConnection, 0, len(items))
	for _, item := range items {
		if connection, ok := item.Data.(*AuthZENPDPConnection); ok {
			result = append(result, *connection)
		}
	}
	return result, nil
}

func (s *fileBasedStore) GetAuthZENPDP(_ context.Context, id string) (*AuthZENPDPConnection, error) {
	item, err := s.GenericFileBasedStore.Get(id)
	if err != nil {
		return nil, nil
	}
	connection, ok := item.(*AuthZENPDPConnection)
	if !ok {
		return nil, errors.New("invalid AuthZEN PDP connection data")
	}
	return connection, nil
}

func (s *fileBasedStore) GetAuthZENPDPByName(_ context.Context, name string) (*AuthZENPDPConnection, error) {
	item, err := s.GenericFileBasedStore.GetByField(name, func(data interface{}) string {
		return data.(*AuthZENPDPConnection).Name
	})
	if err != nil {
		return nil, nil
	}
	connection, ok := item.(*AuthZENPDPConnection)
	if !ok {
		return nil, errors.New("invalid AuthZEN PDP connection data")
	}
	return connection, nil
}

func (s *fileBasedStore) UpdateAuthZENPDP(context.Context, string, AuthZENPDPConnection) error {
	return errors.New("AuthZEN PDP connection is immutable")
}
func (s *fileBasedStore) DeleteAuthZENPDP(context.Context, string) error {
	return errors.New("AuthZEN PDP connection is immutable")
}
