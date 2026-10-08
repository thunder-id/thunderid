// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"regexp"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/valueref"
)

// bundleResource is one document of a captured version.
//
// A version is read as text rather than parsed as YAML. A template-style export is not YAML until its
// placeholders are resolved, since a list can be written as a range over a variable, and resolving
// needs the values of the deployment it is applied to. The top-level scalars are all a diff needs.
type bundleResource struct {
	Type    string
	ID      string
	Name    string
	Content string
}

// key identifies a resource across versions: its type with its id, or its name when it has no id.
func (r bundleResource) key() string {
	identity := r.ID
	if identity == "" {
		identity = r.Name
	}
	return r.Type + "/" + identity
}

var (
	documentSeparator = regexp.MustCompile(`(?m)^---\s*$`)
	// topLevelScalar reads a column-zero key, which is a resource's own field rather than a nested one.
	topLevelScalar = func(key string) *regexp.Regexp {
		return regexp.MustCompile(`(?m)^` + key + `:\s*(.+?)\s*$`)
	}
	resourceTypeField = topLevelScalar("resource_type")
	idField           = topLevelScalar("id")
	nameField         = topLevelScalar("name")
	// valueReference reads a reference a document holds in place of a value. A reference is a whole
	// scalar, a mapping's value or a list's item, quoted or not, so the same text inside a longer
	// value is not one.
	valueReference = regexp.MustCompile(`(?m)(?:^\s*-|:)\s*['"]?(` + regexp.QuoteMeta(valueref.PrefixVariable) +
		`|` + regexp.QuoteMeta(valueref.PrefixSecret) + `)([A-Za-z_][A-Za-z0-9_]*)['"]?\s*$`)
)

// parseBundle splits a version into its resources. A document naming no resource type is skipped:
// it carries nothing an import would act on.
func parseBundle(content string) []bundleResource {
	documents := documentSeparator.Split(content, -1)
	resources := make([]bundleResource, 0, len(documents))
	for _, document := range documents {
		document = strings.TrimSpace(document)
		resourceType := firstMatch(resourceTypeField, document)
		if resourceType == "" {
			continue
		}
		resources = append(resources, bundleResource{
			Type:    resourceType,
			ID:      firstMatch(idField, document),
			Name:    firstMatch(nameField, document),
			Content: document,
		})
	}
	return resources
}

// joinBundle writes documents back as one version.
func joinBundle(documents []string) string {
	return strings.Join(documents, "\n---\n")
}

func firstMatch(field *regexp.Regexp, document string) string {
	match := field.FindStringSubmatch(document)
	if match == nil {
		return ""
	}
	return strings.Trim(match[1], `"'`)
}

// diffBundles reports how to go from what a gateway holds to what is applied to it.
func diffBundles(held, applied []bundleResource) []Change {
	heldByKey := make(map[string]bundleResource, len(held))
	for _, resource := range held {
		heldByKey[resource.key()] = resource
	}
	appliedByKey := make(map[string]bundleResource, len(applied))

	changes := make([]Change, 0, len(applied)+len(held))
	for _, resource := range applied {
		appliedByKey[resource.key()] = resource
		kind := ChangeAdded
		previous, ok := heldByKey[resource.key()]
		if ok {
			kind = ChangeUpdated
			if previous.Content == resource.Content {
				kind = ChangeUnchanged
			}
		}
		change := changeOf(resource, kind)
		change.Lines = lineDiff(previous.Content, resource.Content)
		changes = append(changes, change)
	}
	for _, resource := range held {
		if _, ok := appliedByKey[resource.key()]; !ok {
			change := changeOf(resource, ChangeDeleted)
			change.Lines = lineDiff(resource.Content, "")
			changes = append(changes, change)
		}
	}

	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].ResourceType != changes[j].ResourceType {
			return changes[i].ResourceType < changes[j].ResourceType
		}
		return changes[i].Name+changes[i].ID < changes[j].Name+changes[j].ID
	})
	return changes
}

// maxDiffSteps bounds the work one document's line diff may take, at about 25 ms. Past it, what is
// left to compare is reported as its old lines removed and its new lines added: still a correct diff,
// if not the smallest, so a large document that changed throughout cannot hold up a request.
const maxDiffSteps = 1 << 22

// lineDiff diffs two documents line by line, with the fewest lines removed and added. Identical
// documents have no line diff.
func lineDiff(from, to string) []LineOp {
	if from == to {
		return nil
	}
	return diffLines(splitLines(from), splitLines(to), maxDiffSteps)
}

// diffLines follows Myers' O(ND) algorithm in its linear-space form: the middle of a shortest edit
// is found by searching from both ends at once, and each half is then diffed the same way. Memory
// stays proportional to the documents' length, and the work to the length times the lines changed.
func diffLines(a, b []string, budget int) []LineOp {
	d := newLineDiffer(a, b, budget)
	d.compare(0, len(a), 0, len(b))
	return d.ops
}

// lineDiffer diffs two documents' lines, compared as numbers that are equal where the lines are.
type lineDiffer struct {
	a, b   []string
	x, y   []int
	budget int
	ops    []LineOp
}

func newLineDiffer(a, b []string, budget int) *lineDiffer {
	numbers := map[string]int{}
	number := func(lines []string) []int {
		out := make([]int, len(lines))
		for i, line := range lines {
			n, ok := numbers[line]
			if !ok {
				n = len(numbers)
				numbers[line] = n
			}
			out[i] = n
		}
		return out
	}
	return &lineDiffer{a: a, b: b, x: number(a), y: number(b), budget: budget,
		ops: make([]LineOp, 0, len(a)+len(b))}
}

// compare diffs a[aLo:aHi] against b[bLo:bHi].
func (d *lineDiffer) compare(aLo, aHi, bLo, bHi int) {
	for aLo < aHi && bLo < bHi && d.x[aLo] == d.y[bLo] {
		d.ops = append(d.ops, LineOp{Kind: " ", Text: d.a[aLo]})
		aLo++
		bLo++
	}
	kept := 0
	for aLo < aHi-kept && bLo < bHi-kept && d.x[aHi-1-kept] == d.y[bHi-1-kept] {
		kept++
	}
	aHi -= kept
	bHi -= kept

	splitA, splitB, ok := 0, 0, false
	if aLo < aHi && bLo < bHi {
		splitA, splitB, ok = d.middle(aLo, aHi, bLo, bHi)
	}
	if ok {
		d.compare(aLo, splitA, bLo, splitB)
		d.compare(splitA, aHi, splitB, bHi)
	} else {
		d.replace(aLo, aHi, bLo, bHi)
	}

	for i := aHi; i < aHi+kept; i++ {
		d.ops = append(d.ops, LineOp{Kind: " ", Text: d.a[i]})
	}
}

// replace reports a[aLo:aHi] removed and b[bLo:bHi] added.
func (d *lineDiffer) replace(aLo, aHi, bLo, bHi int) {
	for i := aLo; i < aHi; i++ {
		d.ops = append(d.ops, LineOp{Kind: "-", Text: d.a[i]})
	}
	for j := bLo; j < bHi; j++ {
		d.ops = append(d.ops, LineOp{Kind: "+", Text: d.b[j]})
	}
}

// middle finds where a shortest edit of a[aLo:aHi] into b[bLo:bHi] can be split in two, by
// extending the furthest-reaching paths from the start and from the end, one edit at a time, until
// they meet. It reports false when the budget runs out first.
func (d *lineDiffer) middle(aLo, aHi, bLo, bHi int) (int, int, bool) {
	n, m := aHi-aLo, bHi-bLo
	maxEdits := (n + m + 1) / 2
	offset := maxEdits
	size := 2*maxEdits + 2
	// forward[offset+k] is how far along a the furthest path from the start reaches on diagonal
	// k = x - y; backward the same for the path from the end, counted from the end. -1 is unreached.
	forward := make([]int, size)
	backward := make([]int, size)
	for i := range forward {
		forward[i], backward[i] = -1, -1
	}
	forward[offset+1], backward[offset+1] = 0, 0
	delta := n - m
	// With an odd difference in length, the paths meet on a step from the start; otherwise on one
	// from the end.
	odd := delta%2 != 0
	// Diagonals whose paths have run off the end of a or b are left out of later steps.
	forwardStart, forwardEnd, backwardStart, backwardEnd := 0, 0, 0, 0

	for edits := 0; edits < maxEdits; edits++ {
		for k := -edits + forwardStart; k <= edits-forwardEnd; k += 2 {
			x, y := d.extend(forward, offset+k, k, edits, n, m, func(x, y int) bool {
				return d.x[aLo+x] == d.y[bLo+y]
			})
			if d.budget <= 0 {
				return 0, 0, false
			}
			switch {
			case x > n:
				forwardEnd += 2
			case y > m:
				forwardStart += 2
			case odd:
				other := offset + delta - k
				if other >= 0 && other < size && backward[other] != -1 && x >= n-backward[other] {
					return d.split(aLo, bLo, n, m, x, y)
				}
			}
		}
		for k := -edits + backwardStart; k <= edits-backwardEnd; k += 2 {
			x, y := d.extend(backward, offset+k, k, edits, n, m, func(x, y int) bool {
				return d.x[aHi-1-x] == d.y[bHi-1-y]
			})
			if d.budget <= 0 {
				return 0, 0, false
			}
			switch {
			case x > n:
				backwardEnd += 2
			case y > m:
				backwardStart += 2
			case !odd:
				other := offset + delta - k
				if other >= 0 && other < size && forward[other] != -1 {
					fx := forward[other]
					if fx >= n-x {
						return d.split(aLo, bLo, n, m, fx, fx-(delta-k))
					}
				}
			}
		}
	}
	return 0, 0, false
}

// extend takes the furthest path on diagonal k one edit further, from the neighboring diagonal that
// reaches further, then along every line the two sides share, and records how far it reached.
func (d *lineDiffer) extend(reach []int, at, k, edits, n, m int, same func(x, y int) bool) (int, int) {
	x := reach[at-1] + 1
	if k == -edits || (k != edits && reach[at-1] < reach[at+1]) {
		x = reach[at+1]
	}
	y := x - k
	for x < n && y < m && same(x, y) {
		x++
		y++
		d.budget--
	}
	reach[at] = x
	d.budget--
	return x, y
}

// split turns where the paths met into a split of the two ranges, refusing one that leaves a side
// as large as the whole, which would not make the diff any smaller.
func (d *lineDiffer) split(aLo, bLo, n, m, x, y int) (int, int, bool) {
	if (x == 0 && y == 0) || (x == n && y == m) {
		return 0, 0, false
	}
	return aLo + x, bLo + y, true
}

// splitLines splits a document into its lines, with no trailing empty line.
func splitLines(document string) []string {
	if document == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(document, "\n"), "\n")
}

func changeOf(resource bundleResource, kind ChangeKind) Change {
	return Change{ResourceType: resource.Type, ID: resource.ID, Name: resource.Name, Change: kind}
}

func summarize(changes []Change) DiffSummary {
	var summary DiffSummary
	for _, change := range changes {
		switch change.Change {
		case ChangeAdded:
			summary.Added++
		case ChangeUpdated:
			summary.Updated++
		case ChangeDeleted:
			summary.Deleted++
		case ChangeUnchanged:
			summary.Unchanged++
		}
	}
	return summary
}

// referencesIn returns the variables and secrets a version refers to, each name once and sorted.
// The gateway holds their values, so the version is only complete on a gateway that holds them all.
func referencesIn(content string) (variables, secrets []string) {
	seen := map[string]bool{}
	for _, match := range valueReference.FindAllStringSubmatch(content, -1) {
		reference := match[1] + match[2]
		if seen[reference] {
			continue
		}
		seen[reference] = true
		if match[1] == valueref.PrefixSecret {
			secrets = append(secrets, match[2])
		} else {
			variables = append(variables, match[2])
		}
	}
	sort.Strings(variables)
	sort.Strings(secrets)
	return variables, secrets
}
