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

package flowmgt

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/common"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeFlow = "flow"
	paramTypeFlow    = "Flow"
)

// flowGraphExporter implements declarativeresource.ResourceExporter for flow graphs.
type flowGraphExporter struct {
	service FlowMgtServiceInterface
}

// newFlowGraphExporter creates a new flow graph exporter.
func newFlowGraphExporter(service FlowMgtServiceInterface) *flowGraphExporter {
	return &flowGraphExporter{service: service}
}

// NewFlowGraphExporterForTest creates a new flow graph exporter for testing purposes.
func NewFlowGraphExporterForTest(service FlowMgtServiceInterface) *flowGraphExporter {
	return newFlowGraphExporter(service)
}

// GetResourceType returns the resource type for flow graphs.
func (e *flowGraphExporter) GetResourceType() string {
	return resourceTypeFlow
}

// GetParameterizerType returns the parameterizer type for flow graphs.
func (e *flowGraphExporter) GetParameterizerType() string {
	return paramTypeFlow
}

// GetAllResourceIDs retrieves all flow graph IDs.
func (e *flowGraphExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	flows, err := e.service.ListFlows(ctx, 10000, 0, common.FlowType(""))
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	ids := make([]string, 0, len(flows.Flows))
	for _, flow := range flows.Flows {
		ids = append(ids, flow.ID)
	}
	return ids, nil
}

// GetResourceByID retrieves a flow graph by its ID.
func (e *flowGraphExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *serviceerror.ServiceError,
) {
	flow, err := e.service.GetFlow(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return flow, flow.Name, nil
}

// ValidateResource validates a flow graph resource.
func (e *flowGraphExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	flow, ok := resource.(*CompleteFlowDefinition)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeFlow, id)
	}

	if err := declarativeresource.ValidateResourceName(
		flow.Name, resourceTypeFlow, id, "FLOW_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return flow.Name, nil
}

// GetResourceRules returns the parameterization rules for flow graphs.
// Currently returns empty rules as no parameterization is needed for graphs at this stage.
func (e *flowGraphExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{}
}

// loadDeclarativeResources loads immutable flow graph resources from files.
func loadDeclarativeResources(flowStore flowStoreInterface) error {
	// Type assert to access Storer interface for resource loading
	fileBasedStore, ok := flowStore.(*fileBasedStore)
	if !ok {
		return fmt.Errorf("failed to assert flowStore to *fileBasedStore")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Flow",
		DirectoryName: "flows",
		Parser:        parseToCompleteFlowDefinition,
		Validator:     validateFlowGraphWrapper,
		IDExtractor: func(data interface{}) string {
			flow, ok := data.(*CompleteFlowDefinition)
			if !ok || flow == nil {
				return ""
			}
			return flow.ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileBasedStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load flow graph resources: %w", err)
	}

	return nil
}

// parseToCompleteFlowDefinition parses YAML bytes to CompleteFlowDefinition.
func parseToCompleteFlowDefinition(data []byte) (interface{}, error) {
	var flowDef CompleteFlowDefinition
	err := yaml.Unmarshal(data, &flowDef)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow definition: %w", err)
	}
	return &flowDef, nil
}

// validateFlowGraphWrapper wraps flow validation to match ResourceConfig.Validator signature.
func validateFlowGraphWrapper(dto interface{}) error {
	flowDef, ok := dto.(*CompleteFlowDefinition)
	if !ok {
		return fmt.Errorf("invalid type: expected *CompleteFlowDefinition")
	}

	// Convert to FlowDefinition for validation
	flowDefForValidation := &FlowDefinition{
		Handle:   flowDef.Handle,
		Name:     flowDef.Name,
		FlowType: flowDef.FlowType,
		Nodes:    flowDef.Nodes,
	}

	// Use the service-level validation function
	svcErr := validateFlowDefinition(flowDefForValidation)
	if svcErr != nil {
		return fmt.Errorf("validation failed: %s - %s", svcErr.Code, svcErr.Error)
	}

	return nil
}
