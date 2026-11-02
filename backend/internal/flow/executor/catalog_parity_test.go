// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	"github.com/thunder-id/thunderid/internal/system/cache"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/authn/githubmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
)

// TestCatalogMatchesRegistry holds the static metadata catalog to what the real executors report.
//
// The catalog exists so a build that only validates flow definitions can read executor metadata
// without constructing an executor, which would link the runtime services those constructors need.
// Two descriptions of the same thing drift, so this is what stops them: change an executor's
// metadata without changing the catalog and this fails, naming the executor.
//
// The real flow factory is used rather than a mock, because a mock's CreateExecutor discards the
// metadata it is handed and returns a stub.
func TestCatalogMatchesRegistry(t *testing.T) {
	factory, _ := core.Initialize(cache.Initialize(engineconfig.CacheConfig{Disabled: true}, "test"))
	deps := ExecutorDependencies{
		FlowFactory:    factory,
		EntityProvider: entityprovidermock.NewEntityProviderInterfaceMock(t),
		OAuthSvc:       oauthmock.NewOAuthAuthnServiceInterfaceMock(t),
		OIDCSvc:        oidcmock.NewOIDCAuthnServiceInterfaceMock(t),
		GithubSvc:      githubmock.NewGithubOAuthAuthnServiceInterfaceMock(t),
		GoogleSvc:      googlemock.NewGoogleOIDCAuthnServiceInterfaceMock(t),
	}
	reg := newExecutorRegistry()
	require.NoError(t, registerBuiltInExecutors(reg, deps, nil))

	assert.ElementsMatch(t, defaultBuiltInExecutorNames(), executormeta.Names(),
		"the catalog must name exactly the built-in executors")

	for _, name := range defaultBuiltInExecutorNames() {
		live, err := reg.GetExecutorMeta(name)
		require.NoError(t, err, "executor %q", name)
		static, ok := executormeta.MetaFor(name)
		require.True(t, ok, "executor %q is missing from the catalog", name)

		if live == nil {
			assert.Empty(t, static, "executor %q reports no metadata", name)
			continue
		}
		assert.Equal(t, *live, static, "catalog metadata for %q has drifted from the executor", name)
	}
}
