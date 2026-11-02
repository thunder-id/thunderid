// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/secretresolver"
	"github.com/thunder-id/thunderid/internal/system/valueref"
)

// Lookup adapts the store to the reference resolver, so a reference is replaced with what this
// deployment holds under the name: a variable as it is, a secret decrypted.
func Lookup(resolver ReferenceResolverInterface) secretresolver.Lookup {
	return func(ctx context.Context, collection valueref.Collection, name string) (string, bool, error) {
		read := resolver.ResolveVariable
		if collection == valueref.CollectionSecret {
			read = resolver.ResolveSecret
		}
		value, held, svcErr := read(ctx, name)
		if svcErr != nil {
			return "", false, fmt.Errorf("failed to read %s %s: %s", collection, name, svcErr.Code)
		}
		return value, held, nil
	}
}
