// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

const (
	toolsDir   = "testdata/tools"
	standalone = "testdata/standalone.yaml"
)

func keys(ps []problem) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.key
	}
	sort.Strings(out)
	return out
}

func assertKeys(t *testing.T, got []problem, want ...string) {
	t.Helper()
	sort.Strings(want)
	if g := keys(got); !slices.Equal(g, want) {
		t.Fatalf("problem keys:\n got  %q\n want %q", g, want)
	}
}

func check(t *testing.T, apiDir, standaloneFile string) []problem {
	t.Helper()
	s, err := loadStandalone(standaloneFile)
	if err != nil {
		t.Fatal(err)
	}
	ps, err := checkAPI(apiDir, toolsDir, s)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func TestLoadTools(t *testing.T) {
	tools, problems, err := loadTools(toolsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for n := range tools {
		names = append(names, n)
	}
	sort.Strings(names)
	// The concatenated name and the aliased import are read; helpers.go and a Tool type from
	// another package are not.
	want := []string{"thunderid_create_app", "thunderid_integrate_guide", "thunderid_list_apps", "thunderid_search_apps"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools: got %q, want %q", names, want)
	}
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if got := tools["thunderid_integrate_guide"]; got != filepath.Join(toolsDir, "guide", "tool.go") {
		t.Fatalf("registering file: got %q", got)
	}
}

func TestLoadTools_NameThatIsNotALiteralIsAProblem(t *testing.T) {
	tools, problems, err := loadTools("testdata/tools-unreadable")
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 0 {
		t.Fatalf("tools: got %v, want none", tools)
	}
	assertKeys(t, problems, "x/tools.go unreadable tool name toolName")
}

func TestAPI_EveryFormPasses(t *testing.T) {
	assertKeys(t, check(t, "testdata/api/good", standalone))
}

func TestAPI_ToolNoOperationClaimsNeedsTheStandaloneList(t *testing.T) {
	ps := check(t, "testdata/api/good", "")
	assertKeys(t, ps, "unclaimed tool thunderid_integrate_guide")
	if !strings.Contains(ps[0].msg, filepath.Join(toolsDir, "guide", "tool.go")) ||
		!strings.Contains(ps[0].msg, "standalone-tools.yaml") {
		t.Fatalf("message does not name the file and the fix: %s", ps[0].msg)
	}
}

func TestAPI_StandaloneEntryWithoutAReasonDoesNotCount(t *testing.T) {
	ps, err := checkAPI("testdata/api/good", toolsDir, map[string]string{"thunderid_integrate_guide": " "})
	if err != nil {
		t.Fatal(err)
	}
	assertKeys(t, ps, "unclaimed tool thunderid_integrate_guide")
}

func TestAPI_EveryRule(t *testing.T) {
	ps := check(t, "testdata/api/bad", "")
	assertKeys(t, ps,
		"ops.yaml GET /undecided undecided",
		"ops.yaml GET /bad-expose expose",
		"ops.yaml GET /quoted-true expose",
		"ops.yaml GET /missing-expose expose",
		"ops.yaml GET /malformed malformed",
		"ops.yaml POST /no-tools true-no-tools",
		"ops.yaml POST /unregistered unregistered thunderid_missing_tool",
		"ops.yaml POST /unregistered unregistered thunderid_thunderid_list_apps",
		"ops.yaml POST /unregistered tool-no-name",
		"ops.yaml PUT /bad-tier tier create_app",
		"ops.yaml GET /true-reason true-reason",
		"ops.yaml DELETE /no-reason false-no-reason",
		"ops.yaml GET /false-tools tools-not-exposed",
		"ops.yaml GET /deferred-no-reason deferred-no-reason",
		"ops.yaml GET /issue-key unknown-key issue",
		"ops.yaml GET /unknown-tool-key unknown-key tools entry: title",
		"unclaimed tool thunderid_integrate_guide",
	)
	for _, p := range ps {
		if strings.Contains(p.key, "/undecided") {
			if !strings.Contains(p.msg, filepath.Join("testdata", "api", "bad", "ops.yaml")+" GET /undecided: ") ||
				!strings.Contains(p.msg, "expose: deferred and a reason") {
				t.Fatalf("message does not name the file, the operation and the fix: %s", p.msg)
			}
		}
		if strings.Contains(p.key, "thunderid_thunderid_list_apps") && !strings.Contains(p.msg, `write it as "list_apps"`) {
			t.Fatalf("message does not explain the prefix: %s", p.msg)
		}
		if strings.Contains(p.msg, "line ") && !strings.Contains(p.key, "malformed") {
			t.Fatalf("unexpected line number: %s", p.msg)
		}
	}
}

func TestAPI_FileLevelDecision(t *testing.T) {
	// Operations without their own decision inherit the file's; an invalid file-level decision is
	// reported once rather than at every operation it covers.
	assertKeys(t, check(t, "testdata/api/filelevel", standalone),
		"true.yaml file-level file-level-true",
		"false-no-reason.yaml file-level false-no-reason",
		"false-no-reason.yaml GET /own false-no-reason",
		"deferred-no-reason.yaml file-level deferred-no-reason",
		"issue-key.yaml file-level unknown-key issue",
		"unclaimed tool thunderid_create_app",
		"unclaimed tool thunderid_list_apps",
		"unclaimed tool thunderid_search_apps",
	)
}

// Operations and tools are many to many: an operation may back several tools, and a tool may
// combine several operations, with the same or different tiers.
func TestAPI_OperationsAndToolsAreManyToMany(t *testing.T) {
	ps := check(t, "testdata/api/many", standalone)
	// list_apps and search_apps are each claimed by two operations; each operation's unregistered
	// second tool is reported there, while its registered one still counts as claimed.
	assertKeys(t, ps,
		"apps.yaml GET /apps unregistered thunderid_missing_tool",
		"apps.yaml GET /apps/search unregistered thunderid_missing_tool",
		"unclaimed tool thunderid_create_app",
	)
}

func TestRatchet_OnlyANewProblemFails(t *testing.T) {
	head := check(t, "testdata/ratchet/head/api", "")
	base := check(t, "testdata/ratchet/base/api", "")
	fresh, known := newProblems(head, base)
	assertKeys(t, fresh, "apps.yaml POST /apps/new undecided")
	// GET /legacy undecided, DELETE /legacy/{id} without a reason, and the three unclaimed tools.
	if known != 5 {
		t.Fatalf("known problems: got %d, want 5", known)
	}
}

func TestRatchet_RunAPIWithBase(t *testing.T) {
	fresh, err := runAPI([]string{
		"-api", "testdata/ratchet/head/api", "-tools", toolsDir, "-standalone", standalone,
		"-base-api", "testdata/ratchet/base/api", "-base-tools", toolsDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertKeys(t, fresh, "apps.yaml POST /apps/new undecided")

	fresh, err = runAPI([]string{
		"-api", "testdata/ratchet/base/api", "-tools", toolsDir, "-standalone", standalone,
		"-base-api", "testdata/ratchet/base/api", "-base-tools", toolsDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertKeys(t, fresh)
}

func TestRunAPI_ReadErrors(t *testing.T) {
	if _, err := runAPI([]string{"-api", "testdata/specs", "-tools", toolsDir}); err == nil {
		t.Fatal("expected an error for a directory without OpenAPI files")
	}
	if _, err := runAPI([]string{"-api", "testdata/api/good", "-tools", toolsDir, "-standalone", "missing.yaml"}); err == nil {
		t.Fatal("expected an error for a missing standalone file")
	}
}
