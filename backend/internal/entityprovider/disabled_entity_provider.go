// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entityprovider

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// errNotImplemented is the error returned when a method is not implemented.
var errNotImplemented = NewEntityProviderError(
	ErrorCodeNotImplemented,
	"Method Not Implemented",
	"The method is not implemented by the entity provider.",
)

// disabledEntityProvider is an entity provider that returns an error for all methods.
type disabledEntityProvider struct{}

// newDisabledEntityProvider creates a new disabled entity provider.
func newDisabledEntityProvider() EntityProviderInterface {
	return &disabledEntityProvider{}
}

func (p *disabledEntityProvider) IdentifyEntity(
	_ context.Context, _ map[string]interface{}) (*string, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) SearchEntities(
	_ context.Context, _ map[string]interface{}) ([]*providers.Entity, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) GetLinkedEntities(
	_ context.Context, _, _ string) ([]*providers.Entity, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) GetEntity(
	_ context.Context, _ string) (*providers.Entity, *EntityProviderError) {
	return nil, errNotImplemented
}

// GetEntityProfile is not available without an entity provider.
func (p *disabledEntityProvider) GetEntityProfile(
	_ context.Context, _ string) (*providers.Entity, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) UpdateEntity(_ context.Context, _ string,
	_ *providers.Entity) (*providers.Entity, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) DeleteEntity(_ context.Context, _ string) *EntityProviderError {
	return errNotImplemented
}

func (p *disabledEntityProvider) UpdateCredentials(_ context.Context, _ string,
	_ json.RawMessage) *EntityProviderError {
	return errNotImplemented
}

func (p *disabledEntityProvider) UpdateAttributes(_ context.Context, _ string,
	_ json.RawMessage) *EntityProviderError {
	return errNotImplemented
}

func (p *disabledEntityProvider) UpdateSystemAttributes(_ context.Context, _ string,
	_ json.RawMessage) *EntityProviderError {
	return errNotImplemented
}

func (p *disabledEntityProvider) UpdateSystemCredentials(_ context.Context, _ string,
	_ json.RawMessage) *EntityProviderError {
	return errNotImplemented
}

func (p *disabledEntityProvider) GetTransitiveEntityGroups(
	_ context.Context, _ string) ([]providers.EntityGroup, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) GetTransitiveGroupAncestors(
	_ context.Context, _ string) ([]string, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) ValidateEntityIDs(
	_ context.Context, _ []string) ([]string, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) GetEntitiesByIDs(
	_ context.Context, _ []string) ([]providers.Entity, *EntityProviderError) {
	return nil, errNotImplemented
}

func (p *disabledEntityProvider) GetEntityListCount(
	_ context.Context, _ providers.EntityCategory, _ map[string]interface{}) (int, *EntityProviderError) {
	return 0, errNotImplemented
}

func (p *disabledEntityProvider) GetEntityList(
	_ context.Context, _ providers.EntityCategory, _, _ int, _ map[string]interface{},
) ([]providers.Entity, *EntityProviderError) {
	return nil, errNotImplemented
}
