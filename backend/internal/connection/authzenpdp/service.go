// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"fmt"
	"net/url"
)

// Service provides storage operations for external AuthZEN PDP connections.
type Service struct {
	store Store
}

// NewService creates an AuthZEN PDP connection service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// ValidateEndpoint reports whether endpoint is an absolute HTTP or HTTPS URL.
func ValidateEndpoint(endpoint string) error {
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Host == "" ||
		(parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") {
		return fmt.Errorf("endpoint must be an absolute URL")
	}
	return nil
}

func validateConnection(connection AuthZENPDPConnection) error {
	if err := ValidateEndpoint(connection.Endpoint); err != nil {
		return fmt.Errorf("invalid access evaluation endpoint: %w", err)
	}
	if err := ValidateEndpoint(connection.BatchEndpoint); err != nil {
		return fmt.Errorf("invalid access evaluations endpoint: %w", err)
	}
	return nil
}

// Create stores an external AuthZEN PDP connection.
func (s *Service) Create(ctx context.Context, connection AuthZENPDPConnection) error {
	if err := validateConnection(connection); err != nil {
		return err
	}
	return s.store.Create(ctx, connection)
}

// List returns all external AuthZEN PDP connections.
func (s *Service) List(ctx context.Context) ([]AuthZENPDPConnection, error) {
	return s.store.List(ctx)
}

// Get returns an external AuthZEN PDP connection by ID.
func (s *Service) Get(ctx context.Context, id string) (*AuthZENPDPConnection, error) {
	return s.store.Get(ctx, id)
}

// Update replaces an external AuthZEN PDP connection.
func (s *Service) Update(ctx context.Context, id string, connection AuthZENPDPConnection) error {
	if err := validateConnection(connection); err != nil {
		return err
	}
	return s.store.Update(ctx, id, connection)
}

// Delete removes an external AuthZEN PDP connection.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}
