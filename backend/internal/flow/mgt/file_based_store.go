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
func (f *fileBasedStore) CreateFlow(_ context.Context, flowID string, flow *FlowDefinition) (
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
func (f *fileBasedStore) ListFlows(_ context.Context, limit, offset int, flowType string) (
	[]BasicFlowDefinition, int, error) {
	list, err := f.GenericFileBasedStore.List()
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

// GetFlowByID implements flowStoreInterface.
func (f *fileBasedStore) GetFlowByID(_ context.Context, flowID string) (*providers.CompleteFlowDefinition, error) {
	data, err := f.GenericFileBasedStore.Get(flowID)
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
func (f *fileBasedStore) GetFlowByHandle(_ context.Context, handle string,
	flowType providers.FlowType) (*providers.CompleteFlowDefinition, error) {
	data, err := f.GenericFileBasedStore.GetByField(handle, func(d interface{}) string {
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
func (f *fileBasedStore) UpdateFlow(_ context.Context, flowID string, flow *FlowDefinition) (
	*providers.CompleteFlowDefinition, error) {
	return nil, errors.New("UpdateFlow is not supported in file-based store")
}

// DeleteFlow implements flowStoreInterface.
func (f *fileBasedStore) DeleteFlow(_ context.Context, flowID string) error {
	return errors.New("DeleteFlow is not supported in file-based store")
}

// ListFlowVersions implements flowStoreInterface.
func (f *fileBasedStore) ListFlowVersions(_ context.Context, flowID string) ([]BasicFlowVersion, error) {
	return nil, errors.New("ListFlowVersions is not supported in file-based store")
}

// GetFlowVersion implements flowStoreInterface.
func (f *fileBasedStore) GetFlowVersion(_ context.Context, flowID string, version int) (*FlowVersion, error) {
	return nil, errors.New("GetFlowVersion is not supported in file-based store")
}

// RestoreFlowVersion implements flowStoreInterface.
func (f *fileBasedStore) RestoreFlowVersion(_ context.Context, flowID string, version int) (
	*providers.CompleteFlowDefinition, error) {
	return nil, errors.New("RestoreFlowVersion is not supported in file-based store")
}

// IsFlowExistsByHandle implements flowStoreInterface.
func (f *fileBasedStore) IsFlowExistsByHandle(_ context.Context, handle string,
	flowType providers.FlowType) (bool, error) {
	list, err := f.GenericFileBasedStore.List()
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
func newFileBasedStore() (flowStoreInterface, transaction.Transactioner) {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeFlow),
	}, transaction.NewNoOpTransactioner()
}
