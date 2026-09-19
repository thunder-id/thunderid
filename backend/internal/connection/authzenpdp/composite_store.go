// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authzenpdp provides AuthZEN PDP connection storage and configuration.
package authzenpdp

import (
	"context"
)

type compositeStore struct{ fileStore, dbStore Store }

func newCompositeStore(fileStore, dbStore Store) Store {
	return &compositeStore{fileStore: fileStore, dbStore: dbStore}
}

func (s *compositeStore) CreateAuthZENPDP(ctx context.Context, c AuthZENPDPConnection) error {
	return s.dbStore.CreateAuthZENPDP(ctx, c)
}
func (s *compositeStore) UpdateAuthZENPDP(ctx context.Context, id string, c AuthZENPDPConnection) error {
	return s.dbStore.UpdateAuthZENPDP(ctx, id, c)
}
func (s *compositeStore) DeleteAuthZENPDP(ctx context.Context, id string) error {
	return s.dbStore.DeleteAuthZENPDP(ctx, id)
}

func (s *compositeStore) GetAuthZENPDP(ctx context.Context, id string) (*AuthZENPDPConnection, error) {
	c, err := s.dbStore.GetAuthZENPDP(ctx, id)
	if err != nil {
		return nil, err
	}
	if c != nil {
		return c, nil
	}
	return s.fileStore.GetAuthZENPDP(ctx, id)
}

func (s *compositeStore) GetAuthZENPDPByName(ctx context.Context, name string) (*AuthZENPDPConnection, error) {
	c, err := s.dbStore.GetAuthZENPDPByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if c != nil {
		return c, nil
	}
	return s.fileStore.GetAuthZENPDPByName(ctx, name)
}

func (s *compositeStore) ListAuthZENPDPs(ctx context.Context) ([]AuthZENPDPConnection, error) {
	db, err := s.dbStore.ListAuthZENPDPs(ctx)
	if err != nil {
		return nil, err
	}
	file, err := s.fileStore.ListAuthZENPDPs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]AuthZENPDPConnection, 0, len(db)+len(file))
	seen := make(map[string]struct{}, len(db)+len(file))
	for _, c := range append(db, file...) {
		if _, exists := seen[c.ID]; exists {
			continue
		}
		seen[c.ID] = struct{}{}
		result = append(result, c)
	}
	return result, nil
}
