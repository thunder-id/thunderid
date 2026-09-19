// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type serviceStoreStub struct {
	connection  AuthZENPDPConnection
	connections []AuthZENPDPConnection
	id          string
	createCalls int
	updateCalls int
	err         error
	missing     bool
}

func (s *serviceStoreStub) CreateAuthZENPDP(_ context.Context, connection AuthZENPDPConnection) error {
	s.createCalls++
	s.connection = connection
	return s.err
}

func (s *serviceStoreStub) ListAuthZENPDPs(context.Context) ([]AuthZENPDPConnection, error) {
	return s.connections, s.err
}

func (s *serviceStoreStub) GetAuthZENPDP(_ context.Context, id string) (*AuthZENPDPConnection, error) {
	s.id = id
	if s.err != nil {
		return nil, s.err
	}
	if s.missing {
		return nil, nil
	}
	return &s.connection, nil
}

func (s *serviceStoreStub) GetAuthZENPDPByName(
	_ context.Context, name string,
) (*AuthZENPDPConnection, error) {
	if s.connection.Name == name {
		return &s.connection, nil
	}
	return nil, nil
}

func (s *serviceStoreStub) UpdateAuthZENPDP(
	_ context.Context, id string, connection AuthZENPDPConnection,
) error {
	s.updateCalls++
	s.id = id
	s.connection = connection
	return s.err
}

func (s *serviceStoreStub) DeleteAuthZENPDP(_ context.Context, id string) error {
	s.id = id
	return s.err
}

func TestServiceDelegatesStoreOperations(t *testing.T) {
	ctx := context.Background()
	store := &serviceStoreStub{
		connection:  AuthZENPDPConnection{ID: "pdp-1"},
		connections: []AuthZENPDPConnection{{ID: "pdp-1"}},
	}
	service := NewAuthZENPDPService(store, config.AuthZENPDPConfig{})
	connection := AuthZENPDPConnection{
		ID:            "pdp-2",
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	}

	require.NoError(t, service.CreateAuthZENPDP(ctx, connection))
	require.Equal(t, connection, store.connection)

	connections, err := service.ListAuthZENPDPs(ctx)
	require.NoError(t, err)
	require.Equal(t, store.connections, connections)

	result, err := service.GetAuthZENPDP(ctx, "pdp-1")
	require.NoError(t, err)
	require.Equal(t, "pdp-1", store.id)
	require.Equal(t, store.connection, *result)

	require.NoError(t, service.UpdateAuthZENPDP(ctx, "pdp-3", connection))
	require.Equal(t, "pdp-3", store.id)
	require.NoError(t, service.DeleteAuthZENPDP(ctx, "pdp-4"))
	require.Equal(t, "pdp-4", store.id)
}

func TestServiceAppliesDefaultsToWritesAndReads(t *testing.T) {
	retries := 2
	store := &serviceStoreStub{
		connection:  AuthZENPDPConnection{ID: "pdp-1", TimeoutMS: -1, RetryCount: -1},
		connections: []AuthZENPDPConnection{{ID: "pdp-2", TimeoutMS: -1, RetryCount: -1}},
	}
	service := NewAuthZENPDPService(store, config.AuthZENPDPConfig{TimeoutMS: 1200, RetryCount: &retries})

	connection := service.FromRequest(ConnectionRequest{
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	})
	require.NoError(t, service.CreateAuthZENPDP(context.Background(), connection))
	require.Equal(t, 1200, store.connection.TimeoutMS)
	require.Equal(t, 2, store.connection.RetryCount)

	found, err := service.GetAuthZENPDP(context.Background(), "pdp-1")
	require.NoError(t, err)
	require.Equal(t, 1200, found.TimeoutMS)
	require.Equal(t, 2, found.RetryCount)

	connections, err := service.ListAuthZENPDPs(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1200, connections[0].TimeoutMS)
	require.Equal(t, 2, connections[0].RetryCount)
}

func TestCreateConnectionReturnsAppliedDefaults(t *testing.T) {
	retries := 2
	store := &serviceStoreStub{}
	service := NewAuthZENPDPService(store, config.AuthZENPDPConfig{TimeoutMS: 1200, RetryCount: &retries})

	created, svcErr := service.CreateAuthZENPDPConnection(context.Background(), AuthZENPDPConnection{
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
		RetryCount:    -1,
	})

	require.Nil(t, svcErr)
	require.Equal(t, 1200, created.TimeoutMS)
	require.Equal(t, 2, created.RetryCount)
	require.Equal(t, *created, store.connection)
}

func TestServiceRejectsInvalidConnectionBeforePersistence(t *testing.T) {
	store := &serviceStoreStub{}
	service := NewAuthZENPDPService(store, config.AuthZENPDPConfig{})
	connection := AuthZENPDPConnection{
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "ftp://pdp.example.com/evaluations",
	}

	require.Error(t, service.CreateAuthZENPDP(context.Background(), connection))
	require.Error(t, service.UpdateAuthZENPDP(context.Background(), "pdp-1", connection))
	require.Zero(t, store.createCalls)
	require.Zero(t, store.updateCalls)
}

func TestUpdateConnectionRejectsMissingConnection(t *testing.T) {
	store := &serviceStoreStub{missing: true}
	service := NewAuthZENPDPService(store, config.AuthZENPDPConfig{})

	updated, svcErr := service.UpdateAuthZENPDPConnection(context.Background(), "pdp-1", AuthZENPDPConnection{
		Name:          "AuthZEN PDP",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	})

	require.Nil(t, updated)
	require.Equal(t, ErrorNotFound.Code, svcErr.Code)
	require.Zero(t, store.updateCalls)
}
