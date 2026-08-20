// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// loadDeclarativeResources loads the read-only server-config documents from declarative files into the
// file store. Each document is validated through the same per-section handler that validates API writes.
func loadDeclarativeResources(fileStore *fileBasedStore,
	handlers map[ConfigName]ServerConfigHandlerInterface) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "ServerConfig",
		DirectoryName: "server_configs",
		Parser:        parseServerConfigDoc,
		IDExtractor: func(data interface{}) string {
			return string(data.(*serverConfigDoc).Name)
		},
		Validator: func(data interface{}) error {
			return validateServerConfigDoc(data, fileStore, handlers)
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load server config resources: %w", err)
	}
	return nil
}

// parseServerConfigDoc parses one declarative document into a serverConfigDoc, converting its value node
// to the same JSON form the API and DB use.
func parseServerConfigDoc(data []byte) (interface{}, error) {
	var doc struct {
		Name  string    `yaml:"name"`
		Value yaml.Node `yaml:"value"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	value, err := yamlNodeToJSON(doc.Value)
	if err != nil {
		return nil, err
	}
	return &serverConfigDoc{Name: ConfigName(doc.Name), Value: value}, nil
}

// validateServerConfigDoc gates the name, rejects duplicates already loaded, then decodes the value and
// routes it to the handler's Validate (no prior layers, since this document is the read-only layer itself).
func validateServerConfigDoc(data interface{}, fileStore *fileBasedStore,
	handlers map[ConfigName]ServerConfigHandlerInterface) error {
	doc, ok := data.(*serverConfigDoc)
	if !ok {
		return fmt.Errorf("serverconfig: unexpected declarative type %T", data)
	}
	if !doc.Name.IsValid() {
		return fmt.Errorf("serverconfig: unsupported server config %q", doc.Name)
	}
	// Loading is not a read on behalf of a deployment: this is the uniqueness check as the file is
	// parsed, before any request exists.
	if _, exists := fileStore.GetByName(context.Background(), doc.Name); exists {
		return fmt.Errorf("serverconfig: server config %q defined more than once", doc.Name)
	}
	handler, ok := handlers[doc.Name]
	if !ok || handler == nil {
		return fmt.Errorf("serverconfig: no handler registered for %q", doc.Name)
	}
	value, err := handler.Decode(doc.Value)
	if err != nil {
		return fmt.Errorf("serverconfig: invalid value for %q: %w", doc.Name, err)
	}
	// Loading a declarative resource happens at startup, outside any request.
	return handler.Validate(context.Background(), value, nil, nil)
}

// yamlNodeToJSON decodes a YAML node into a generic value and re-encodes it as JSON, producing the same
// representation the API and database use.
func yamlNodeToJSON(node yaml.Node) (json.RawMessage, error) {
	var intermediate interface{}
	if err := node.Decode(&intermediate); err != nil {
		return nil, err
	}
	return json.Marshal(intermediate)
}
