// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
)

type ActionsTestSuite struct {
	suite.Suite
}

func TestActionsTestSuite(t *testing.T) {
	suite.Run(t, new(ActionsTestSuite))
}

func (s *ActionsTestSuite) SetupTest() {
	security.InitSystemPermissions("")
}

// catalogActions reads each catalog resource and the actions declared on it from the catalog
// permissions, such as "system:app:view".
func catalogActions() map[security.ResourceType][]string {
	catalog := map[security.ResourceType][]string{}
	for _, perm := range security.GetSystemPermissions().Catalog {
		if parts := strings.Split(perm, ":"); len(parts) == 3 {
			resource := security.ResourceType(parts[1])
			catalog[resource] = append(catalog[resource], parts[2])
		}
	}
	return catalog
}

// TestRequiredActionsMatchTheCatalog keeps the table in step with the system permission catalog, so the
// response reports every catalog action and nothing else, each lists its own action first, and no listed
// action needs the root permission.
func (s *ActionsTestSuite) TestRequiredActionsMatchTheCatalog() {
	catalog := catalogActions()
	s.Len(requiredActions, len(catalog))
	for resource, names := range catalog {
		s.Len(requiredActions[resource], len(names), resource)
		for _, name := range names {
			actions := requiredActions[resource][name]
			if !s.NotEmpty(actions, "%s:%s", resource, name) {
				continue
			}
			if resource != security.ResourceTypeI18n || name != "create" {
				s.Equal("system:"+string(resource)+":"+name, security.ResolveActionPermission(actions[0]),
					"the first action of %s:%s is not its own", resource, name)
			}
			for _, action := range actions {
				s.NotEqual(security.GetSystemRootPermission(), security.ResolveActionPermission(action),
					"%s:%s needs %s, which falls back to root", resource, name, action)
			}
		}
	}
}
