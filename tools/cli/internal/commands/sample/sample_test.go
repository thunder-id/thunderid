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

package sample

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
)

// writeConfig creates <dir>/<rel>/<slug>-config.yaml and its env file.
func writeConfig(t *testing.T, dir, rel string) (yamlPath, envPath string) {
	t.Helper()
	configDir := product.Slug + "-config"
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlPath = filepath.Join(full, configDir+".yaml")
	envPath = filepath.Join(full, product.Slug+".env")
	if err := os.WriteFile(yamlPath, []byte("resources: []\n"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := os.WriteFile(envPath, []byte("KEY=value\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	return yamlPath, envPath
}

func TestFindSampleConfig(t *testing.T) {
	configDir := product.Slug + "-config"

	t.Run("auth-mode subdirectory layout", func(t *testing.T) {
		dir := t.TempDir()
		wantYAML, wantEnv := writeConfig(t, dir, filepath.Join(configDir, defaultAuthMode))

		gotYAML, gotEnv, sampleDir, err := findSampleConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotYAML != wantYAML {
			t.Errorf("yaml: got %q, want %q", gotYAML, wantYAML)
		}
		if gotEnv != wantEnv {
			t.Errorf("env: got %q, want %q", gotEnv, wantEnv)
		}
		if sampleDir != dir {
			t.Errorf("sampleDir: got %q, want %q", sampleDir, dir)
		}
	})

	t.Run("legacy flat layout", func(t *testing.T) {
		dir := t.TempDir()
		wantYAML, wantEnv := writeConfig(t, dir, configDir)

		gotYAML, gotEnv, _, err := findSampleConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotYAML != wantYAML {
			t.Errorf("yaml: got %q, want %q", gotYAML, wantYAML)
		}
		if gotEnv != wantEnv {
			t.Errorf("env: got %q, want %q", gotEnv, wantEnv)
		}
	})

	t.Run("nested extraction subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join("wayfinder-v1", configDir, defaultAuthMode)
		wantYAML, _ := writeConfig(t, dir, nested)

		gotYAML, _, sampleDir, err := findSampleConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotYAML != wantYAML {
			t.Errorf("yaml: got %q, want %q", gotYAML, wantYAML)
		}
		if wantBase := filepath.Join(dir, "wayfinder-v1"); sampleDir != wantBase {
			t.Errorf("sampleDir: got %q, want %q", sampleDir, wantBase)
		}
	})

	t.Run("auth-mode preferred over flat", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, configDir)
		wantYAML, _ := writeConfig(t, dir, filepath.Join(configDir, defaultAuthMode))

		gotYAML, _, _, err := findSampleConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotYAML != wantYAML {
			t.Errorf("yaml: got %q, want %q (auth-mode should win)", gotYAML, wantYAML)
		}
	})

	t.Run("missing config", func(t *testing.T) {
		if _, _, _, err := findSampleConfig(t.TempDir()); err == nil {
			t.Fatal("expected error for missing config, got nil")
		}
	})
}

func TestSampleServicePorts(t *testing.T) {
	b2c := sampleServicePorts(false)
	if got := len(b2c); got != 5 {
		t.Fatalf("b2c ports: got %d, want 5 (%v)", got, b2c)
	}
	for _, p := range []int{5173, 8787, 8788, 2525, 8795} {
		if !slices.Contains(b2c, p) {
			t.Errorf("b2c ports missing %d: %v", p, b2c)
		}
	}
	if slices.Contains(b2c, 8790) {
		t.Errorf("b2c ports must not include the ai-agent port 8790: %v", b2c)
	}

	if ai := sampleServicePorts(true); !slices.Contains(ai, 8790) {
		t.Errorf("ai ports must include the ai-agent port 8790: %v", ai)
	}
}

func TestWriteResources(t *testing.T) {
	thunderRoot := t.TempDir()
	yamlPath := filepath.Join(t.TempDir(), "resources.yaml")
	content := "" +
		"# resource_type: application\n" +
		"id: wayfinder-app\n" +
		"clientId: \"{{.WAYFINDER_CLIENT_ID}}\"\n" +
		"---\n" +
		"# resource_type: user_type\n" +
		"id: wayfinder-customer-type\n" +
		"name: Customer\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	vars := map[string]string{"WAYFINDER_CLIENT_ID": "WAYFINDER"}
	if err := writeResources(yamlPath, vars, thunderRoot); err != nil {
		t.Fatalf("writeResources: %v", err)
	}

	// Resources must land under config/resources/<dir>, the directory the server
	// loads declarative resources from at startup.
	appFile := filepath.Join(thunderRoot, "config", "resources", "applications", "wayfinder-app.yaml")
	got, err := os.ReadFile(appFile)
	if err != nil {
		t.Fatalf("expected application at %s: %v", appFile, err)
	}
	if !strings.Contains(string(got), "clientId: \"WAYFINDER\"") {
		t.Errorf("template var not substituted; got:\n%s", got)
	}

	// user_type has no entry in typeToDir and must fall back to "<type>s".
	utFile := filepath.Join(thunderRoot, "config", "resources", "user_types", "wayfinder-customer-type.yaml")
	if _, err := os.Stat(utFile); err != nil {
		t.Errorf("expected user_type at %s: %v", utFile, err)
	}

	// The legacy repository/resources path must no longer be used.
	if _, err := os.Stat(filepath.Join(thunderRoot, "repository")); !os.IsNotExist(err) {
		t.Errorf("resources written to legacy repository/ path")
	}
}
