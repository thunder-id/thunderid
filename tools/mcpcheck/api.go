// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	extension  = "x-thunderid-mcp"
	toolPrefix = "thunderid_"
	decideHint = "add x-thunderid-mcp with expose: true and its tools, expose: false and a reason, " +
		"or expose: deferred and a reason"
)

var methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

type decision struct {
	Expose yaml.Node   `yaml:"expose"`
	Tools  []toolClaim `yaml:"tools"`
	Reason string      `yaml:"reason"`
}

type toolClaim struct {
	Name string `yaml:"name"`
	Tier string `yaml:"tier"`
}

// finding is a problem with a decision, before it is placed at an operation or a file.
type finding struct {
	code, msg string
}

func runAPI(args []string) ([]problem, error) {
	fl := flag.NewFlagSet("api", flag.ExitOnError)
	apiDir := fl.String("api", "", "directory holding the OpenAPI files")
	toolsDir := fl.String("tools", "", "directory to scan for tools.go and tool.go files")
	standaloneFile := fl.String("standalone", "", "YAML map of tools not backed by an API operation to the reason")
	baseAPIDir := fl.String("base-api", "", "the base branch's API directory; with -base-tools, only new problems fail")
	baseToolsDir := fl.String("base-tools", "", "the base branch's tools directory")
	if err := fl.Parse(args); err != nil || fl.NArg() > 0 {
		usage()
	}
	if *apiDir == "" || *toolsDir == "" || (*baseAPIDir == "") != (*baseToolsDir == "") {
		usage()
	}
	standalone, err := loadStandalone(*standaloneFile)
	if err != nil {
		return nil, err
	}
	problems, err := checkAPI(*apiDir, *toolsDir, standalone)
	if err != nil || *baseAPIDir == "" {
		return problems, err
	}
	baseProblems, err := checkAPI(*baseAPIDir, *baseToolsDir, standalone)
	if err != nil {
		return nil, fmt.Errorf("base branch: %w", err)
	}
	fresh, known := newProblems(problems, baseProblems)
	fmt.Printf("%d problem(s) already on the base branch are not counted\n", known)
	return fresh, nil
}

// newProblems keeps the problems the base branch does not have, so existing gaps never block a
// change while anything the change adds fails.
func newProblems(head, base []problem) (fresh []problem, known int) {
	onBase := map[string]bool{}
	for _, p := range base {
		onBase[p.key] = true
	}
	for _, p := range head {
		if onBase[p.key] {
			known++
		} else {
			fresh = append(fresh, p)
		}
	}
	return fresh, known
}

func loadStandalone(file string) (map[string]string, error) {
	standalone := map[string]string{}
	if file == "" {
		return standalone, nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, &standalone); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return standalone, nil
}

func checkAPI(apiDir, toolsDir string, standalone map[string]string) ([]problem, error) {
	tools, problems, err := loadTools(toolsDir)
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(apiDir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no *.yaml files in %s", apiDir)
	}
	claimed := map[string]bool{}
	for _, f := range files {
		found, err := checkAPIFile(f, tools, claimed)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}

	names := make([]string, 0, len(tools))
	for n := range tools {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if claimed[n] || strings.TrimSpace(standalone[n]) != "" {
			continue
		}
		problems = append(problems, problem{
			key: "unclaimed tool " + n,
			msg: fmt.Sprintf("%s: tool %s is not claimed by any API operation; list {name: %s, tier: ...} under the "+
				"x-thunderid-mcp tools of the operation it serves, or add it with a reason to "+
				"tools/mcpcheck/standalone-tools.yaml", tools[n], n, unprefixed(n)),
		})
	}
	return problems, nil
}

func checkAPIFile(file string, tools map[string]string, claimed map[string]bool) ([]problem, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var doc struct {
		MCP   yaml.Node                       `yaml:"x-thunderid-mcp"`
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	base := filepath.Base(file)
	var problems []problem
	place := func(key, where string, fs []finding) {
		for _, f := range fs {
			problems = append(problems, problem{key: key + " " + f.code, msg: where + ": " + f.msg})
		}
	}

	// A file-level decision covers every operation without its own. When it is invalid, it is
	// reported once here rather than again at each operation it would have covered.
	hasDefault := doc.MCP.Kind != 0
	if hasDefault {
		d, fs, ok := decodeDecision(&doc.MCP)
		if ok {
			fs = append(fs, checkDecision(d, true)...)
		}
		place(base+" file-level", file+" (file-level "+extension+")", fs)
	}

	paths := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, path := range paths {
		for _, m := range methods {
			node, ok := doc.Paths[path][m]
			if !ok {
				continue
			}
			var op struct {
				MCP yaml.Node `yaml:"x-thunderid-mcp"`
			}
			if err := node.Decode(&op); err != nil {
				return nil, fmt.Errorf("%s %s %s: %w", file, strings.ToUpper(m), path, err)
			}
			route := strings.ToUpper(m) + " " + path
			key := base + " " + route
			where := file + " " + route
			if op.MCP.Kind == 0 {
				if !hasDefault {
					place(key, where, []finding{{"undecided", "no exposure decision; " + decideHint}})
				}
				continue
			}
			d, fs, ok := decodeDecision(&op.MCP)
			if ok {
				fs = append(fs, checkDecision(d, false)...)
				fs = append(fs, claimTools(d, tools, claimed)...)
			}
			place(key, where, fs)
		}
	}
	return problems, nil
}

// decodeDecision decodes a decision and reports any key it does not know; ok is false when the
// decision cannot be read at all.
func decodeDecision(n *yaml.Node) (d decision, fs []finding, ok bool) {
	if err := n.Decode(&d); err != nil {
		// The YAML error carries a line number, so it stays out of the code.
		return d, []finding{{"malformed", fmt.Sprintf("%s must be a mapping of expose, tools and reason (%v)",
			extension, strings.Join(strings.Fields(err.Error()), " "))}}, false
	}
	fs = unknownKeys(n, "", "expose", "tools", "reason")
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "tools" {
			for _, t := range n.Content[i+1].Content {
				fs = append(fs, unknownKeys(t, "tools entry: ", "name", "tier")...)
			}
		}
	}
	return d, fs, true
}

func unknownKeys(n *yaml.Node, where string, known ...string) []finding {
	var out []finding
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i].Value; !slices.Contains(known, k) {
			out = append(out, finding{"unknown-key " + where + k, fmt.Sprintf("%sunknown key %q; the keys are %s",
				where, k, strings.Join(known, ", "))})
		}
	}
	return out
}

// exposeValue reads expose as "true", "false" or "deferred"; ok is false for anything else.
func exposeValue(n yaml.Node) (string, bool) {
	if n.Kind != yaml.ScalarNode {
		return "", false
	}
	switch {
	case n.Tag == "!!bool":
		var b bool
		if err := n.Decode(&b); err != nil {
			return "", false
		}
		return fmt.Sprint(b), true
	case n.Tag == "!!str" && n.Value == "deferred":
		return n.Value, true
	}
	return "", false
}

// checkDecision checks the form of a decision. A file-level decision may only exclude or defer.
func checkDecision(d decision, fileLevel bool) []finding {
	expose, ok := exposeValue(d.Expose)
	if !ok {
		got := "missing"
		if d.Expose.Kind != 0 {
			got = fmt.Sprintf("%q", d.Expose.Value)
		}
		return []finding{{"expose", fmt.Sprintf("expose is %s; set it to true, false or deferred", got)}}
	}
	var out []finding
	if expose != "true" && len(d.Tools) > 0 {
		out = append(out, finding{"tools-not-exposed", "tools apply only to expose: true; remove them"})
	}
	switch expose {
	case "true":
		if fileLevel {
			return []finding{{"file-level-true", "a file-level decision may not set expose: true; " +
				"decide each exposed operation on the operation itself"}}
		}
		if d.Reason != "" {
			out = append(out, finding{"true-reason", "reason applies only to expose: false or deferred; remove it"})
		}
		if len(d.Tools) == 0 {
			out = append(out, finding{"true-no-tools", "expose: true needs at least one entry under tools, " +
				"as {name: <tool name without thunderid_>, tier: read | write | destructive}"})
		}
		for _, t := range d.Tools {
			if !tiers[t.Tier] {
				out = append(out, finding{"tier " + t.Name, fmt.Sprintf("tool %q has tier %q; use read, write or destructive",
					t.Name, t.Tier)})
			}
		}
	case "false":
		if strings.TrimSpace(d.Reason) == "" {
			out = append(out, finding{"false-no-reason", "expose: false needs a reason saying why the operation is not a tool"})
		}
	case "deferred":
		if strings.TrimSpace(d.Reason) == "" {
			out = append(out, finding{"deferred-no-reason", "expose: deferred needs a reason saying what the tool waits for"})
		}
	}
	return out
}

// claimTools marks the tools an exposed operation names as claimed, and reports any that is not
// registered.
func claimTools(d decision, tools map[string]string, claimed map[string]bool) []finding {
	if v, _ := exposeValue(d.Expose); v != "true" {
		return nil
	}
	var out []finding
	for _, t := range d.Tools {
		if t.Name == "" {
			out = append(out, finding{"tool-no-name", "every entry under tools needs a name"})
			continue
		}
		name := toolPrefix + t.Name
		if _, ok := tools[name]; ok {
			claimed[name] = true
			continue
		}
		msg := fmt.Sprintf("tool %s is not registered in any tools.go or tool.go; register it, or fix the name", name)
		if strings.HasPrefix(t.Name, toolPrefix) {
			msg += fmt.Sprintf(" (write it as %q, without the %s prefix)", unprefixed(t.Name), toolPrefix)
		}
		out = append(out, finding{"unregistered " + name, msg})
	}
	return out
}
