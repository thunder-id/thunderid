// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeFlow = "flow"
	paramTypeFlow    = "Flow"
)

// flowGraphExporter implements declarativeresource.ResourceExporter for flow graphs.
type flowGraphExporter struct {
	service    FlowMgtServiceInterface
	sharingSvc sharing.SharingServiceInterface
}

// FlowUnitValues are the values one organization unit saved for the variables of a flow, as a document carries them,
// keyed by the identifier of each variable.
type FlowUnitValues struct {
	OUID   string                 `yaml:"ouId" json:"ouId"`
	Values map[string]interface{} `yaml:"values" json:"values"`
}

// flowDeclarativeResource is a flow as an export writes it, with the sharing policies that govern it and the values
// the organization units it is shared with saved carried alongside.
type flowDeclarativeResource struct {
	providers.CompleteFlowDefinition `yaml:",inline"`

	SharingPolicies []sharing.PolicyRequest `yaml:"sharingPolicies,omitempty" json:"sharingPolicies,omitempty"`
	VariableValues  []FlowUnitValues        `yaml:"variableValues,omitempty" json:"variableValues,omitempty"`
}

// flowOf reads the flow out of either shape the export pipeline hands around: the document this package writes, and
// the bare flow.
func flowOf(resource interface{}) (*providers.CompleteFlowDefinition, bool) {
	switch v := resource.(type) {
	case *flowDeclarativeResource:
		return &v.CompleteFlowDefinition, true
	case *providers.CompleteFlowDefinition:
		return v, true
	default:
		return nil, false
	}
}

// newFlowGraphExporter creates a new flow graph exporter.
func newFlowGraphExporter(
	service FlowMgtServiceInterface, sharingSvc sharing.SharingServiceInterface,
) *flowGraphExporter {
	return &flowGraphExporter{service: service, sharingSvc: sharingSvc}
}

// NewFlowGraphExporterForTest creates a new flow graph exporter for testing purposes.
func NewFlowGraphExporterForTest(
	service FlowMgtServiceInterface, sharingSvc sharing.SharingServiceInterface,
) *flowGraphExporter {
	return newFlowGraphExporter(service, sharingSvc)
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

// GetResourceByID retrieves a flow graph by its ID, with the sharing policies recorded for it and the values the
// organization units it is shared with saved.
func (e *flowGraphExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError,
) {
	flow, err := e.service.GetFlow(ctx, id)
	if err != nil {
		return nil, "", err
	}
	exported := &flowDeclarativeResource{CompleteFlowDefinition: *flow}
	// A document that silently lost its sharing would re-import as a flow nobody else can use, so this fails the
	// export rather than thinning it.
	replayable, sharingErr := e.sharingSvc.ExportPolicies(ctx, FlowSharingType, id)
	if sharingErr != nil {
		return nil, "", translateSharingError(sharingErr)
	}
	for _, r := range replayable {
		exported.SharingPolicies = append(exported.SharingPolicies, r.Request)
	}

	// The values are what a unit chose within the terms of those policies, so a document that left them out would
	// import as a flow every unit has to be set up for again.
	exported.VariableValues, sharingErr = e.unitValues(ctx, id)
	if sharingErr != nil {
		return nil, "", sharingErr
	}
	return exported, flow.Name, nil
}

// unitValues reads the values each organization unit saved for a flow, in the order of the units.
func (e *flowGraphExporter) unitValues(ctx context.Context, flowID string) (
	[]FlowUnitValues, *tidcommon.ServiceError,
) {
	held, sharingErr := e.sharingSvc.ListOverlayValues(ctx, FlowSharingType, flowID)
	if sharingErr != nil {
		return nil, translateSharingError(sharingErr)
	}
	ouIDs := make([]string, 0, len(held))
	for ouID, byField := range held {
		if len(byField) > 0 {
			ouIDs = append(ouIDs, ouID)
		}
	}
	slices.Sort(ouIDs)

	var out []FlowUnitValues
	for _, ouID := range ouIDs {
		saved, svcErr := e.service.GetVariables(ctx, flowID, ouID)
		if svcErr != nil {
			return nil, svcErr
		}
		if len(saved.Values) > 0 {
			out = append(out, FlowUnitValues{OUID: ouID, Values: saved.Values})
		}
	}
	return out, nil
}

// ValidateResource validates a flow graph resource.
func (e *flowGraphExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	flow, ok := flowOf(resource)
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
func loadDeclarativeResources(
	flowStore flowStoreInterface, flowValidator FlowValidatorInterface, ouSvc ouProvider,
) error {
	// Type assert to access Storer interface for resource loading
	fileBasedStore, ok := flowStore.(*fileBasedStore)
	if !ok {
		return fmt.Errorf("failed to assert flowStore to *fileBasedStore")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Flow",
		DirectoryName: "flows",
		Parser:        parseToCompleteFlowDefinition,
		Validator:     validateFlowGraphWrapper(flowValidator, ouSvc),
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
// to validate flow definitions, matching the ResourceConfig.Validator signature. A flow has to name its
// organization unit by ouId or ouHandle. The handle is resolved to the unit's id, so the flow is stored with the id.
func validateFlowGraphWrapper(v FlowValidatorInterface, ouSvc ouProvider) func(interface{}) error {
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

		if flowDef.OUID == "" && flowDef.OUHandle == "" {
			return fmt.Errorf("flow resource %q needs an organization unit, give its ouId or ouHandle", flowDef.ID)
		}
		// Declarative resources load at startup with no authenticated subject, so the owner is resolved with the
		// runtime context.
		ouID, ouErr := ouSvc.ResolveOwner(
			security.WithRuntimeContext(context.Background()), flowDef.OUID, flowDef.OUHandle, flowOwnerErrors)
		if ouErr != nil {
			return fmt.Errorf("organization unit of flow resource %q could not be resolved: %s",
				flowDef.ID, ouErr.ErrorDescription.DefaultValue)
		}
		flowDef.OUID = ouID
		flowDef.OUHandle = ""

		return nil
	}
}

// ParseFlowSharing decodes the sharing policies a flow document carries and the values the organization units saved.
// Used by the runtime import service, which applies them once the flow itself is written.
func ParseFlowSharing(node *yaml.Node) ([]sharing.PolicyRequest, []FlowUnitValues, error) {
	var doc flowDeclarativeResource
	if err := node.Decode(&doc); err != nil {
		return nil, nil, fmt.Errorf("failed to parse flow document: %w", err)
	}
	return doc.SharingPolicies, doc.VariableValues, nil
}

// makeFlowSharingConfig creates the loader config the sharing framework reads a flow's declared policies with. It names
// the flows directory, because a policy is carried inside the flow that declares it rather than in a document of its
// own.
func (s *flowMgtService) makeFlowSharingConfig() sharing.DeclarativeLoaderConfig {
	return sharing.DeclarativeLoaderConfig{
		ResourceType:  FlowSharingType,
		DirectoryName: "flows",
		Parser:        s.parseFlowSharing,
	}
}

// parseFlowSharing reads the sharing half of one flow document. The organization unit is resolved here because a
// document may name it by handle, and the terms are checked against the variables the document declares.
func (s *flowMgtService) parseFlowSharing(data []byte) (*sharing.DeclaredResourcePolicies, error) {
	var doc flowDeclarativeResource
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse flow YAML: %w", err)
	}
	// Most documents declare no policy, and checking one that has nothing to declare would be a second pass for an
	// answer already known.
	if len(doc.SharingPolicies) == 0 {
		return &sharing.DeclaredResourcePolicies{ResourceID: doc.ID}, nil
	}

	ctx := security.WithRuntimeContext(context.Background())
	if doc.OUID == "" && doc.OUHandle == "" {
		return nil, fmt.Errorf("flow resource %q needs an organization unit, give its ouId or ouHandle", doc.ID)
	}
	ouID, svcErr := s.ouSvc.ResolveOwner(ctx, doc.OUID, doc.OUHandle, flowOwnerErrors)
	if svcErr != nil {
		return nil, fmt.Errorf("organization unit of flow resource %q could not be resolved: %s",
			doc.ID, svcErr.ErrorDescription.DefaultValue)
	}
	doc.OUID = ouID
	for _, policy := range doc.SharingPolicies {
		if svcErr := s.validatePolicyVariables(ctx, &doc.CompleteFlowDefinition, policy); svcErr != nil {
			return nil, fmt.Errorf("error validating the sharing policies of flow '%s': %s",
				doc.Name, svcErr.ErrorDescription.DefaultValue)
		}
	}
	return &sharing.DeclaredResourcePolicies{
		ResourceID:   doc.ID,
		ResourceName: doc.Name,
		OwningOUID:   doc.OUID,
		Policies:     doc.SharingPolicies,
	}, nil
}

// loadDeclarativeSharing seeds the sharing policies the flow files declare. It runs under the same condition as the
// flows themselves.
func (s *flowMgtService) loadDeclarativeSharing() error {
	if mode := getFlowStoreMode(); mode != serverconst.StoreModeComposite && mode != serverconst.StoreModeDeclarative {
		return nil
	}
	return s.sharingSvc.LoadDeclarativeResources(context.Background(), s.makeFlowSharingConfig())
}
