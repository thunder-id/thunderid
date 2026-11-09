// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import "time"

const (
	operationCreate = "create"
	operationUpdate = "update"
	// operationSkip reports a document this deployment does not hold, such as a project on a server
	// that does not serve projects.
	operationSkip = "skip"

	statusSuccess = "success"
	statusFailed  = "failed"

	operationDelete = "delete"
)

// ImportRequest carries the YAML payload and variable values used to resolve templates, plus any
// resources to remove.
type ImportRequest struct {
	Content   string                 `json:"content"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	DryRun    bool                   `json:"dryRun,omitempty"`
	Options   *ImportOptions         `json:"options,omitempty"`
	// Deletions removes runtime resources that are no longer part of the desired configuration. It
	// complements the declarative upsert in Content, which can create and update but never remove.
	Deletions []ResourceDeletion `json:"deletions,omitempty"`
	// SharingRemovals names the sharing policies the content no longer carries for the resources it writes. They are
	// removed once the resource is written, and a policy absent from the content is never removed otherwise.
	SharingRemovals []SharingPolicyRemoval `json:"sharingRemovals,omitempty"`
	// Managed is set by the managed import endpoint and never read from the body. What a managed
	// import writes is recorded as deployed by a control plane and becomes read only here.
	Managed bool `json:"-"`
}

// ResourceDeletion identifies a runtime resource to remove during an import.
type ResourceDeletion struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	// Members names, for a role_assignment or group_member removal, who to take off the role or group
	// named by ID. The role or group itself stays.
	Members []RemovedMember `json:"members,omitempty"`
}

// RemovedMember is an assignee taken off a role, or a member taken off a group, during an import.
type RemovedMember struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	// OUID is the organization unit a role assignment was made in. A role's assignee needs it.
	OUID string `json:"ouId,omitempty"`
}

// ImportOptions controls runtime import behavior.
type ImportOptions struct {
	Upsert          *bool  `json:"upsert,omitempty"`
	ContinueOnError *bool  `json:"continueOnError,omitempty"`
	Target          string `json:"target,omitempty"`
	// ForceSharingEdits applies a change to a sharing policy that the sharing policies API refuses, such as a
	// different reach, by rewriting the policy and removing the reshares that lose their standing. A control plane
	// sets it when it publishes a version to a gateway.
	ForceSharingEdits bool `json:"forceSharingEdits,omitempty"`
}

// IsUpsertEnabled returns whether upsert behavior is enabled.
// Defaults to true when the option is omitted.
func (o *ImportOptions) IsUpsertEnabled() bool {
	if o == nil || o.Upsert == nil {
		return true
	}

	return *o.Upsert
}

// IsContinueOnErrorEnabled returns whether import should continue after item-level failures.
// Defaults to true when the option is omitted.
func (o *ImportOptions) IsContinueOnErrorEnabled() bool {
	if o == nil || o.ContinueOnError == nil {
		return true
	}

	return *o.ContinueOnError
}

// ImportResponse captures overall and per-document outcomes.
type ImportResponse struct {
	Summary *ImportSummary      `json:"summary"`
	Results []ImportItemOutcome `json:"results"`
	// SharingRemovalsAccepted is set when the request carried sharingRemovals, to say this deployment acts on them.
	// An older deployment ignores the field and does not set it, so a caller can tell its removals were not applied.
	SharingRemovalsAccepted bool `json:"sharingRemovalsAccepted,omitempty"`
}

// DeleteResourceRequest identifies a declarative resource file to remove.
type DeleteResourceRequest struct {
	ResourceType string `json:"resourceType"`
	ResourceKey  string `json:"resourceKey"`
}

// DeleteResourceResponse reports the deleted declarative resource file outcome.
type DeleteResourceResponse struct {
	ResourceType string `json:"resourceType"`
	ResourceKey  string `json:"resourceKey"`
	DeletedFile  string `json:"deletedFile"`
}

// ImportSummary aggregates import metrics.
type ImportSummary struct {
	TotalDocuments int `json:"totalDocuments"`
	Imported       int `json:"imported"`
	// Deleted counts the resources removed via the request's deletions.
	Deleted    int       `json:"deleted,omitempty"`
	Failed     int       `json:"failed"`
	ImportedAt time.Time `json:"importedAt"`
}

// ImportItemOutcome reports the result of one resource document.
type ImportItemOutcome struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	Operation    string `json:"operation,omitempty"`
	Status       string `json:"status"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
	// RemovedSharing lists the sharing policies the import removed from the resource to match the document: ones
	// the document no longer names and the reshares that lost their standing with them.
	RemovedSharing []RemovedSharingPolicy `json:"removedSharing,omitempty"`
}

// RemovedSharingPolicy is one sharing policy an import removed.
type RemovedSharingPolicy struct {
	PolicyID       string `json:"policyId"`
	InitiatingOUID string `json:"initiatingOuId,omitempty"`
	// Reshare is true for a policy an organization unit issued beneath the owner's, false for the owner's own.
	Reshare bool `json:"reshare"`
}
