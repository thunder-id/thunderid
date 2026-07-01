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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
	SetConfig(ctx context.Context, name serverconfig.ConfigName, value json.RawMessage) *common.ServiceError
}

// serverConfigDeclarativeYAML is a server-config section document as produced by export: a section name
// and its value.
type serverConfigDeclarativeYAML struct {
	Name  string    `yaml:"name"`
	Value yaml.Node `yaml:"value"`
}

// importServerConfig applies a server-config section to the writable layer via SetConfig. The section is
// identified by name; SetConfig upserts the writable layer and the declarative layer (if any) is left
// untouched. There is no create/update distinction, so the operation is always reported as an update.
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

	value, err := serverConfigValueToJSON(req.Value)
	if err != nil {
		return decodeErrorOutcome(resourceTypeServerConfig, "", req.Name, err)
	}

	if dryRun {
		return successOutcome(resourceTypeServerConfig, req.Name, req.Name, operationUpdate)
	}

	if svcErr := s.serverConfigService.SetConfig(ctx, serverconfig.ConfigName(req.Name), value); svcErr != nil {
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
