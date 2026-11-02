// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"time"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// Version is a captured state of this deployment's configuration.
//
// A version is identified by a hash of what it captured, so the same configuration is always the same
// version, and it never changes once captured: applying one to a gateway is a deliberate choice and an
// earlier one stays available to apply again.
type Version struct {
	// Seq orders a deployment's versions, from 1, and is kept for retention and what a gateway holds. It
	// is shown as the version's number, which stays the same as older versions are removed; a version
	// is named by its hash.
	Seq int `json:"number"`
	// Hash is the SHA-256 of the captured configuration, in hex. A shorter prefix also names it.
	Hash string `json:"version"`
	// Name is what the version is called; Note says more about it.
	Name      string    `json:"name,omitempty"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	// CreatedBy is the user the version was captured for: who captured it, or whose change a capture
	// after a change followed. A current version replaced in place names the latest change's user.
	CreatedBy *Actor `json:"createdBy,omitempty"`
	// Resources is the exported configuration, one document per resource. A listing leaves it out.
	Resources string `json:"resources,omitempty"`
	// Skipped names the resources the export could not write, each with why, as an export reports
	// them. Only the capture's own answer carries it; it is not kept with the version.
	Skipped []declarativeresource.ExportError `json:"skipped,omitempty"`
	// Unchanged is set on a capture's answer when the configuration matched a version already captured,
	// which is answered instead of capturing the same configuration again.
	Unchanged bool `json:"unchanged,omitempty"`
	// References lists each place the version refers to a variable or a secret. Only a read of one
	// version carries it.
	References []ValueReference `json:"references,omitempty"`
	// Current marks the newest version when it was captured as the current one: a capture asked to
	// keep the configuration current replaces it until a gateway holds it. A listing sets it on the
	// newest version alone.
	Current bool `json:"current,omitempty"`
	// variables are the values the export carried beside the documents, sealed as one secret. A
	// reference-style export carries none. They are never returned by a read.
	variables string
}

// Kinds of value a reference names.
const (
	ReferenceVariable = "variable"
	ReferenceSecret   = "secret"
)

// ValueReference is one place a version refers to a variable or a secret by name: the resource that
// holds the reference and the field it stands in, so the value can be told apart by what it is for
// rather than by its name alone.
type ValueReference struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	// ResourceName is how the resource is known: its name, or a user's username.
	ResourceName string `json:"resourceName,omitempty"`
	// Field is the key the reference is the value of, or the list's key for a list item.
	Field string `json:"field,omitempty"`
	// List is set when the reference is a list's item: the value it names holds the list's items, as a
	// JSON list of strings or a single string.
	List bool `json:"list,omitempty"`
}

// CaptureRequest is the body of a capture.
type CaptureRequest struct {
	Name string `json:"name,omitempty"`
	Note string `json:"note,omitempty"`
	// Current keeps the configuration as the current version rather than adding one: the newest
	// version is replaced when it was captured as current and no gateway holds it or could be
	// reverted to it. Once a gateway holds it, it stays, and the next such capture adds a new current
	// version. A console that captures on its own sets this, so it keeps one version up to date
	// rather than one for every change.
	Current bool `json:"current,omitempty"`
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
	// AppliedBy is the user who applied the version, or reverted the gateway to it.
	AppliedBy *Actor `json:"appliedBy,omitempty"`
	// heldValues is when each value the version refers to was last set, as the gateway's store said
	// when it was applied, for the next apply to tell which were set again since.
	heldValues string
}

// ApplyRequest is the body of an apply. Version is a version's hash, a prefix of at least seven of its
// characters, or "latest", and defaults to "latest".
//
// Selection names, by key, the changes to apply. Every other change the diff reports is left alone,
// and the gateway goes on leaving it alone in later applies until one selects it again. Without a
// selection, the gateway's standing choices apply; an empty one leaves every change alone.
//
// Skip names, by key, changes to leave alone in this apply only: they are neither written nor
// removed, and the gateway's standing choice for them is kept as it is. A skipped change stays owed to
// the gateway, so the next diff offers it again, until an apply writes or removes it or a selection
// leaves it alone for good. A caller that applies what another gateway runs skips what that gateway
// left out, which is not this gateway's choice to keep. A selection decides the other changes.
type ApplyRequest struct {
	Version   string    `json:"version,omitempty"`
	DryRun    bool      `json:"dryRun,omitempty"`
	Selection *[]string `json:"selection,omitempty"`
	Skip      []string  `json:"skip,omitempty"`
	// ConfirmRemovals lets the apply go ahead when it would remove more resources than the
	// deployment's threshold; without it such an apply is refused, so a rollback that would empty
	// a gateway is seen before it is sent.
	ConfirmRemovals bool `json:"confirmRemovals,omitempty"`
}

// RevertRequest is the body of a revert.
type RevertRequest struct {
	DryRun          bool `json:"dryRun,omitempty"`
	ConfirmRemovals bool `json:"confirmRemovals,omitempty"`
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

// Change is one resource's difference. Key names the resource for a selection. Excluded marks a
// change the gateway is set to leave alone, which an apply neither writes nor removes. Skipped marks a
// change an earlier apply skipped, offered again until an apply takes it.
type Change struct {
	Key          string     `json:"key"`
	ResourceType string     `json:"resourceType"`
	ID           string     `json:"id,omitempty"`
	Name         string     `json:"name,omitempty"`
	Change       ChangeKind `json:"change"`
	Excluded     bool       `json:"excluded,omitempty"`
	Skipped      bool       `json:"skipped,omitempty"`
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

// DiffSummary counts the changes by kind, leaving out those the gateway is set to leave alone.
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
	// MisplacedSecrets are the places the version refers to a secret from a field that holds no
	// credential. A gateway refuses such a resource on import, so they are reported here first.
	MisplacedSecrets []ValueReference `json:"misplacedSecrets,omitempty"`
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
	// Changed lists the values the version refers to that were set on the gateway after the version
	// it holds was applied, so a value that changed outside any version is seen before it takes
	// effect. Names only, never values.
	Changed *ChangedValues `json:"changed,omitempty"`
	// Queued reports that the latest version was not applied now, because another apply to the
	// gateway runs, and will be applied once that finishes. QueuedVersion is the version it will be.
	Queued        bool   `json:"queued,omitempty"`
	QueuedVersion string `json:"queuedVersion,omitempty"`
}

// ChangedValues are the variables and secrets a version refers to whose values were set on a gateway
// after the version it holds was applied.
type ChangedValues struct {
	Variables []string `json:"variables,omitempty"`
	Secrets   []string `json:"secrets,omitempty"`
	// References says where the version refers to each changed value.
	References []ValueReference `json:"references,omitempty"`
}

// empty reports whether nothing changed.
func (c ChangedValues) empty() bool {
	return len(c.Variables) == 0 && len(c.Secrets) == 0
}

// MissingValues are the variables and secrets a version refers to that a gateway does not hold.
type MissingValues struct {
	Variables []string `json:"variables,omitempty"`
	Secrets   []string `json:"secrets,omitempty"`
	// References says where the version refers to each missing value.
	References []ValueReference `json:"references,omitempty"`
}

// empty reports whether nothing is missing.
func (m MissingValues) empty() bool {
	return len(m.Variables) == 0 && len(m.Secrets) == 0
}
