// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
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
	return User{
		ID:         e.ID,
		OUID:       e.OUID,
		Type:       e.Type,
		Attributes: e.Attributes,
		IsReadOnly: e.IsReadOnly,
	}
}

// entityToUserForContext converts an Entity to a User and reports a control plane owned one as read
// only.
//
// The flag is set here rather than left to the caller because it is what a client renders from: a
// user this deployment cannot change should not be offered an edit or delete control that the server
// is going to refuse.
func entityToUserForContext(ctx context.Context, e *providers.Entity) User {
	user := entityToUser(e)
	if !user.IsReadOnly && managedresource.IsManaged(ctx, managedresource.TypeUser, e.ID) {
		user.IsReadOnly = true
	}
	return user
}

// entitiesToUsersForContext converts entities to users, marking the control plane owned ones read
// only. The managed ids are read once rather than per user, so a listing costs one extra query.
func entitiesToUsersForContext(ctx context.Context, entities []providers.Entity) []User {
	users := entitiesToUsers(entities)
	managed := managedresource.Default().ManagedIDs(ctx, managedresource.TypeUser)
	if len(managed) == 0 {
		return users
	}
	for i := range users {
		if managed[users[i].ID] {
			users[i].IsReadOnly = true
		}
	}
	return users
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
