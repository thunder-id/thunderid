// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type serviceStoreStub struct {
	connection  AuthZENPDPConnection
	connections []AuthZENPDPConnection
	id          string
	createCalls int
	updateCalls int
	err         error
}

func (s *serviceStoreStub) Create(_ context.Context, connection AuthZENPDPConnection) error {
	s.createCalls++
	s.connection = connection
	return s.err
}

func (s *serviceStoreStub) List(context.Context) ([]AuthZENPDPConnection, error) {
	return s.connections, s.err
}

func (s *serviceStoreStub) Get(_ context.Context, id string) (*AuthZENPDPConnection, error) {
	s.id = id
	if s.err != nil {
		return nil, s.err
	}
	return &s.connection, nil
}

func (s *serviceStoreStub) Update(_ context.Context, id string, connection AuthZENPDPConnection) error {
	s.updateCalls++
	s.id = id
	s.connection = connection
	return s.err
}

func (s *serviceStoreStub) Delete(_ context.Context, id string) error {
	s.id = id
	return s.err
}

func TestServiceDelegatesStoreOperations(t *testing.T) {
	ctx := context.Background()
	store := &serviceStoreStub{
		connection:  AuthZENPDPConnection{ID: "pdp-1"},
		connections: []AuthZENPDPConnection{{ID: "pdp-1"}},
	}
	service := NewService(store)
	connection := AuthZENPDPConnection{
		ID:            "pdp-2",
		Name:          "External PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	}

	require.NoError(t, service.Create(ctx, connection))
	require.Equal(t, connection, store.connection)

	connections, err := service.List(ctx)
	require.NoError(t, err)
	require.Equal(t, store.connections, connections)

	result, err := service.Get(ctx, "pdp-1")
	require.NoError(t, err)
	require.Equal(t, "pdp-1", store.id)
	require.Equal(t, store.connection, *result)

	require.NoError(t, service.Update(ctx, "pdp-3", connection))
	require.Equal(t, "pdp-3", store.id)
	require.NoError(t, service.Delete(ctx, "pdp-4"))
	require.Equal(t, "pdp-4", store.id)
}

func TestServiceRejectsInvalidConnectionBeforePersistence(t *testing.T) {
	store := &serviceStoreStub{}
	service := NewService(store)
	connection := AuthZENPDPConnection{
		Name:          "External PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "ftp://pdp.example.com/evaluations",
	}

	require.Error(t, service.Create(context.Background(), connection))
	require.Error(t, service.Update(context.Background(), "pdp-1", connection))
	require.Zero(t, store.createCalls)
	require.Zero(t, store.updateCalls)
}
