// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"errors"
	"fmt"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeServerConfig = "server_config"
	paramTypeServerConfig    = "ServerConfig"
)

// serverConfigExportDoc is the YAML-serializable form of a server-config section for export: the section
// name and its effective value. It round-trips with the declarative document parsed by the loader.
type serverConfigExportDoc struct {
	Name  string      `yaml:"name" json:"name"`
	Value interface{} `yaml:"value" json:"value"`
}

// serverConfigExporter implements declarativeresource.ResourceExporter for server-config sections,
// exporting each section's effective (merged) value as a declarative document. The per-section
// handlers decode an exported document back into the section's typed value when it is viewed.
type serverConfigExporter struct {
	service  ServerConfigService
	handlers map[ConfigName]ServerConfigHandlerInterface
}

// newServerConfigExporter creates a new server-config exporter.
func newServerConfigExporter(service ServerConfigService,
	handlers map[ConfigName]ServerConfigHandlerInterface) *serverConfigExporter {
	return &serverConfigExporter{service: service, handlers: handlers}
}

// GetResourceType returns the resource type for server config sections.
func (e *serverConfigExporter) GetResourceType() string {
	return resourceTypeServerConfig
}

// GetParameterizerType returns the parameterizer type for server config sections.
func (e *serverConfigExporter) GetParameterizerType() string {
	return paramTypeServerConfig
}

// GetAllResourceIDs returns the supported section names; one file is exported per section.
func (e *serverConfigExporter) GetAllResourceIDs(ctx context.Context) ([]string, *common.ServiceError) {
	names, svcErr := e.service.ListConfigNames(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	ids := make([]string, len(names))
	for i, name := range names {
		ids[i] = string(name)
	}
	return ids, nil
}

// GetResourceByID returns the section's effective value as an export document.
func (e *serverConfigExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *common.ServiceError,
) {
	layers, svcErr := e.service.GetConfig(ctx, ConfigName(id))
	if svcErr != nil {
		return nil, "", svcErr
	}
	return &serverConfigExportDoc{Name: id, Value: layers.Merged}, id, nil
}

// ValidateResource validates the export document and extracts its name.
func (e *serverConfigExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger) (string, *declarativeresource.ExportError) {
	doc, ok := resource.(*serverConfigExportDoc)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeServerConfig, id)
	}
	if exportErr := declarativeresource.ValidateResourceName(
		ctx, doc.Name, resourceTypeServerConfig, id, "SERVER_CONFIG_VALIDATION_ERROR", logger); exportErr != nil {
		return "", exportErr
	}
	return doc.Name, nil
}

// GetResourceRules returns the parameterization rules; server config values carry no parameterized fields.
func (e *serverConfigExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{Variables: []string{}, ArrayVariables: []string{}}
}

// ViewResource shows an exported section as GET /server-config/{name} returns it, holding only what
// the document says. An import writes the document to the writable layer, so it is shown as that
// layer over an absent read-only layer, with their merge as the effective value.
func (e *serverConfigExporter) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	return declarativeresource.DecodeView(document, func(exported *exportedServerConfig) (interface{}, error) {
		handler, ok := e.handlers[ConfigName(exported.Name)]
		if !ok || handler == nil {
			return nil, fmt.Errorf("serverconfig: no handler registered for %q", exported.Name)
		}
		value, valueErr := yamlNodeToJSON(exported.Value)
		readOnly, readOnlyErr := handler.Decode(nil)
		writable, err := handler.Decode(value)
		if err != nil {
			return nil, fmt.Errorf("serverconfig: invalid value for %q: %w", exported.Name, err)
		}
		return ServerConfigLayers{
			ReadOnly: readOnly,
			Writable: writable,
			Merged:   handler.Merge(readOnly, writable),
		}, errors.Join(valueErr, readOnlyErr)
	})
}

// exportedServerConfig is a section as its export writes it.
type exportedServerConfig struct {
	Name  string    `yaml:"name"`
	Value yaml.Node `yaml:"value"`
}
