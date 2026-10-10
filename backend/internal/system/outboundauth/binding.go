// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"context"
	"fmt"
	"slices"
)

// Authenticator authenticates one outbound target of type T, such as an SMTP session. A caller
// hands it the target and never sees which method was configured.
type Authenticator[T any] interface {
	// Authenticate presents the configured credentials on target. A method that sends no
	// credentials returns nil without touching target.
	Authenticate(ctx context.Context, target T) error
}

// Binding pairs a method with the Authenticator a transport builds for it.
type Binding[T any] struct {
	Type Type
	New  func(authConfig Config) Authenticator[T]
}

// Bindings is a transport's table of the methods it can carry. A transport keeps one table and
// derives both SupportedTypes and New from it, so the set a caller validates against cannot
// drift from the set the transport can build. It is a slice rather than a map so the order is
// stable.
type Bindings[T any] []Binding[T]

// SupportedTypes returns the methods in b, in binding order. Callers hand it to Validate so a
// configuration naming a method the transport cannot carry is refused when it is configured
// rather than when the call is made.
func (b Bindings[T]) SupportedTypes() []Type {
	types := make([]Type, 0, len(b))
	for _, binding := range b {
		types = append(types, binding.Type)
	}
	return types
}

// New returns the Authenticator bound to cfg's method. An empty type means TypeNone. An error
// means cfg names a method the transport cannot carry, which validation should already have
// refused.
func (b Bindings[T]) New(authConfig Config) (Authenticator[T], error) {
	authType := authConfig.Type
	if authType == "" {
		authType = TypeNone
	}

	index := slices.IndexFunc(b, func(binding Binding[T]) bool { return binding.Type == authType })
	if index < 0 {
		return nil, fmt.Errorf("authentication type is not supported by this transport: %s", authType)
	}
	return b[index].New(authConfig), nil
}
