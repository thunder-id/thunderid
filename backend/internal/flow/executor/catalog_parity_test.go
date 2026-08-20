/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
// The catalog exists so a control plane can validate flow definitions without constructing an
// executor, which would link the data-plane services those constructors need. Two descriptions of
// the same thing drift, so this is what stops them: change an executor's metadata without changing
// the catalog and this fails, naming the executor.
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
