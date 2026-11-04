// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"time"

	"github.com/thunder-id/thunderid/internal/system/utils"
)

// Project is a group of identity configuration. Every project includes the default organization unit.
type Project struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ProjectRequest is the body of a create or update project request.
// ID is honored only on create, so a caller that manages projects across systems can keep one ID.
type ProjectRequest struct {
	ID          string `json:"id,omitempty"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}

// ProjectListResponse is a page of projects.
type ProjectListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Projects     []Project    `json:"projects"`
	Links        []utils.Link `json:"links"`
}
