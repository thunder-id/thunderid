// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

const (
	modulePath   = "github.com/thunder-id/thunderid/"
	eventBusPath = modulePath + "internal/system/eventbus"
)

// forbiddenDependencies are the prefixes the bus must not link.
var forbiddenDependencies = []string{
	eventBusPath + "/",
	"go.opentelemetry.io/",
	modulePath + "internal/system/observability",
}

type DependencyTestSuite struct {
	suite.Suite
}

func TestDependencyTestSuite(t *testing.T) {
	suite.Run(t, new(DependencyTestSuite))
}

func (s *DependencyTestSuite) TestBusLinksNoSubPackageOrOpenTelemetryPackage() {
	out, err := exec.Command("go", "list", "-deps", eventBusPath).Output()
	s.Require().NoError(err, "go list -deps")

	deps := strings.Fields(string(out))
	s.Contains(deps, eventBusPath)
	for _, dep := range deps {
		for _, prefix := range forbiddenDependencies {
			s.False(strings.HasPrefix(dep, prefix), dep)
		}
	}
}
