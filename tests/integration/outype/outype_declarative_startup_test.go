// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// OUTypeDeclarativeStartupTestSuite covers that the server refuses to start with an invalid declarative
// organization unit type.
type OUTypeDeclarativeStartupTestSuite struct {
	suite.Suite
}

func TestOUTypeDeclarativeStartupTestSuite(t *testing.T) {
	suite.Run(t, new(OUTypeDeclarativeStartupTestSuite))
}

func (s *OUTypeDeclarativeStartupTestSuite) TearDownTest() {
	s.NoError(testutils.RestartServer())
	s.NoError(testutils.ObtainAdminAccessToken())
}

func (s *OUTypeDeclarativeStartupTestSuite) TestInvalidTypeFailsStartup() {
	tests := []struct {
		name     string
		resource string
		wantErr  string
	}{
		{
			name: "credential attribute",
			resource: "resource_type: ou_type\nid: decl-startup-credential\nhandle: decl-startup-credential\n" +
				"displayName: Credential\nouHandle: default\nschema:\n  apiKey:\n    type: string\n" +
				"    credential: true\n",
			wantErr: "declares a credential attribute, which organization unit types cannot declare",
		},
		{
			name: "another category",
			resource: "resource_type: ou_type\nid: decl-startup-category\nhandle: decl-startup-category\n" +
				"displayName: Category\ncategory: user\nouHandle: default\nschema:\n  region:\n    type: string\n",
			wantErr: "declares category",
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			resourcesPath := filepath.Join(s.T().TempDir(), "resources.yaml")
			s.Require().NoError(os.WriteFile(resourcesPath, []byte(tc.resource), 0o600))
			before, err := testutils.ReadServerLog()
			s.Require().NoError(err)

			s.Require().NoError(testutils.StartServerExpectingExit(30*time.Second, "-resources="+resourcesPath),
				"the server must refuse the invalid organization unit type and exit")

			serverLog, err := testutils.ReadServerLog()
			s.Require().NoError(err)
			s.Contains(serverLog[len(before):], tc.wantErr)
		})
	}
}
