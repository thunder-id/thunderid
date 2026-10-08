// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"time"

	"github.com/thunder-id/thunderid/internal/system/export"
)

// Version is a captured state of this deployment's configuration.
//
// A version is identified by a hash of what it captured, so the same configuration is always the same
// version, and it never changes once captured: applying one to a gateway is a deliberate choice and an
// earlier one stays available to apply again.
type Version struct {
	// Seq orders a deployment's versions. It is kept for ordering, retention and what a gateway holds,
	// and is not shown: a version is named by its hash.
	Seq int `json:"-"`
	// Hash is the SHA-256 of the captured configuration, in hex. A shorter prefix also names it.
	Hash string `json:"version"`
	// Name is what the version is called; Note says more about it.
	Name      string    `json:"name,omitempty"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	// Resources is the exported configuration, one document per resource. A listing leaves it out.
	Resources string `json:"resources,omitempty"`
	// Skipped names the resources the export could not write, each with why, as an export reports
	// them. Only the capture's own answer carries it; it is not kept with the version.
	Skipped []export.ExportError `json:"skipped,omitempty"`
	// Unchanged is set on a capture's answer when the configuration matched a version already captured,
	// which is answered instead of capturing the same configuration again.
	Unchanged bool `json:"unchanged,omitempty"`
	// variables are the values the export carried beside the documents, sealed as one secret. A
	// reference-style export carries none. They are never returned by a read.
	variables string
}

// CaptureRequest is the body of a capture.
type CaptureRequest struct {
	Name string `json:"name,omitempty"`
	Note string `json:"note,omitempty"`
}

// AppliedVersion is what a gateway holds: the version last applied to it, and the one before, which
// is what a revert returns it to.
type AppliedVersion struct {
	GatewayID string `json:"gatewayId"`
	// AppliedVersion and PreviousVersion are the versions' places in order, 0 for none; Applied and
	// Previous are their hashes, which is how they are shown.
	AppliedVersion  int        `json:"-"`
	PreviousVersion int        `json:"-"`
	Applied         string     `json:"appliedVersion,omitempty"`
	Previous        string     `json:"previousVersion,omitempty"`
	AppliedAt       *time.Time `json:"appliedAt,omitempty"`
}

// ApplyRequest is the body of an apply. Version is a version's hash, a prefix of at least seven of its
// characters, or "latest", and defaults to "latest".
type ApplyRequest struct {
	Version string `json:"version,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
}

// RevertRequest is the body of a revert.
type RevertRequest struct {
	DryRun bool `json:"dryRun,omitempty"`
}

// ChangeKind says how a resource differs between what a gateway holds and what is applied to it.
type ChangeKind string

const (
	// ChangeAdded is a resource the gateway does not hold yet.
	ChangeAdded ChangeKind = "added"
	// ChangeUpdated is a resource the gateway holds in another form.
	ChangeUpdated ChangeKind = "updated"
	// ChangeDeleted is a resource the gateway holds that the applied version no longer has.
	ChangeDeleted ChangeKind = "deleted"
	// ChangeUnchanged is a resource the gateway already holds as it is.
	ChangeUnchanged ChangeKind = "unchanged"
)

// Change is one resource's difference.
type Change struct {
	ResourceType string     `json:"resourceType"`
	ID           string     `json:"id,omitempty"`
	Name         string     `json:"name,omitempty"`
	Change       ChangeKind `json:"change"`
	// Lines is the resource's document diffed line by line, from what the gateway holds to what is
	// applied. An unchanged resource has none.
	Lines []LineOp `json:"lines,omitempty"`
}

// LineOp is one line of a diff: Kind is " " for a line both sides have, "+" for one only the applied
// side has, and "-" for one only the held side has.
type LineOp struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// DiffSummary counts the changes by kind.
type DiffSummary struct {
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

// Diff is how a version differs from what a gateway holds.
type Diff struct {
	// FromVersion and ToVersion are the hashes of what the gateway holds and of what is applied.
	FromVersion string      `json:"fromVersion,omitempty"`
	ToVersion   string      `json:"toVersion"`
	Summary     DiffSummary `json:"summary"`
	Changes     []Change    `json:"changes"`
}

// ApplyResult is what an apply or a revert reports.
type ApplyResult struct {
	GatewayID string `json:"gatewayId"`
	DryRun    bool   `json:"dryRun"`
	Diff      Diff   `json:"diff"`
	// Import is the gateway's own account of the import, passed through as it answered.
	Import json.RawMessage `json:"import,omitempty"`
	// Recorded reports whether the gateway's applied version moved, which every completed apply does
	// and a dry run never does. Import says which resources, if any, the gateway refused.
	Recorded bool `json:"recorded"`
	// Missing lists the values the version refers to that the gateway does not hold. Only a dry run
	// reports them: an apply that would leave one unset is refused instead.
	Missing *MissingValues `json:"missing,omitempty"`
}

// MissingValues are the variables and secrets a version refers to that a gateway does not hold.
type MissingValues struct {
	Variables []string `json:"variables,omitempty"`
	Secrets   []string `json:"secrets,omitempty"`
}

// empty reports whether nothing is missing.
func (m MissingValues) empty() bool {
	return len(m.Variables) == 0 && len(m.Secrets) == 0
}
