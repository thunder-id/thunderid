// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// groupsFile lists the test groups. Each group runs against its own server and databases, and the
	// groups run at the same time. The suites of a group run one after another.
	groupsFile = "groups.yaml"

	// defaultGroup runs every suite no group in groupsFile lists.
	defaultGroup = "default"

	// groupPortStep separates the ports of the group servers. The default group keeps the default port.
	groupPortStep = 100

	modulePath = "github.com/thunder-id/thunderid/tests/integration"
)

var (
	groupNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	testFuncPattern  = regexp.MustCompile(`(?m)^func (Test\w+)\(t \*testing\.T\)`)
)

type groupsManifest struct {
	Groups []testGroup `yaml:"groups"`
}

// testGroup is a set of whole packages and of individual suites from other packages.
type testGroup struct {
	Name     string              `yaml:"name"`
	Packages []string            `yaml:"packages"`
	Suites   map[string][]string `yaml:"suites"`
}

// testInvocation is one go test run of a group: packages, optionally narrowed by a -run or -skip
// filter.
type testInvocation struct {
	packages []string
	filter   []string
}

// groupResult is how a group's run ended.
type groupResult struct {
	name     string
	duration time.Duration
	err      error
}

// shouldRunGroups reports whether this run splits into groups. Selecting tests with -run or -package,
// and the Control Plane run, keep a single server.
func shouldRunGroups() bool {
	return testutils.TestGroup() == "" && !controlPlane && testRun == "" && testPackage == "./..."
}

// loadGroups reads groupsFile and returns the default group followed by the groups it lists.
func loadGroups() ([]testGroup, error) {
	data, err := os.ReadFile(groupsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", groupsFile, err)
	}
	var manifest groupsManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", groupsFile, err)
	}

	groups := []testGroup{{Name: defaultGroup}}
	names := map[string]bool{defaultGroup: true}
	for _, group := range manifest.Groups {
		if !groupNamePattern.MatchString(group.Name) {
			return nil, fmt.Errorf("group name %q must be lower case letters, digits and underscores", group.Name)
		}
		if names[group.Name] {
			return nil, fmt.Errorf("group %q is defined more than once", group.Name)
		}
		names[group.Name] = true
		groups = append(groups, group)
	}
	return groups, nil
}

// planGroups resolves every group to the go test runs that cover its suites, and checks that each
// listed package and suite exists and belongs to one group only.
func planGroups(groups []testGroup, packages []string) (map[string][]testInvocation, error) {
	known := map[string]bool{}
	for _, pkg := range packages {
		known[pkg] = true
	}
	wholeOwner := map[string]string{}
	suiteOwner := map[string]map[string]string{}

	for _, group := range groups[1:] {
		for _, rel := range group.Packages {
			pkg := importPath(rel)
			if !known[pkg] {
				return nil, fmt.Errorf("group %q lists unknown package %q", group.Name, rel)
			}
			if owner, ok := wholeOwner[pkg]; ok {
				return nil, fmt.Errorf("package %q is listed by groups %q and %q", rel, owner, group.Name)
			}
			wholeOwner[pkg] = group.Name
		}
		for rel, suites := range group.Suites {
			pkg := importPath(rel)
			if !known[pkg] {
				return nil, fmt.Errorf("group %q lists suites of unknown package %q", group.Name, rel)
			}
			defined, err := testFunctions(rel)
			if err != nil {
				return nil, err
			}
			if suiteOwner[pkg] == nil {
				suiteOwner[pkg] = map[string]string{}
			}
			for _, suite := range suites {
				if !defined[suite] {
					return nil, fmt.Errorf("group %q lists %s, which package %q does not define", group.Name, suite, rel)
				}
				if owner, ok := suiteOwner[pkg][suite]; ok {
					return nil, fmt.Errorf("%s is listed by groups %q and %q", suite, owner, group.Name)
				}
				suiteOwner[pkg][suite] = group.Name
			}
		}
	}
	for pkg, owners := range suiteOwner {
		if owner, ok := wholeOwner[pkg]; ok {
			for _, suiteGroup := range owners {
				return nil, fmt.Errorf("package %q is listed by group %q, so group %q cannot take its suites",
					strings.TrimPrefix(pkg, modulePath+"/"), owner, suiteGroup)
			}
		}
	}

	plans := map[string][]testInvocation{}
	var unclaimed []string
	for _, pkg := range packages {
		if _, ok := wholeOwner[pkg]; ok {
			continue
		}
		claimed := suiteOwner[pkg]
		if len(claimed) == 0 {
			unclaimed = append(unclaimed, pkg)
			continue
		}
		plans[defaultGroup] = append(plans[defaultGroup], testInvocation{
			packages: []string{pkg}, filter: []string{"-skip", suitePattern(keys(claimed))},
		})
	}
	if len(unclaimed) > 0 {
		plans[defaultGroup] = append([]testInvocation{{packages: unclaimed}}, plans[defaultGroup]...)
	}
	for _, group := range groups[1:] {
		var whole []string
		for _, rel := range group.Packages {
			whole = append(whole, importPath(rel))
		}
		if len(whole) > 0 {
			plans[group.Name] = append(plans[group.Name], testInvocation{packages: whole})
		}
		for _, rel := range sortedKeys(group.Suites) {
			plans[group.Name] = append(plans[group.Name], testInvocation{
				packages: []string{importPath(rel)}, filter: []string{"-run", suitePattern(group.Suites[rel])},
			})
		}
	}
	return plans, nil
}

// runGroups starts one runner process per group, each with its own server, and waits for them all.
func runGroups() int {
	groups, err := loadGroups()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	packages, err := packagesToTest()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if _, err := planGroups(groups, packages); err != nil {
		fmt.Println(err)
		return 1
	}
	if err := cleanTestCache(); err != nil {
		fmt.Println(err)
		return 1
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("failed to locate the runner executable: %v\n", err)
		return 1
	}

	defaultPort, err := strconv.Atoi(testutils.DefaultServerPort)
	if err != nil {
		fmt.Printf("invalid default server port: %v\n", err)
		return 1
	}

	var output sync.Mutex
	results := make([]groupResult, len(groups))
	var wg sync.WaitGroup
	for i, group := range groups {
		port := strconv.Itoa(defaultPort + i*groupPortStep)
		fmt.Printf("Starting test group %s on port %s\n", group.Name, port)
		wg.Add(1)
		go func(i int, name, port string) {
			defer wg.Done()
			started := time.Now()
			err := runGroupProcess(executable, name, port, &output)
			results[i] = groupResult{name: name, duration: time.Since(started), err: err}
		}(i, group.Name, port)
	}
	wg.Wait()

	failed := false
	fmt.Println("================================================================")
	fmt.Println("Test groups:")
	for _, result := range results {
		status := "passed"
		if result.err != nil {
			status = "FAILED"
			failed = true
		}
		fmt.Printf("  %-24s %s in %s\n", result.name, status, result.duration.Round(time.Second))
	}
	if failed {
		return 1
	}
	return 0
}

// runGroupProcess runs the runner again for one group and prefixes each line it prints with the
// group's name.
func runGroupProcess(executable, group, port string, output *sync.Mutex) error {
	reader, writer := io.Pipe()
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Env = append(os.Environ(), testutils.TestGroupEnv+"="+group, "SERVER_PORT="+port)
	cmd.Stdout = writer
	cmd.Stderr = writer

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			output.Lock()
			fmt.Printf("[%s] %s\n", group, scanner.Text())
			output.Unlock()
		}
		_, _ = io.Copy(io.Discard, reader)
	}()

	err := cmd.Run()
	_ = writer.Close()
	<-done
	return err
}

// runGroupTests runs this process's group: the go test runs planGroups resolves for it, one after
// another.
func runGroupTests(group string) error {
	groups, err := loadGroups()
	if err != nil {
		return err
	}
	packages, err := packagesToTest()
	if err != nil {
		return err
	}
	plans, err := planGroups(groups, packages)
	if err != nil {
		return err
	}

	var failures []string
	for _, invocation := range plans[group] {
		fmt.Printf("Test package: %s %s\n", strings.Join(invocation.filter, " "), strings.Join(invocation.packages, " "))
		if err := goTestCommand(invocation.filter, invocation.packages).Run(); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

// testFunctions returns the top-level test functions the package in dir defines.
func testFunctions(dir string) (map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, err
	}
	defined := map[string]bool{}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		for _, match := range testFuncPattern.FindAllStringSubmatch(string(content), -1) {
			defined[match[1]] = true
		}
	}
	return defined, nil
}

func importPath(rel string) string {
	return modulePath + "/" + strings.Trim(rel, "/")
}

// suitePattern matches exactly the named top-level tests.
func suitePattern(names []string) string {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	return "^(" + strings.Join(sorted, "|") + ")$"
}

func keys(m map[string]string) []string {
	var out []string
	for key := range m {
		out = append(out, key)
	}
	return out
}

func sortedKeys(m map[string][]string) []string {
	var out []string
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
