// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSpec(t *testing.T) {
	tests := []struct {
		file string
		want []string
	}{
		// An operation appears on several rows, one per tool, and a tool on several rows, one per
		// operation it combines.
		{"good-required.txt", nil},
		{"good-not-required.txt", nil},
		{"good-deferred-ref.txt", nil},
		{"good-deferred-url.txt", nil},
		{"bad-missing.txt", []string{`no "### MCP exposure" section`}},
		{"bad-wrong-level.txt", []string{`no "### MCP exposure" section`}},
		{"bad-order.txt", []string{`comes before "### API"`}},
		{"bad-no-decision.txt", []string{`no "Decision:" line`}},
		{"bad-decision-value.txt", []string{`decision "maybe" is not one of`}},
		{"bad-no-reason.txt", []string{`"not required" needs a "Reason:" line`}},
		{"bad-deferred-no-issue.txt", []string{`"deferred" needs an "Issue:" line`}},
		{"bad-deferred-bad-issue.txt", []string{`issue "https://github.com/someone/else/issues/42" is not`}},
		{"bad-no-table.txt", []string{`"required" needs a table`}},
		{"bad-rows.txt", []string{
			`operation "list connections" is not METHOD /path`,
			`GET /connections/{id}: tool "getConnection" is not a tool name`,
			`POST /connections: tier "admin"`,
			`DELETE /connections/{id}: issue "soon" is not`,
			`PUT /connections/{id}: tier ""`,
			`POST /connections/{id}/test: tier "sometimes"`,
			`POST /connections/{id}/rotate: a "not exposed" row needs its reason in the tool cell, as "not exposed: <reason>"`,
			`POST /connections/{id}/disable: a "not exposed" row needs its reason`,
			`needs exactly three cells`,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("testdata", "specs", tt.file)
			got, err := runSpec([]string{path})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d problem(s), want %d: %v", len(got), len(tt.want), got)
			}
			for i, w := range tt.want {
				if !strings.HasPrefix(got[i].msg, path+": ") || !strings.Contains(got[i].msg, w) {
					t.Errorf("problem %d: got %q, want it to name the file and contain %q", i, got[i].msg, w)
				}
			}
		})
	}
}

func TestSpec_ReadError(t *testing.T) {
	if _, err := runSpec([]string{"testdata/specs/missing.md"}); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
