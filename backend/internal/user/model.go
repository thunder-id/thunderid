/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package user

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/utils"
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
	StorageType       string              `json:"storageType"`
	StorageAlgo       hash.CredAlgorithm  `json:"storageAlgo"`
	StorageAlgoParams hash.CredParameters `json:"storageAlgoParams"`
	Value             string              `json:"value"`
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
	TotalResults int                  `json:"totalResults"`
	StartIndex   int                  `json:"startIndex"`
	Count        int                  `json:"count"`
	Groups       []entity.EntityGroup `json:"groups"`
	Links        []utils.Link         `json:"links"`
}

// CreateUserRequest represents the request body for creating a user.
type CreateUserRequest struct {
	OUID       string          `json:"ouId"`
	Type       string          `json:"type"`
	Groups     []string        `json:"groups,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
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

// CreateUserByPathRequest represents the request body for creating a user under a handle path.
type CreateUserByPathRequest struct {
	Type       string          `json:"type"`
	Groups     []string        `json:"groups,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

// entityToUser converts an Entity to a User.
func entityToUser(e *entity.Entity) User {
	return User{
		ID:         e.ID,
		OUID:       e.OUID,
		Type:       e.Type,
		Attributes: e.Attributes,
		IsReadOnly: e.IsReadOnly,
	}
}

// entitiesToUsers converts a slice of Entity to a slice of User.
func entitiesToUsers(entities []entity.Entity) []User {
	users := make([]User, len(entities))
	for i := range entities {
		users[i] = entityToUser(&entities[i])
	}
	return users
}

// userToEntity converts a User to an Entity for storage.
func userToEntity(u *User) *entity.Entity {
	return &entity.Entity{
		ID:         u.ID,
		Category:   entity.EntityCategoryUser,
		Type:       u.Type,
		OUID:       u.OUID,
		State:      entity.EntityStateActive,
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
