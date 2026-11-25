// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultaudit

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

const (
	modulePath       = "github.com/thunder-id/thunderid/"
	defaultAuditPath = modulePath + "internal/system/eventbus/defaultaudit"
)

// forbiddenDependencies are the prefixes audit must not link: the observability pipeline, so an audit
// event can never reach an ObservabilityProvider, and OpenTelemetry.
var forbiddenDependencies = []string{
	modulePath + "internal/system/observability",
	"go.opentelemetry.io/",
}

type DependencyTestSuite struct {
	suite.Suite
}

func TestDependencyTestSuite(t *testing.T) {
	suite.Run(t, new(DependencyTestSuite))
}

func (s *DependencyTestSuite) TestAuditLinksNoObservabilityOrOpenTelemetryPackage() {
	out, err := exec.Command("go", "list", "-deps", defaultAuditPath).Output()
	s.Require().NoError(err, "go list -deps")

	deps := strings.Fields(string(out))
	s.Contains(deps, modulePath+"internal/system/eventbus/audit")
	s.Contains(deps, modulePath+"internal/system/eventbus/stdoutsink")
	for _, dep := range deps {
		for _, prefix := range forbiddenDependencies {
			s.False(strings.HasPrefix(dep, prefix), dep)
		}
	}
}
