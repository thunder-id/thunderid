// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package setup runs the ThunderID setup script and manages the background server process.
package setup

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
)

// LogDir returns the directory where ThunderID background logs are written
// (e.g. ./thunderid/v0.41.0/logs/).
func LogDir(installPath string) string {
	return filepath.Join(installPath, "logs")
}

// LogFile returns the dated log file path for the current day
// (e.g. ./thunderid/v0.41.0/logs/thunderid-2026-06-05.log).
func LogFile(installPath string) string {
	return filepath.Join(LogDir(installPath), product.Slug+"-"+time.Now().Format("2006-01-02")+".log")
}

// pruneOldLogs removes log files older than 7 days from LogDir.
func pruneOldLogs(installPath string) {
	dir := LogDir(installPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -7)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, e.Name())) //nolint:errcheck
		}
	}
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func findScript(installPath, name string) string {
	root := filepath.Join(installPath, name)
	if _, err := os.Stat(root); err == nil {
		return root
	}
	entries, err := os.ReadDir(installPath)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		nested := filepath.Join(installPath, e.Name(), name)
		if _, err := os.Stat(nested); err == nil {
			return nested
		}
	}
	return ""
}

// FindThunderRoot returns the directory containing the setup script.
func FindThunderRoot(installPath string) (string, error) {
	scriptName := "setup.sh"
	if isWindows() {
		scriptName = "setup.ps1"
	}
	script := findScript(installPath, scriptName)
	if script == "" {
		return "", fmt.Errorf("setup script not found in %s", installPath)
	}
	return filepath.Dir(script), nil
}

// AdminCredentials holds the admin username and password surfaced by setup, so the
// caller can display them after the setup spinner has finished rather than printing
// them mid-run (which interleaves with the spinner and is hidden by the REPL's
// alternate screen).
type AdminCredentials struct {
	Username string
	Password string
}

// RunSetup executes the platform setup script non-interactively on the default port.
func RunSetup(installPath string, verbose bool) (*AdminCredentials, error) {
	return RunSetupOnPort(installPath, verbose, 0)
}

// RunSetupOnPort executes the platform setup script with an optional custom port.
// Pass port=0 to keep the configured one. When the setup run generated an admin
// password, the parsed credentials are returned; otherwise the returned credentials
// are nil.
func RunSetupOnPort(installPath string, verbose bool, port int) (*AdminCredentials, error) {
	root, err := FindThunderRoot(installPath)
	if err != nil {
		return nil, err
	}

	// setup.sh derives the base URL from deployment.yaml, so the port has to be
	// written there before it runs: there is no environment override.
	if err := EnsureServerPort(installPath, port); err != nil {
		return nil, err
	}

	var cmd *exec.Cmd
	if isWindows() {
		cmd = exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-File", "setup.ps1")
	} else {
		cmd = exec.Command("bash", "setup.sh")
	}
	cmd.Dir = root
	adminUser := os.Getenv("THUNDERID_ADMIN_USERNAME")
	if adminUser == "" {
		adminUser = "admin"
	}
	// Left empty when not supplied: setup.sh/setup.ps1 treat an empty value as not
	// provided, so they generate a random password rather than falling back to a
	// fixed, predictable one.
	adminPass := os.Getenv("THUNDERID_ADMIN_PASSWORD")
	cmd.Env = append(os.Environ(),
		"ADMIN_USERNAME="+adminUser,
		"ADMIN_PASSWORD="+adminPass,
	)
	cmd.Stdin = nil // no stdin → prevents any remaining interactive prompts

	if verbose {
		// Mirror to the terminal live, but also capture so the credentials can be
		// re-surfaced inside the REPL (which draws on the alternate screen).
		var outBuf bytes.Buffer
		cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, err
		}
		return parseAdminCredentials(outBuf.String()), nil
	}

	// Non-verbose: capture stdout+stderr so we can surface them on failure.
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(errBuf.String() + "\n" + outBuf.String())
		detail = strings.TrimSpace(detail)
		if detail != "" {
			return nil, fmt.Errorf("%w\n\n%s", err, detail)
		}
		return nil, fmt.Errorf("%w\n\nRun with --verbose for full setup output", err)
	}
	return parseAdminCredentials(outBuf.String()), nil
}

// GenerateAdminPassword returns a random 12-character password using the same
// character set and constraints as setup.sh (at least one digit and one special
// character). The CLI generates it up-front so it can be shown as the default value
// in the interactive prompt before setup runs.
func GenerateAdminPassword() string {
	const (
		digits  = "0123456789"
		special = "@#%+=_.?-"
		letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		charset = letters + digits + special
		length  = 12
	)
	for {
		b := make([]byte, length)
		for i := range b {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				panic("crypto/rand unavailable: " + err.Error())
			}
			b[i] = charset[n.Int64()]
		}
		s := string(b)
		if strings.ContainsAny(s, digits) && strings.ContainsAny(s, special) {
			return s
		}
	}
}

// parseAdminCredentials extracts the admin username and password from captured setup
// output. setup.sh/setup.ps1 print an "Admin credentials:" block followed by a blank
// line, but only when the password was generated this run; returns nil when no such
// block is present.
func parseAdminCredentials(output string) *AdminCredentials {
	start := strings.Index(output, "Admin credentials:")
	if start == -1 {
		return nil
	}
	block := output[start:]
	if idx := strings.Index(block, "\n\n"); idx != -1 {
		block = block[:idx+1]
	} else if idx := strings.Index(block, "\r\n\r\n"); idx != -1 {
		block = block[:idx+2]
	}
	creds := &AdminCredentials{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if v := strings.TrimPrefix(line, "Username:"); v != line {
			creds.Username = strings.TrimSpace(v)
		} else if v := strings.TrimPrefix(line, "Password:"); v != line {
			creds.Password = strings.TrimSpace(v)
		}
	}
	if creds.Username == "" && creds.Password == "" {
		return nil
	}
	return creds
}

// StartBackground starts ThunderID detached from the terminal on the default port.
func StartBackground(installPath string, verbose bool) (*exec.Cmd, error) {
	return StartBackgroundOnPort(installPath, verbose, 0)
}

// StartBackgroundOnPort starts ThunderID detached from the terminal with an optional custom port.
// Pass port=0 to keep the configured one. Logs go to the state directory.
// The returned *exec.Cmd has already been started; call cmd.Process.Kill() to stop it.
func StartBackgroundOnPort(installPath string, verbose bool, port int) (*exec.Cmd, error) {
	root, err := FindThunderRoot(installPath)
	if err != nil {
		return nil, err
	}

	// The server binds the port from deployment.yaml; BACKEND_PORT below only drives
	// start.sh's pre-flight check. Write the port first so the two agree.
	if err := EnsureServerPort(installPath, port); err != nil {
		return nil, err
	}

	os.MkdirAll(LogDir(installPath), 0o755) //nolint:errcheck
	pruneOldLogs(installPath)
	out, err := os.OpenFile(LogFile(installPath), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		out, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}

	var cmd *exec.Cmd
	if isWindows() {
		startPs1 := filepath.Join(root, "start.ps1")
		if _, err := os.Stat(startPs1); err == nil {
			cmd = exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-File", "start.ps1")
		} else {
			binary := filepath.Join(root, product.Slug+".exe")
			if _, err := os.Stat(binary); err != nil {
				return nil, fmt.Errorf("no start.ps1 or %s.exe found in %s", product.Slug, root)
			}
			cmd = exec.Command(binary)
		}
	} else {
		startSh := filepath.Join(root, "start.sh")
		if _, err := os.Stat(startSh); err == nil {
			cmd = exec.Command("bash", "start.sh")
		} else {
			binary := filepath.Join(root, "thunder")
			if _, err := os.Stat(binary); err != nil {
				return nil, fmt.Errorf("no start.sh or "+product.Name+" binary found in %s", root)
			}
			cmd = exec.Command(binary)
		}
	}

	cmd.Dir = root
	if port > 0 {
		cmd.Env = append(os.Environ(), fmt.Sprintf("BACKEND_PORT=%d", port))
	}
	if verbose {
		cmd.Stdout = io.MultiWriter(out, os.Stderr)
		cmd.Stderr = io.MultiWriter(out, os.Stderr)
	} else {
		cmd.Stdout = out
		cmd.Stderr = out
	}
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// Start finds and runs the ThunderID start script or binary with inherited stdio.
func Start(installPath string, args []string) error {
	root, err := FindThunderRoot(installPath)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd

	if isWindows() {
		startPs1 := filepath.Join(root, "start.ps1")
		if _, err := os.Stat(startPs1); err == nil {
			cmd = exec.Command("powershell.exe", append([]string{"-ExecutionPolicy", "Bypass", "-File", "start.ps1"}, args...)...)
			cmd.Dir = root
		} else {
			binary := filepath.Join(root, product.Slug+".exe")
			if _, err := os.Stat(binary); err != nil {
				return fmt.Errorf("no start.ps1 or %s.exe found in %s", product.Slug, root)
			}
			cmd = exec.Command(binary, args...)
			cmd.Dir = root
		}
	} else {
		startSh := filepath.Join(root, "start.sh")
		if _, err := os.Stat(startSh); err == nil {
			cmd = exec.Command("bash", append([]string{"start.sh"}, args...)...)
			cmd.Dir = root
		} else {
			binary := filepath.Join(root, "thunder")
			if _, err := os.Stat(binary); err != nil {
				return fmt.Errorf("no start.sh or "+product.Name+" binary found in %s", root)
			}
			cmd = exec.Command(binary, args...)
			cmd.Dir = root
		}
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WaitForPortFree blocks until no process is accepting connections on the given TCP port,
// or until timeout elapses. Returns true if the port became free, false if it timed out.
func WaitForPortFree(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("localhost:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err != nil {
			return true
		}
		_ = conn.Close()
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// IsPortInUse returns true if a process is already accepting connections on the given TCP port.
func IsPortInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// FindFreePort returns the first free TCP port at or above start.
func FindFreePort(start int) int {
	for port := start; port < 65535; port++ {
		if !IsPortInUse(port) {
			return port
		}
	}
	return start
}

// deploymentConfigPath returns the deployment.yaml the install reads its
// configuration from, or "" when none is present.
func deploymentConfigPath(installPath string) string {
	candidates := []string{
		filepath.Join(installPath, "deployment.yaml"),
		filepath.Join(installPath, "backend", "cmd", "server", "deployment.yaml"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// serverPortLineIndex returns the index of the server.port line in lines, or -1.
// Only a direct child of server: counts: a port: nested one level deeper belongs
// to another mapping and is a different setting.
func serverPortLineIndex(lines []string) int {
	inServer := false
	childIndent := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "server:" {
			inServer = true
			childIndent = -1
			continue
		}
		if !inServer {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent == 0 {
			inServer = false
			continue
		}
		if childIndent == -1 {
			childIndent = indent // first key under server: sets the child level
		}
		if indent == childIndent && strings.HasPrefix(trimmed, "port:") {
			return i
		}
	}
	return -1
}

// ReadServerPort returns the port configured in the install's deployment.yaml,
// which is the only place the server reads its port from, or 0 when it cannot be
// determined.
func ReadServerPort(installPath string) int {
	if installPath == "" {
		return 0
	}
	configPath := deploymentConfigPath(installPath)
	if configPath == "" {
		return 0
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	i := serverPortLineIndex(lines)
	if i < 0 {
		return 0
	}
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), "port:"))
	if idx := strings.Index(value, "#"); idx != -1 {
		value = strings.TrimSpace(value[:idx])
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

// ServerPort returns the port the install serves on: the configured one, or the
// default when deployment.yaml does not say. This is the single answer to "which port
// is this install on" that the start, attach, upgrade and sample paths all need.
func ServerPort(installPath string) int {
	if port := ReadServerPort(installPath); port > 0 {
		return port
	}
	return health.DefaultPort
}

// EnsureServerPort makes port the value the server will read from deployment.yaml,
// so the port the CLI works with and the port the server binds cannot diverge.
// Passing port=0 leaves the configured value untouched.
func EnsureServerPort(installPath string, port int) error {
	if port <= 0 || ReadServerPort(installPath) == port {
		return nil
	}
	return UpdateServerPort(installPath, port)
}

// UpdateServerPort rewrites the server.port value in the deployment.yaml found under installPath.
func UpdateServerPort(installPath string, port int) error {
	configPath := deploymentConfigPath(installPath)
	if configPath == "" {
		return fmt.Errorf("deployment.yaml not found in %s", installPath)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	i := serverPortLineIndex(lines)
	if i < 0 {
		return fmt.Errorf("server.port not found in %s", configPath)
	}
	indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
	lines[i] = indent + fmt.Sprintf("port: %d", port)
	return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0o644)
}

// listenerPIDs returns the PIDs listening on the given TCP port. An empty result with
// a nil error means nothing is listening; an error means the listeners could not be
// inspected at all (the lookup tool is missing), which callers must not mistake for
// a free port.
func listenerPIDs(port int) ([]int, error) {
	var out []byte
	var err error
	if isWindows() {
		out, err = exec.Command("cmd", "/c",
			fmt.Sprintf("netstat -aon | findstr LISTENING | findstr :%d", port)).Output()
	} else {
		out, err = exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port)).Output()
	}
	if err != nil {
		// Both tools exit non-zero when nothing matches, which is not a failure.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not inspect the processes on port %d: %w", port, err)
	}
	if isWindows() {
		return parseNetstatPIDs(string(out), port), nil
	}
	return parsePIDs(strings.Fields(string(out))), nil
}

// PortHolder describes one process listening on a local TCP port.
type PortHolder struct {
	Port int
	PID  int    // 0 when the process holding the port could not be identified
	Name string // process name, empty when it could not be read
}

// String renders the holder as one prompt line, e.g. "5173  node (pid 61201)".
func (h PortHolder) String() string {
	switch {
	case h.PID == 0:
		return fmt.Sprintf("%d  unknown process", h.Port)
	case h.Name == "":
		return fmt.Sprintf("%d  pid %d", h.Port, h.PID)
	default:
		return fmt.Sprintf("%d  %s (pid %d)", h.Port, h.Name, h.PID)
	}
}

// PortHolders returns the processes listening on the given ports, skipping the free
// ones, so a prompt can name what it is about to stop. A port that is taken but whose
// owner cannot be looked up yields a holder with PID 0: the conflict is real even when
// the process behind it is not visible.
func PortHolders(ports ...int) []PortHolder {
	var holders []PortHolder
	for _, port := range ports {
		pids, err := listenerPIDs(port)
		if err != nil || len(pids) == 0 {
			if IsPortInUse(port) {
				holders = append(holders, PortHolder{Port: port})
			}
			continue
		}
		// One process can hold several sockets on a port (IPv4 and IPv6), and the
		// lookup reports each of them.
		seen := make(map[int]bool, len(pids))
		for _, pid := range pids {
			if seen[pid] {
				continue
			}
			seen[pid] = true
			holders = append(holders, PortHolder{Port: port, PID: pid, Name: processName(pid)})
		}
	}
	return holders
}

// processName returns the executable name of pid, or "" when it cannot be read.
func processName(pid int) string {
	if isWindows() {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV").Output()
		if err != nil {
			return ""
		}
		return parseTasklistName(string(out))
	}
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	// ps reports a full path for some processes; the basename is what a prompt needs.
	return filepath.Base(strings.TrimSpace(string(out)))
}

// parseTasklistName returns the image name from a `tasklist /NH /FO CSV` row, or "".
// A run that matched nothing still exits 0 and prints an informational line, which
// parses as a single field; a real row also carries pid, session and memory size.
func parseTasklistName(out string) string {
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil || len(records) == 0 || len(records[0]) < 2 {
		return ""
	}
	return records[0][0]
}

// parseNetstatPIDs extracts the PIDs from `netstat -aon` LISTENING lines whose local
// address ends in :port, so a remote port with the same number is ignored.
func parseNetstatPIDs(out string, port int) []int {
	var pids []int
	suffix := fmt.Sprintf(":%d", port)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasSuffix(fields[1], suffix) {
			continue
		}
		pids = append(pids, parsePIDs(fields[len(fields)-1:])...)
	}
	return pids
}

// parsePIDs converts PID strings to positive ints, skipping anything unparseable.
func parsePIDs(fields []string) []int {
	var pids []int
	for _, f := range fields {
		pid, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

// KillPort asks every process listening on the given TCP port to stop. It returns an
// error only when the listeners could not be looked up; use FreePort when the port
// has to be confirmed free.
func KillPort(port int) error {
	pids, err := listenerPIDs(port)
	if err != nil {
		return err
	}
	signalPIDs(pids, false)
	return nil
}

// FreePort stops whatever is listening on the given TCP port and waits for the port to
// be released, escalating from a polite stop to a forced kill. It reports why the port
// is still taken instead of leaving the caller to assume success.
func FreePort(port int, timeout time.Duration) error {
	pids, err := listenerPIDs(port)
	if err != nil {
		return err
	}
	if len(pids) == 0 && !IsPortInUse(port) {
		return nil
	}
	signalPIDs(pids, false)

	// Give the polite stop most of the budget, then escalate with what is left.
	if WaitForPortFree(port, timeout*3/4) {
		return nil
	}
	survivors, err := listenerPIDs(port)
	if err != nil {
		return err
	}
	signalPIDs(survivors, true)
	if WaitForPortFree(port, timeout/4) {
		return nil
	}
	if len(survivors) > 0 {
		return fmt.Errorf("port %d is still held by %s", port, formatPIDs(survivors))
	}
	return fmt.Errorf("port %d is still in use and the process holding it could not be identified", port)
}

// signalPIDs stops the given processes: forced kills use SIGKILL, polite ones SIGTERM.
// Windows has no SIGTERM, so taskkill stands in for both: without /f it asks the process
// to close, with /f it is terminated. /t covers the children too, matching the process
// group the unix path signals — the server is started through a launcher script, so the
// process that holds the port is usually a child.
func signalPIDs(pids []int, force bool) {
	for _, pid := range pids {
		if isWindows() {
			args := []string{"/pid", strconv.Itoa(pid), "/t"}
			if force {
				args = append(args, "/f")
			}
			_ = exec.Command("taskkill", args...).Run()
			continue
		}
		p, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		sig := syscall.SIGTERM
		if force {
			sig = syscall.SIGKILL
		}
		p.Signal(sig) //nolint:errcheck
	}
}

// formatPIDs renders PIDs as "pid 12" or "pids 12, 34".
func formatPIDs(pids []int) string {
	parts := make([]string, 0, len(pids))
	for _, pid := range pids {
		parts = append(parts, strconv.Itoa(pid))
	}
	if len(parts) == 1 {
		return "pid " + parts[0]
	}
	return "pids " + strings.Join(parts, ", ")
}

// TailFile returns the last n lines of the file at path, trailing blank lines trimmed,
// or nil when the file cannot be read or is empty.
func TailFile(path string, n int) []string {
	if n <= 0 {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, "\r")
	}
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// LogTail returns the last n lines of the current background log as one block, or ""
// when the log cannot be read. It is how the CLI surfaces the server's own reason for
// a failed start (a rejected bind, a bad configuration) instead of guessing.
func LogTail(installPath string, n int) string {
	return strings.Join(TailFile(LatestLogFile(installPath), n), "\n")
}

// LatestLogFile returns the most recently modified log file in LogDir, falling back to
// today's LogFile. A start that crosses midnight writes to the file it truncated on the
// previous day, so the dated path alone would point at the wrong log.
func LatestLogFile(installPath string) string {
	dir := LogDir(installPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return LogFile(installPath)
	}
	newest, newestTime := "", time.Time{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest, newestTime = filepath.Join(dir, e.Name()), info.ModTime()
		}
	}
	if newest == "" {
		return LogFile(installPath)
	}
	return newest
}
