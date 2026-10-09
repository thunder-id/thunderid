// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

// entityRuntimeData is an entity's row in ENTITY_RUNTIME_DATA. It is not cached with the profile.
type entityRuntimeData struct {
	ID         string
	State      providers.EntityState
	Attributes json.RawMessage
	Revision   int64
}
