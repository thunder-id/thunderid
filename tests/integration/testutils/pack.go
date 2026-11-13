// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The pack suite runs the quickstart pack as shipped: auth-dp, the Data Plane it manages and the
// Control Plane, already wired together. Tests author on the Control Plane and prove the result on
// the Data Plane, so the harness starts all three planes and gives each test a client per plane.
const (
	// PackAuthDPPort, PackDPPort and PackCPPort are the ports the pack's planes answer on in the
	// default test group. The pack's scripts and bootstrap resources hardcode them, so another group's
	// unzipped copy has them rewritten by PackPortOffsetEnv's offset.
	PackAuthDPPort = 8091
	PackDPPort     = 8090
	PackCPPort     = 8095

	// PackPortOffsetEnv carries the number added to each of the pack's ports, so test groups running at
	// the same time each have a pack of their own. The runner sets it on the process of each group.
	PackPortOffsetEnv = "PACK_PORT_OFFSET"

	// PackExtractedDir is where the pack is unzipped, and PackLogDir where each plane logs. A test
	// group has directories of its own, named after it.
	PackExtractedDir = "../../target/out/.test-pack"
	PackLogDir       = "../../target/out/.test-pack-logs"

	// PackAdminUsername signs into the Control Plane through auth-dp, and PackServiceDeskUsername into
	// the Data Plane's Admin Console. Both use PackPassword.
	PackAdminUsername       = "admin"
	PackServiceDeskUsername = "servicedeskadmin"
	PackPassword            = AdminPassword

	// PackCPConsoleClientID is the client auth-dp signs the Control Plane's operators in with, and
	// PackAdminConsoleClientID the Data Plane's own Admin Console client, which the Control Plane
	// registers and syncs to the Data Plane.
	PackCPConsoleClientID    = "CP_CONSOLE"
	PackAdminConsoleClientID = "ADMIN_CONSOLE"

	// PackGatewayName is the name the pack registers the Data Plane under on the Control Plane.
	PackGatewayName = "dp"

	// PackHomeEnv carries the unzipped pack's path to the test packages.
	PackHomeEnv = "PACK_HOME"

	// packRemovalThreshold replaces gateway.removal_confirmation_threshold on the Control Plane, so a
	// test that deletes many resources is not refused with GTW-1030.
	packRemovalThreshold = 1000

	packControlPlane = "control plane"
	packDataPlane    = "data plane"

	packLoginAttempts = 3
	packReadyTimeout  = 2 * time.Minute
	packSyncTimeout   = 90 * time.Second
)

var (
	packHome  string
	packPlane = []*packProcess{
		{name: "auth-dp", dir: "auth-dp", script: "start.sh", port: PackAuthDPPort},
		{name: "dp", dir: "dp", script: "start.sh", port: PackDPPort},
		{name: "cp", dir: "cp", script: "start-cp.sh", port: PackCPPort},
	}

	// packLoginMu serializes the sign-ins, which point the shared OAuth helpers at one plane at a time.
	packLoginMu sync.Mutex
	packTokens  = map[string]*packToken{}
	packTokenMu sync.Mutex
)

type packProcess struct {
	name   string
	dir    string
	script string
	port   int
	cmd    *exec.Cmd
	log    *os.File
}

type packToken struct {
	value   string
	expires time.Time
}

// IsPackRun reports whether this process belongs to a run against the quickstart pack.
func IsPackRun() bool {
	return os.Getenv(IntegrationPlaneEnv) == "pack"
}

// packPortOffset is the number added to each of the pack's ports in this process.
func packPortOffset() int {
	offset, _ := strconv.Atoi(os.Getenv(PackPortOffsetEnv))
	return offset
}

// PackAuthDPURL, PackDPURL and PackCPURL are the base URLs of the pack's planes in this test group.
func PackAuthDPURL() string { return packURL(PackAuthDPPort) }
func PackDPURL() string     { return packURL(PackDPPort) }
func PackCPURL() string     { return packURL(PackCPPort) }

func packURL(port int) string {
	return fmt.Sprintf("https://localhost:%d", port+packPortOffset())
}

// packGroupDir is a directory of this test group: the directory itself in the default group, and
// one named after the group otherwise.
func packGroupDir(dir string) string {
	if group := TestGroup(); group != "" && group != "default" {
		return dir + "-" + group
	}
	return dir
}

// PackHome returns the unzipped pack's directory: the one PreparePack made in the runner, or the one
// the runner handed to a test package in PackHomeEnv.
func PackHome() string {
	if packHome != "" {
		return packHome
	}
	return os.Getenv(PackHomeEnv)
}

// PreparePack unzips the newest quickstart pack into PackExtractedDir.
func PreparePack() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("the pack suite drives the pack's bash scripts and does not run on windows")
	}
	zipFile, err := findPackZip()
	if err != nil {
		return err
	}
	log.Printf("Using quickstart pack %s", zipFile)

	extractedDir := packGroupDir(PackExtractedDir)
	if err := os.RemoveAll(extractedDir); err != nil {
		return fmt.Errorf("failed to clear the pack directory: %w", err)
	}
	if err := os.MkdirAll(extractedDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create the pack directory: %w", err)
	}
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		if err := extractFile(f, extractedDir); err != nil {
			return err
		}
	}

	home, err := findPackHome(extractedDir)
	if err != nil {
		return err
	}
	if packHome, err = filepath.Abs(home); err != nil {
		return err
	}
	if err := rewritePackPorts(packHome, packPortOffset()); err != nil {
		return err
	}
	return os.Setenv(PackHomeEnv, packHome)
}

// packPortPattern matches the pack's three ports as whole numbers.
var packPortPattern = regexp.MustCompile(`\b(` + strconv.Itoa(PackDPPort) + `|` + strconv.Itoa(PackAuthDPPort) + `|` +
	strconv.Itoa(PackCPPort) + `)\b`)

// packTextFileExtensions are the files of the pack that name its ports: scripts, deployment and
// bootstrap resources, and the consoles' runtime configuration (config.js). Binaries, keys and
// certificates are left as they are.
var packTextFileExtensions = map[string]bool{
	".sh": true, ".yaml": true, ".yml": true, ".js": true, ".json": true, ".env": true, ".html": true,
}

// rewritePackPorts moves every port the unzipped pack names up by offset, so the planes of a test
// group do not meet the planes of another. The pack hardcodes its ports in its scripts, its
// deployment.yaml files and its bootstrap resources, which is why they are edited as text.
func rewritePackPorts(home string, offset int) error {
	if offset == 0 {
		return nil
	}
	return filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !packTextFileExtensions[filepath.Ext(path)] {
			return err
		}
		// Of the consoles' scripts only the runtime configuration names a port.
		if filepath.Ext(path) == ".js" && filepath.Base(path) != "config.js" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rewritten := packPortPattern.ReplaceAllFunc(content, func(port []byte) []byte {
			number, _ := strconv.Atoi(string(port))
			return []byte(strconv.Itoa(number + offset))
		})
		if bytes.Equal(content, rewritten) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(path, rewritten, info.Mode().Perm())
	})
}

// findPackZip finds the newest quickstart pack in the target directory. Its name is the product, a
// version and the platform, which sets it apart from the gateway and Control Plane packs that carry
// a word before the version.
func findPackZip() (string, error) {
	goos, goarch := detectOSAndArchitecture()
	files, err := filepath.Glob(filepath.Join(TargetDir, fmt.Sprintf("thunderid-[0-9v]*-%s-%s.zip", goos, goarch)))
	if err != nil {
		return "", err
	}
	version := func(file string) string {
		name := strings.TrimSuffix(filepath.Base(file), fmt.Sprintf("-%s-%s.zip", goos, goarch))
		return strings.TrimPrefix(strings.TrimPrefix(name, "thunderid-"), "v")
	}
	var newest string
	for _, file := range files {
		if newest == "" || compareVersions(version(file), version(newest)) > 0 {
			newest = file
		}
	}
	if newest == "" {
		return "", fmt.Errorf("no quickstart pack matching thunderid-<version>-%s-%s.zip in %s, run build_quickstart first",
			goos, goarch, TargetDir)
	}
	return newest, nil
}

// findPackHome finds the directory holding the three planes. A zip built on a Unix host has a root
// folder; one built elsewhere may not.
func findPackHome(root string) (string, error) {
	candidates := []string{root}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			candidates = append(candidates, filepath.Join(root, entry.Name()))
		}
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "setup.sh")); err != nil {
			continue
		}
		missing := ""
		for _, plane := range packPlane {
			if _, err := os.Stat(filepath.Join(dir, plane.dir)); err != nil {
				missing = plane.dir
			}
		}
		if missing == "" {
			return dir, nil
		}
	}
	return "", fmt.Errorf("the zip is not a quickstart pack: no folder with setup.sh and auth-dp, dp and cp")
}

// RunPackSetup runs the pack's setup.sh once, with the accounts and keys the test packages sign in
// and call with pinned instead of generated. setup.sh kills whatever holds the pack's ports.
func RunPackSetup() error {
	cmd := exec.Command("bash", "setup.sh")
	cmd.Dir = packHome
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"ADMIN_USERNAME="+PackAdminUsername,
		"ADMIN_PASSWORD="+PackPassword,
		"SERVICE_DESK_USERNAME="+PackServiceDeskUsername,
		"SERVICE_DESK_PASSWORD="+PackPassword,
		"QUICKSTART_GATEWAY_KEY="+ManagementAPIKey,
	)
	log.Println("Running the pack's setup.sh, which bootstraps all three planes...")
	return cmd.Run()
}

// ApplyPackOverlays adds what the tests need to the planes' deployment.yaml. Each file already holds
// a top-level section of most names, and the loader refuses a key defined twice, so the Control
// Plane's gateway section is edited in place and the Data Plane's new section is appended.
func ApplyPackOverlays() error {
	cpYaml := filepath.Join(packHome, "cp", "deployment.yaml")
	err := insertAfterLine(cpYaml, "gateway:", fmt.Sprintf("  removal_confirmation_threshold: %d",
		packRemovalThreshold))
	if err != nil {
		return err
	}

	dpYaml := filepath.Join(packHome, "dp", "deployment.yaml")
	current, err := os.ReadFile(dpYaml)
	if err != nil {
		return err
	}
	if hasTopLevelKey(string(current), "identity_provider") {
		return fmt.Errorf("%s already has an identity_provider section", dpYaml)
	}
	overlay := fmt.Sprintf("\nidentity_provider:\n  google_base_url: %q\n  github_base_url: %q\n",
		"http://localhost:8093", "http://localhost:8092")
	return os.WriteFile(dpYaml, append(current, []byte(overlay)...), 0o600)
}

// hasTopLevelKey reports whether a YAML document has the key at its top level.
func hasTopLevelKey(content, key string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, key+":") {
			return true
		}
	}
	return false
}

// insertAfterLine inserts a line after the first line equal to anchor.
func insertAfterLine(path, anchor, line string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	for i, existing := range lines {
		if strings.TrimRight(existing, " \t\r") != anchor {
			continue
		}
		out := append([]string{}, lines[:i+1]...)
		out = append(out, line)
		out = append(out, lines[i+1:]...)
		return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o600)
	}
	return fmt.Errorf("%s has no %q line to add %q after", path, anchor, strings.TrimSpace(line))
}

// StartPack starts auth-dp, the Data Plane and the Control Plane, each from its own script with its
// own log, and waits until all three answer.
func StartPack() error {
	logDirPath := packGroupDir(PackLogDir)
	if err := os.MkdirAll(logDirPath, os.ModePerm); err != nil {
		return err
	}
	logDir, err := filepath.Abs(logDirPath)
	if err != nil {
		return err
	}
	for _, plane := range packPlane {
		logFile, err := os.Create(filepath.Join(logDir, plane.name+".log"))
		if err != nil {
			return err
		}
		cmd := exec.Command("bash", plane.script)
		cmd.Dir = filepath.Join(packHome, plane.dir)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			_ = logFile.Close()
			StopPack()
			return fmt.Errorf("failed to start %s: %w", plane.name, err)
		}
		plane.cmd, plane.log = cmd, logFile
		log.Printf("Started %s (pid %d), log: %s", plane.name, cmd.Process.Pid, logFile.Name())
	}
	for _, plane := range packPlane {
		if err := waitForHealth(packURL(plane.port), packReadyTimeout); err != nil {
			StopPack()
			return fmt.Errorf("%s did not start, see %s: %w", plane.name,
				filepath.Join(logDir, plane.name+".log"), err)
		}
	}
	return nil
}

// waitForHealth polls a plane's liveness endpoint until it answers.
func waitForHealth(baseURL string, timeout time.Duration) error {
	client := packPlainClient()
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health/liveness")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Errorf("liveness answered %d", resp.StatusCode)
		} else {
			last = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return last
}

// StopPack stops the three planes: a stop signal to each script, which stops the server it started,
// and then whatever still holds a port.
func StopPack() {
	for _, plane := range packPlane {
		if plane.cmd != nil && plane.cmd.Process != nil {
			_ = sendStopSignal(plane.cmd.Process)
		}
	}
	for _, plane := range packPlane {
		if plane.cmd != nil {
			done := make(chan struct{})
			go func(cmd *exec.Cmd) { _ = cmd.Wait(); close(done) }(plane.cmd)
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				_ = plane.cmd.Process.Kill()
			}
			plane.cmd = nil
		}
		if plane.log != nil {
			_ = plane.log.Close()
			plane.log = nil
		}
		killPort(plane.port + packPortOffset())
	}
}

// killPort kills whatever listens on a port, in case a server outlived its script.
func killPort(port int) {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port)).Output()
	if err != nil {
		return
	}
	for _, pid := range strings.Fields(string(out)) {
		_ = exec.Command("kill", "-9", pid).Run()
	}
}

// PackEnv is the environment a test package needs to find the pack.
func PackEnv() []string {
	return []string{
		IntegrationPlaneEnv + "=pack",
		PackHomeEnv + "=" + packHome,
	}
}

// packLogin signs a user in through a client on one plane with the authorization code flow and PKCE,
// and returns the token. The shared OAuth helpers call TestServerURL, so it is pointed at the plane
// for the length of the sign-in.
//
// A plane refuses a grant as revoked when a change to the user's roles or scopes, such as a sync that
// re-imports them, landed in the same second the grant was established. The sign-in is tried again
// after that second has passed.
func packLogin(baseURL, clientID, redirectURI, scope, resource, username, password string) (*TokenResponse, error) {
	packLoginMu.Lock()
	defer packLoginMu.Unlock()
	previous := TestServerURL
	TestServerURL = baseURL
	defer func() { TestServerURL = previous }()

	var response *TokenResponse
	var err error
	for attempt := 0; attempt < packLoginAttempts; attempt++ {
		response, err = ObtainAccessTokenWithPassword(clientID, redirectURI, scope, username, password, true, "",
			resource)
		if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
			return response, err
		}
		time.Sleep(1100 * time.Millisecond)
	}
	return response, err
}

// cachedPackToken returns a plane's token, signing in again when it has under five minutes left.
func cachedPackToken(plane string, signIn func() (*TokenResponse, error)) (string, error) {
	packTokenMu.Lock()
	defer packTokenMu.Unlock()
	if t := packTokens[plane]; t != nil && time.Now().Before(t.expires.Add(-5*time.Minute)) {
		return t.value, nil
	}
	response, err := signIn()
	if err != nil {
		return "", fmt.Errorf("failed to sign in to the %s: %w", plane, err)
	}
	packTokens[plane] = &packToken{
		value:   response.AccessToken,
		expires: time.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
	}
	return response.AccessToken, nil
}

// forgetPackToken drops a plane's token, so the next request signs in again.
func forgetPackToken(plane string) {
	packTokenMu.Lock()
	defer packTokenMu.Unlock()
	delete(packTokens, plane)
}

// PackCPToken returns a Control Plane token. The Control Plane issues none of its own: auth-dp signs
// the pack's admin in through CP_CONSOLE, for the Control Plane's own URL as the resource.
func PackCPToken() (string, error) {
	return cachedPackToken(packControlPlane, func() (*TokenResponse, error) {
		return packLogin(PackAuthDPURL(), PackCPConsoleClientID, PackCPURL()+"/console", "system", PackCPURL(),
			PackAdminUsername, PackPassword)
	})
}

// PackDPToken returns a Data Plane token for the Service Desk Admin, through the Data Plane's own
// Admin Console client.
func PackDPToken() (string, error) {
	return cachedPackToken(packDataPlane, func() (*TokenResponse, error) {
		return packLogin(PackDPURL(), PackAdminConsoleClientID, PackDPURL()+"/console", "system",
			PackDPURL()+"/mcp", PackServiceDeskUsername, PackPassword)
	})
}

// PackLoginAs signs a user in on the Data Plane through a client, with the authorization code flow and
// PKCE, as a person signing in to an application does. The error says why the sign-in was refused.
func PackLoginAs(clientID, redirectURI, scope, username, password string) (*TokenResponse, error) {
	return packLogin(PackDPURL(), clientID, redirectURI, scope, "", username, password)
}

// packPlainClient is an HTTP client that trusts the pack's self-signed certificate and sends no
// credentials of its own.
func packPlainClient() *http.Client {
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
}

// PackPlane is a client for one plane of the pack.
type PackPlane struct {
	// BaseURL is the plane's address.
	BaseURL string
	client  *http.Client
	headers map[string]string
	// name is the key of the plane's cached token, when it has one. A request refused as
	// unauthorized is sent once more with a fresh token, for a token the plane revoked since.
	name string
}

// PackCP is a client for the Control Plane that sends the pack admin's token.
func PackCP() *PackPlane {
	return &PackPlane{BaseURL: PackCPURL(), client: NewHTTPClientWithTokenProvider(PackCPToken), name: packControlPlane}
}

// PackDP is a client for the Data Plane that sends the Service Desk Admin's token.
func PackDP() *PackPlane {
	return &PackPlane{BaseURL: PackDPURL(), client: NewHTTPClientWithTokenProvider(PackDPToken), name: packDataPlane}
}

// PackDPManagement is a client for the Data Plane that presents the management API key, as a
// deployment pipeline does, for the variable store and the import.
func PackDPManagement() *PackPlane {
	return &PackPlane{BaseURL: PackDPURL(), client: packPlainClient(),
		headers: map[string]string{ManagementAPIKeyHeader: ManagementAPIKey}}
}

// PackAnonymous is a client for the plane at baseURL that sends no credentials.
func PackAnonymous(baseURL string) *PackPlane {
	return &PackPlane{BaseURL: baseURL, client: packPlainClient()}
}

// Do sends a request with a JSON body and returns the status and the raw response. A non-nil out
// receives the decoded response.
func (p *PackPlane) Do(method, path string, in, out any) (int, []byte, error) {
	return p.DoWithHeaders(method, path, in, out, nil)
}

// DoWithHeaders is Do with extra request headers.
func (p *PackPlane) DoWithHeaders(method, path string, in, out any, headers map[string]string) (int, []byte, error) {
	var payload []byte
	if in != nil {
		var err error
		if payload, err = json.Marshal(in); err != nil {
			return 0, nil, err
		}
	}
	status, raw, err := p.send(method, path, payload, out, headers)
	if err == nil && status == http.StatusUnauthorized && p.name != "" {
		forgetPackToken(p.name)
		status, raw, err = p.send(method, path, payload, out, headers)
	}
	return status, raw, err
}

// send sends one request.
func (p *PackPlane) send(method, path string, payload []byte, out any, headers map[string]string) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, p.BaseURL+path, body)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range p.headers {
		req.Header.Set(name, value)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if out != nil && len(raw) > 0 && resp.StatusCode < http.StatusMultipleChoices {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, raw, fmt.Errorf("failed to decode %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, raw, nil
}

// PostForm sends a form-encoded body, as the token endpoint takes, and returns the status and the raw
// response. A client that authenticates with a secret passes it in headers as a Basic Authorization.
func (p *PackPlane) PostForm(path string, form url.Values, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, p.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}
