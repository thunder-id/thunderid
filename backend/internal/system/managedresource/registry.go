/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package managedresource records which resources this deployment does not own and refuses local
// changes to them.
//
// A Data Plane receives its configuration from a Control Plane through the import API. Anything that
// arrives that way belongs to the Control Plane: editing it here would work until the next promotion
// silently overwrote it, which is worse than not being allowed to edit it at all. So the import
// records what it writes, and the management APIs consult that record before changing anything.
//
// Resources created on this deployment are untouched by any of this. A user who signs up on a Data
// Plane is owned by it and stays editable.
package managedresource

import (
	"context"
	"sync"
)

// Registry answers whether a resource is owned elsewhere, and records ownership as imports run.
type Registry struct {
	store   storeInterface
	enabled bool
}

// New builds a Registry. When enabled is false every resource is treated as locally owned, which is
// what a server with no control plane in front of it wants.
func New(enabled bool, deploymentID string) *Registry {
	r := &Registry{enabled: enabled}
	if enabled {
		r.store = newStore(deploymentID)
	}
	return r
}

// Enabled reports whether ownership is being tracked at all.
func (r *Registry) Enabled() bool {
	return r != nil && r.enabled && r.store != nil
}

// Mark records a resource as owned by the control plane. It is called by the import for every
// resource it writes.
func (r *Registry) Mark(ctx context.Context, resourceType, resourceID string) error {
	if !r.Enabled() || resourceType == "" || resourceID == "" {
		return nil
	}
	return r.store.Mark(ctx, resourceType, resourceID)
}

// Unmark drops the record, so a resource removed by an import does not leave one behind to be
// inherited by a later resource that reuses the id.
func (r *Registry) Unmark(ctx context.Context, resourceType, resourceID string) error {
	if !r.Enabled() || resourceType == "" || resourceID == "" {
		return nil
	}
	return r.store.Unmark(ctx, resourceType, resourceID)
}

// IsManaged reports whether the resource is owned by the control plane.
//
// A lookup failure answers true. The registry exists to stop a local write from being silently
// overwritten later, and refusing a write that should have been allowed is recoverable, whereas
// allowing one that should have been refused is the thing being prevented.
func (r *Registry) IsManaged(ctx context.Context, resourceType, resourceID string) bool {
	if !r.Enabled() || isImporting(ctx) {
		return false
	}
	managed, err := r.store.IsManaged(ctx, resourceType, resourceID)
	if err != nil {
		return true
	}
	return managed
}

// ManagedIDs returns the managed ids of one resource type, for a list endpoint that would otherwise
// issue a lookup per row.
func (r *Registry) ManagedIDs(ctx context.Context, resourceType string) map[string]bool {
	if !r.Enabled() {
		return nil
	}
	ids, err := r.store.ManagedIDs(ctx, resourceType)
	if err != nil {
		return nil
	}
	return ids
}

// importingKey marks a context as belonging to an import, which is the one caller allowed to change a
// managed resource.
type importingKey struct{}

// WithImport marks the context as an import, so the guard lets its writes through.
func WithImport(ctx context.Context) context.Context {
	return context.WithValue(ctx, importingKey{}, true)
}

// isImporting reports whether the context belongs to an import.
func isImporting(ctx context.Context) bool {
	importing, ok := ctx.Value(importingKey{}).(bool)
	return ok && importing
}

var (
	defaultMu       sync.RWMutex
	defaultRegistry = New(false, "")
)

// SetDefault installs the registry the domain services consult. It is package level because the guard
// is called from every service that can change a resource, and threading a dependency through all of
// them buys nothing.
func SetDefault(r *Registry) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if r == nil {
		r = New(false, "")
	}
	defaultRegistry = r
}

// Default returns the installed registry. It is never nil, so a caller never has to check.
func Default() *Registry {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultRegistry
}

// IsManaged reports whether the resource is owned by the control plane, using the installed registry.
func IsManaged(ctx context.Context, resourceType, resourceID string) bool {
	return Default().IsManaged(ctx, resourceType, resourceID)
}
