/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package entity provides the unified entity management layer for identity principals.
package entity

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// entityWithCredentials wraps an providers.Entity with its credential data.
type entityWithCredentials struct {
	Entity            *providers.Entity
	SchemaCredentials json.RawMessage
	SystemCredentials json.RawMessage
}

// EntityIdentifier represents an indexed identifier for fast entity lookup.
type EntityIdentifier struct {
	EntityID string `json:"entityId"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Source   string `json:"source"`
}

// AuthenticateResult represents the result of an entity authentication.
type AuthenticateResult struct {
	EntityID       string                   `json:"entityId"`
	EntityCategory providers.EntityCategory `json:"entityCategory"`
	EntityType     string                   `json:"entityType"`
	OUID           string                   `json:"ouId"`
}

// StoredCredential represents a single credential entry stored in the entity's schema or
// system credentials column.
type StoredCredential struct {
	StorageAlgo       cryptolib.CredAlgorithm  `json:"storageAlgo"`
	StorageAlgoParams cryptolib.CredParameters `json:"storageAlgoParams"`
	Value             string                   `json:"value"`
}

// DeclarativeLoaderConfig configures declarative resource loading for a specific entity category.
// Consumer packages (e.g., user) provide parser and validator callbacks for type-specific processing.
type DeclarativeLoaderConfig struct {
	// Directory is the YAML directory name under declarative_resources/ (e.g., "users").
	Directory string
	// Category is the entity category for these resources.
	Category providers.EntityCategory
	// Parser converts YAML bytes into an providers.Entity with optional credentials.
	// Returns the entity, schema credentials (JSON), system credentials (JSON), and any error.
	// Either credential may be nil if not applicable for the entity category.
	Parser func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error)
	// Validator validates the parsed entity. Called after parsing, before storing.
	Validator func(entity *providers.Entity, svc EntityServiceInterface) error
	// IDExtractor extracts the entity ID from the parsed entity for storage key.
	IDExtractor func(entity *providers.Entity) string
}

// entityStoreEntry wraps an providers.Entity with its credentials for internal file-based storage.
// Credentials are stored alongside the entity in declarative mode but never exposed via GetEntity.
type entityStoreEntry struct {
	Entity            providers.Entity
	Credentials       json.RawMessage
	SystemCredentials json.RawMessage
}
