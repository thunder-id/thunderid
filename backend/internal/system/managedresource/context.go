// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package managedresource

import "context"

type managedImportKey struct{}

// WithManagedImport marks a context as the managed import's, the one write path that may change what
// a control plane deployed.
func WithManagedImport(ctx context.Context) context.Context {
	return context.WithValue(ctx, managedImportKey{}, true)
}

// IsManagedImport reports whether a context is the managed import's.
func IsManagedImport(ctx context.Context) bool {
	managed, _ := ctx.Value(managedImportKey{}).(bool)
	return managed
}

// IsReadOnly reports whether the id is managed as one of the given types and the caller is not the
// managed import, which is what every write path other than the managed import refuses. A nil store,
// as a store built without one has, answers false.
func IsReadOnly(ctx context.Context, s StoreInterface, id string, resourceTypes ...string) (bool, error) {
	if s == nil || IsManagedImport(ctx) {
		return false, nil
	}
	return s.IsManaged(ctx, id, resourceTypes...)
}

// IsDeployed reports whether a control plane deployed the id as one of the given types, which a read
// shows as read only. A nil store answers false.
func IsDeployed(ctx context.Context, s StoreInterface, id string, resourceTypes ...string) (bool, error) {
	if s == nil {
		return false, nil
	}
	return s.IsManaged(ctx, id, resourceTypes...)
}

// MarkDeployed calls mark on each listed item a control plane deployed as one of the given types, in
// one read of the store. A nil store marks nothing.
func MarkDeployed[T any](ctx context.Context, s StoreInterface, items []T, idOf func(*T) string,
	mark func(*T), resourceTypes ...string) error {
	if s == nil || len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = idOf(&items[i])
	}
	deployed, err := s.ManagedIDs(ctx, ids, resourceTypes...)
	if err != nil {
		return err
	}
	for i := range items {
		if deployed[ids[i]] {
			mark(&items[i])
		}
	}
	return nil
}
