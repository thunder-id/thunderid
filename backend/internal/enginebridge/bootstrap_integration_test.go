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

package enginebridge

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

func TestInitializeRegistersRuntimeRoutes(t *testing.T) {
	serverHome := filepath.Join("..", "..", "cmd", "server")
	deployment := filepath.Join(serverHome, "repository", "conf", "deployment.yaml")
	if _, err := os.Stat(deployment); err != nil {
		t.Skip("cmd/server deployment config not available")
	}
	ensureServerTestAssets(t, serverHome)

	mux := http.NewServeMux()
	hostOnly := true
	err := Initialize(thunderidengine.EngineConfig{
		ConfigPath: serverHome,
		HostOnly:   &hostOnly,
		Providers:  testProviders(),
		Executors: thunderidengine.ExecutorConfig{
			Names: []string{
				"BasicAuthExecutor",
				"AuthAssertExecutor",
				"AuthorizationExecutor",
			},
		},
	}, mux)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}
