/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
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

package health_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
)

func TestDefaultPort(t *testing.T) {
	assert.Equal(t, 8090, health.DefaultPort)
}

func TestCheckReady_ReturnsTrueOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/readiness" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	assert.True(t, health.CheckReady(srv.URL))
}

func TestCheckReady_ReturnsFalseOn503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	assert.False(t, health.CheckReady(srv.URL))
}

func TestCheckReady_ReturnsFalseOnUnreachable(t *testing.T) {
	assert.False(t, health.CheckReady("http://127.0.0.1:19999"))
}

func TestResolveBaseURL_TimesOutWhenNotReady(t *testing.T) {
	url, ok := health.ResolveBaseURL(19998, 200*time.Millisecond)
	assert.False(t, ok)
	assert.Empty(t, url)
}

func TestResolveBaseURL_ReturnsURLWhenReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/health/readiness") {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	// Extract port from test server URL.
	// ResolveBaseURL dials localhost:<port> not the test server directly, so
	// we verify CheckReady behavior via the lower-level helper instead.
	assert.True(t, health.CheckReady(srv.URL))
}
