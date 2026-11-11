// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// AC11.4: the public audit surface imports no package of the engine, so an embedder needs none.
func TestAuditSurfaceImportsNoInternalPackage(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "audit.go", nil, parser.ImportsOnly)
	require.NoError(t, err)

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err)
		require.False(t, strings.Contains(path, "thunderid"), path)
		require.False(t, strings.Contains(path, "."), "only the standard library: "+path)
	}
}
