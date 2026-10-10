// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
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
func (e *flowGraphExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	flows, err := e.service.ListFlows(ctx, 10000, 0, providers.FlowType(""))
	if err != nil {
		return nil, &tidcommon.InternalServerError
	}
	ids := make([]string, 0, len(flows.Flows))
	for _, flow := range flows.Flows {
		ids = append(ids, flow.ID)
	}
	return ids, nil
}

// GetResourceByID retrieves a flow graph by its ID.
func (e *flowGraphExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError,
) {
	flow, err := e.service.GetFlow(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return flow, flow.Name, nil
}

// ValidateResource validates a flow graph resource.
func (e *flowGraphExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	flow, ok := resource.(*providers.CompleteFlowDefinition)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeFlow, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
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

// ViewResource shows an exported flow as a read of it returns it. A flow has no parts: an export
// carries only the active version and nothing of what uses the flow.
func (e *flowGraphExporter) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	return declarativeresource.DecodeView(document, func(flow *providers.CompleteFlowDefinition) (
		interface{}, error) {
		return flow, nil
	})
}

// loadDeclarativeResources loads immutable flow graph resources from files.
func loadDeclarativeResources(flowStore flowStoreInterface, flowValidator FlowValidatorInterface) error {
	// Type assert to access Storer interface for resource loading
	fileBasedStore, ok := flowStore.(*fileBasedStore)
	if !ok {
		return fmt.Errorf("failed to assert flowStore to *fileBasedStore")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Flow",
		DirectoryName: "flows",
		Parser:        parseToCompleteFlowDefinition,
		Validator:     validateFlowGraphWrapper(flowValidator),
		IDExtractor: func(data interface{}) string {
			flow, ok := data.(*providers.CompleteFlowDefinition)
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

// parseToCompleteFlowDefinition parses YAML bytes to providers.CompleteFlowDefinition.
func parseToCompleteFlowDefinition(data []byte) (interface{}, error) {
	var flowDef providers.CompleteFlowDefinition
	err := yaml.Unmarshal(data, &flowDef)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow definition: %w", err)
	}
	return &flowDef, nil
}

// validateFlowGraphWrapper returns a validator closure that uses the flowValidator
// to validate flow definitions, matching the ResourceConfig.Validator signature.
func validateFlowGraphWrapper(v FlowValidatorInterface) func(interface{}) error {
	return func(dto interface{}) error {
		flowDef, ok := dto.(*providers.CompleteFlowDefinition)
		if !ok {
			return fmt.Errorf("invalid type: expected *providers.CompleteFlowDefinition")
		}

		flowDefForValidation := &FlowDefinition{
			Handle:       flowDef.Handle,
			Name:         flowDef.Name,
			FlowType:     flowDef.FlowType,
			Nodes:        flowDef.Nodes,
			Interceptors: flowDef.Interceptors,
		}

		svcErr := v.ValidateFlowDefinition(context.Background(), flowDefForValidation)
		if svcErr != nil {
			return fmt.Errorf("validation failed: %s - %s", svcErr.Code, svcErr.Error)
		}

		return nil
	}
}
