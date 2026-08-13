// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sample

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

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

func writeFakeNPM(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "npm")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write fake npm: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path
}

func withSampleProcessSeams(t *testing.T) {
	t.Helper()
	originalGrace := sampleStartGrace
	originalReadiness := sampleWaitForReadiness
	t.Cleanup(func() {
		sampleStartGrace = originalGrace
		sampleWaitForReadiness = originalReadiness
	})
}

func TestStartSampleServices_ReturnsLiveProcessHandle(t *testing.T) {
	withSampleProcessSeams(t)
	writeFakeNPM(t, "sleep 30")
	sampleStartGrace = 20 * time.Millisecond

	process, err := startSampleServices(t.TempDir(), false)

	if err != nil {
		t.Fatalf("start sample services: %v", err)
	}
	if process == nil || process.Cmd == nil || process.Cmd.Process == nil {
		t.Fatalf("missing live process handle: %+v", process)
	}
	if err := StopProcessTree(process); err != nil {
		t.Fatalf("stop sample process: %v", err)
	}
}

func TestStartSampleServices_ImmediateFailureSurfaces(t *testing.T) {
	withSampleProcessSeams(t)
	writeFakeNPM(t, "echo missing-script >&2\nexit 7")
	// Generous grace: the select returns as soon as npm exits, so a long window
	// only guards against slow process spawns on loaded machines.
	sampleStartGrace = 30 * time.Second

	process, err := startSampleServices(t.TempDir(), false)

	if process != nil {
		t.Fatalf("failed start returned process: %+v", process)
	}
	if err == nil || !strings.Contains(err.Error(), "missing-script") {
		t.Fatalf("expected immediate npm failure, got %v", err)
	}
}

func TestWaitForSampleReadiness_CleansUpSpawnedTree(t *testing.T) {
	withSampleProcessSeams(t)
	writeFakeNPM(t, "sleep 30")
	sampleStartGrace = 20 * time.Millisecond
	process, err := startSampleServices(t.TempDir(), false)
	if err != nil {
		t.Fatalf("start sample services: %v", err)
	}
	sampleWaitForReadiness = func(string, bool, time.Duration) error {
		return errors.New("frontend did not start")
	}

	err = waitForSampleReadiness(process, t.TempDir(), false, time.Second)

	if err == nil || !strings.Contains(err.Error(), "frontend did not start") {
		t.Fatalf("expected readiness error, got %v", err)
	}
	select {
	case <-process.done:
	case <-time.After(2 * time.Second):
		t.Fatal("sample process still running after readiness failure")
	}
}

func TestStopProcessTree_SignalsGroupAfterWrapperExit(t *testing.T) {
	originalTerminate := sampleTerminateProcessTree
	originalRunning := sampleProcessTreeRunning
	t.Cleanup(func() {
		sampleTerminateProcessTree = originalTerminate
		sampleProcessTreeRunning = originalRunning
	})
	done := make(chan struct{})
	close(done)
	process := &Process{
		Cmd:  &exec.Cmd{Process: &os.Process{Pid: 123}},
		done: done,
	}
	called := false
	sampleTerminateProcessTree = func(*exec.Cmd, bool) error {
		called = true
		return nil
	}
	sampleProcessTreeRunning = func(*exec.Cmd) bool { return !called }

	if err := StopProcessTree(process); err != nil {
		t.Fatalf("stop process tree: %v", err)
	}
	if !called {
		t.Fatal("process group was not signaled after npm wrapper exited")
	}
}

func TestStopProcessTree_StopsOwnedTreeAfterWrapperExit(t *testing.T) {
	done := make(chan struct{})
	close(done)
	called := false
	process := &Process{
		Cmd:  &exec.Cmd{Process: &os.Process{Pid: 123}},
		done: done,
		stopOwnedTree: func() error {
			called = true
			return nil
		},
	}

	if err := StopProcessTree(process); err != nil {
		t.Fatalf("stop owned tree: %v", err)
	}
	if !called {
		t.Fatal("owned process tree was not stopped")
	}
}

func TestWriteResources(t *testing.T) {
	thunderRoot := t.TempDir()
	yamlPath := filepath.Join(t.TempDir(), "resources.yaml")
	// Sample configs declare resource_type as a plain key, the same form the server
	// reads it in.
	content := "" +
		"resource_type: application\n" +
		"id: wayfinder-app\n" +
		"clientId: \"{{.WAYFINDER_CLIENT_ID}}\"\n" +
		"---\n" +
		"resource_type: user_type\n" +
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

// A commented-out marker is not a supported declaration and must be ignored.
func TestWriteResourcesCommentedResourceType(t *testing.T) {
	thunderIDRoot := t.TempDir()
	yamlPath := filepath.Join(t.TempDir(), "resources.yaml")
	content := "# resource_type: application\nid: legacy-app\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	if err := writeResources(yamlPath, nil, thunderIDRoot); err != nil {
		t.Fatalf("writeResources: %v", err)
	}

	if _, err := os.Stat(filepath.Join(thunderIDRoot, "config", "resources")); !os.IsNotExist(err) {
		t.Error("commented resource_type must not be written")
	}
}

// serviceDir creates <root>/<service> and returns the path of its .env.
func serviceDir(t *testing.T, root, service string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, service), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", service, err)
	}
	return filepath.Join(root, service, ".env")
}

// readEnv parses a written .env back into a map.
func readEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	vals, err := parseEnvFile(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return vals
}

// sampleVars mirrors the variables the wayfinder thunderid.env supplies.
var sampleVars = map[string]string{
	"WAYFINDER_CLIENT_ID":         "WAYFINDER",
	"AGENT_CLIENT_ID":             "WAYFINDER-CONCIERGE",
	"AGENT_CLIENT_SECRET":         "agent-secret",
	"UPGRADE_AGENT_CLIENT_ID":     "WAYFINDER-UPGRADE-AGENT",
	"UPGRADE_AGENT_CLIENT_SECRET": "upgrade-secret",
}

// The agent reads its API key from LLM_API_KEY for every provider. Renaming it to
// a provider-specific variable leaves the agent without a key and it exits at
// startup, so the name must survive verbatim.
func TestWriteServiceEnvUsesSampleKeyNames(t *testing.T) {
	for _, provider := range []string{"anthropic", "gemini", "google"} {
		t.Run(provider, func(t *testing.T) {
			sampleDir := t.TempDir()
			envPath := serviceDir(t, sampleDir, "ai-agent")
			opts := Options{
				EnvTarget: "ai-agent",
				Config:    map[string]string{"LLM_PROVIDER": provider, "LLM_API_KEY": "secret-key"},
			}

			if err := writeServiceEnv(sampleDir, "https://localhost:8090", sampleVars, opts); err != nil {
				t.Fatalf("writeServiceEnv: %v", err)
			}

			env := readEnv(t, envPath)
			if env["LLM_API_KEY"] != "secret-key" {
				t.Errorf("LLM_API_KEY: got %q, want %q", env["LLM_API_KEY"], "secret-key")
			}
			for _, renamed := range []string{"GOOGLE_API_KEY", "ANTHROPIC_API_KEY"} {
				if _, ok := env[renamed]; ok {
					t.Errorf("%s must not be written — the sample reads LLM_API_KEY", renamed)
				}
			}
			if env["LLM_PROVIDER"] != provider {
				t.Errorf("LLM_PROVIDER: got %q, want %q", env["LLM_PROVIDER"], provider)
			}
		})
	}
}

// The upgrade scheduler authenticates with its own CIBA-only client, which the
// sample reads from UPGRADE_AGENT_ID / UPGRADE_AGENT_SECRET.
func TestWriteServiceEnvWritesAgentCredentials(t *testing.T) {
	sampleDir := t.TempDir()
	envPath := serviceDir(t, sampleDir, "ai-agent")
	opts := Options{EnvTarget: "ai-agent"}

	if err := writeServiceEnv(sampleDir, "https://localhost:9443", sampleVars, opts); err != nil {
		t.Fatalf("writeServiceEnv: %v", err)
	}

	env := readEnv(t, envPath)
	want := map[string]string{
		"THUNDER_BASE_URL":     "https://localhost:9443",
		"AGENT_ID":             "WAYFINDER-CONCIERGE",
		"AGENT_SECRET":         "agent-secret",
		"UPGRADE_AGENT_ID":     "WAYFINDER-UPGRADE-AGENT",
		"UPGRADE_AGENT_SECRET": "upgrade-secret",
		"AGENT_ACCESS_SCOPE":   "agent:access",
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s: got %q, want %q", k, env[k], v)
		}
	}
}

// A try-out has no CIBA email or SMS flow configured, so the upgrade scheduler
// must be off unless the operator turned it on themselves.
func TestWriteServiceEnvDisablesUpgradeScheduler(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		sampleDir := t.TempDir()
		envPath := serviceDir(t, sampleDir, "ai-agent")

		if err := writeServiceEnv(sampleDir, "https://localhost:8090", sampleVars, Options{EnvTarget: "ai-agent"}); err != nil {
			t.Fatalf("writeServiceEnv: %v", err)
		}

		if got := readEnv(t, envPath)["UPGRADE_SCHEDULER_ENABLED"]; got != "false" {
			t.Errorf("UPGRADE_SCHEDULER_ENABLED: got %q, want false", got)
		}
	})

	t.Run("operator opted in", func(t *testing.T) {
		sampleDir := t.TempDir()
		envPath := serviceDir(t, sampleDir, "ai-agent")
		if err := os.WriteFile(envPath, []byte("UPGRADE_SCHEDULER_ENABLED=true\n"), 0o644); err != nil {
			t.Fatalf("write env: %v", err)
		}

		if err := writeServiceEnv(sampleDir, "https://localhost:8090", sampleVars, Options{EnvTarget: "ai-agent"}); err != nil {
			t.Fatalf("writeServiceEnv: %v", err)
		}

		if got := readEnv(t, envPath)["UPGRADE_SCHEDULER_ENABLED"]; got != "true" {
			t.Errorf("an explicit opt-in must be preserved: got %q", got)
		}
	})
}

// Keys the sample ships defaults for must survive the rewrite; keys the CLI owns
// must be refreshed rather than kept from an earlier run.
func TestWriteServiceEnvPreservesShippedKeys(t *testing.T) {
	sampleDir := t.TempDir()
	envPath := serviceDir(t, sampleDir, "ai-agent")
	shipped := "MCP_SERVER_URL=http://localhost:8787/mcp\n" +
		"UPGRADE_SCHEDULER_ENABLED=false\n" +
		"THUNDER_BASE_URL=https://localhost:8090\n"
	if err := os.WriteFile(envPath, []byte(shipped), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	opts := Options{EnvTarget: "ai-agent"}
	if err := writeServiceEnv(sampleDir, "https://localhost:9443", sampleVars, opts); err != nil {
		t.Fatalf("writeServiceEnv: %v", err)
	}

	env := readEnv(t, envPath)
	if env["MCP_SERVER_URL"] != "http://localhost:8787/mcp" {
		t.Errorf("MCP_SERVER_URL was dropped: %q", env["MCP_SERVER_URL"])
	}
	if env["UPGRADE_SCHEDULER_ENABLED"] != "false" {
		t.Errorf("UPGRADE_SCHEDULER_ENABLED was dropped: %q", env["UPGRADE_SCHEDULER_ENABLED"])
	}
	if env["THUNDER_BASE_URL"] != "https://localhost:9443" {
		t.Errorf("THUNDER_BASE_URL: got %q, want the current URL", env["THUNDER_BASE_URL"])
	}
}

func TestWriteFrontendEnv(t *testing.T) {
	sampleDir := t.TempDir()
	envPath := serviceDir(t, sampleDir, "frontend")
	if err := os.WriteFile(envPath, []byte("VITE_THUNDER_APP_ID=wayfinder-app\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := writeFrontendEnv(sampleDir, "https://localhost:9443", sampleVars, true); err != nil {
		t.Fatalf("writeFrontendEnv: %v", err)
	}

	env := readEnv(t, envPath)
	if env["VITE_THUNDER_CLIENT_ID"] != "WAYFINDER" {
		t.Errorf("client id: got %q, want the id from thunderid.env", env["VITE_THUNDER_CLIENT_ID"])
	}
	if env["VITE_THUNDER_BASE_URL"] != "https://localhost:9443" {
		t.Errorf("base URL: got %q", env["VITE_THUNDER_BASE_URL"])
	}
	if env["VITE_AI_FEATURES_ENABLED"] != "true" {
		t.Errorf("AI flag: got %q, want true", env["VITE_AI_FEATURES_ENABLED"])
	}
	if env["VITE_THUNDER_APP_ID"] != "wayfinder-app" {
		t.Errorf("VITE_THUNDER_APP_ID was dropped: %q", env["VITE_THUNDER_APP_ID"])
	}

	if err := writeFrontendEnv(sampleDir, "https://localhost:9443", nil, false); err != nil {
		t.Fatalf("writeFrontendEnv without vars: %v", err)
	}
	env = readEnv(t, envPath)
	if env["VITE_THUNDER_CLIENT_ID"] != "WAYFINDER" {
		t.Errorf("client id fallback: got %q", env["VITE_THUNDER_CLIENT_ID"])
	}
	if env["VITE_AI_FEATURES_ENABLED"] != "false" {
		t.Errorf("AI flag: got %q, want false", env["VITE_AI_FEATURES_ENABLED"])
	}
}

// The backend validates tokens against THUNDER_BASE_URL, which ships with the
// default port baked in — it has to follow the port the product actually uses.
func TestWriteBaseURLEnvs(t *testing.T) {
	sampleDir := t.TempDir()
	backendEnv := serviceDir(t, sampleDir, "backend")
	if err := os.WriteFile(backendEnv, []byte("THUNDER_BASE_URL=https://localhost:8090\nAUTHORIZATION_MODE=scope\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := writeBaseURLEnvs(sampleDir, "https://localhost:9443"); err != nil {
		t.Fatalf("writeBaseURLEnvs: %v", err)
	}

	env := readEnv(t, backendEnv)
	if env["THUNDER_BASE_URL"] != "https://localhost:9443" {
		t.Errorf("THUNDER_BASE_URL: got %q, want the resolved URL", env["THUNDER_BASE_URL"])
	}
	if env["AUTHORIZATION_MODE"] != "scope" {
		t.Errorf("AUTHORIZATION_MODE was dropped: %q", env["AUTHORIZATION_MODE"])
	}
	// The lounge is absent here — a missing service must not fail the run.
	if _, err := os.Stat(filepath.Join(sampleDir, "lounge", ".env")); !os.IsNotExist(err) {
		t.Error("lounge env must not be created when the service is absent")
	}
}

// `npm run dev` survives a single workspace crashing, so readiness is decided by
// the ports rather than by the parent process.
func TestWaitForServices(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()
	openPort := listener.Addr().(*net.TCPAddr).Port

	if err := waitForServices([]serviceCheck{{port: openPort, name: "frontend"}}, "", time.Second); err != nil {
		t.Fatalf("open port must be reported ready: %v", err)
	}

	closed, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	deadPort := closed.Addr().(*net.TCPAddr).Port
	_ = closed.Close()

	logPath := filepath.Join(t.TempDir(), "sample.log")
	if err := os.WriteFile(logPath, []byte("Error: Please set an API key\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	err = waitForServices([]serviceCheck{{port: deadPort, name: "ai-agent"}}, logPath, 100*time.Millisecond)
	if err == nil {
		t.Fatal("a service that never binds its port must fail the run")
	}
	if !strings.Contains(err.Error(), "ai-agent") {
		t.Errorf("error must name the service: %v", err)
	}
	if !strings.Contains(err.Error(), "Please set an API key") {
		t.Errorf("error must include the log tail: %v", err)
	}
}
