// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type declarativeSystemCredentialHasher interface {
	hashPlaintextCredentials(creds json.RawMessage) (json.RawMessage, error)
}

// loadDeclarativeResources loads declarative resources for a given configuration
// into the entity file-based store. The config provides consumer-specific parser and
// validator callbacks so entity service doesn't need to understand type-specific YAML formats.
func loadDeclarativeResources(
	store entityStoreInterface,
	svc EntityServiceInterface,
	config DeclarativeLoaderConfig,
) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityDeclarativeLoader"))

	// Extract the file-based store from the store hierarchy.
	var fileStore *entityFileBasedStore
	switch s := store.(type) {
	case *entityFileBasedStore:
		fileStore = s
	case *entityCompositeStore:
		if fs, ok := s.fileStore.(*entityFileBasedStore); ok {
			fileStore = fs
		}
	}
	if fileStore == nil {
		return nil // mutable mode: no file store, skip loading
	}

	// Build the parser wrapper that converts consumer's parser output to entityStoreEntry
	parser := func(data []byte) (interface{}, error) {
		if config.Parser == nil {
			return nil, fmt.Errorf("parser is required for category: %s", config.Category)
		}
		entity, credentials, systemCredentials, err := config.Parser(data)
		if err != nil {
			return nil, err
		}

		if len(systemCredentials) > 0 {
			hasher, ok := svc.(declarativeSystemCredentialHasher)
			if !ok {
				return nil, fmt.Errorf("entity service cannot hash declarative system credentials")
			}

			systemCredentials, err = hasher.hashPlaintextCredentials(systemCredentials)
			if err != nil {
				return nil, fmt.Errorf("failed to hash declarative system credentials: %w", err)
			}
		}

		if entity == nil {
			return nil, fmt.Errorf("parser returned nil entity without error")
		}

		var initial struct {
			State providers.EntityState `yaml:"state" json:"state"`
		}
		if err := yaml.Unmarshal(data, &initial); err != nil {
			return nil, fmt.Errorf("failed to parse initial entity state: %w", err)
		}
		entity.State = initial.State
		if entity.State == "" {
			entity.State = providers.EntityStateActive
		}

		// Runtime values are never supplied by declarative profiles.
		entity.RuntimeAttributes = nil

		resource := &entityStoreEntry{
			Entity:            *entity,
			Credentials:       credentials,
			SystemCredentials: systemCredentials,
		}
		return resource, nil
	}

	// Build the validator wrapper
	validator := func(data interface{}) error {
		resource, ok := data.(*entityStoreEntry)
		if !ok {
			return fmt.Errorf("invalid type: expected *entityStoreEntry")
		}

		if config.Validator != nil {
			return config.Validator(&resource.Entity, svc)
		}
		return nil
	}

	// Build the ID extractor wrapper
	idExtractor := func(data interface{}) string {
		resource, ok := data.(*entityStoreEntry)
		if !ok {
			// Declarative resource loading runs at startup, outside any request.
			logger.Error(context.Background(), "IDExtractor: type assertion failed for entityStoreEntry")
			return ""
		}
		if config.IDExtractor != nil {
			return config.IDExtractor(&resource.Entity)
		}
		return resource.Entity.ID
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  fmt.Sprintf("Entity[%s]", config.Category),
		DirectoryName: config.Directory,
		Parser:        parser,
		Validator:     validator,
		IDExtractor:   idExtractor,
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load declarative resources for %s: %w", config.Category, err)
	}

	return nil
}
