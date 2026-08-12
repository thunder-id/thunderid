// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package sample downloads and launches use-case sample applications.
package sample

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/release"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
	"github.com/thunder-id/thunderid/tools/cli/internal/ui/spinner"
	"github.com/thunder-id/thunderid/tools/cli/internal/utils"
)

// Options carries use-case-specific configuration collected by the CLI before the sample starts.
type Options struct {
	Config    map[string]string // key/value pairs to write into the target service's .env
	EnvTarget string            // sample sub-dir to write the .env into (e.g. "ai-agent")
	Features  []string          // feature tags, e.g. ["ai"] — drive optional services and frontend flags
	Port      int               // port the running product is bound to; 0 = the default port
}

// hasFeature reports whether tag is present in opts.Features.
func hasFeature(opts Options, tag string) bool {
	for _, f := range opts.Features {
		if f == tag {
			return true
		}
	}
	return false
}

// knownSamples lists available use-case samples.
var knownSamples = map[string]struct {
	description string
	sampleURL   string
}{
	"wayfinder": {
		description: "B2C consumer app: Login, Sign-Up, Profile, Account Recovery, Internal User Onboarding",
		sampleURL:   "http://localhost:5173",
	},
}

// typeToDir mirrors the awk mapping in start.sh's setup_declarative_resources.
var typeToDir = map[string]string{
	"application":       "applications",
	"connection":        "connections",
	"flow":              "flows",
	"group":             "groups",
	"layout":            "layouts",
	"organization_unit": "organization_units",
	"resource_server":   "resource_servers",
	"role":              "roles",
	"theme":             "themes",
	"translation":       "translations",
	"user":              "users",
	"user_schema":       "user_schemas",
}

// ProgressEvent is a single update from RunAsync's progress channel.
// When Overwrite is true the receiver should replace its previous progress line
// in-place (mimicking \r behavior) rather than appending a new one.
type ProgressEvent struct {
	Line      string
	Overwrite bool
}

// Result is returned by RunAsync when the operation completes.
type Result struct {
	Proc      *exec.Cmd
	SampleURL string
	ServerURL string   // base URL confirmed by ResolveBaseURL; empty on error
	Features  []string // mirrors Options.Features so callers can display mode-aware output
	Err       error
}

// Run downloads the named sample, writes its resources into the product repository,
// restarts the product, and starts the sample's services. Progress is written to stdout.
// In verbose mode every step is printed on its own line; otherwise a progress bar
// overwrites in-place (matching the product download experience).
func Run(sampleName, installPath string, verbose bool, opts Options) error {
	_, sampleURL, serverURL, err := runWithResult(sampleName, installPath, opts,
		func(msg string) { fmt.Println("  " + msg) },
		func(pct int, msg string) {
			if verbose {
				if pct >= 0 {
					fmt.Printf("  %s  %d%%\n", msg, pct)
				} else {
					fmt.Println("  " + msg)
				}
			} else {
				if pct < 0 {
					fmt.Printf("\r\033[2K  %s", msg)
				} else {
					fmt.Printf("\r\033[2K  %s  %s  %3d%%", spinner.Render(pct), msg, pct)
				}
			}
		},
	)
	if err == nil {
		printSummary(sampleName, serverURL, sampleURL, opts.Features)
	}
	return err
}

// RunAsync runs the workflow in a goroutine, streaming ProgressEvents on the first
// channel. In non-verbose mode, events with Overwrite=true replace the previous
// bottom-status line (progress bar and inline status); non-overwrite events are
// appended to the message list. In verbose mode all events are non-overwrite.
// The second channel receives exactly one Result.
func RunAsync(sampleName, installPath string, verbose bool, opts Options) (<-chan ProgressEvent, <-chan Result) {
	progress := make(chan ProgressEvent, 64)
	result := make(chan Result, 1)

	send := func(line string) {
		select {
		case progress <- ProgressEvent{Line: line}:
		default:
		}
	}
	sendBar := func(line string) {
		if verbose {
			send(line)
			return
		}
		select {
		case progress <- ProgressEvent{Line: line, Overwrite: true}:
		default:
		}
	}

	go func() {
		defer close(progress)
		defer close(result)
		proc, sampleURL, serverURL, err := runWithResult(sampleName, installPath, opts,
			send,
			func(pct int, msg string) {
				if pct < 0 {
					send(msg)
				} else {
					sendBar(fmt.Sprintf("%s  %s  %3d%%", spinner.Render(pct), msg, pct))
				}
			},
		)
		result <- Result{Proc: proc, SampleURL: sampleURL, ServerURL: serverURL, Features: opts.Features, Err: err}
	}()
	return progress, result
}

func runWithResult(
	sampleName, installPath string,
	opts Options,
	progress func(string),
	onDownload release.ProgressFunc,
) (*exec.Cmd, string, string, error) {
	meta, ok := knownSamples[sampleName]
	if !ok {
		return nil, "", "", fmt.Errorf("unknown sample %q — available: %s", sampleName, availableList())
	}

	if err := checkNodeVersion(); err != nil {
		return nil, "", "", err
	}

	// Fetch latest version.
	version, err := release.FetchLatestVersion()
	if err != nil {
		return nil, "", "", fmt.Errorf("could not fetch latest version: %w", err)
	}

	// Download sample into the shared samples directory inside the product base dir.
	// Invalidate the cache when the fetched release version differs from what was previously downloaded.
	cacheDir := filepath.Join(filepath.Dir(installPath), "samples", sampleName)
	cachedVersion := readCachedSampleVersion(cacheDir)
	if cachedVersion != version {
		if cachedVersion != "" {
			_ = os.RemoveAll(cacheDir)
		}
		if err := release.DownloadSample(sampleName, version, cacheDir, onDownload); err != nil {
			return nil, "", "", fmt.Errorf("download failed: %w", err)
		}
		_ = writeCachedSampleVersion(cacheDir, version)
		progress(fmt.Sprintf("✓ Downloaded %s sample v%s", sampleName, version))
	} else {
		progress(fmt.Sprintf("Using existing %s sample at %s/samples/%s", sampleName, product.Slug, sampleName))
	}

	// Find config files inside the extracted sample (may be in a subdirectory).
	configYAML, configEnv, sampleDir, err := findSampleConfig(cacheDir)
	if err != nil {
		return nil, "", "", err
	}

	// Parse env variables.
	vars, err := parseEnvFile(configEnv)
	if err != nil {
		return nil, "", "", fmt.Errorf("could not read env file: %w", err)
	}

	// Stop the product. The REPL may have moved it off the default port after a
	// conflict, so every port operation below follows opts.Port when it is set.
	port := opts.Port
	if port <= 0 {
		port = health.DefaultPort
	}
	progress("Stopping " + product.Name + "...")
	setup.KillPort(port)
	setup.WaitForPortFree(port, 15*time.Second)

	// Find ThunderID root and write resource files.
	thunderRoot, err := setup.FindThunderRoot(installPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("could not find %s root: %w", product.Name, err)
	}
	progress("Writing wayfinder resources...")
	if err := writeResources(configYAML, vars, thunderRoot); err != nil {
		return nil, "", "", fmt.Errorf("could not write resources: %w", err)
	}

	// Start the product.
	progress("Starting " + product.Name + "...")
	proc, err := setup.StartBackgroundOnPort(installPath, false, opts.Port)
	if err != nil {
		return nil, "", "", fmt.Errorf("could not start %s: %w", product.Name, err)
	}

	// Wait for the product to be ready.
	serverURL, ready := health.ResolveBaseURL(port, 60*time.Second)
	if !ready {
		return proc, meta.sampleURL, "",
			fmt.Errorf("%s did not become ready within 60 seconds — check logs at %s",
				product.Name, setup.LogDir(installPath))
	}
	progress(fmt.Sprintf("%s ready at %s", product.Name, serverURL))

	// Install workspace dependencies if not already present.
	if _, err := os.Stat(filepath.Join(sampleDir, "node_modules")); os.IsNotExist(err) {
		progress("Installing dependencies...")
		installCmd := exec.Command("npm", "install", "--silent")
		installCmd.Dir = sampleDir
		if out, installErr := installCmd.CombinedOutput(); installErr != nil {
			return proc, meta.sampleURL, serverURL,
				fmt.Errorf("npm install failed: %w\n%s", installErr, out)
		}
	}

	// Write service .env files so each process starts with the right credentials.
	aiEnabled := hasFeature(opts, "ai")
	if err := writeFrontendEnv(sampleDir, serverURL, vars, aiEnabled); err != nil {
		return proc, meta.sampleURL, serverURL, fmt.Errorf("could not write frontend env: %w", err)
	}
	if err := writeBaseURLEnvs(sampleDir, serverURL); err != nil {
		return proc, meta.sampleURL, serverURL, fmt.Errorf("could not write service env: %w", err)
	}
	if aiEnabled && opts.EnvTarget != "" {
		if err := writeServiceEnv(sampleDir, serverURL, vars, opts); err != nil {
			return proc, meta.sampleURL, serverURL, fmt.Errorf("could not write %s env: %w", opts.EnvTarget, err)
		}
	}

	// Free ports held by a previous run's sample services before restarting. A
	// stale frontend still bound to 5173, for example, would otherwise push the
	// new dev server to another port while the browser keeps hitting the old one.
	ports := sampleServicePorts(aiEnabled)
	for _, p := range ports {
		setup.KillPort(p)
	}
	for _, p := range ports {
		setup.WaitForPortFree(p, 10*time.Second)
	}

	// Start sample services.
	progress("Starting " + sampleName + " services...")
	if err := startSampleServices(sampleDir, aiEnabled); err != nil {
		return proc, meta.sampleURL, serverURL, fmt.Errorf("could not start sample: %w", err)
	}

	// `npm run dev` keeps running when a single workspace exits, so a crashed
	// service is invisible to the parent process. Confirm each one is accepting
	// connections before reporting the sample as ready.
	progress("Waiting for " + sampleName + " services...")
	if err := waitForSampleServices(sampleDir, aiEnabled, sampleReadyTimeout); err != nil {
		return proc, meta.sampleURL, serverURL, err
	}

	return proc, meta.sampleURL, serverURL, nil
}

// defaultAuthMode is the sample auth mode the CLI provisions. The wayfinder
// config is organized by auth mode (e.g. "redirect", "app-native"); the
// redirect flow is the default consumer experience.
const defaultAuthMode = "redirect"

// findSampleConfig locates the config YAML and env file within dir.
// The ZIP may extract to a nested subdirectory, so we search one level deep.
// The config may live under an auth-mode subdirectory such as
// <slug>-config/redirect (current layout) or directly under <slug>-config
// (legacy layout); both are checked, in that order.
func findSampleConfig(dir string) (configYAML, configEnv, sampleDir string, err error) {
	candidates := []string{dir}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			candidates = append(candidates, filepath.Join(dir, e.Name()))
		}
	}
	configDir := product.Slug + "-config"
	configSubDirs := []string{filepath.Join(configDir, defaultAuthMode), configDir}
	for _, base := range candidates {
		for _, sub := range configSubDirs {
			yaml := filepath.Join(base, sub, configDir+".yaml")
			env := filepath.Join(base, sub, product.Slug+".env")
			if _, err := os.Stat(yaml); err == nil {
				return yaml, env, base, nil
			}
		}
	}
	return "", "", "", fmt.Errorf("%s/%s.yaml not found in %s",
		filepath.Join(configDir, defaultAuthMode), configDir, dir)
}

// parseEnvFile reads KEY=VALUE lines into a map.
func parseEnvFile(path string) (map[string]string, error) {
	vars := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		// Env file is optional.
		return vars, nil
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		if k = strings.TrimSpace(k); k != "" {
			vars[k] = strings.TrimSpace(v)
		}
	}
	return vars, scanner.Err()
}

// SampleDir returns the cache directory for the named sample given the versioned install path.
func SampleDir(installPath, sampleName string) string {
	return filepath.Join(filepath.Dir(installPath), "samples", sampleName)
}

// ReadServiceEnv reads key/value pairs from <sampleDir>/<envTarget>/.env so the
// REPL can pre-populate its prompts on subsequent runs. Returns an empty map if
// the file does not exist.
func ReadServiceEnv(sampleDir, envTarget string) map[string]string {
	vals, _ := parseEnvFile(filepath.Join(sampleDir, envTarget, ".env"))
	return vals
}

// writeResources splits the multi-document YAML, substitutes template variables,
// and writes each document to thunderRoot/config/resources/<type>/<id>.yaml,
// the directory the server loads declarative resources from at startup.
func writeResources(yamlPath string, vars map[string]string, thunderRoot string) error {
	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}

	content := substituteVars(string(raw), vars)
	docs := splitYAML(content)

	reResourceType := regexp.MustCompile(`(?m)^resource_type:\s*(\S+)`)
	reID := regexp.MustCompile(`(?m)^(?:id|handle):\s*(\S+)`)

	for i, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		m := reResourceType.FindStringSubmatch(doc)
		if m == nil {
			continue
		}
		resourceType := m[1]

		dir, ok := typeToDir[resourceType]
		if !ok {
			dir = resourceType + "s"
		}

		var filename string
		if idM := reID.FindStringSubmatch(doc); idM != nil {
			filename = idM[1] + ".yaml"
		} else {
			filename = fmt.Sprintf("%s_%04d.yaml", resourceType, i)
		}

		target := filepath.Join(thunderRoot, "config", "resources", dir)
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, filename), []byte(doc+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// substituteVars replaces {{.KEY}} template placeholders with values from vars.
func substituteVars(content string, vars map[string]string) string {
	re := regexp.MustCompile(`\{\{\.(\w+)\}\}`)
	return re.ReplaceAllStringFunc(content, func(m string) string {
		key := re.FindStringSubmatch(m)[1]
		if v, ok := vars[key]; ok {
			return v
		}
		return m
	})
}

// splitYAML splits a multi-document YAML string on "---" separators.
func splitYAML(content string) []string {
	var docs []string
	var cur strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "---" {
			if s := strings.TrimSpace(cur.String()); s != "" {
				docs = append(docs, s)
			}
			cur.Reset()
		} else {
			cur.WriteString(line)
			cur.WriteByte('\n')
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		docs = append(docs, s)
	}
	return docs
}

// writeFrontendEnv updates frontend/.env with ThunderID client config and the
// VITE_AI_FEATURES_ENABLED flag so the React dev server picks up the right mode.
// Keys the sample ships (VITE_THUNDER_APP_ID, VITE_AUTH_IS_REDIRECT_BASED, ...)
// are preserved.
func writeFrontendEnv(sampleDir, baseURL string, vars map[string]string, aiEnabled bool) error {
	path := filepath.Join(sampleDir, "frontend", ".env")
	env, err := parseEnvFile(path)
	if err != nil {
		return err
	}
	clientID := vars["WAYFINDER_CLIENT_ID"]
	if clientID == "" {
		clientID = "WAYFINDER"
	}
	env["VITE_THUNDER_CLIENT_ID"] = clientID
	env["VITE_THUNDER_BASE_URL"] = baseURL
	env["VITE_AI_FEATURES_ENABLED"] = strconv.FormatBool(aiEnabled)
	return writeEnvFile(path, env)
}

// baseURLServices are sample services whose .env ships with the default
// ThunderID URL baked in. They need rewriting whenever the product runs on a
// different port, or their token validation points at the wrong server.
var baseURLServices = []string{"backend", "lounge"}

// writeBaseURLEnvs points every such service at the URL the product actually
// came up on. Services the sample does not ship are skipped.
func writeBaseURLEnvs(sampleDir, baseURL string) error {
	for _, service := range baseURLServices {
		path := filepath.Join(sampleDir, service, ".env")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		env, err := parseEnvFile(path)
		if err != nil {
			return err
		}
		env["THUNDER_BASE_URL"] = baseURL
		if err := writeEnvFile(path, env); err != nil {
			return err
		}
	}
	return nil
}

// writeServiceEnv updates <opts.EnvTarget>/.env with the ThunderID credentials from
// the thunderid.env vars map plus every key/value in opts.Config. Keys are written
// under the names the sample reads; keys already in the file that the CLI has no
// value for (MCP_SERVER_URL, UPGRADE_SCHEDULER_ENABLED, ...) are preserved.
func writeServiceEnv(sampleDir, baseURL string, vars map[string]string, opts Options) error {
	path := filepath.Join(sampleDir, opts.EnvTarget, ".env")
	env, err := parseEnvFile(path)
	if err != nil {
		return err
	}

	env["THUNDER_BASE_URL"] = baseURL
	setIfNotEmpty(env, "AGENT_ID", vars["AGENT_CLIENT_ID"])
	setIfNotEmpty(env, "AGENT_SECRET", vars["AGENT_CLIENT_SECRET"])
	setIfNotEmpty(env, "UPGRADE_AGENT_ID", vars["UPGRADE_AGENT_CLIENT_ID"])
	setIfNotEmpty(env, "UPGRADE_AGENT_SECRET", vars["UPGRADE_AGENT_CLIENT_SECRET"])
	env["AGENT_REDIRECT_URI"] = "http://localhost:5173/agent-callback"
	env["AGENT_ACCESS_SCOPE"] = "agent:access"

	// The upgrade scheduler polls for pending upgrades over CIBA, which needs the
	// email or SMS flow configured. A try-out has neither, so it stays off unless
	// the operator turned it on themselves.
	if _, ok := env["UPGRADE_SCHEDULER_ENABLED"]; !ok {
		env["UPGRADE_SCHEDULER_ENABLED"] = "false"
	}

	for k, v := range opts.Config {
		env[k] = v
	}
	return writeEnvFile(path, env)
}

// setIfNotEmpty assigns value to key only when the source variable was populated,
// so a missing entry in thunderid.env leaves any existing value alone.
func setIfNotEmpty(env map[string]string, key, value string) {
	if value != "" {
		env[key] = value
	}
}

// writeEnvFile writes KEY=VALUE lines sorted by key, so repeated runs produce an
// identical file instead of reshuffling with Go's map iteration order.
func writeEnvFile(path string, env map[string]string) error {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + env[k] + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// sampleServicePorts returns the localhost ports the sample's dev services bind,
// so a previous run's orphaned processes can be freed before restarting. The
// ai-agent (8790) only runs in AI mode.
func sampleServicePorts(aiEnabled bool) []int {
	ports := []int{
		5173, // frontend (Vite)
		8787, // backend API
		8788, // SMTP inbox UI
		2525, // SMTP server
		8795, // lounge kiosk
	}
	if aiEnabled {
		ports = append(ports, 8790) // ai-agent
	}
	return ports
}

// sampleReadyTimeout bounds how long the sample's services get to bind their
// ports. A cold `npm run dev` builds the frontend first, so this is generous.
const sampleReadyTimeout = 120 * time.Second

// serviceCheck is one readiness probe: the port a service binds and the name to
// report when it never opens.
type serviceCheck struct {
	port int
	name string
}

// readinessChecks returns the services that must be up for the sample's
// walkthroughs to work. The SMTP inbox and lounge kiosk are optional extras, so
// they are started but not gated on.
func readinessChecks(aiEnabled bool) []serviceCheck {
	checks := []serviceCheck{
		{port: 5173, name: "frontend"},
		{port: 8787, name: "backend API"},
	}
	if aiEnabled {
		checks = append(checks, serviceCheck{port: 8790, name: "ai-agent"})
	}
	return checks
}

// waitForSampleServices blocks until every required service accepts connections.
func waitForSampleServices(sampleDir string, aiEnabled bool, timeout time.Duration) error {
	logPath := filepath.Join(sampleDir, "logs", "sample.log")
	return waitForServices(readinessChecks(aiEnabled), logPath, timeout)
}

// waitForServices polls each check until its port opens, and reports the offending
// service with the tail of its log once the shared deadline passes.
func waitForServices(checks []serviceCheck, logPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for _, check := range checks {
		for !setup.IsPortInUse(check.port) {
			if time.Now().After(deadline) {
				return fmt.Errorf("%s did not start on port %d — see %s\n%s",
					check.name, check.port, logPath, tailLog(logPath, 20))
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

// startSampleServices launches the sample services in the background via npm.
// For AgentID mode (aiEnabled=true) it runs `npm run dev` (all three services);
// otherwise it runs `npm run dev:b2c` (backend + frontend only).
func startSampleServices(sampleDir string, aiEnabled bool) error {
	logsDir := filepath.Join(sampleDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(logsDir, "sample.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logFile, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}

	script := "dev:b2c"
	if aiEnabled {
		script = "dev"
	}

	npmExe := "npm"
	if runtime.GOOS == "windows" {
		npmExe = "npm.cmd"
	}
	logPath := filepath.Join(logsDir, "sample.log")
	cmd := exec.Command(npmExe, "run", script)
	cmd.Dir = sampleDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile // never write to os.Stderr — it corrupts the Bubble Tea display
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return err
	}

	// Detect immediate failures (e.g. missing npm script) before returning.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		tail := tailLog(logPath, 10)
		if err != nil {
			return fmt.Errorf("sample services failed to start:\n%s", tail)
		}
	case <-time.After(2 * time.Second):
		// Still running — startup succeeded.
	}
	return nil
}

// tailLog returns the last n lines of the file at path, or a fallback message.
func tailLog(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "(no log available)"
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func printSummary(sampleName, baseURL, sampleURL string, features []string) {
	fmt.Println()
	fmt.Printf("  ✓ %s is ready at %s\n", product.Name, baseURL)
	fmt.Printf("  ✓ Wayfinder is running at %s\n", sampleURL)
	fmt.Println()

	if sampleName == "wayfinder" {
		fmt.Println("  Try these walkthroughs:")
		fmt.Println()
		if hasFeature(Options{Features: features}, "ai") {
			fmt.Println("    AI Concierge        → click the chat bubble and ask about flights")
			fmt.Println("    Book via Agent      → ask the concierge to book a flight — approve the consent prompt")
			fmt.Println("    Agent Identity      → open " + sampleURL + "/signin-as-agent")
		} else {
			fmt.Println("    Login               → sign in as john.doe / john.doe")
			fmt.Println("    Self Sign-Up        → create a new account at the frontend")
			fmt.Println("    View Profile        → sign in, open the Profile tab")
			fmt.Println("    Account Recovery    → click \"Forgot password?\" (requires SMTP in deployment.yaml)")
			fmt.Println("    Onboard Users       → sign in as alex.carter / alex.carter (Admin)")
		}
	}
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println()
}

func readCachedSampleVersion(dir string) string {
	data, _ := os.ReadFile(filepath.Join(dir, ".version"))
	return strings.TrimSpace(string(data))
}

func writeCachedSampleVersion(dir, version string) error {
	return os.WriteFile(filepath.Join(dir, ".version"), []byte(version+"\n"), 0o644)
}

// checkNodeVersion returns an error if Node.js is missing or older than
// utils.MinNodeVersion. Sample apps are installed and run via npm, so an
// unsupported Node.js version must stop the command before it downloads or
// touches anything.
func checkNodeVersion() error {
	version, err := utils.DetectNodeVersion()
	if err != nil {
		return err
	}
	if !utils.MeetsMinNodeVersion(version) {
		return fmt.Errorf("node.js v%s detected — v%s or later is required to run sample apps.\n%s",
			version, utils.MinNodeVersion, utils.NodeUpgradeHint())
	}
	return nil
}

func availableList() string {
	names := make([]string, 0, len(knownSamples))
	for k := range knownSamples {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
