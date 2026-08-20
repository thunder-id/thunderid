/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package diff computes a resource-level difference between two declarative bundles. Comparison is at
// the granularity of a whole resource document (added, updated, deleted, unchanged); for updated
// resources a line-level diff is included so the change can be reviewed before a promote or revert.
package diff

import (
	"sort"

	"github.com/thunder-id/thunderid/internal/envmgr/bundle"
)

// ChangeType classifies how a resource differs between the two bundles.
type ChangeType string

const (
	// Added means the resource exists only in the new bundle.
	Added ChangeType = "added"
	// Updated means the resource exists in both bundles with different content.
	Updated ChangeType = "updated"
	// Deleted means the resource exists only in the old bundle.
	Deleted ChangeType = "deleted"
	// Unchanged means the resource is identical in both bundles.
	Unchanged ChangeType = "unchanged"
)

// LineOp is one line of a unified line diff. Kind is " " (context), "+" (added) or "-" (removed).
type LineOp struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// ResourceChange describes how a single resource differs between the two bundles.
type ResourceChange struct {
	Key      string     `json:"key"`
	Type     string     `json:"type"`
	ID       string     `json:"id,omitempty"`
	Name     string     `json:"name,omitempty"`
	Category string     `json:"category,omitempty"`
	Change   ChangeType `json:"change"`
	Lines    []LineOp   `json:"lines,omitempty"`
}

// Summary counts changes by type.
type Summary struct {
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

// Diff is the full comparison result.
type Diff struct {
	Changes []ResourceChange `json:"changes"`
	Summary Summary          `json:"summary"`
}

// HasChanges reports whether anything other than unchanged resources is present.
func (d Diff) HasChanges() bool {
	return d.Summary.Added+d.Summary.Updated+d.Summary.Deleted > 0
}

// Compute produces the diff of old -> new at resource granularity.
func Compute(old, new []bundle.Resource) Diff {
	oldIdx := bundle.Index(old)
	newIdx := bundle.Index(new)

	seen := map[string]bool{}
	changes := make([]ResourceChange, 0, len(new))

	for _, r := range new {
		if seen[r.Key()] {
			continue
		}
		seen[r.Key()] = true
		if prev, ok := oldIdx[r.Key()]; ok {
			if prev.Content == r.Content {
				changes = append(changes, resourceChange(r, Unchanged, nil))
			} else {
				changes = append(changes, resourceChange(r, Updated, lineDiff(prev.Content, r.Content)))
			}
		} else {
			changes = append(changes, resourceChange(r, Added, lineDiff("", r.Content)))
		}
	}
	for _, r := range old {
		if seen[r.Key()] {
			continue
		}
		if _, ok := newIdx[r.Key()]; ok {
			continue
		}
		seen[r.Key()] = true
		changes = append(changes, resourceChange(r, Deleted, lineDiff(r.Content, "")))
	}

	sortChanges(changes)
	return Diff{Changes: changes, Summary: summarize(changes)}
}

func resourceChange(r bundle.Resource, ct ChangeType, lines []LineOp) ResourceChange {
	return ResourceChange{
		Key: r.Key(), Type: r.Type, ID: r.ID, Name: r.Name, Category: r.Category, Change: ct, Lines: lines,
	}
}

func summarize(changes []ResourceChange) Summary {
	var s Summary
	for _, c := range changes {
		switch c.Change {
		case Added:
			s.Added++
		case Updated:
			s.Updated++
		case Deleted:
			s.Deleted++
		case Unchanged:
			s.Unchanged++
		}
	}
	return s
}

// sortChanges orders changes deterministically by type then key so output is stable.
// changeRank orders the change kinds. Everything that actually changed comes before what did not, so
// a reviewer reads the part of the diff that matters first rather than hunting for it among resources
// that are identical on both sides.
func changeRank(c ChangeType) int {
	switch c {
	case Added:
		return 0
	case Updated:
		return 1
	case Deleted:
		return 2
	case Unchanged:
		return 3
	}
	return 4
}

func sortChanges(changes []ResourceChange) {
	sort.SliceStable(changes, func(i, j int) bool {
		if ri, rj := changeRank(changes[i].Change), changeRank(changes[j].Change); ri != rj {
			return ri < rj
		}
		if changes[i].Type != changes[j].Type {
			return changes[i].Type < changes[j].Type
		}
		return changes[i].Key < changes[j].Key
	})
}

// lineDiff produces a unified line diff of a -> b using a longest-common-subsequence walk.
func lineDiff(a, b string) []LineOp {
	if a == b {
		return nil
	}
	aLines := splitLines(a)
	bLines := splitLines(b)

	// LCS length table.
	m, n := len(aLines), len(bLines)
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if aLines[i] == bLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var ops []LineOp
	i, j := 0, 0
	for i < m && j < n {
		switch {
		case aLines[i] == bLines[j]:
			ops = append(ops, LineOp{Kind: " ", Text: aLines[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, LineOp{Kind: "-", Text: aLines[i]})
			i++
		default:
			ops = append(ops, LineOp{Kind: "+", Text: bLines[j]})
			j++
		}
	}
	for ; i < m; i++ {
		ops = append(ops, LineOp{Kind: "-", Text: aLines[i]})
	}
	for ; j < n; j++ {
		ops = append(ops, LineOp{Kind: "+", Text: bLines[j]})
	}
	return ops
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
