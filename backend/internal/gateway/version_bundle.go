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
		if previous, ok := heldByKey[resource.key()]; ok {
			kind = ChangeUpdated
			if previous.Content == resource.Content {
				kind = ChangeUnchanged
			}
		}
		changes = append(changes, changeOf(resource, kind))
	}
	for _, resource := range held {
		if _, ok := appliedByKey[resource.key()]; !ok {
			changes = append(changes, changeOf(resource, ChangeDeleted))
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
