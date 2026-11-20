// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AuditTestSuite struct {
	suite.Suite
}

func TestAuditTestSuite(t *testing.T) {
	suite.Run(t, new(AuditTestSuite))
}

// AC9.4: the public audit surface imports no package of the engine, so an embedder needs none.
func (s *AuditTestSuite) TestAuditSurfaceImportsNoInternalPackage() {
	file, err := parser.ParseFile(token.NewFileSet(), "audit.go", nil, parser.ImportsOnly)
	s.Require().NoError(err)

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		s.Require().NoError(err)
		s.False(strings.Contains(path, "thunderid"), path)
		s.False(strings.Contains(path, "."), "only the standard library: "+path)
	}
}
