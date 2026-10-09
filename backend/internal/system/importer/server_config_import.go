// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// serverConfigAdapter is the subset of the server-config service used to import a section's value into
// the writable (db) layer.
type serverConfigAdapter interface {
	SetConfig(ctx context.Context, name serverconfig.ConfigName,
		value json.RawMessage, merge ...bool) *common.ServiceError
}

// importBehaviorReplace and importBehaviorMerge are the values a document's importBehavior field can ask
// for; replace is the default when the field is omitted.
const (
	importBehaviorReplace = "replace"
	importBehaviorMerge   = "merge"
)

// serverConfigDeclarativeYAML is a server-config section document as produced by export: a section name,
// its value, and an optional import behavior ("merge"/"replace").
type serverConfigDeclarativeYAML struct {
	Name           string    `yaml:"name"`
	Value          yaml.Node `yaml:"value"`
	ImportBehavior string    `yaml:"importBehavior,omitempty"`
}

// importServerConfig applies a server-config section to the writable layer via SetConfig. The section is
// identified by name; with importBehavior "merge" the write adds to the current writable value for a
// section that owns a collection and otherwise replaces it, the same as SetConfig with merge always false.
// There is no create/update distinction, so the operation is always reported as an update.
func (s *importService) importServerConfig(ctx context.Context, doc parsedDocument, dryRun bool) ImportItemOutcome {
	if s.serverConfigService == nil {
		return unsupportedAdapterOutcome(resourceTypeServerConfig, "server config")
	}

	var req serverConfigDeclarativeYAML
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeServerConfig, "", req.Name, err)
	}
	if req.Name == "" {
		return ImportItemOutcome{
			ResourceType: resourceTypeServerConfig,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      "server config name is required",
		}
	}
	if req.Value.Kind == 0 {
		return ImportItemOutcome{
			ResourceType: resourceTypeServerConfig,
			ResourceName: req.Name,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      "server config value is required",
		}
	}

	// A typo would otherwise silently replace the writable layer and drop entries the document never
	// mentioned.
	if b := req.ImportBehavior; b != "" && b != importBehaviorReplace && b != importBehaviorMerge {
		return ImportItemOutcome{
			ResourceType: resourceTypeServerConfig,
			ResourceName: req.Name,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      `server config importBehavior must be "replace" or "merge"`,
		}
	}

	value, err := serverConfigValueToJSON(req.Value)
	if err != nil {
		return decodeErrorOutcome(resourceTypeServerConfig, "", req.Name, err)
	}

	if dryRun {
		return successOutcome(resourceTypeServerConfig, req.Name, req.Name, operationUpdate)
	}

	merge := req.ImportBehavior == importBehaviorMerge
	if svcErr := s.serverConfigService.SetConfig(ctx, serverconfig.ConfigName(req.Name), value, merge); svcErr != nil {
		return serviceErrorOutcome(resourceTypeServerConfig, req.Name, req.Name, operationUpdate, svcErr)
	}
	return successOutcome(resourceTypeServerConfig, req.Name, req.Name, operationUpdate)
}

// serverConfigValueToJSON converts a YAML value node into the JSON the server-config API consumes.
func serverConfigValueToJSON(node yaml.Node) (json.RawMessage, error) {
	var decoded any
	if err := node.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("failed to decode server config value: %w", err)
	}
	return json.Marshal(decoded)
}
