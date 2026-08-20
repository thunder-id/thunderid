// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

type fileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// Create implements declarativeresource.Storer interface for resource loader
func (f *fileBasedStore) Create(id string, data interface{}) error {
	flow, ok := data.(*providers.CompleteFlowDefinition)
	if !ok {
		declarativeresource.LogTypeAssertionError("flow", id)
		return errors.New("invalid flow data type")
	}
	_, err := f.CreateFlow(context.Background(), flow.ID, &FlowDefinition{
		Handle:       flow.Handle,
		Name:         flow.Name,
		FlowType:     flow.FlowType,
		Interceptors: flow.Interceptors,
		Nodes:        flow.Nodes,
	})
	return err
}

// CreateFlow implements flowStoreInterface.
func (f *fileBasedStore) CreateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (
	*providers.CompleteFlowDefinition, error) {
	completeFlow := &providers.CompleteFlowDefinition{
		ID:            flowID,
		Handle:        flow.Handle,
		Name:          flow.Name,
		FlowType:      flow.FlowType,
		ActiveVersion: 1,
		Interceptors:  flow.Interceptors,
		Nodes:         flow.Nodes,
		CreatedAt:     "",
		UpdatedAt:     "",
	}
	return completeFlow, f.GenericFileBasedStore.Create(flowID, completeFlow)
}

// ListFlows implements flowStoreInterface.
func (f *fileBasedStore) ListFlows(ctx context.Context, limit, offset int, flowType string) (
	[]BasicFlowDefinition, int, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	var flows []BasicFlowDefinition
	for _, item := range list {
		if flow, ok := item.Data.(*providers.CompleteFlowDefinition); ok {
			// Filter by flow type if provided
			if flowType != "" && string(flow.FlowType) != flowType {
				continue
			}

			basicFlow := BasicFlowDefinition{
				ID:            flow.ID,
				Handle:        flow.Handle,
				FlowType:      flow.FlowType,
				Name:          flow.Name,
				ActiveVersion: flow.ActiveVersion,
			}
			flows = append(flows, basicFlow)
		}
	}

	// Apply pagination
	totalCount := len(flows)
	if offset >= totalCount {
		return []BasicFlowDefinition{}, totalCount, nil
	}

	endIndex := offset + limit
	if endIndex > totalCount {
		endIndex = totalCount
	}

	return flows[offset:endIndex], totalCount, nil
}

// ListActiveFlowsWithNodes implements flowStoreInterface. File-based flows already carry their node
// definitions, so it returns every stored flow.
func (f *fileBasedStore) ListActiveFlowsWithNodes(ctx context.Context) (
	[]*providers.CompleteFlowDefinition, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, err
	}

	flows := make([]*providers.CompleteFlowDefinition, 0, len(list))
	for _, item := range list {
		if flow, ok := item.Data.(*providers.CompleteFlowDefinition); ok {
			flows = append(flows, flow)
		}
	}

	return flows, nil
}

// GetFlowByID implements flowStoreInterface.
func (f *fileBasedStore) GetFlowByID(ctx context.Context, flowID string) (*providers.CompleteFlowDefinition, error) {
	data, err := f.GenericFileBasedStore.Get(ctx, flowID)
	if err != nil {
		return nil, errFlowNotFound
	}
	flow, ok := data.(*providers.CompleteFlowDefinition)
	if !ok {
		declarativeresource.LogTypeAssertionError("flow", flowID)
		return nil, errFlowNotFound
	}
	return flow, nil
}

// GetFlowByHandle implements flowStoreInterface.
func (f *fileBasedStore) GetFlowByHandle(ctx context.Context, handle string,
	flowType providers.FlowType) (*providers.CompleteFlowDefinition, error) {
	data, err := f.GenericFileBasedStore.GetByField(ctx, handle, func(d interface{}) string {
		if flow, ok := d.(*providers.CompleteFlowDefinition); ok && flow.FlowType == flowType {
			return flow.Handle
		}
		return ""
	})
	if err != nil {
		return nil, errFlowNotFound
	}
	flow, ok := data.(*providers.CompleteFlowDefinition)
	if !ok {
		declarativeresource.LogTypeAssertionError("flow", handle)
		return nil, errFlowNotFound
	}
	return flow, nil
}

// UpdateFlow implements flowStoreInterface.
func (f *fileBasedStore) UpdateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (
	*providers.CompleteFlowDefinition, error) {
	return nil, errors.New("UpdateFlow is not supported in file-based store")
}

// DeleteFlow implements flowStoreInterface.
func (f *fileBasedStore) DeleteFlow(ctx context.Context, flowID string) error {
	return errors.New("DeleteFlow is not supported in file-based store")
}

// InvalidateCache is a no-op for the file-based store; nothing is cached at this layer.
func (f *fileBasedStore) InvalidateCache(ctx context.Context, _, _ string, _ providers.FlowType) {}

// ListFlowVersions implements flowStoreInterface.
func (f *fileBasedStore) ListFlowVersions(ctx context.Context, flowID string) ([]BasicFlowVersion, error) {
	return nil, errors.New("ListFlowVersions is not supported in file-based store")
}

// GetFlowVersion implements flowStoreInterface.
func (f *fileBasedStore) GetFlowVersion(ctx context.Context, flowID string, version int) (*FlowVersion, error) {
	return nil, errors.New("GetFlowVersion is not supported in file-based store")
}

// RestoreFlowVersion implements flowStoreInterface.
func (f *fileBasedStore) RestoreFlowVersion(ctx context.Context, flowID string, version int) (
	*providers.CompleteFlowDefinition, error) {
	return nil, errors.New("RestoreFlowVersion is not supported in file-based store")
}

// IsFlowExistsByHandle implements flowStoreInterface.
func (f *fileBasedStore) IsFlowExistsByHandle(ctx context.Context, handle string,
	flowType providers.FlowType) (bool, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return false, err
	}

	for _, item := range list {
		if flow, ok := item.Data.(*providers.CompleteFlowDefinition); ok {
			if flow.Handle == handle && flow.FlowType == flowType {
				return true, nil
			}
		}
	}

	return false, nil
}

// newFileBasedStore creates a new instance of a file-based store.
func newFileBasedStore() (flowStoreInterface, providers.Transactioner) {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeFlow),
	}, transaction.NewNoOpTransactioner()
}
