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
	// usernameField reads a user's username, which is nested under its attributes.
	usernameField = regexp.MustCompile(`(?m)^\s+username:\s*(.+?)\s*$`)
	idField       = topLevelScalar("id")
	nameField     = topLevelScalar("name")
	// valueReference reads a reference a document holds in place of a value. A reference is a whole
	// scalar, a mapping's value or a list's item, quoted or not, so the same text inside a longer
	// value is not one.
	//
	// Only block style is read, which is how an export writes a mapping or a list: a reference inside
	// flow style, such as `{a: var:X}` or `[var:X]`, is not found.
	//
	// The groups are the line's indent, a list item's key, a mapping's key, the prefix and the name.
	valueReference = regexp.MustCompile(`^(\s*)(?:-\s+` + mappingKey + `\s*:\s*|-\s*|` + mappingKey + `\s*:\s*)` +
		`['"]?(` + regexp.QuoteMeta(valueref.PrefixVariable) + `|` + regexp.QuoteMeta(valueref.PrefixSecret) +
		`)([A-Za-z_][A-Za-z0-9_]*)['"]?\s*$`)
	// keyOnly reads a line that opens a mapping or a list under a key.
	keyOnly = regexp.MustCompile(`^(\s*)(?:-\s+)?` + mappingKey + `\s*:\s*$`)
)

// resourceTypeUser is the resource type an export writes a user under.
const resourceTypeUser = "user"

// mappingKey reads a block mapping's key, quoted or plain, so a key such as `"a.b"` or `a.b` is read
// as one. A plain key starts with no character that would open a list, flow style or a comment.
const mappingKey = `("[^"]*"|'[^']*'|[^\s'"#:,\-\[\]{}][^:#]*?)`

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

	sortChanges(changes)
	return changes
}

// sortChanges orders changes by resource type, then by name and id.
func sortChanges(changes []Change) {
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].ResourceType != changes[j].ResourceType {
			return changes[i].ResourceType < changes[j].ResourceType
		}
		return changes[i].Name+changes[i].ID < changes[j].Name+changes[j].ID
	})
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
	return Change{Key: resource.key(), ResourceType: resource.Type, ID: resource.ID, Name: resource.Name,
		Change: kind}
}

func summarize(changes []Change) DiffSummary {
	var summary DiffSummary
	for _, change := range changes {
		if change.Excluded {
			continue
		}
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
	for _, reference := range referencesOf(content) {
		if seen[reference.Kind+reference.Name] {
			continue
		}
		seen[reference.Kind+reference.Name] = true
		if reference.Kind == ReferenceSecret {
			secrets = append(secrets, reference.Name)
		} else {
			variables = append(variables, reference.Name)
		}
	}
	sort.Strings(variables)
	sort.Strings(secrets)
	return variables, secrets
}

// referencesOf returns each place a version refers to a variable or a secret, in document order.
//
// A reference is a whole scalar, a mapping's value or a list's item, quoted or not, so the same text
// inside a longer value is not one. A list item's field is the key of the list it is in.
func referencesOf(content string) []ValueReference {
	var references []ValueReference
	for _, resource := range parseBundle(content) {
		lines := strings.Split(resource.Content, "\n")
		for i, line := range lines {
			match := valueReference.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			field := unquoteKey(match[2] + match[3])
			list := field == ""
			if list {
				field = enclosingKey(lines, i, len(match[1]))
			}
			kind := ReferenceVariable
			if match[4] == valueref.PrefixSecret {
				kind = ReferenceSecret
			}
			references = append(references, ValueReference{
				Name:         match[5],
				Kind:         kind,
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				ResourceName: labelOf(resource),
				Field:        field,
				List:         list,
			})
		}
	}
	return references
}

// referencesTo returns where a version refers to the values missing names, so each can be told
// apart by what it is for.
func referencesTo(content string, missing *MissingValues) []ValueReference {
	wanted := map[string]bool{}
	for _, name := range missing.Variables {
		wanted[ReferenceVariable+name] = true
	}
	for _, name := range missing.Secrets {
		wanted[ReferenceSecret+name] = true
	}
	var references []ValueReference
	for _, reference := range referencesOf(content) {
		if wanted[reference.Kind+reference.Name] {
			references = append(references, reference)
		}
	}
	return references
}

// enclosingKey finds the key of the list an item at the given indent belongs to: the nearest key
// above that opens a block at the same indent or less.
func enclosingKey(lines []string, item, indent int) string {
	for i := item - 1; i >= 0; i-- {
		match := keyOnly.FindStringSubmatch(lines[i])
		if len(match) == 3 && len(match[1]) <= indent {
			return unquoteKey(match[2])
		}
	}
	return ""
}

// unquoteKey returns a mapping key without the quotes around it.
func unquoteKey(key string) string {
	if len(key) >= 2 && (key[0] == '"' || key[0] == '\'') && key[len(key)-1] == key[0] {
		return key[1 : len(key)-1]
	}
	return key
}

// labelOf is how a resource is known to a person: its name, or a user's username. Only a user is
// known by a username, since another resource may hold a nested username of its own.
func labelOf(resource bundleResource) string {
	if resource.Name != "" || resource.Type != resourceTypeUser {
		return resource.Name
	}
	return firstMatch(usernameField, resource.Content)
}
