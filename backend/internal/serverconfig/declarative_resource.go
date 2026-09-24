// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// seedCORSFromDeploymentConfig injects the deployment.yaml cors.allowedOrigins section into the
// read-only declarative layer ahead of the declarative_resources/server_configs file scan, so a
// conflicting cors.yaml resource file is rejected by the same duplicate check the file loader uses.
func seedCORSFromDeploymentConfig(fileStore *fileBasedStore,
	handlers map[ConfigName]ServerConfigHandlerInterface) error {
	origins := config.GetServerRuntime().Config.CORS.AllowedOrigins
	if len(origins) == 0 {
		return nil
	}
	value, err := json.Marshal(cors.OriginConfig{AllowedOrigins: origins})
	if err != nil {
		return fmt.Errorf("serverconfig: failed to encode deployment.yaml cors config: %w", err)
	}
	doc := &serverConfigDoc{Name: ConfigNameCORS, Value: value}
	if err := validateServerConfigDoc(doc, fileStore, handlers); err != nil {
		return fmt.Errorf("serverconfig: invalid deployment.yaml cors config: %w", err)
	}
	return fileStore.Create(string(ConfigNameCORS), doc)
}

// seedMutableCORSFromDeploymentConfig ensures the deployment.yaml cors.allowedOrigins section is always
// present in the writable (db) layer under the mutable store mode, which has no read-only layer to hold
// it. The deployment.yaml value is merged with any existing writable value through the section's own
// Merge, so origins added at runtime through the server-config API are preserved rather than clobbered.
func seedMutableCORSFromDeploymentConfig(store serverConfigStoreInterface,
	handlers map[ConfigName]ServerConfigHandlerInterface) error {
	origins := config.GetServerRuntime().Config.CORS.AllowedOrigins
	if len(origins) == 0 {
		return nil
	}
	handler, ok := handlers[ConfigNameCORS]
	if !ok || handler == nil {
		return fmt.Errorf("serverconfig: no handler registered for %q", ConfigNameCORS)
	}

	deploymentValue, err := json.Marshal(cors.OriginConfig{AllowedOrigins: origins})
	if err != nil {
		return fmt.Errorf("serverconfig: failed to encode deployment.yaml cors config: %w", err)
	}
	deploymentCfg, err := handler.Decode(deploymentValue)
	if err != nil {
		return fmt.Errorf("serverconfig: invalid deployment.yaml cors config: %w", err)
	}
	if err := handler.Validate(deploymentCfg, nil, nil); err != nil {
		return fmt.Errorf("serverconfig: invalid deployment.yaml cors config: %w", err)
	}

	ctx := context.Background()
	layers, err := store.GetServerConfig(ctx, ConfigNameCORS)
	if err != nil {
		return fmt.Errorf("serverconfig: failed to read existing writable cors config: %w", err)
	}
	existingCfg, err := handler.Decode(layers.Writable)
	if err != nil {
		return fmt.Errorf("serverconfig: failed to decode existing writable cors config: %w", err)
	}

	merged := handler.Merge(deploymentCfg, existingCfg)
	mergedValue, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("serverconfig: failed to encode merged cors config: %w", err)
	}
	return store.UpsertServerConfig(ctx, ServerConfig{Name: ConfigNameCORS, Value: mergedValue})
}

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
	if _, exists := fileStore.GetByName(doc.Name); exists {
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
	return handler.Validate(value, nil, nil)
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
