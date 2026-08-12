// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package setup runs the ThunderID setup script and manages the background server process.
package setup

import (
	"bytes"
	"crypto/rand"
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
// Pass port=0 to use the default. When the setup run generated an admin password,
// the parsed credentials are returned; otherwise the returned credentials are nil.
func RunSetupOnPort(installPath string, verbose bool, port int) (*AdminCredentials, error) {
	root, err := FindThunderRoot(installPath)
	if err != nil {
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
	env := append(os.Environ(),
		"ADMIN_USERNAME="+adminUser,
		"ADMIN_PASSWORD="+adminPass,
	)
	if port > 0 {
		env = append(env, fmt.Sprintf("THUNDERID_PORT=%d", port))
	}
	cmd.Env = env
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
// Pass port=0 to use the default. Logs go to the state directory.
// The returned *exec.Cmd has already been started; call cmd.Process.Kill() to stop it.
func StartBackgroundOnPort(installPath string, verbose bool, port int) (*exec.Cmd, error) {
	root, err := FindThunderRoot(installPath)
	if err != nil {
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

// UpdateServerPort rewrites the server.port value in the deployment.yaml found under installPath.
func UpdateServerPort(installPath string, port int) error {
	candidates := []string{
		filepath.Join(installPath, "deployment.yaml"),
		filepath.Join(installPath, "backend", "cmd", "server", "deployment.yaml"),
	}
	var configPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			configPath = p
			break
		}
	}
	if configPath == "" {
		return fmt.Errorf("deployment.yaml not found in %s", installPath)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	inServer := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "server:" {
			inServer = true
			continue
		}
		if inServer {
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				inServer = false
				continue
			}
			if strings.HasPrefix(trimmed, "port:") {
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indent + fmt.Sprintf("port: %d", port)
				return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0o644)
			}
		}
	}
	return fmt.Errorf("server.port not found in %s", configPath)
}

var (
	killPortInUse   = IsPortInUse
	killPortCommand = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).CombinedOutput()
	}
	killPortSignal = func(pid int, signal syscall.Signal) error {
		process, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return process.Signal(signal)
	}
	killPortWait = WaitForPortFree
)

// KillPort terminates processes listening on port and verifies that the port is released.
func KillPort(port int) error {
	return killPortWithOS(port, runtime.GOOS)
}

func killPortWithOS(port int, goos string) error {
	if !killPortInUse(port) {
		return nil
	}

	if goos == "windows" {
		query := fmt.Sprintf(
			"(Get-NetTCPConnection -LocalPort %d -State Listen -ErrorAction SilentlyContinue).OwningProcess | Sort-Object -Unique",
			port)
		out, err := killPortCommand("powershell.exe", "-NoProfile", "-Command", query)
		if err != nil {
			return fmt.Errorf("failed to discover listener on port %d: %w: %s", port, err, strings.TrimSpace(string(out)))
		}
		pids, err := parseListenerPIDs(out, port)
		if err != nil {
			return err
		}
		for _, pid := range pids {
			taskkillOut, taskkillErr := killPortCommand("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
			if taskkillErr != nil {
				return fmt.Errorf("failed to terminate PID %d on port %d: %w: %s",
					pid, port, taskkillErr, strings.TrimSpace(string(taskkillOut)))
			}
		}
		if !killPortWait(port, 5*time.Second) {
			return fmt.Errorf("port %d remains occupied after termination", port)
		}
		return nil
	}

	out, err := killPortCommand("lsof", "-nP", fmt.Sprintf("-tiTCP:%d", port), "-sTCP:LISTEN")
	if err != nil {
		if !killPortInUse(port) {
			return nil
		}
		return fmt.Errorf("failed to discover listener on port %d: %w: %s", port, err, strings.TrimSpace(string(out)))
	}
	parsedPIDs, err := parseListenerPIDs(out, port)
	if err != nil {
		return err
	}
	for _, pid := range parsedPIDs {
		if signalErr := killPortSignal(pid, syscall.SIGTERM); signalErr != nil {
			return fmt.Errorf("failed to terminate PID %d on port %d: %w", pid, port, signalErr)
		}
	}
	if killPortWait(port, 5*time.Second) {
		return nil
	}
	for _, pid := range parsedPIDs {
		if signalErr := killPortSignal(pid, syscall.SIGKILL); signalErr != nil {
			return fmt.Errorf("failed to force terminate PID %d on port %d: %w", pid, port, signalErr)
		}
	}
	if !killPortWait(port, 5*time.Second) {
		return fmt.Errorf("port %d remains occupied after termination", port)
	}
	return nil
}

func parseListenerPIDs(out []byte, port int) ([]int, error) {
	pidStrings := strings.Fields(string(out))
	if len(pidStrings) == 0 {
		return nil, fmt.Errorf("no listener process found for occupied port %d", port)
	}
	pids := make([]int, 0, len(pidStrings))
	for _, pidString := range pidStrings {
		pid, parseErr := strconv.Atoi(strings.TrimSpace(pidString))
		if parseErr != nil || pid <= 0 {
			return nil, fmt.Errorf("invalid listener PID %q for port %d", pidString, port)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}
