// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/mockservices"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// controlPlanePackages holds the suites that run against the Control Plane binary rather than
	// the all-in-one server. Each run tests only its own plane's suites.
	controlPlanePackages = "./controlplane/..."

	// serverStartTimeout bounds the wait for the server to answer after it starts.
	serverStartTimeout = 2 * time.Minute

	// managementTokenLifetime outlasts any run, since a token from the trusted issuer is never
	// refreshed.
	managementTokenLifetime = 6 * time.Hour
)

var (
	// serverPort is the port of this run's server. Each test group has its own.
	serverPort = testutils.ServerPort()

	zipFilePattern string
	testRun        string
	testPackage    string
	// controlPlane is set by INTEGRATION_PLANE=control (make test_integration_cp).
	controlPlane bool
)

func main() {
	parseFlags()
	controlPlane = os.Getenv("INTEGRATION_PLANE") == "control"

	// The external services the tests stand in for run once for the whole run, shared by every group.
	if testutils.TestGroup() == "" {
		stopMockServices, err := mockservices.Start()
		if err != nil {
			fmt.Printf("Failed to start the mock services: %v\n", err)
			os.Exit(1)
		}
		defer stopMockServices()
	}
	if shouldRunGroups() {
		os.Exit(runGroups())
	}
	initTests()

	// The Control Plane run also boots the all-in-one server as the Data Plane the Control Plane
	// configures, so a suite can take an export from one plane to the other.
	if controlPlane {
		err := testutils.PrepareDataPlane()
		if err != nil {
			fmt.Printf("Failed to prepare the data plane: %v\n", err)
			os.Exit(1)
		}
	}

	// Step 1: Unzip the product
	err := testutils.UnzipProduct()
	if err != nil {
		fmt.Printf("Failed to unzip product: %v\n", err)
		os.Exit(1)
	}

	if controlPlane {
		err = testutils.InstallControlPlaneBinary()
		if err != nil {
			fmt.Printf("Failed to install the control plane binary: %v\n", err)
			os.Exit(1)
		}
	}

	// Step 2: Replace the resource files in the unzipped directory.
	err = testutils.ReplaceResources(zipFilePattern)
	if err != nil {
		fmt.Printf("Failed to replace resources: %v\n", err)
		os.Exit(1)
	}

	if group := testutils.TestGroup(); group != "" {
		err = testutils.ConfigureTestGroup(group, serverPort)
		if err != nil {
			fmt.Printf("Failed to configure test group %s: %v\n", group, err)
			os.Exit(1)
		}
	}

	// Step 3: Copy declarative resource fixtures for composite mode testing. The Control Plane
	// suite configures no composite store, so it has no use for them.
	if !controlPlane {
		err = testutils.CopyDeclarativeResources(zipFilePattern)
		if err != nil {
			fmt.Printf("Failed to copy declarative resources: %v\n", err)
			os.Exit(1)
		}
	}

	// Step 4: Run the init script to create the SQLite database
	err = testutils.RunInitScript(zipFilePattern)
	if err != nil {
		fmt.Printf("Failed to run init script: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Run setup.sh
	// This starts server without security, runs bootstrap scripts, and stops server
	fmt.Println("Running bootstrap scripts...")
	err = testutils.RunSetupScript()
	if err != nil {
		fmt.Printf("Failed to run setup script: %v\n", err)
		os.Exit(1)
	}

	// A Control Plane issues no tokens, so the identity provider it trusts must be up before it
	// serves a request.
	var issuer *testutils.TrustedIssuer
	if controlPlane {
		issuer, err = testutils.StartTrustedIssuer()
		if err != nil {
			fmt.Printf("Failed to start the trusted issuer: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = issuer.Stop() }()
	}

	// Step 6: Start server
	fmt.Println("Starting server with security enabled...")
	err = testutils.StartServer(serverPort, zipFilePattern)
	if err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}
	defer testutils.StopServer()

	if controlPlane {
		fmt.Println("Starting data plane...")
		err = testutils.StartDataPlane()
		if err != nil {
			fmt.Printf("Failed to start the data plane: %v\n", err)
			testutils.StopServer()
			os.Exit(1)
		}
		defer testutils.StopDataPlane()
	}

	// Wait for the server to start. Test groups start their servers at the same time, so allow for a
	// slow start on a busy machine.
	fmt.Println("Waiting for the server to start...")
	if err := testutils.WaitForServerReady(serverStartTimeout); err != nil {
		fmt.Printf("Server did not start: %v\n", err)
		testutils.StopServer()
		testutils.StopDataPlane()
		os.Exit(1)
	}

	// Step 7: Obtain admin access token once for all test packages
	fmt.Println("Obtaining admin access token...")
	if controlPlane {
		err = useTrustedIssuerTokens(issuer)
	} else {
		err = testutils.ObtainAdminAccessToken()
	}
	if err != nil {
		fmt.Printf("Failed to obtain admin access token: %v\n", err)
		testutils.StopServer()
		testutils.StopDataPlane()
		os.Exit(1)
	}

	// Step 8: Run all tests
	err = runTests()
	if err != nil {
		fmt.Printf("there are test failures: %v\n", err)
		testutils.StopServer()
		testutils.StopDataPlane()
		os.Exit(1)
	}

	fmt.Println("All tests completed successfully!")
}

func parseFlags() {
	flag.StringVar(&testRun, "run", "", "Run only tests matching the regular expression (passed to go test -run)")
	flag.StringVar(&testPackage, "package", "./...", "Package(s) to test (default: ./...)")
	flag.Parse()
}

// useTrustedIssuerTokens makes the trusted issuer's tokens the ones the Control Plane suites send:
// one carrying the system scope for every test client, one without it for the test that it is
// refused, and one for a user the issuer's user endpoint does not know.
func useTrustedIssuerTokens(issuer *testutils.TrustedIssuer) error {
	admin, err := testutils.IssueManagementToken(issuer, "system", managementTokenLifetime)
	if err != nil {
		return err
	}
	if err := testutils.UseAdminAccessToken(admin, managementTokenLifetime); err != nil {
		return err
	}
	unscoped, err := testutils.IssueManagementToken(issuer, "", managementTokenLifetime)
	if err != nil {
		return err
	}
	if err := os.Setenv(testutils.UnscopedTokenEnv, unscoped); err != nil {
		return err
	}
	stranger, err := testutils.IssueManagementTokenFor(issuer, testutils.TrustedIssuerStrangerSubject, "system",
		managementTokenLifetime)
	if err != nil {
		return err
	}
	return os.Setenv(testutils.StrangerTokenEnv, stranger)
}

func initTests() {
	if controlPlane {
		fmt.Println("Plane: control")
		testutils.UseControlPlane()
	}

	// Read database type from environment variable
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "sqlite" // Default to SQLite
	}
	fmt.Printf("Database type: %s\n", dbType)

	zipFilePattern = testutils.GetZipFilePattern()
	if zipFilePattern == "" {
		fmt.Println("Failed to determine the zip file pattern.")
		os.Exit(1)
	}

	// Initialize test context with the detected configuration
	testutils.InitializeTestContext(serverPort, zipFilePattern, dbType)

	fmt.Printf("Using zip file pattern: %s\n", zipFilePattern)
}

func runTests() error {
	if group := testutils.TestGroup(); group != "" {
		return runGroupTests(group)
	}

	if err := cleanTestCache(); err != nil {
		return err
	}

	var filter []string
	if testRun != "" {
		filter = []string{"-run", testRun}
		fmt.Printf("Test filter: -run %s\n", testRun)
	}
	packages, err := packagesToTest()
	if err != nil {
		return err
	}
	fmt.Printf("Test package: %s\n", strings.Join(packages, " "))

	return goTestCommand(filter, packages).Run()
}

// cleanTestCache cleans the test cache to avoid getting results from previous runs.
// This is important to avoid false positives in test results as the
// server and integration test suite are two separate applications.
func cleanTestCache() error {
	cmd := exec.Command("go", "clean", "-testcache")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clean test cache: %w", err)
	}
	return nil
}

// goTestCommand builds the go test run of the given packages against this run's server, with
// gotestsum when it is installed.
func goTestCommand(filter []string, packages []string) *exec.Cmd {
	var cmdName string
	var args []string

	if _, err := exec.LookPath("gotestsum"); err == nil {
		fmt.Println("Running integration tests using gotestsum...")
		cmdName = "gotestsum"
		args = append(args, "--format", "testname", "--", "-p=1")
	} else {
		fmt.Println("Running integration tests using go test...")
		cmdName = "go"
		args = append(args, "test", "-p=1", "-v")
	}
	args = append(args, filter...)
	args = append(args, packages...)

	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Export test-context state so that test subprocesses can self-initialize
	// via ensureInitialized() without requiring an explicit InitializeTestContext call.
	cmd.Env = append(os.Environ(),
		"SERVER_EXTRACTED_HOME="+testutils.GetExtractedProductHome(),
		"SERVER_PORT="+serverPort,
		"ZIP_PATTERN="+zipFilePattern,
		"SERVER_PID="+strconv.Itoa(testutils.GetServerPID()),
	)
	return cmd
}

// packagesToTest resolves the default of every package to the suites of the plane under test. A
// package named explicitly is tested as given.
func packagesToTest() ([]string, error) {
	if testPackage != "./..." {
		return []string{testPackage}, nil
	}
	if controlPlane {
		return []string{controlPlanePackages}, nil
	}
	out, err := exec.Command("go", "list", "./...").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list test packages: %w", err)
	}
	controlPlaneRoot := strings.TrimSuffix(strings.TrimPrefix(controlPlanePackages, "."), "/...")
	var packages []string
	for _, pkg := range strings.Fields(string(out)) {
		if strings.HasSuffix(pkg, controlPlaneRoot) || strings.Contains(pkg, controlPlaneRoot+"/") {
			continue
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}
