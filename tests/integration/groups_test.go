// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type GroupsTestSuite struct {
	suite.Suite
	workDir string
}

func TestGroupsTestSuite(t *testing.T) {
	suite.Run(t, new(GroupsTestSuite))
}

func (ts *GroupsTestSuite) SetupTest() {
	dir, err := os.Getwd()
	ts.Require().NoError(err)
	ts.workDir = dir
	ts.Require().NoError(os.Chdir(ts.T().TempDir()))

	ts.writeFile("alpha/alpha_test.go", "package alpha\n\nfunc TestAlphaOneSuite(t *testing.T) {}\n\n"+
		"func TestAlphaTwoSuite(t *testing.T) {}\n")
	ts.writeFile("beta/beta_test.go", "package beta\n\nfunc TestBetaSuite(t *testing.T) {}\n")
}

func (ts *GroupsTestSuite) TearDownTest() {
	ts.Require().NoError(os.Chdir(ts.workDir))
}

func (ts *GroupsTestSuite) writeFile(name, content string) {
	ts.Require().NoError(os.MkdirAll(filepath.Dir(name), 0o755))
	ts.Require().NoError(os.WriteFile(name, []byte(content), 0o600))
}

func (ts *GroupsTestSuite) packages() []string {
	return []string{importPath("alpha"), importPath("beta"), importPath("gamma")}
}

func (ts *GroupsTestSuite) plan(manifest string) (map[string][]testInvocation, error) {
	ts.writeFile(groupsFile, manifest)
	groups, err := loadGroups()
	if err != nil {
		return nil, err
	}
	return planGroups(groups, ts.packages())
}

func (ts *GroupsTestSuite) TestWithoutGroupsTheDefaultGroupRunsEverything() {
	plans, err := ts.plan("groups: []\n")
	ts.Require().NoError(err)

	ts.Equal(map[string][]testInvocation{defaultGroup: {{packages: ts.packages()}}}, plans)
}

func (ts *GroupsTestSuite) TestAGroupTakesWholePackages() {
	plans, err := ts.plan("groups:\n  - name: first\n    packages: [beta]\n")
	ts.Require().NoError(err)

	ts.Equal([]testInvocation{{packages: []string{importPath("alpha"), importPath("gamma")}}}, plans[defaultGroup])
	ts.Equal([]testInvocation{{packages: []string{importPath("beta")}}}, plans["first"])
}

func (ts *GroupsTestSuite) TestTheDefaultGroupSkipsTheSuitesAGroupTakes() {
	plans, err := ts.plan("groups:\n  - name: first\n    suites:\n      alpha: [TestAlphaTwoSuite]\n")
	ts.Require().NoError(err)

	ts.Equal([]testInvocation{
		{packages: []string{importPath("beta"), importPath("gamma")}},
		{packages: []string{importPath("alpha")}, filter: []string{"-skip", "^(TestAlphaTwoSuite)$"}},
	}, plans[defaultGroup])
	ts.Equal([]testInvocation{
		{packages: []string{importPath("alpha")}, filter: []string{"-run", "^(TestAlphaTwoSuite)$"}},
	}, plans["first"])
}

func (ts *GroupsTestSuite) TestInvalidManifestsAreRefused() {
	cases := map[string]string{
		"unknown package":      "groups:\n  - name: first\n    packages: [delta]\n",
		"package listed twice": "groups:\n  - name: first\n    packages: [beta]\n  - name: second\n    packages: [beta]\n",
		"unknown suite":        "groups:\n  - name: first\n    suites:\n      alpha: [TestMissingSuite]\n",
		"suite listed twice": "groups:\n  - name: first\n    suites:\n      alpha: [TestAlphaOneSuite]\n" +
			"  - name: second\n    suites:\n      alpha: [TestAlphaOneSuite]\n",
		"suites of a listed package": "groups:\n  - name: first\n    packages: [alpha]\n" +
			"  - name: second\n    suites:\n      alpha: [TestAlphaOneSuite]\n",
		"invalid name":        "groups:\n  - name: First-Group\n    packages: [beta]\n",
		"default name":        "groups:\n  - name: default\n    packages: [beta]\n",
		"group defined twice": "groups:\n  - name: first\n    packages: [alpha]\n  - name: first\n    packages: [beta]\n",
	}
	for name, manifest := range cases {
		ts.Run(name, func() {
			_, err := ts.plan(manifest)
			ts.Error(err)
		})
	}
}
