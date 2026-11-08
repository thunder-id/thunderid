// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// DroppedPolicy is a sharing policy a version no longer carries for a resource it sends again. The gateway removes
// it when it applies the version, so it counts as a removal and is shown before the apply.
type DroppedPolicy struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	// InitiatingOUID is the organization unit that issued the policy, empty for the resource's owner.
	InitiatingOUID string `json:"initiatingOuId,omitempty"`
}

var sharingPoliciesKey = regexp.MustCompile(`^sharingPolicies:`)

// sharedResourceTypes are the resource types whose sharing policies a gateway applies from an import, and so the ones a
// dropped policy can be removed from. An agent is not among them: its policies are declared in its own resource file
// on each deployment, and an import never replays them. This mirrors the importer's own list, which a test keeps in
// step.
var sharedResourceTypes = []string{"flow", "application", "role", "user", "connection"}

// SharedResourceTypes returns the resource types whose dropped sharing policies are sent to a gateway to remove.
func SharedResourceTypes() []string {
	return slices.Clone(sharedResourceTypes)
}

// policiesBlock returns the text of a document's top-level sharingPolicies entry, from its key to the next top-level
// key, and whether it has one. A line at the left edge ends the block unless it is a list item or a comment, since
// a list may be written there. The rest of the document is not read: an application's lists are written as template
// ranges, which are not YAML, and the policies must still be read from such a document.
func policiesBlock(content string) (string, bool) {
	var block []string
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimRight(line, " \t\r")
		if !inBlock {
			if sharingPoliciesKey.MatchString(trimmed) {
				inBlock = true
				block = append(block, trimmed)
			}
			continue
		}
		if endsBlock(trimmed) {
			break
		}
		block = append(block, trimmed)
	}
	return strings.Join(block, "\n"), inBlock
}

// endsBlock reports whether a line starts the next top-level key. A line at the left edge continues the block when it
// is a list item, a comment, the end of a flow list, or a template directive: a directive is kept so that a list a
// template writes is seen as one that cannot be read, and not as one that stops early.
func endsBlock(line string) bool {
	if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	for _, continuation := range []string{"-", "#", "]", "{{"} {
		if strings.HasPrefix(line, continuation) {
			return false
		}
	}
	return true
}

// policyInitiators lists the issuing organization unit of each policy under a document's sharingPolicies key, in
// order, an owner's policy listed as the empty string. Only that block is parsed, as YAML, so a list written in flow
// style, a comment, or a quoted value is read as it is written. It reports false when the block cannot be read, such
// as when a template range writes the list, so the caller does not mistake a document it cannot read for one with no
// policies.
func policyInitiators(content string) ([]string, bool) {
	block, found := policiesBlock(content)
	if !found {
		return nil, true
	}
	var parsed struct {
		SharingPolicies []struct {
			InitiatingOUID string `yaml:"initiatingOuId"`
		} `yaml:"sharingPolicies"`
	}
	if err := yaml.Unmarshal([]byte(quotePlaceholders(block)), &parsed); err != nil {
		return nil, false
	}
	initiators := make([]string, 0, len(parsed.SharingPolicies))
	for _, policy := range parsed.SharingPolicies {
		initiators = append(initiators, policy.InitiatingOUID)
	}
	return initiators, true
}

// droppedPolicies lists the policies the documents about to be sent no longer carry, against the documents the
// gateway holds. Only a resource that is sent again is compared: one removed outright is a removal already.
//
// Some resources are not compared. One with no id cannot be named to the gateway, so listing its policies would report
// a removal that is never sent. One of a type a gateway does not apply sharing for would only fail the removal. One
// whose policies cannot be read in either document is returned as unreadable so the caller can say so, since reading it
// as having none would name every policy as dropped.
func droppedPolicies(held, sent []bundleResource) (dropped []DroppedPolicy, unreadable []bundleResource) {
	heldByKey := make(map[string]bundleResource, len(held))
	for _, resource := range held {
		heldByKey[resource.key()] = resource
	}
	for _, resource := range sent {
		previous, ok := heldByKey[resource.key()]
		if !ok || resource.ID == "" || !slices.Contains(sharedResourceTypes, resource.Type) {
			continue
		}
		now, nowReadable := policyInitiators(resource.Content)
		before, beforeReadable := policyInitiators(previous.Content)
		if !nowReadable || !beforeReadable {
			unreadable = append(unreadable, resource)
			continue
		}
		kept := make(map[string]int)
		for _, initiator := range now {
			kept[initiator]++
		}
		for _, initiator := range before {
			if kept[initiator] > 0 {
				kept[initiator]--
				continue
			}
			dropped = append(dropped, DroppedPolicy{ResourceType: resource.Type, ResourceID: resource.ID,
				ResourceName: resource.Name, InitiatingOUID: initiator})
		}
	}
	return dropped, unreadable
}

// sharingRemovalsOf turns the dropped policies into what is sent to the gateway to remove.
func sharingRemovalsOf(dropped []DroppedPolicy) []gatewaySharingRemoval {
	removals := make([]gatewaySharingRemoval, 0, len(dropped))
	for _, policy := range dropped {
		removals = append(removals, gatewaySharingRemoval{
			ResourceType: policy.ResourceType, ID: policy.ResourceID, InitiatingOUID: policy.InitiatingOUID})
	}
	return removals
}

// How an import is answered for a resource whose sharing removals the gateway did not act on.
const (
	sharingRemovalsNotAcceptedCode    = "GTW-1043"
	sharingRemovalsNotAcceptedMessage = "the gateway did not act on the sharing removals sent with this resource, " +
		"so it may be an older version"
)

// rejectUnacceptedRemovals turns the answer to an import that carried sharing removals into a refusal of the resources
// they were for, when the gateway did not say it acts on them. An older gateway ignores the removals and still
// answers success, so the master would record the new document and the dropped policy would never be seen again, even
// after the gateway is upgraded. Refused, the resource stays as the gateway last accepted it and is offered again.
// An answer that cannot be read is left as it is: it is already taken as not saying what the gateway wrote.
func rejectUnacceptedRemovals(answer json.RawMessage, removals []gatewaySharingRemoval) json.RawMessage {
	if len(removals) == 0 {
		return answer
	}
	var parsed map[string]json.RawMessage
	if json.Unmarshal(answer, &parsed) != nil {
		return answer
	}
	var accepted bool
	if raw, ok := parsed["sharingRemovalsAccepted"]; ok && json.Unmarshal(raw, &accepted) == nil && accepted {
		return answer
	}
	var results []map[string]json.RawMessage
	if raw, ok := parsed["results"]; ok && json.Unmarshal(raw, &results) != nil {
		return answer
	}

	type resource struct{ resourceType, id string }
	pending := map[resource]bool{}
	for _, removal := range removals {
		pending[resource{removal.ResourceType, removal.ID}] = true
	}
	failure := func() map[string]json.RawMessage {
		code, _ := json.Marshal(sharingRemovalsNotAcceptedCode)
		message, _ := json.Marshal(sharingRemovalsNotAcceptedMessage)
		status, _ := json.Marshal(outcomeFailed)
		return map[string]json.RawMessage{"status": status, "code": code, "message": message}
	}
	text := func(result map[string]json.RawMessage, key string) string {
		var value string
		_ = json.Unmarshal(result[key], &value)
		return value
	}
	for _, result := range results {
		key := resource{text(result, "resourceType"), text(result, "resourceId")}
		if !pending[key] || text(result, "operation") == operationDeleted {
			continue
		}
		delete(pending, key)
		if text(result, "status") == outcomeSuccess {
			for field, value := range failure() {
				result[field] = value
			}
		}
	}
	// A resource the answer does not name is not taken as written, which it would be if the gateway refused nothing.
	for _, removal := range removals {
		key := resource{removal.ResourceType, removal.ID}
		if !pending[key] {
			continue
		}
		delete(pending, key)
		entry := failure()
		entry["resourceType"], _ = json.Marshal(removal.ResourceType)
		entry["resourceId"], _ = json.Marshal(removal.ID)
		results = append(results, entry)
	}

	rewritten, err := json.Marshal(results)
	if err != nil {
		return answer
	}
	parsed["results"] = rewritten
	out, err := json.Marshal(parsed)
	if err != nil {
		return answer
	}
	return out
}
