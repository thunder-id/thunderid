// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// User represents a user in the system.
type User struct {
	ID         string          `json:"id,omitempty"`
	OUID       string          `json:"ouId,omitempty"`
	OUHandle   string          `json:"ouHandle,omitempty"`
	Type       string          `json:"type,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	Display    string          `json:"display,omitempty"`
	IsReadOnly bool            `json:"isReadOnly"`
	CreatedAt  string          `json:"createdAt,omitempty"`
	UpdatedAt  string          `json:"updatedAt,omitempty"`
}

// Credential represents the credentials of a user.
type Credential struct {
	StorageType       string                   `json:"storageType"`
	StorageAlgo       cryptolib.CredAlgorithm  `json:"storageAlgo"`
	StorageAlgoParams cryptolib.CredParameters `json:"storageAlgoParams"`
	Value             string                   `json:"value"`
}

// Credentials represents the credential storage structure where credentials are organized by type.
// Key: Credential type (e.g., "password", "pin", "secret", "passkey")
// Value: Array of credentials of that type
type Credentials map[CredentialType][]Credential

// UserListResponse represents the response for listing users with pagination.
type UserListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Users        []User       `json:"users"`
	Links        []utils.Link `json:"links"`
}

// UserGroup represents a group with basic information for user endpoints.
type UserGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	OUID string `json:"ouId"`
}

// UserGroupListResponse represents the response for listing groups that a user belongs to.
type UserGroupListResponse struct {
	TotalResults int                     `json:"totalResults"`
	StartIndex   int                     `json:"startIndex"`
	Count        int                     `json:"count"`
	Groups       []providers.EntityGroup `json:"groups"`
	Links        []utils.Link            `json:"links"`
}

// CreateUserRequest represents the request body for creating a user.
type CreateUserRequest struct {
	OUID       string          `json:"ouId"                 native:"required"`
	Type       string          `json:"type"                 native:"required"`
	Groups     []string        `json:"groups,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty" native:"omitempty"`
}

// UpdateUserRequest represents the request body for updating a user.
type UpdateUserRequest struct {
	OUID       string          `json:"ouId,omitempty"`
	Type       string          `json:"type,omitempty"`
	Groups     []string        `json:"groups,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

// UpdateSelfUserRequest represents the request body for updating the authenticated user.
type UpdateSelfUserRequest struct {
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

// UpdateUserCredentialsRequest represents the request body for updating user credentials by an admin.
type UpdateUserCredentialsRequest struct {
	Credentials json.RawMessage `json:"credentials,omitempty"`
}

// CreateUserByPathRequest represents the request body for creating a user under a handle path.
type CreateUserByPathRequest struct {
	Type       string          `json:"type"                 native:"required"`
	Groups     []string        `json:"groups,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

// entityToUser converts an Entity to a User.
func entityToUser(e *providers.Entity) User {
	user := User{
		ID:         e.ID,
		OUID:       e.OUID,
		Type:       e.Type,
		Attributes: e.Attributes,
		IsReadOnly: e.IsReadOnly,
	}
	applyEntityTimestamps(&user, e)
	return user
}

// applyEntityTimestamps copies the store-owned timestamps onto the user as RFC 3339 strings.
// Declarative users have no stored row, so their zero timestamps stay empty and are omitted.
func applyEntityTimestamps(u *User, e *providers.Entity) {
	if e == nil {
		return
	}
	if !e.CreatedAt.IsZero() {
		u.CreatedAt = e.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !e.UpdatedAt.IsZero() {
		u.UpdatedAt = e.UpdatedAt.UTC().Format(time.RFC3339)
	}
}

// entitiesToUsers converts a slice of Entity to a slice of User.
func entitiesToUsers(entities []providers.Entity) []User {
	users := make([]User, len(entities))
	for i := range entities {
		users[i] = entityToUser(&entities[i])
	}
	return users
}

// userToEntity converts a User to an Entity for storage.
func userToEntity(u *User) *providers.Entity {
	return &providers.Entity{
		ID:         u.ID,
		Category:   providers.EntityCategoryUser,
		Type:       u.Type,
		OUID:       u.OUID,
		State:      providers.EntityStateActive,
		Attributes: u.Attributes,
	}
}

// credentialsToJSON marshals user Credentials to JSON for entity storage.
func credentialsToJSON(creds Credentials) (json.RawMessage, error) {
	if len(creds) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(creds)
	if err != nil {
		return nil, err
	}
	return data, nil
}
