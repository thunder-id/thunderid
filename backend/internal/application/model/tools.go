// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

// ApplicationListOutput represents the output for list_applications tool.
type ApplicationListOutput struct {
	TotalCount   int                        `json:"total_count" jsonschema:"Total number of applications available."`
	Applications []BasicApplicationResponse `json:"applications" jsonschema:"List of applications."`
}

// ApplicationListInput represents the optional scope of the list_applications tool.
type ApplicationListInput struct {
	ProjectID string `json:"projectId,omitempty" jsonschema:"Project ID. Lists its root organization unit or ouId."`
	OUID      string `json:"ouId,omitempty" jsonschema:"Organization unit ID whose applications to list."`
}

// ClientIDInput represents input for client ID-based lookups.
type ClientIDInput struct {
	ClientID string `json:"client_id" jsonschema:"OAuth client ID to search for"`
}
