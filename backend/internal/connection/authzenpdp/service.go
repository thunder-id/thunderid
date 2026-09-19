// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// AuthZENPDPServiceInterface defines AuthZEN PDP connection management operations.
type AuthZENPDPServiceInterface interface {
	FromRequest(request ConnectionRequest) AuthZENPDPConnection
	CreateAuthZENPDPConnection(
		ctx context.Context,
		connection AuthZENPDPConnection,
	) (*AuthZENPDPConnection, *tidcommon.ServiceError)
	UpdateAuthZENPDPConnection(
		ctx context.Context,
		id string,
		connection AuthZENPDPConnection,
	) (*AuthZENPDPConnection, *tidcommon.ServiceError)
	CreateAuthZENPDP(ctx context.Context, connection AuthZENPDPConnection) error
	CreateDeclarativeAuthZENPDP(connection AuthZENPDPConnection) error
	ListAuthZENPDPs(ctx context.Context) ([]AuthZENPDPConnection, error)
	GetAuthZENPDP(ctx context.Context, id string) (*AuthZENPDPConnection, error)
	UpdateAuthZENPDP(ctx context.Context, id string, connection AuthZENPDPConnection) error
	DeleteAuthZENPDP(ctx context.Context, id string) error
}

// AuthZENPDPService provides storage operations for AuthZEN PDP connections.
type AuthZENPDPService struct {
	store    Store
	defaults config.AuthZENPDPConfig
}

var _ AuthZENPDPServiceInterface = (*AuthZENPDPService)(nil)

// NewAuthZENPDPService creates an AuthZEN PDP connection service.
func NewAuthZENPDPService(store Store, defaults config.AuthZENPDPConfig) *AuthZENPDPService {
	return &AuthZENPDPService{store: store, defaults: defaults}
}

// CreateDeclarativeAuthZENPDP stores a declarative connection in the file-backed store.
func (s *AuthZENPDPService) CreateDeclarativeAuthZENPDP(connection AuthZENPDPConnection) error {
	if declarative, ok := s.store.(*compositeStore); ok {
		return declarative.fileStore.CreateAuthZENPDP(context.Background(), connection)
	}
	if _, ok := s.store.(*fileBasedStore); ok {
		return s.store.CreateAuthZENPDP(context.Background(), connection)
	}
	// Custom stores used by callers and tests own their persistence semantics.
	return s.store.CreateAuthZENPDP(context.Background(), connection)
}

// FromRequest converts an API request into an AuthZEN PDP connection.
func (s *AuthZENPDPService) FromRequest(request ConnectionRequest) AuthZENPDPConnection {
	return fromRequest(request)
}

// ApplyDefaults fills unset connection-level timeout and retry settings.
func (s *AuthZENPDPService) ApplyDefaults(connection *AuthZENPDPConnection) {
	if connection.TimeoutMS <= 0 {
		connection.TimeoutMS = s.defaults.TimeoutMS
	}
	if connection.RetryCount < 0 {
		connection.RetryCount = retryCountDefault(s.defaults)
	}
}

// CreateAuthZENPDPConnection validates and stores an AuthZEN PDP connection.
func (s *AuthZENPDPService) CreateAuthZENPDPConnection(
	ctx context.Context,
	connection AuthZENPDPConnection,
) (*AuthZENPDPConnection, *tidcommon.ServiceError) {
	if svcErr := declarativeresource.CheckDeclarativeCreate(); svcErr != nil {
		return nil, svcErr
	}
	if err := normalizeEndpoints(&connection); err != nil {
		return nil, &ErrorInvalidEndpoint
	}
	s.ApplyDefaults(&connection)
	existing, err := s.GetAuthZENPDPByName(ctx, connection.Name)
	if err != nil {
		return nil, &tidcommon.InternalServerError
	}
	if existing != nil {
		return nil, &ErrorAlreadyExists
	}
	connection.ID = sysutils.GenerateUUID()
	if err := s.store.CreateAuthZENPDP(ctx, connection); err != nil {
		return nil, &tidcommon.InternalServerError
	}
	return &connection, nil
}

// UpdateAuthZENPDPConnection validates and replaces an AuthZEN PDP connection.
func (s *AuthZENPDPService) UpdateAuthZENPDPConnection(
	ctx context.Context,
	id string,
	connection AuthZENPDPConnection,
) (*AuthZENPDPConnection, *tidcommon.ServiceError) {
	if svcErr := declarativeresource.CheckDeclarativeUpdate(); svcErr != nil {
		return nil, svcErr
	}
	current, err := s.GetAuthZENPDP(ctx, id)
	if err != nil {
		return nil, &tidcommon.InternalServerError
	}
	if current == nil {
		return nil, &ErrorNotFound
	}
	if err := normalizeEndpoints(&connection); err != nil {
		return nil, &ErrorInvalidEndpoint
	}
	s.ApplyDefaults(&connection)
	existing, err := s.GetAuthZENPDPByName(ctx, connection.Name)
	if err != nil {
		return nil, &tidcommon.InternalServerError
	}
	if existing != nil && existing.ID != id {
		return nil, &ErrorAlreadyExists
	}
	if err := s.store.UpdateAuthZENPDP(ctx, id, connection); err != nil {
		return nil, &tidcommon.InternalServerError
	}
	updated, err := s.GetAuthZENPDP(ctx, id)
	if err != nil || updated == nil {
		return nil, &tidcommon.InternalServerError
	}
	return updated, nil
}

// CreateAuthZENPDP stores an AuthZEN PDP connection.
func (s *AuthZENPDPService) CreateAuthZENPDP(ctx context.Context, connection AuthZENPDPConnection) error {
	s.ApplyDefaults(&connection)
	if err := normalizeEndpoints(&connection); err != nil {
		return err
	}
	return s.store.CreateAuthZENPDP(ctx, connection)
}

// ListAuthZENPDPs returns all AuthZEN PDP connections.
func (s *AuthZENPDPService) ListAuthZENPDPs(ctx context.Context) ([]AuthZENPDPConnection, error) {
	connections, err := s.store.ListAuthZENPDPs(ctx)
	if err != nil {
		return nil, err
	}
	for i := range connections {
		s.ApplyDefaults(&connections[i])
	}
	return connections, nil
}

// GetAuthZENPDP returns an AuthZEN PDP connection by ID.
func (s *AuthZENPDPService) GetAuthZENPDP(ctx context.Context, id string) (*AuthZENPDPConnection, error) {
	connection, err := s.store.GetAuthZENPDP(ctx, id)
	if err != nil || connection == nil {
		return connection, err
	}
	s.ApplyDefaults(connection)
	return connection, nil
}

// GetAuthZENPDPByName returns an AuthZEN PDP connection by name.
func (s *AuthZENPDPService) GetAuthZENPDPByName(
	ctx context.Context, name string,
) (*AuthZENPDPConnection, error) {
	connection, err := s.store.GetAuthZENPDPByName(ctx, name)
	if err != nil || connection == nil {
		return connection, err
	}
	s.ApplyDefaults(connection)
	return connection, nil
}

// UpdateAuthZENPDP replaces an AuthZEN PDP connection.
func (s *AuthZENPDPService) UpdateAuthZENPDP(
	ctx context.Context, id string, connection AuthZENPDPConnection,
) error {
	s.ApplyDefaults(&connection)
	if err := normalizeEndpoints(&connection); err != nil {
		return err
	}
	return s.store.UpdateAuthZENPDP(ctx, id, connection)
}

// DeleteAuthZENPDP removes an AuthZEN PDP connection.
func (s *AuthZENPDPService) DeleteAuthZENPDP(ctx context.Context, id string) error {
	return s.store.DeleteAuthZENPDP(ctx, id)
}

func retryCountDefault(defaults config.AuthZENPDPConfig) int {
	if defaults.RetryCount == nil {
		return 0
	}
	return *defaults.RetryCount
}

func normalizeEndpoints(connection *AuthZENPDPConnection) error {
	if connection == nil {
		return fmt.Errorf("connection is required")
	}
	connection.Endpoint = strings.TrimSpace(connection.Endpoint)
	connection.BatchEndpoint = strings.TrimSpace(connection.BatchEndpoint)
	return validateConnection(*connection)
}

func validateConnection(connection AuthZENPDPConnection) error {
	if err := validateEndpoint(connection.Endpoint); err != nil {
		return fmt.Errorf("invalid access evaluation endpoint: %w", err)
	}
	if err := validateEndpoint(connection.BatchEndpoint); err != nil {
		return fmt.Errorf("invalid access evaluations endpoint: %w", err)
	}
	return nil
}

func validateEndpoint(endpoint string) error {
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Host == "" ||
		(parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") {
		return fmt.Errorf("endpoint must be an absolute URL")
	}
	return nil
}
