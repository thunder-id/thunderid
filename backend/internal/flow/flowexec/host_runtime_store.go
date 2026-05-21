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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package flowexec

import (
	"context"

	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type hostContextStore struct {
	host thunderidengine.RuntimeStore
}

// NewContextStoreFromRuntime adapts a host RuntimeStore for flow context persistence.
func NewContextStoreFromRuntime(host thunderidengine.RuntimeStore) ContextStoreInterface {
	return &hostContextStore{host: host}
}

func (s *hostContextStore) StoreFlowContext(
	ctx context.Context, dbModel FlowContextDB, expirySeconds int64,
) error {
	return s.host.StoreFlowContext(ctx, toPublicFlowContext(dbModel), expirySeconds)
}

func (s *hostContextStore) GetFlowContext(ctx context.Context, executionID string) (*FlowContextDB, error) {
	pub, err := s.host.GetFlowContext(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, nil
	}
	internal := fromPublicFlowContext(*pub)
	return &internal, nil
}

func (s *hostContextStore) UpdateFlowContext(ctx context.Context, dbModel FlowContextDB) error {
	return s.host.UpdateFlowContext(ctx, toPublicFlowContext(dbModel))
}

func (s *hostContextStore) DeleteFlowContext(ctx context.Context, executionID string) error {
	return s.host.DeleteFlowContext(ctx, executionID)
}

// PublicFlowContextFromInternal converts an internal flow context row to the host model.
func PublicFlowContextFromInternal(dbModel FlowContextDB) thunderidengine.FlowContext {
	return toPublicFlowContext(dbModel)
}

// InternalFlowContextFromPublic converts a host flow context to the internal model.
func InternalFlowContextFromPublic(pub thunderidengine.FlowContext) FlowContextDB {
	return fromPublicFlowContext(pub)
}

func toPublicFlowContext(dbModel FlowContextDB) thunderidengine.FlowContext {
	return thunderidengine.FlowContext{
		ExecutionID: dbModel.ExecutionID,
		Context:     dbModel.Context,
		ExpiryTime:  dbModel.ExpiryTime,
		CreatedAt:   dbModel.CreatedAt,
		UpdatedAt:   dbModel.UpdatedAt,
	}
}

func fromPublicFlowContext(pub thunderidengine.FlowContext) FlowContextDB {
	return FlowContextDB{
		ExecutionID: pub.ExecutionID,
		Context:     pub.Context,
		ExpiryTime:  pub.ExpiryTime,
		CreatedAt:   pub.CreatedAt,
		UpdatedAt:   pub.UpdatedAt,
	}
}
