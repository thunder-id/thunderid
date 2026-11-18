// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"regexp"
	"sort"
	"strings"
	"sync"

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
	// scan holds what reading the document finds, read the first time it is asked for and shared by
	// every copy of the resource, so a bundle kept between requests is read once.
	scan *resourceScan
}

// resourceScan is what a resource's document says beyond its top-level identity, each part read the
// first time it is asked for: where it refers to values, what it may name, and the fields it is told
// apart by.
type resourceScan struct {
	referencesOnce sync.Once
	// references are the places the document refers to a variable or a secret, in document order.
	references []ValueReference

	namesOnce sync.Once
	// candidates are the scalars that may name another resource by id, in document order.
	candidates []string
	// typesNamed are the types the document names by handle, each as its kind and handle.
	typesNamed []string

	fieldsOnce sync.Once
	// handle and channel are the document's own top-level handle and channel, and owner the project
	// the resource belongs to, "" for none.
	handle, channel, owner string
}

// scanned returns what the resource's document says, read once per parsed resource. A resource built
// without a scan, as a test may, is read each time.
func (r bundleResource) scanned() *resourceScan {
	if r.scan == nil {
		return &resourceScan{}
	}
	return r.scan
}

// referencesOf returns where the resource's document refers to values.
func (r bundleResource) referencesOf() []ValueReference {
	scan := r.scanned()
	scan.referencesOnce.Do(func() { scan.references = referencesInLines(r, strings.Split(r.Content, "\n")) })
	return scan.references
}

// namesOf returns the scalars the resource's document may name another resource by, and the types it
// names by handle.
func (r bundleResource) namesOf() (candidates, types []string) {
	scan := r.scanned()
	scan.namesOnce.Do(func() {
		lines := strings.Split(r.Content, "\n")
		scan.candidates = candidatesIn(lines)
		scan.typesNamed = typesNamedIn(r, lines)
	})
	return scan.candidates, scan.typesNamed
}

// fieldsOf returns the resource's own handle, channel and project.
func (r bundleResource) fieldsOf() (handle, channel, owner string) {
	scan := r.scanned()
	scan.fieldsOnce.Do(func() {
		scan.handle = firstMatch(handleField, r.Content)
		if r.Type == resourceTypeNotificationTemplate {
			scan.channel = firstMatch(channelField, r.Content)
		}
		if r.Type == resourceTypeProject {
			scan.owner = r.ID
		} else {
			scan.owner = firstMatch(projectIDField, r.Content)
		}
	})
	return scan.handle, scan.channel, scan.owner
}

// key identifies a resource across versions.
func (r bundleResource) key() string { return resourceKey(r.Type, r.ID, r.Name) }

// resourceKey identifies a resource across versions: its type with its id, or its name when it has
// no id.
func resourceKey(resourceType, id, name string) string {
	if id == "" {
		id = name
	}
	return resourceType + "/" + id
}

// topLevelField reads a column-zero key, which is a resource's own field rather than a nested one.
type topLevelField struct {
	// opening is how a line holding the key opens.
	opening string
	pattern *regexp.Regexp
}

// topLevelScalar reads a column-zero key's value.
func topLevelScalar(key string) topLevelField {
	return topLevelField{opening: key + ":", pattern: regexp.MustCompile(`(?m)^` + key + `:\s*(.+?)\s*$`)}
}

var (
	documentSeparator = regexp.MustCompile(`(?m)^---\s*$`)
	resourceTypeField = topLevelScalar("resource_type")
	// displayNameField and handleField read the name a theme, a layout or a user type is shown by.
	displayNameField = topLevelScalar("displayName")
	handleField      = topLevelScalar("handle")
	idField          = topLevelScalar("id")
	nameField        = topLevelScalar("name")
	// languageField names a translation, which has neither an id nor a name: one document holds one
	// language's translations.
	languageField = topLevelScalar("language")
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
	// keyedValue reads a mapping's key and the value after it. The groups are what is kept, the key
	// and the value.
	keyedValue = regexp.MustCompile(`^(\s*` + mappingKey + `\s*:\s+)(\S.*)$`)
)

// maskedValue is what a diff shows in place of a value it keeps to itself.
const maskedValue = "********"

// resourceTypeUser is the resource type an export writes a user under.
const resourceTypeUser = "user"

// resourceTypeRole and resourceTypeGroup are the resource types an export writes a role and a group under.
const (
	resourceTypeRole  = "role"
	resourceTypeGroup = "group"
)

// usernameAttribute is the attribute a user is known by.
const usernameAttribute = "username"

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
			Name:    nameOf(document),
			Content: document,
			scan:    &resourceScan{},
		})
	}
	return resources
}

// nameOf reads a resource's name, or a translation's language in its place.
func nameOf(document string) string {
	if name := firstMatch(nameField, document); name != "" {
		return name
	}
	return firstMatch(languageField, document)
}

// joinBundle writes documents back as one version.
func joinBundle(documents []string) string {
	return strings.Join(documents, "\n---\n")
}

// firstMatch reads a top-level field's value, unquoted. The field's pattern only matches at the start
// of a line that opens with its key, so a document without such a line is not searched, and one with it
// is searched from the first, where the pattern's own search would start finding matches.
func firstMatch(field topLevelField, document string) string {
	at := lineStartOf(document, field.opening)
	if at < 0 {
		return ""
	}
	match := field.pattern.FindStringSubmatch(document[at:])
	if match == nil {
		return ""
	}
	return strings.Trim(match[1], `"'`)
}

// lineStartOf finds the first place in a document a line opens with prefix, or -1 when none does.
func lineStartOf(document, prefix string) int {
	if strings.HasPrefix(document, prefix) {
		return 0
	}
	at := strings.Index(document, "\n"+prefix)
	if at < 0 {
		return -1
	}
	return at + 1
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
		change.Lines = documentDiff(resource.Type, previous.Content, resource.Content)
		changes = append(changes, change)
	}
	for _, resource := range held {
		if _, ok := appliedByKey[resource.key()]; !ok {
			change := changeOf(resource, ChangeDeleted)
			change.Lines = documentDiff(resource.Type, resource.Content, "")
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

// documentDiff diffs a resource's documents line by line. A user's attribute values are personal
// data, so a user's diff shows them masked: which attributes changed stays visible, not what they
// hold. The lines are compared as they are, so a changed value still shows as removed and added.
func documentDiff(resourceType, from, to string) []LineOp {
	ops := lineDiff(from, to)
	if resourceType != resourceTypeUser || ops == nil {
		return ops
	}
	shownFrom, shownTo := maskAttributeValues(splitLines(from)), maskAttributeValues(splitLines(to))
	i, j := 0, 0
	for k := range ops {
		switch ops[k].Kind {
		case " ":
			ops[k].Text = shownFrom[i]
			i++
			j++
		case "-":
			ops[k].Text = shownFrom[i]
			i++
		case "+":
			ops[k].Text = shownTo[j]
			j++
		}
	}
	return ops
}

// maskAttributeValues returns a user's document lines with each value under its attributes masked.
// Each attribute's key stays, and so does the username, since it is how the user is known. A line
// nested deeper than the attributes is part of a value whatever its shape, so it is masked whole.
func maskAttributeValues(lines []string) []string {
	masked := make([]string, len(lines))
	inside, indent := false, ""
	for i, line := range lines {
		masked[i] = line
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		lineIndent := line[:len(line)-len(trimmed)]
		if lineIndent == "" {
			inside, indent = strings.TrimRight(line, " ") == "attributes:", ""
			continue
		}
		if !inside {
			continue
		}
		if indent == "" {
			indent = lineIndent
		}
		if lineIndent != indent {
			masked[i] = lineIndent + maskedValue
			continue
		}
		if match := keyedValue.FindStringSubmatch(line); match != nil {
			if unquoteKey(match[2]) != usernameAttribute {
				masked[i] = match[1] + maskedValue
			}
			continue
		}
		if !keyOnly.MatchString(line) {
			masked[i] = lineIndent + maskedValue
		}
	}
	return masked
}

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
	return Change{Key: resource.key(), ResourceType: resource.Type, ID: resource.ID, Name: labelOf(resource),
		Change: kind}
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
	return referenceNames(referencesOf(content))
}

// referenceNames returns the variables and secrets references name, each name once and sorted.
func referenceNames(references []ValueReference) (variables, secrets []string) {
	seen := map[string]bool{}
	for _, reference := range references {
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
	return referencesAmong(parseBundle(content))
}

// referencesAmong returns each place resources refer to a variable or a secret, in document order.
func referencesAmong(resources []bundleResource) []ValueReference {
	var references []ValueReference
	for _, resource := range resources {
		references = append(references, resource.referencesOf()...)
	}
	return references
}

// referencesInLines returns each place one resource's document, split into its lines, refers to a
// variable or a secret. A line without either prefix holds no reference, so it is not matched.
func referencesInLines(resource bundleResource, lines []string) []ValueReference {
	var references []ValueReference
	label, named := "", false
	for i, line := range lines {
		if !strings.Contains(line, valueref.PrefixVariable) && !strings.Contains(line, valueref.PrefixSecret) {
			continue
		}
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
		if !named {
			label, named = labelOf(resource), true
		}
		references = append(references, ValueReference{
			Name:         match[5],
			Kind:         kind,
			ResourceType: resource.Type,
			ResourceID:   resource.ID,
			ResourceName: label,
			Field:        field,
			List:         list,
		})
	}
	return references
}

// referencesFor returns the references to the values missing names, so each can be told apart by what
// it is for.
func referencesFor(references []ValueReference, missing *MissingValues) []ValueReference {
	wanted := map[string]bool{}
	for _, name := range missing.Variables {
		wanted[ReferenceVariable+name] = true
	}
	for _, name := range missing.Secrets {
		wanted[ReferenceSecret+name] = true
	}
	var found []ValueReference
	for _, reference := range references {
		if wanted[reference.Kind+reference.Name] {
			found = append(found, reference)
		}
	}
	return found
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

// labelOf is how a resource is known to a person: its name, or else its display name or handle, as a
// theme, a layout or a user type has, or the name it holds among its attributes: a user's username,
// email or name, an agent's name. Attributes are read only for a user and an agent, and only from their
// own attributes block, since another resource may hold nested fields of the same names.
func labelOf(resource bundleResource) string {
	if resource.Name != "" {
		return resource.Name
	}
	if label := firstMatch(displayNameField, resource.Content); label != "" {
		return label
	}
	if label := firstMatch(handleField, resource.Content); label != "" {
		return label
	}
	var keys []string
	switch resource.Type {
	case resourceTypeUser:
		keys = []string{usernameAttribute, "email", "name"}
	case resourceTypeAgent:
		keys = []string{"name", usernameAttribute, "email"}
	}
	for _, key := range keys {
		if label := attributeOf(resource.Content, key); label != "" {
			return label
		}
	}
	return ""
}

// resourceTypeAgent is the resource type an export writes an agent under.
const resourceTypeAgent = "agent"

// attributeOf reads a scalar of a document's top-level attributes block, unquoted, or "" when it has
// none.
func attributeOf(document, key string) string {
	inside, indent := false, ""
	for _, line := range strings.Split(document, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if !inside {
			inside = strings.TrimRight(line, " ") == "attributes:"
			continue
		}
		if trimmed == "" {
			continue
		}
		lineIndent := line[:len(line)-len(trimmed)]
		if lineIndent == "" {
			return ""
		}
		if indent == "" {
			indent = lineIndent
		}
		if lineIndent != indent || !strings.HasPrefix(trimmed, key+":") {
			continue
		}
		return strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, key+":")), `"'`)
	}
	return ""
}
