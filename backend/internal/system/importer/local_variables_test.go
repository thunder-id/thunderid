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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/secretresolver"
)

const localVarContent = "clientId: {{.MY_APP_CLIENT_ID}}\nclientSecret: {{.MY_APP_CLIENT_SECRET}}\n" +
	"redirectUris:\n  {{- range .MY_APP_REDIRECT_URIS}}\n  - {{.}}\n  {{- end}}\n"

func TestFillSecretPlaceholders_PrefersTheCallersValues(t *testing.T) {
	filled := fillSecretPlaceholders(context.Background(), localVarContent,
		map[string]interface{}{"MY_APP_CLIENT_ID": "from-caller"})

	assert.Equal(t, "from-caller", filled["MY_APP_CLIENT_ID"],
		"an explicitly supplied value must never be overridden")
}

func TestFillSecretPlaceholders_IgnoresThisHostsEnvironment(t *testing.T) {
	t.Setenv("MY_APP_CLIENT_ID", "from-env")
	t.Setenv("MY_APP_CLIENT_SECRET", "from-env")
	t.Setenv("MY_APP_REDIRECT_URIS_0", "https://a")

	filled := fillSecretPlaceholders(context.Background(), localVarContent, nil)

	// Configuration belongs to whoever is applying it. Taking a value from this host instead would let
	// a deployment quietly apply something other than what was sent to it.
	assert.Empty(t, filled, "no placeholder should be resolved from the environment")
}

func TestFillSecretPlaceholders_LeavesUnknownPlaceholdersAbsent(t *testing.T) {
	filled := fillSecretPlaceholders(context.Background(), localVarContent, nil)

	// Nothing invents a value: an unresolved placeholder stays absent so the template resolution
	// reports it rather than silently writing an empty credential.
	_, present := filled["MY_APP_CLIENT_SECRET"]
	assert.False(t, present)
}

func TestFillSecretPlaceholders_ResolvesASecretFromTheProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/secrets/MY_APP_CLIENT_SECRET" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name": "MY_APP_CLIENT_SECRET", "value": "provider-secret",
		})
	}))
	defer srv.Close()

	previous := secretresolver.Default()
	secretresolver.SetDefault(secretresolver.New(secretresolver.Config{BaseURL: srv.URL}))
	defer secretresolver.SetDefault(previous)

	t.Setenv("MY_APP_CLIENT_SECRET", "env-secret")

	filled := fillSecretPlaceholders(context.Background(), localVarContent, nil)

	// A credential is the one thing this deployment supplies, because it never travels with the
	// configuration. The environment is still not consulted.
	assert.Equal(t, "provider-secret", filled["MY_APP_CLIENT_SECRET"])
}

func TestImportResources_FailsWhenAPlaceholderHasNoValue(t *testing.T) {
	t.Setenv("SMOKE_APP_NAME", "Smoke App")

	svc := newTestImportService(nil)
	_, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: application\nid: app-env-1\nname: {{.SMOKE_APP_NAME}}\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	// The value exists in this host's environment, and that is exactly why the import must still fail:
	// it was not sent, so applying it would write something the caller never asked for.
	require.NotNil(t, err)
	assert.Equal(t, "IMP-1003", err.Code)
}
