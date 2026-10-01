// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"strings"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// registry holds the declaration of every resource type onboarded onto the framework. Optional
// capabilities are discovered by type assertion rather than a separate registration call.
type registry struct {
	mu     sync.RWMutex
	decls  map[ResourceType]ResourceOverlayFieldDeclaration
	logger *log.Logger
}

// newRegistry creates an empty resource-type registry.
func newRegistry() *registry {
	return &registry{
		decls:  make(map[ResourceType]ResourceOverlayFieldDeclaration),
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SharingRegistry")),
	}
}

// register adds or replaces a resource type's declaration. A nil declaration is ignored so a
// service that failed to initialize cannot panic a later lookup.
func (r *registry) register(decl ResourceOverlayFieldDeclaration) {
	if decl == nil {
		r.logger.Warn(context.Background(), "Ignoring nil resource type declaration registration")
		return
	}
	// OwnerResolver is the one capability the framework cannot do without. Ownership is asked of
	// the resource type, never read off a policy: the chain-scoped fetch returns the policies
	// bearing on the asking organization unit, and an owner's own policies point at other units, so
	// the owner's question fetches nothing to read an owner from. A type that cannot answer would
	// have its owner told it cannot see its own resource, which is worse than refusing to onboard
	// it: this way the mistake surfaces as ErrorResourceTypeNotRegistered on the first call.
	if _, ok := decl.(OwnerResolver); !ok {
		r.logger.Error(context.Background(),
			"Refusing to register a resource type that cannot resolve ownership",
			log.String("resourceType", string(decl.ResourceType())))
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.decls[decl.ResourceType()] = decl
}

// get returns the declaration registered for resourceType, if any.
func (r *registry) get(resourceType ResourceType) (ResourceOverlayFieldDeclaration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	decl, ok := r.decls[resourceType]
	return decl, ok
}

// field resolves a rule key to the field declaration governing it.
//
// A dynamic key such as "nodes.mfa-step" resolves to its declared prefix, which is what lets a
// resource type declare a map field once instead of enumerating every member id.
func (r *registry) field(resourceType ResourceType, fieldKey string) (FieldDeclaration, bool) {
	decl, ok := r.get(resourceType)
	if !ok {
		return FieldDeclaration{}, false
	}
	for _, f := range decl.Fields() {
		if f.Key == fieldKey {
			return f, true
		}
	}
	for _, f := range decl.Fields() {
		if f.DynamicKeys && strings.HasPrefix(fieldKey, f.Key+".") {
			return f, true
		}
	}
	return FieldDeclaration{}, false
}

// defaultRule returns the rule that applies when no policy names a field.
func (r *registry) defaultRule(resourceType ResourceType, fieldKey string) (OverlayRule, bool) {
	f, ok := r.field(resourceType, fieldKey)
	if !ok || f.Default == nil {
		return OverlayRule{}, false
	}
	return *f.Default, true
}

// fallbackKey returns the coarser field consulted when no rule names fieldKey itself.
func (r *registry) fallbackKey(resourceType ResourceType, fieldKey string) (string, bool) {
	f, ok := r.field(resourceType, fieldKey)
	if !ok {
		return "", false
	}
	// A dynamic member falls back to the prefix that declared it before any declared fallback.
	if f.DynamicKeys && fieldKey != f.Key {
		return f.Key, true
	}
	if f.FallbackKey == "" {
		return "", false
	}
	return f.FallbackKey, true
}
