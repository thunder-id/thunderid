// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"strings"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// AuthZENPDPRuntimeConfig contains runtime settings for an AuthZEN PDP connection.
type AuthZENPDPRuntimeConfig = authzenpdp.AuthZENPDPRuntimeConfig

// AuthZENPDPSubjectAttributeMapping maps user-type attributes to PDP attributes.
type AuthZENPDPSubjectAttributeMapping = authzenpdp.SubjectAttributeMapping

// ListAuthZENPDPRuntimeConfigs returns saved external PDP configurations for authorization routing.
func ListAuthZENPDPRuntimeConfigs(ctx context.Context) ([]authzenpdp.AuthZENPDPRuntimeConfig, error) {
	return authzenpdp.ListAuthZENPDPRuntimeConfigs(ctx)
}

// GetAuthZENPDPRuntimeConfig returns a saved external PDP configuration for authorization routing.
var GetAuthZENPDPRuntimeConfig = authzenpdp.GetAuthZENPDPRuntimeConfig

// authZENPDPConnectionRequest aliases the AuthZEN PDP connection API request model.
type authZENPDPConnectionRequest = authzenpdp.ConnectionRequest

// authZENPDPConnectionResponse aliases the AuthZEN PDP connection API response model.
type authZENPDPConnectionResponse = authzenpdp.ConnectionResponse

// authZENPDPConnection aliases the persisted AuthZEN PDP connection model.
type authZENPDPConnection = authzenpdp.AuthZENPDPConnection

// authZENPDPSubjectAttributeMapping aliases the AuthZEN PDP subject mapping model.
type authZENPDPSubjectAttributeMapping = authzenpdp.SubjectAttributeMapping

// authZENPDPStoreInterface defines the connection store operations used by the connection handler.
type authZENPDPStoreInterface interface {
	create(ctx context.Context, connection authZENPDPConnection) error
	list(ctx context.Context) ([]authZENPDPConnection, error)
	get(ctx context.Context, id string) (*authZENPDPConnection, error)
	update(ctx context.Context, id string, connection authZENPDPConnection) error
	delete(ctx context.Context, id string) error
}

// authZENPDPStoreAdapter adapts the AuthZEN PDP package service to the connection package interface.
type authZENPDPStoreAdapter struct {
	service *authzenpdp.Service
}

func newAuthZENPDPStore() authZENPDPStoreInterface {
	return &authZENPDPStoreAdapter{service: authzenpdp.NewService(authzenpdp.NewStore())}
}

func (s *authZENPDPStoreAdapter) create(ctx context.Context, connection authZENPDPConnection) error {
	return s.service.Create(ctx, connection)
}

func (s *authZENPDPStoreAdapter) list(ctx context.Context) ([]authZENPDPConnection, error) {
	return s.service.List(ctx)
}

func (s *authZENPDPStoreAdapter) get(ctx context.Context, id string) (*authZENPDPConnection, error) {
	return s.service.Get(ctx, id)
}

func (s *authZENPDPStoreAdapter) update(ctx context.Context, id string, connection authZENPDPConnection) error {
	return s.service.Update(ctx, id, connection)
}

func (s *authZENPDPStoreAdapter) delete(ctx context.Context, id string) error {
	return s.service.Delete(ctx, id)
}

func authZENPDPFromRequest(req authZENPDPConnectionRequest) authZENPDPConnection {
	return authzenpdp.FromRequest(req)
}

func authZENPDPToResponse(connection authZENPDPConnection) authZENPDPConnectionResponse {
	return authzenpdp.ToResponse(connection)
}

func splitSubjectPropertyMappings(value string) map[string]string {
	return authzenpdp.SplitSubjectPropertyMappings(value)
}

func joinSubjectPropertyMappings(mappings map[string]string) string {
	return authzenpdp.JoinSubjectPropertyMappings(mappings)
}

func normalizedSubjectMapping(connection authZENPDPConnection) ([]string, map[string]string) {
	return authzenpdp.NormalizedSubjectMapping(connection)
}

func validateAuthZENPDPEndpoints(connection authZENPDPConnection) *tidcommon.ServiceError {
	if err := authzenpdp.ValidateEndpoint(connection.Endpoint); err != nil {
		return &ErrorInvalidAuthZENPDPEndpoint
	}
	if err := authzenpdp.ValidateEndpoint(connection.BatchEndpoint); err != nil {
		return &ErrorInvalidAuthZENPDPEndpoint
	}
	return nil
}

func normalizeAuthZENPDPEndpoints(connection *authZENPDPConnection) *tidcommon.ServiceError {
	if connection == nil {
		return &tidcommon.InternalServerError
	}
	connection.Endpoint = strings.TrimSpace(connection.Endpoint)
	connection.BatchEndpoint = strings.TrimSpace(connection.BatchEndpoint)
	return validateAuthZENPDPEndpoints(*connection)
}
