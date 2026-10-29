// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import "time"

// Master is what one gateway runs, resource by resource: every resource deployed to it, as the
// gateway last accepted it, with how each one got there.
//
// A captured version is a deployment's whole configuration as designed; a master is a gateway's
// truth. An apply sends a gateway only what differs, and the master is updated from the import's own
// outcome, one resource at a time, so a resource the gateway refused is recorded as failed beside the
// ones it accepted. A master is never sent as a whole. Each apply writes a new master and keeps the
// one before, so a gateway's history is its masters, and a resource's history is its entry across them.
type Master struct {
	GatewayID string `json:"gatewayId"`
	// Seq orders a gateway's masters, from 1; 0 is the empty master of a gateway nothing was applied to.
	Seq int `json:"number"`
	// Hash names the master by what it holds: the resources and each entry's state and source.
	Hash      string    `json:"master,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	// CreatedBy is the user whose apply wrote the master.
	CreatedBy *Actor `json:"createdBy,omitempty"`
	// Resources is every resource the gateway holds, one document each, in the same form a captured
	// version holds them. A listing leaves it out.
	Resources string `json:"resources,omitempty"`
	// Summary counts the entries by state.
	Summary MasterSummary `json:"summary"`
	// Entries is the manifest: one entry per resource key, sorted by key. A listing leaves it out.
	Entries []MasterEntry `json:"entries,omitempty"`
}

// EntryState says what a master records of a resource.
type EntryState string

const (
	// EntryDeployed is a resource the gateway accepted; its document is in the master.
	EntryDeployed EntryState = "deployed"
	// EntryFailed is a resource the gateway refused on its last apply. The master keeps the document
	// the gateway last accepted, if it ever did, and the failure, so the resource is offered again.
	EntryFailed EntryState = "failed"
	// EntryRemoved is a resource removed from the gateway. The entry has no document and stays until
	// something is deployed under its key again, so the removal is visible.
	EntryRemoved EntryState = "removed"
)

// MasterEntry is how one resource got onto a gateway.
type MasterEntry struct {
	Key          string `json:"key"`
	ResourceType string `json:"resourceType"`
	ID           string `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	// ContentHash is the SHA-256 of the document as the master holds it; none for a removed resource
	// or one the gateway never accepted.
	ContentHash string `json:"contentHash,omitempty"`
	// SourceVersion is the hash of the version the last apply of this resource sent it from.
	SourceVersion string `json:"sourceVersion,omitempty"`
	// Origin is the label the apply gave for where the version came from, such as another
	// environment, and is recorded as given.
	Origin  string        `json:"origin,omitempty"`
	State   EntryState    `json:"state"`
	Failure *EntryFailure `json:"failure,omitempty"`
	// At and By are when the entry was last written and by whose apply.
	At time.Time `json:"at,omitempty"`
	By *Actor    `json:"by,omitempty"`
}

// EntryFailure is what the gateway answered when it refused a resource.
type EntryFailure struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// MasterSummary counts a master's entries by state.
type MasterSummary struct {
	Deployed int `json:"deployed"`
	Failed   int `json:"failed"`
	Removed  int `json:"removed"`
}

// ResourceHistory is one resource's entry across a gateway's masters, newest first.
type ResourceHistory struct {
	GatewayID string                 `json:"gatewayId"`
	Key       string                 `json:"key"`
	History   []ResourceHistoryEntry `json:"history"`
}

// ResourceHistoryEntry is a resource's entry in one master.
type ResourceHistoryEntry struct {
	// Master is the hash of the master the entry is from, and Number its place in order.
	Master string      `json:"master"`
	Number int         `json:"number"`
	Entry  MasterEntry `json:"entry"`
}
