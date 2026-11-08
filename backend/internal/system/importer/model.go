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
}
