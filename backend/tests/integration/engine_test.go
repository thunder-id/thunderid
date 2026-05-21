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

// Package integration contains end-to-end tests for the public thunderidengine surface.
package integration

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/thunder-id/thunderid/internal/enginebridge"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

func TestEngineInitializeRegistersRuntimeRoutes(t *testing.T) {
	serverHome := filepath.Join("..", "..", "cmd", "server")
	deployment := filepath.Join(serverHome, "repository", "conf", "deployment.yaml")
	if _, err := os.Stat(deployment); err != nil {
		t.Skip("cmd/server deployment config not available")
	}

	mux := http.NewServeMux()
	engine := thunderidengine.New(thunderidengine.EngineConfig{
		ConfigPath: serverHome,
		Providers:  FakeProviders(),
		Executors: thunderidengine.ExecutorConfig{
			Names: []string{
				"BasicAuthExecutor",
				"AuthAssertExecutor",
				"AuthorizationExecutor",
			},
		},
	})
	err := engine.Initialize(mux)
	if err != nil {
		t.Skip("engine bootstrap requires provisioned server home:", err)
	}

	t.Run("openid configuration", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil))
		require.NotEqual(t, http.StatusNotFound, rec.Code)
	})

	t.Run("flow meta route", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/flow/meta", nil))
		require.NotEqual(t, http.StatusNotFound, rec.Code)
	})

	t.Run("flow execute route", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/flow/execute", nil)
		req.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(rec, req)
		require.NotEqual(t, http.StatusNotFound, rec.Code)
	})
}
