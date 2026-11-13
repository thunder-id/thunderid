// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// packPackages holds the suites that run against the quickstart pack. Each run tests only its own
// plane's suites, as the Control Plane run does.
const packPackages = "./pack/..."

// runPack runs the pack suite: it unzips the quickstart pack, sets it up once, starts auth-dp, the
// Data Plane and the Control Plane, and runs the tests that author on the Control Plane and prove
// the result on the Data Plane. It returns the process exit code.
func runPack() int {
	fmt.Println("Plane: pack")
	// A group's runner leaves the cache to the runner that started the groups.
	if testutils.TestGroup() == "" {
		if err := cleanTestCache(); err != nil {
			fmt.Printf("%v\n", err)
			return 1
		}
	}

	if err := testutils.PreparePack(); err != nil {
		fmt.Printf("Failed to prepare the pack: %v\n", err)
		return 1
	}
	fmt.Println("Running the pack's setup...")
	if err := testutils.RunPackSetup(); err != nil {
		fmt.Printf("Failed to set up the pack: %v\n", err)
		return 1
	}
	if err := testutils.ApplyPackOverlays(); err != nil {
		fmt.Printf("Failed to configure the pack: %v\n", err)
		return 1
	}

	fmt.Println("Starting auth-dp, the data plane and the control plane...")
	if err := testutils.StartPack(); err != nil {
		fmt.Printf("Failed to start the pack: %v\n", err)
		return 1
	}
	defer testutils.StopPack()

	// Both tokens are fetched here once, so a plane that cannot sign its admin in fails the run
	// before any test does. The tests sign in for themselves and keep what they get.
	fmt.Println("Signing in to the control plane and the data plane...")
	if _, err := testutils.PackCPToken(); err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}
	if _, err := testutils.PackDPToken(); err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}

	if group := testutils.TestGroup(); group != "" {
		if err := runGroupTests(group); err != nil {
			fmt.Printf("there are test failures: %v\n", err)
			return 1
		}
		return 0
	}

	packages := []string{packPackages}
	if testPackage != "./..." {
		packages = []string{testPackage}
	}
	var filter []string
	if testRun != "" {
		filter = []string{"-run", testRun}
		fmt.Printf("Test filter: -run %s\n", testRun)
	}
	cmd := newGoTestCommand(filter, packages)
	cmd.Env = append(os.Environ(), testutils.PackEnv()...)
	if err := cmd.Run(); err != nil {
		fmt.Printf("there are test failures: %v\n", err)
		return 1
	}
	fmt.Println("All tests completed successfully!")
	return 0
}

// packTestPackages lists the packages under packPackages that hold tests, which is what the pack
// groups split between them. The shared helpers of packtest have none.
func packTestPackages() ([]string, error) {
	out, err := exec.Command("go", "list", "-f",
		"{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}", packPackages).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list the pack test packages: %w", err)
	}
	return strings.Fields(string(out)), nil
}
