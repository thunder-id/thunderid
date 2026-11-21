// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// A specification decides MCP exposure in a section of this form:
//
//	### MCP exposure
//
//	Decision: required
//
//	| Operation | Tool | Tier |
//	|---|---|---|
//	| GET /connections | thunderid_list_connections | read |
//	| POST /connections/{id}/test | not exposed: runs a live request against the endpoint | |
//	| DELETE /connections/{id} | deferred #1234 | destructive |
//
// An operation backing several tools has a row per tool, and a tool combining several operations a
// row per operation. "not required" takes a Reason: line and "deferred" an Issue: line instead of
// the table.

var (
	mcpSectionRe = regexp.MustCompile(`(?m)^###[ \t]+MCP exposure[ \t]*$`)
	apiSectionRe = regexp.MustCompile(`(?m)^###[ \t]+API[ \t]*$`)
	nextHeading  = regexp.MustCompile(`(?m)^#{1,3}[ \t]`)
	decisionRe   = regexp.MustCompile(`(?m)^Decision:[ \t]*(.*?)[ \t]*$`)
	reasonRe     = regexp.MustCompile(`(?m)^Reason:[ \t]*(.*?)[ \t]*$`)
	issueRe      = regexp.MustCompile(`(?m)^Issue:[ \t]*(.*?)[ \t]*$`)
	operationRe  = regexp.MustCompile(`^(GET|PUT|POST|DELETE|OPTIONS|HEAD|PATCH|TRACE) /\S*$`)
	toolNameRe   = regexp.MustCompile(`^thunderid_[a-z0-9]+(_[a-z0-9]+)*$`)
	issueRefRe   = regexp.MustCompile(`^#[1-9][0-9]*$`)
	issueURLRe   = regexp.MustCompile(`^https://github\.com/thunder-id/thunderid/issues/[1-9][0-9]*$`)
	separatorRe  = regexp.MustCompile(`^\|?[\s:|-]+\|?$`)
	tableHeader  = []string{"operation", "tool", "tier"}
)

const issueRefForm = "#<n> or https://github.com/thunder-id/thunderid/issues/<n>"

func runSpec(files []string) ([]problem, error) {
	if len(files) == 0 {
		usage()
	}
	var out []problem
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		for _, msg := range checkSpec(string(b)) {
			out = append(out, problem{key: f + " " + msg, msg: f + ": " + msg})
		}
	}
	return out, nil
}

func checkSpec(doc string) []string {
	loc := mcpSectionRe.FindStringIndex(doc)
	if loc == nil {
		return []string{`no "### MCP exposure" section; add one after "### API" (see docs-internals/templates/spec.md)`}
	}
	var out []string
	if api := apiSectionRe.FindStringIndex(doc); len(api) > 0 && api[0] > loc[0] {
		out = append(out, `"### MCP exposure" comes before "### API"; move it after`)
	}
	body := doc[loc[1]:]
	if next := nextHeading.FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}

	m := decisionRe.FindStringSubmatch(body)
	if m == nil {
		return append(out, `MCP exposure: no "Decision:" line; write Decision: required, not required or deferred`)
	}
	switch m[1] {
	case "not required":
		if r := reasonRe.FindStringSubmatch(body); len(r) < 2 || r[1] == "" {
			out = append(out, `MCP exposure: "not required" needs a "Reason:" line saying why no operation is a tool`)
		}
	case "deferred":
		if i := issueRe.FindStringSubmatch(body); len(i) < 2 || i[1] == "" {
			out = append(out, `MCP exposure: "deferred" needs an "Issue:" line holding `+issueRefForm)
		} else if !isIssueRef(i[1]) {
			out = append(out, fmt.Sprintf("MCP exposure: issue %q is not %s", i[1], issueRefForm))
		}
	case "required":
		out = append(out, checkTable(body)...)
	default:
		out = append(out, fmt.Sprintf("MCP exposure: decision %q is not one of required, not required or deferred", m[1]))
	}
	return out
}

func checkTable(body string) []string {
	var out []string
	inTable, rows := false, 0
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}
		cells := splitRow(line)
		if !inTable {
			inTable = equalFold(cells, tableHeader)
			continue
		}
		if separatorRe.MatchString(line) {
			continue
		}
		rows++
		out = append(out, checkRow(cells)...)
	}
	if rows == 0 {
		out = append(out, `MCP exposure: "required" needs a table "| Operation | Tool | Tier |" with a row per operation`)
	}
	return out
}

func checkRow(cells []string) []string {
	if len(cells) != len(tableHeader) {
		return []string{fmt.Sprintf("MCP exposure: row %q needs exactly three cells: operation, tool and tier",
			strings.Join(cells, " | "))}
	}
	op, tool, tier := cells[0], cells[1], cells[2]
	var out []string
	if !operationRe.MatchString(op) {
		out = append(out, fmt.Sprintf("MCP exposure: operation %q is not METHOD /path", op))
	}
	tierOptional := false
	switch {
	case strings.HasPrefix(tool, "not exposed"):
		tierOptional = true
		if r, ok := strings.CutPrefix(tool, "not exposed:"); !ok || strings.TrimSpace(r) == "" {
			out = append(out, fmt.Sprintf(`MCP exposure: %s: a "not exposed" row needs its reason in the tool cell, `+
				`as "not exposed: <reason>"`, op))
		}
	case strings.HasPrefix(tool, "deferred "):
		if ref := strings.TrimSpace(strings.TrimPrefix(tool, "deferred ")); !isIssueRef(ref) {
			out = append(out, fmt.Sprintf("MCP exposure: %s: issue %q is not %s", op, ref, issueRefForm))
		}
	case !toolNameRe.MatchString(tool):
		out = append(out, fmt.Sprintf("MCP exposure: %s: tool %q is not a tool name (thunderid_<snake_case>), "+
			`"not exposed: <reason>" or "deferred <issue>"`, op, tool))
	}
	if !tiers[tier] && (tier != "" || !tierOptional) {
		out = append(out, fmt.Sprintf("MCP exposure: %s: tier %q is not read, write or destructive", op, tier))
	}
	return out
}

// splitRow returns the trimmed cells of a table row, with surrounding backticks removed.
func splitRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	cells := strings.Split(line, "|")
	for i, c := range cells {
		cells[i] = strings.Trim(strings.TrimSpace(c), "`")
	}
	return cells
}

func equalFold(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

func isIssueRef(s string) bool {
	return issueRefRe.MatchString(s) || issueURLRe.MatchString(s)
}
