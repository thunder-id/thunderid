/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package testutils

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	TargetDir                   = "../../target/dist"
	ExtractedDir                = "../../target/out/.test"
	TestDeploymentYamlPath      = "./resources/deployment.yaml"
	DefaultConfigJSONPath       = "../../backend/cmd/server/config/default.json"
	TestDatabaseSchemaDirectory = "resources/dbscripts"
	DatabaseFileBasePath        = "database/"
)

// ServerBinary is the name of the server binary, platform-dependent.
var ServerBinary string

func init() {
	if runtime.GOOS == "windows" {
		ServerBinary = "thunderid.exe"
	} else {
		ServerBinary = "thunderid"
	}
}

// Package-level variables for server configuration
var (
	serverPort           string
	zipFilePattern       string
	extractedProductHome string
	serverCmd            *exec.Cmd
	serverPid            int
	isInitialized        bool
	subprocessMode       bool
	dbType               string
)

// InitializeTestContext initializes the package-level variables for server configuration.
func InitializeTestContext(port string, zipPattern string, databaseType string) {
	serverPort = port
	zipFilePattern = zipPattern
	dbType = databaseType
	isInitialized = true
}

// GetExtractedProductHome returns the absolute path to the extracted product directory.
// main.go uses this to propagate the path to test subprocesses via an env var.
// Returning an absolute path ensures it resolves correctly regardless of the working
// directory of the consumer (e.g. a test subprocess running from a sub-package directory).
func GetExtractedProductHome() string {
	if extractedProductHome == "" {
		panic("Extracted product home is not set")
	}

	abs, err := filepath.Abs(extractedProductHome)
	if err != nil {
		return extractedProductHome
	}
	return abs
}

// GetServerPID returns the PID of the running the server process, or 0 if not started.
// main.go uses this to propagate the PID to test subprocesses via an env var so that
// they can stop and restart the server when required.
func GetServerPID() int {
	return serverPid
}

// ensureInitialized checks if the test context has been initialized.
// When the test context has not been set up via InitializeTestContext (e.g. when a
// test subprocess was started by runTests in main.go), it attempts a lazy
// initialization from environment variables that main.go exports before running tests:
//
//	SERVER_EXTRACTED_HOME – path to the extracted product directory
//	SERVER_PORT    – port the server is listening on
//	ZIP_PATTERN    – zip file glob used to locate the product archive
//	SERVER_PID     – PID of the running server process
func ensureInitialized() {
	if isInitialized {
		return
	}

	// Attempt lazy initialization from environment variables injected by main.go.
	if home := os.Getenv("SERVER_EXTRACTED_HOME"); home != "" {
		extractedProductHome = home
		serverPort = os.Getenv("SERVER_PORT")
		zipFilePattern = os.Getenv("ZIP_PATTERN")
		dbType = os.Getenv("DB_TYPE")
		if dbType == "" {
			dbType = "sqlite"
		}
		if pidStr := os.Getenv("SERVER_PID"); pidStr != "" {
			if pid, err := strconv.Atoi(pidStr); err == nil {
				serverPid = pid
			}
		}
		subprocessMode = true
		isInitialized = true
		return
	}

	panic("Test context not initialized. Call InitializeTestContext() first.")
}

func UnzipProduct() error {
	// Find the zip file.
	files, err := findMatchingZipFile(zipFilePattern)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("zip file not found in target directory")
	}

	// Unzip the file
	zipFile := files[0]
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(ExtractedDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create extraction directory: %v", err)
	}

	// Determine the extraction target directory.
	// Some zips (e.g., built on macOS/Linux) include a root directory prefix matching the zip name,
	// while others (e.g., built on Windows with ZipFile.CreateFromDirectory) do not.
	// Detect this by checking if any entry starts with the expected prefix.
	// We scan all entries because macOS archives may include metadata entries (e.g., __MACOSX/)
	// before the actual content directory.
	expectedPrefix := filepath.Base(zipFile[:len(zipFile)-4]) + "/"
	hasRootDir := false
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, expectedPrefix) {
			hasRootDir = true
			break
		}
	}

	extractDir := ExtractedDir
	if !hasRootDir {
		// Zip entries don't have the root directory prefix; extract into a subdirectory
		extractDir = filepath.Join(ExtractedDir, filepath.Base(zipFile[:len(zipFile)-4]))
		if err := os.MkdirAll(extractDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create extraction subdirectory: %v", err)
		}
	}

	for _, f := range r.File {
		err := extractFile(f, extractDir)
		if err != nil {
			return err
		}
	}

	productHome, err := getExtractedProductHome()
	if err != nil {
		return err
	}
	extractedProductHome = productHome

	// Set executable permissions for the server binary (not needed on Windows)
	if runtime.GOOS != "windows" {
		serverPath := filepath.Join(productHome, ServerBinary)
		if err := os.Chmod(serverPath, 0755); err != nil {
			return fmt.Errorf("failed to set executable permissions for server binary: %v", err)
		}
	}

	return nil
}

func extractFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Guard against zip path traversal (e.g., entries containing "../")
	path := filepath.Join(dest, f.Name)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dest)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path in zip: %s", f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(path, os.ModePerm)
	}

	os.MkdirAll(filepath.Dir(path), os.ModePerm)
	mode := f.Mode()
	if mode == 0 {
		mode = 0666
	}
	outFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)

	return err
}

// getExtractedProductHome constructs the path to the unzipped folder.
func getExtractedProductHome() (string, error) {
	files, err := findMatchingZipFile(zipFilePattern)
	if err != nil || len(files) == 0 {
		return "", fmt.Errorf("zip file not found in target directory")
	}
	zipFile := files[0]

	return filepath.Join(ExtractedDir, filepath.Base(zipFile[:len(zipFile)-4])), nil
}

// findMatchingZipFile finds zip files that match our specific version pattern criteria
func findMatchingZipFile(zipFilePattern string) ([]string, error) {
	path := filepath.Join(TargetDir, zipFilePattern)
	files, err := filepath.Glob(path)
	if err != nil {
		return nil, err
	}

	// Filter the files to only include those that have a version number or 'v' after 'thunderid-'
	var matchingFiles []string
	for _, file := range files {
		baseName := filepath.Base(file)
		parts := strings.Split(baseName, "-")
		if len(parts) >= 3 {
			// Check if the second part starts with a number or 'v'
			secondPart := parts[1]
			if len(secondPart) > 0 && (secondPart[0] == 'v' || (secondPart[0] >= '0' && secondPart[0] <= '9')) {
				matchingFiles = append(matchingFiles, file)
			}
		}
	}

	// Prefer the newest package version first to avoid selecting stale distributions.
	sort.SliceStable(matchingFiles, func(i, j int) bool {
		versionI := extractVersionFromZipName(filepath.Base(matchingFiles[i]))
		versionJ := extractVersionFromZipName(filepath.Base(matchingFiles[j]))

		cmp := compareVersions(versionI, versionJ)
		if cmp != 0 {
			return cmp > 0
		}

		infoI, errI := os.Stat(matchingFiles[i])
		infoJ, errJ := os.Stat(matchingFiles[j])
		if errI == nil && errJ == nil {
			return infoI.ModTime().After(infoJ.ModTime())
		}

		return matchingFiles[i] > matchingFiles[j]
	})

	return matchingFiles, nil
}

func extractVersionFromZipName(zipName string) string {
	parts := strings.Split(zipName, "-")
	if len(parts) < 3 {
		return ""
	}
	return strings.TrimPrefix(parts[1], "v")
}

// parseVersionSegment splits a segment like "0-rc1" into its numeric value and optional
// pre-release suffix. A segment with no suffix has preRelease == "".
func parseVersionSegment(seg string) (numeric int, preRelease string) {
	if idx := strings.Index(seg, "-"); idx >= 0 {
		if n, err := strconv.Atoi(seg[:idx]); err == nil {
			return n, seg[idx+1:]
		}
	}
	if n, err := strconv.Atoi(seg); err == nil {
		return n, ""
	}
	return 0, ""
}

func compareVersions(versionA string, versionB string) int {
	segmentsA := strings.Split(versionA, ".")
	segmentsB := strings.Split(versionB, ".")

	maxLen := len(segmentsA)
	if len(segmentsB) > maxLen {
		maxLen = len(segmentsB)
	}

	for i := 0; i < maxLen; i++ {
		var segA, segB string
		if i < len(segmentsA) {
			segA = segmentsA[i]
		}
		if i < len(segmentsB) {
			segB = segmentsB[i]
		}

		numA, preA := parseVersionSegment(segA)
		numB, preB := parseVersionSegment(segB)

		if numA != numB {
			if numA > numB {
				return 1
			}
			return -1
		}

		// Same numeric part: release (no suffix) > pre-release (has suffix).
		if preA == "" && preB != "" {
			return 1
		}
		if preA != "" && preB == "" {
			return -1
		}
		if preA != preB {
			if preA > preB {
				return 1
			}
			return -1
		}
	}

	return 0
}

func ReplaceResources(zipFilePattern string) error {
	log.Println("Replacing resources...")

	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current directory: %v", err)
	} else {
		log.Printf("Current working directory: %s", cwd)
	}

	destPath := filepath.Join(extractedProductHome, "deployment.yaml")

	// Ensure the destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create conf directory: %v", err)
	}

	err = copyFile(TestDeploymentYamlPath, destPath)
	if err != nil {
		return fmt.Errorf("failed to replace deployment.yaml: %v", err)
	}

	defaultConfigDestPath := filepath.Join(extractedProductHome, "config", "default.json")

	if err := os.MkdirAll(filepath.Dir(defaultConfigDestPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create resources conf directory: %v", err)
	}

	err = copyFile(DefaultConfigJSONPath, defaultConfigDestPath)
	if err != nil {
		return fmt.Errorf("failed to replace default.json: %v", err)
	}

	return nil
}

// CopyDeclarativeResources copies declarative resource fixtures from the test resources directory
// into the extracted product's config/resources directory.
// This enables the server to load declarative resources on startup.
func CopyDeclarativeResources(zipFilePattern string) error {
	log.Println("Copying declarative resources...")

	srcPath := "./resources/declarative_resources"
	destPath := filepath.Join(extractedProductHome, "config", "resources")

	// Check if source directory exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		log.Println("No declarative resources directory found, skipping copy")
		return nil
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create resources directory: %v", err)
	}

	// Copy each declarative resource subdirectory
	resourceDirs := []string{
		"agents",
		"applications",
		"flows",
		"identity_providers",
		"layouts",
		"organization_units",
		"resource_servers",
		"roles",
		"server_configs",
		"themes",
		"user_types",
		"users",
	}

	for _, dir := range resourceDirs {
		srcSubPath := filepath.Join(srcPath, dir)
		destSubPath := filepath.Join(destPath, dir)

		// Check if source subdirectory exists
		if _, err := os.Stat(srcSubPath); os.IsNotExist(err) {
			log.Printf("Declarative resource directory %s not found, skipping", dir)
			continue
		}

		// Create destination subdirectory
		if err := os.MkdirAll(destSubPath, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create declarative resource directory %s: %v", dir, err)
		}

		// Copy the directory contents
		if err := copyDirectory(srcSubPath, destSubPath); err != nil {
			return fmt.Errorf("failed to copy declarative resources for %s: %v", dir, err)
		}

		log.Printf("Copied declarative resources for %s", dir)
	}

	return nil
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)

	return err
}

func copyDirectory(src, dest string) error {
	srcDir, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcDir.Close()

	entries, err := srcDir.Readdir(-1)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			err = os.MkdirAll(destPath, os.ModePerm)
			if err != nil {
				return err
			}
			err = copyDirectory(srcPath, destPath)
			if err != nil {
				return err
			}
		} else {
			err = copyFile(srcPath, destPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func RunInitScript(zipFilePattern string) error {
	log.Println("Running init script...")

	// Skip database initialization for PostgreSQL
	if dbType == "postgres" {
		log.Println("Skipping database initialization for PostgreSQL (already initialized in workflow)")
		return nil
	}

	// Ensure the database directory exists
	dbDir := filepath.Join(extractedProductHome, DatabaseFileBasePath)
	if err := os.MkdirAll(dbDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create database directory: %v", err)
	}

	// Verify sqlite3 CLI is available before attempting database initialization
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return fmt.Errorf("sqlite3 CLI not found in PATH: please install sqlite3 to run SQLite integration tests")
	}

	// Initialize each SQLite database
	databases := []struct {
		name       string
		schemaDir  string
		dbFileName string
	}{
		{"configdb", "dbscripts/configdb", "configdb.db"},
		{"runtimedb", "dbscripts/runtimedb", "runtimedb.db"},
		{"userdb", "dbscripts/userdb", "userdb.db"},
		{"operationdb", "dbscripts/operationdb", "operationdb.db"},
	}

	for _, db := range databases {
		schemaPath := filepath.Join(extractedProductHome, db.schemaDir, "sqlite.sql")
		dbPath := filepath.Join(extractedProductHome, DatabaseFileBasePath, db.dbFileName)

		if err := initSQLiteDB(db.name, schemaPath, dbPath); err != nil {
			return err
		}
	}

	return nil
}

// initSQLiteDB creates a SQLite database from a schema file using the sqlite3 CLI.
func initSQLiteDB(name, schemaPath, dbPath string) error {
	log.Printf("Initializing SQLite database: %s", name)

	// Resolve to absolute paths for sqlite3 compatibility on Windows
	absSchemaPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to resolve schema path for %s: %v", name, err)
	}
	absDbPath, err := filepath.Abs(dbPath)
	if err != nil {
		return fmt.Errorf("failed to resolve db path for %s: %v", name, err)
	}

	// Remove existing database file for a clean start
	if err := os.Remove(absDbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing %s database: %v", name, err)
	}

	// Read schema file and pipe it to sqlite3 via stdin (avoids .read path issues on Windows)
	schemaFile, err := os.Open(absSchemaPath)
	if err != nil {
		return fmt.Errorf("failed to open schema file for %s: %v", name, err)
	}
	defer schemaFile.Close()

	cmd := exec.Command("sqlite3", absDbPath)
	cmd.Stdin = schemaFile
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize %s database: %v", name, err)
	}

	// Enable WAL mode
	cmd = exec.Command("sqlite3", absDbPath, "PRAGMA journal_mode=WAL;")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable WAL mode for %s: %v", name, err)
	}

	log.Printf("Successfully initialized %s database", name)
	return nil
}

// pidFilePath returns the path to the well-known PID file that always reflects the
// PID of the currently running the server process, even after subprocess restarts.
// Returns an empty string when extractedProductHome is not yet set, so callers
// can skip PID-file operations safely.
func pidFilePath() string {
	if extractedProductHome == "" {
		return ""
	}
	return filepath.Join(extractedProductHome, "thunderid.pid")
}

// writePidFile writes the given PID to the the server PID file.
func writePidFile(pid int) {
	path := pidFilePath()
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Printf("Warning: could not write PID file: %v", err)
	}
}

// readPidFile reads the PID from the server PID file. Returns 0 if the file does
// not exist, cannot be parsed, or extractedProductHome is not set.
func readPidFile() int {
	path := pidFilePath()
	if path == "" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// removePidFile deletes the server PID file.
func removePidFile() {
	path := pidFilePath()
	if path == "" {
		return
	}
	os.Remove(path)
}

func StartServer(port string, _ string) error {
	log.Println("Starting server...")
	return startServerInternal(port)
}

// startServerInternal launches the server binary with the standard args plus any extra args supplied.
// All callers share this implementation so logging and env-var setup stay consistent.
func startServerInternal(port string, extraArgs ...string) error {
	if err := ensureDirectAuthSecretInDeploymentConfig(); err != nil {
		return fmt.Errorf("failed to ensure Direct Auth secret in deployment.yaml: %w", err)
	}

	serverPath := filepath.Join(extractedProductHome, ServerBinary)
	args := append([]string{"-serverHome=" + extractedProductHome}, extraArgs...)
	cmd := exec.Command(serverPath, args...)

	// logFile is non-nil only when subprocessMode opens a log file. The parent
	// must close its copy after cmd.Start() so it does not leak the FD.
	var logFile *os.File
	if subprocessMode {
		// Running inside a test binary (go test subprocess). Server's stdout/stderr
		// must NOT inherit the test process's pipes — go test waits for all I/O to
		// drain before declaring the test done, and the long-lived server process
		// would keep those pipes open indefinitely, causing a 60s WaitDelay timeout.
		// Redirect to a log file in the extracted product home so output is not lost.
		logPath := filepath.Join(extractedProductHome, "thunderid-restart.log")
		var openErr error
		logFile, openErr = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr != nil {
			log.Printf("Warning: could not open server log file %s, discarding output: %v", logPath, openErr)
			cmd.Stdout = nil
			cmd.Stderr = nil
		} else {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Preserve GOCOVERDIR environment variable for coverage collection
	envVars := []string{
		"PORT=" + port,
	}

	if goCoverDir := os.Getenv("GOCOVERDIR"); goCoverDir != "" {
		envVars = append(envVars, "GOCOVERDIR="+goCoverDir)
		log.Printf("Coverage collection enabled: GOCOVERDIR=%s\n", goCoverDir)
	}
	cmd.Env = append(os.Environ(), envVars...)

	err := cmd.Start()
	if err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return fmt.Errorf("failed to start server: %v", err)
	}
	// The child process has inherited the log file FD; close the parent's copy.
	if logFile != nil {
		logFile.Close()
	}
	serverCmd = cmd
	serverPid = cmd.Process.Pid
	writePidFile(serverPid)

	return nil
}

// RestartServerWithResourcesFile stops the running server and restarts it with the -resources flag
// pointing to the given single-file declarative resources YAML. Call RestartServer to return to
// normal mode afterwards.
func RestartServerWithResourcesFile(resourcesFilePath string) error {
	ensureInitialized()
	log.Printf("Restarting server with resources file: %s", resourcesFilePath)

	StopServer()
	time.Sleep(3 * time.Second)

	if err := startServerInternal(serverPort, "-resources="+resourcesFilePath); err != nil {
		return fmt.Errorf("failed to start server with resources file: %w", err)
	}

	if err := waitForServerReady(30 * time.Second); err != nil {
		return fmt.Errorf("server did not become ready after restart: %w", err)
	}

	return nil
}

func StopServer() {
	log.Println("Stopping server...")

	if serverCmd != nil {
		// Normal case: we started this process and hold its Cmd handle.
		if err := sendStopSignal(serverCmd.Process); err != nil {
			log.Printf("Failed to send stop signal: %v, forcing kill...", err)
			serverCmd.Process.Kill()
			serverCmd.Wait()
		} else {
			// Wait for the process to exit (with timeout)
			done := make(chan error, 1)
			go func() {
				done <- serverCmd.Wait()
			}()

			select {
			case <-done:
				// Process exited gracefully
				// Give a brief moment for coverage files to be fully flushed to disk
				time.Sleep(100 * time.Millisecond)
			case <-time.After(3 * time.Second):
				// Timeout - force kill
				log.Println("Server did not stop gracefully, forcing kill...")
				serverCmd.Process.Kill()
				<-done // Wait for the goroutine's Wait() call to complete
			}
		}
		serverCmd = nil
	} else if serverPid != 0 {
		// Subprocess case: we know the PID but did not start the process via Cmd.
		// os.FindProcess always succeeds on Unix; use the handle only for signalling.
		proc, err := os.FindProcess(serverPid)
		if err == nil && proc != nil {
			if err := sendStopSignal(proc); err != nil {
				log.Printf("Failed to send stop signal to PID %d: %v, forcing kill...", serverPid, err)
				proc.Kill()
			} else {
				// We cannot call proc.Wait() for a process we did not fork,
				// so we give the server a grace period and then ensure it is dead.
				// Check liveness before killing to avoid hitting a recycled PID.
				time.Sleep(3 * time.Second)
				if isProcessAlive(proc) {
					proc.Kill()
				}
			}
			time.Sleep(100 * time.Millisecond)
		} else {
			log.Printf("Could not find process with PID %d: %v", serverPid, err)
		}
	}

	serverCmd = nil
	serverPid = 0

	// Kill any residual the server process that may have been started by a test
	// subprocess (e.g. after a config swap) whose PID is different from the one
	// we just killed. The PID file is always updated by StartServer regardless of
	// which process (parent or subprocess) called it.
	if residualPid := readPidFile(); residualPid != 0 {
		if proc, err := os.FindProcess(residualPid); err == nil && proc != nil {
			_ = sendStopSignal(proc)
			time.Sleep(2 * time.Second)
			if isProcessAlive(proc) {
				_ = proc.Kill()
			}
		}
		removePidFile()
	}
}

// RestartServer stops the current server and starts a new one with the same configuration.
func RestartServer() error {
	ensureInitialized()
	log.Println("Restarting server...")

	// Stop the current server if it exists
	StopServer()

	// Wait a moment for the port to be released
	time.Sleep(3 * time.Second)

	// Start a new server instance
	err := StartServer(serverPort, zipFilePattern)
	if err != nil {
		return fmt.Errorf("failed to restart server: %v", err)
	}

	if err := waitForServerReady(30 * time.Second); err != nil {
		return fmt.Errorf("server did not become ready after restart: %w", err)
	}

	return nil
}

// waitForServerReady polls the server's health endpoint until it responds with a 2xx
// status or the timeout is exceeded. A polling interval of 500ms is used.
func waitForServerReady(timeout time.Duration) error {
	healthURL := "https://localhost:" + serverPort + "/health/liveness"
	client := GetHTTPClient()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				log.Println("Server is ready")
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("server did not become ready within %s", timeout)
}

// UpdateDeploymentConfig overwrites the extracted product's deployment.yaml with the
// file at srcPath. srcPath should be relative to the calling test package's directory.
// After calling this, restart the server with RestartServer for changes to take effect.
func UpdateDeploymentConfig(srcPath string) error {
	ensureInitialized()

	destPath := filepath.Join(extractedProductHome, "deployment.yaml")
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to ensure conf directory exists: %w", err)
	}

	if err := copyFile(srcPath, destPath); err != nil {
		return fmt.Errorf("failed to update deployment.yaml from %s: %w", srcPath, err)
	}

	log.Printf("Deployment config updated from %s", srcPath)
	return nil
}

// PatchDeploymentConfig reads the live deployment.yaml from the extracted product,
// merges the provided patch over the existing top-level keys, and writes it back.
// All keys not present in patch are preserved exactly as-is, making this safe to call
// in any environment (SQLite, PostgreSQL, etc.).
func PatchDeploymentConfig(patch map[string]interface{}) error {
	ensureInitialized()

	configPath := filepath.Join(extractedProductHome, "deployment.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read deployment.yaml: %w", err)
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse deployment.yaml: %w", err)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	for k, v := range patch {
		cfg[k] = v
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal updated deployment.yaml: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write updated deployment.yaml: %w", err)
	}

	return nil
}

// ensureDirectAuthSecretInDeploymentConfig guarantees server.security.direct_auth_secret is set to the
// fixture value in the extracted product's deployment.yaml. The server is secure by default (an empty
// secret blocks the Direct API endpoints), and suites that rewrite deployment.yaml via UpdateDeploymentConfig
// can drop server.security. Reapplying the secret before every server start keeps the gate satisfied
// regardless of prior config churn, matching the header the test clients inject.
func ensureDirectAuthSecretInDeploymentConfig() error {
	configPath := filepath.Join(extractedProductHome, "deployment.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read deployment.yaml: %w", err)
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse deployment.yaml: %w", err)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	server, ok := cfg["server"].(map[string]interface{})
	if !ok {
		server = make(map[string]interface{})
	}
	security, ok := server["security"].(map[string]interface{})
	if !ok {
		security = make(map[string]interface{})
	}
	if security["direct_auth_secret"] == DirectAuthHeaderValue {
		return nil
	}
	security["direct_auth_secret"] = DirectAuthHeaderValue
	server["security"] = security
	cfg["server"] = server

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal updated deployment.yaml: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write updated deployment.yaml: %w", err)
	}

	return nil
}

// ReadDeploymentConfigKey returns a top-level key from the deployment.yaml, or nil if missing.
func ReadDeploymentConfigKey(key string) (interface{}, error) {
	ensureInitialized()

	configPath := filepath.Join(extractedProductHome, "deployment.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployment.yaml: %w", err)
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse deployment.yaml: %w", err)
	}

	return cfg[key], nil
}

// RunSetupScript runs the setup script from the extracted product directory.
// This script starts the server without security, runs bootstrap scripts, and stops the server.
func RunSetupScript() error {
	ensureInitialized()

	// Get absolute path to extracted product home
	absProductHome, err := filepath.Abs(extractedProductHome)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		log.Println("Running setup.ps1 from extracted product...")
		setupScript := filepath.Join(absProductHome, "setup.ps1")
		cmd = exec.Command("pwsh", "-File", setupScript)
	} else {
		log.Println("Running setup.sh from extracted product...")
		setupScript := filepath.Join(absProductHome, "setup.sh")
		cmd = exec.Command("bash", setupScript)
	}

	cmd.Dir = absProductHome // Run from product directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Pin the Direct Auth Secret and admin credentials so every setup run (main and re-runs) seeds
	// the same values the test clients use, instead of generating fresh ones each time.
	cmd.Env = append(os.Environ(),
		"DIRECT_AUTH_SECRET="+DirectAuthHeaderValue,
		"ADMIN_USERNAME="+AdminUsername,
		"ADMIN_PASSWORD="+AdminPassword,
	)

	log.Println("Setup script will start server, run bootstrap, and stop server automatically")

	return cmd.Run()
}

func GetZipFilePattern() string {
	goos, goarch := detectOSAndArchitecture()
	// Use a more general pattern, the filtering will happen in findMatchingZipFile
	return fmt.Sprintf("thunderid-*-%s-%s.zip", goos, goarch)
}

// GetDBType returns the configured database type ("sqlite" or "postgres").
func GetDBType() string {
	ensureInitialized()
	return dbType
}

// QueryConfigDB executes a SQL query against the configdb SQLite database and returns
// the trimmed output produced by the sqlite3 CLI. Returns an error if the database
// type is not SQLite or if the sqlite3 CLI is unavailable.
func QueryConfigDB(sqlQuery string) (string, error) {
	ensureInitialized()
	if dbType != "sqlite" {
		return "", fmt.Errorf("QueryConfigDB is only supported for sqlite, current db type: %s", dbType)
	}
	dbPath := filepath.Join(extractedProductHome, DatabaseFileBasePath, "configdb.db")
	absDBPath, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve configdb path: %v", err)
	}
	cmd := exec.Command("sqlite3", absDBPath, sqlQuery)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("sqlite3 query failed: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// detectOSAndArchitecture detects the OS and architecture using Go environment variables
// or falls back to system detection if environment variables are not available
func detectOSAndArchitecture() (string, string) {
	// Try to get from environment variables first
	goos := os.Getenv("GOOS")
	goarch := os.Getenv("GOARCH")

	// If GOOS is not set, try to detect from system
	if goos == "" {
		// Try using go env command first
		cmd := exec.Command("go", "env", "GOOS")
		output, err := cmd.Output()
		if err == nil {
			goos = strings.TrimSpace(string(output))
		}

		// Fallback to uname if go env didn't work
		if goos == "" {
			cmd := exec.Command("uname", "-s")
			output, err := cmd.Output()
			if err == nil {
				osName := strings.TrimSpace(string(output))
				switch {
				case osName == "Darwin":
					goos = "darwin"
				case osName == "Linux":
					goos = "linux"
				case strings.HasPrefix(osName, "MINGW") ||
					strings.HasPrefix(osName, "MSYS") ||
					strings.HasPrefix(osName, "CYGWIN"):
					goos = "windows"
				}
			}
		}
	}

	// If GOARCH is not set, try to detect from system
	if goarch == "" {
		// Try using go env command first
		cmd := exec.Command("go", "env", "GOARCH")
		output, err := cmd.Output()
		if err == nil {
			goarch = strings.TrimSpace(string(output))
		}

		// Fall back to uname if go env didn't work
		if goarch == "" {
			cmd := exec.Command("uname", "-m")
			output, err := cmd.Output()
			if err == nil {
				arch := strings.TrimSpace(string(output))
				switch arch {
				case "x86_64", "amd64":
					goarch = "amd64"
				case "arm64", "aarch64":
					goarch = "arm64"
				}
			}
		}
	}

	// Normalize OS name according to distribution packaging
	if goos == "darwin" {
		goos = "macos"
	} else if goos == "windows" {
		goos = "win"
	}

	// Normalize architecture
	if goarch == "amd64" {
		goarch = "x64"
	}

	return goos, goarch
}
