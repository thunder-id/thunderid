// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"reflect"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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
// exporting each section's effective (merged) value as a declarative document.
type serverConfigExporter struct {
	service ServerConfigService
}

// newServerConfigExporter creates a new server-config exporter.
func newServerConfigExporter(service ServerConfigService) *serverConfigExporter {
	return &serverConfigExporter{service: service}
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
	ids := make([]string, 0, len(names))
	for _, name := range names {
		// A section that holds no value is not configuration, and exporting it would carry an empty
		// value into whatever it is imported to, wiping a setting that deployment made for itself. The
		// default resource server is the case that bites: an empty one leaves a data plane unable to
		// issue a token for a login that asks for a permission scope.
		// A section this deployment does not serve is skipped, not fatal. The supported names are the
		// same everywhere, but a plane registers a handler only for what it is responsible for: a
		// control plane serves no SSO session lifetime, because that is the data plane's. Failing here
		// would abandon every other section too, so a control plane would export no configuration at
		// all, and the default resource server would never reach the data plane that needs it.
		layers, layerErr := e.service.GetConfig(ctx, name)
		if layerErr != nil {
			continue
		}
		if isZeroConfig(layers.Merged) {
			continue
		}
		ids = append(ids, string(name))
	}
	return ids, nil
}

// isZeroConfig reports whether a merged section value carries nothing.
func isZeroConfig(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	return v.IsZero()
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
