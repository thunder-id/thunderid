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

package enginebridge

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type entityBridge struct {
	client thunderidengine.ClientProvider
}

func newEntityBridge(client thunderidengine.ClientProvider) *entityBridge {
	return &entityBridge{client: client}
}

func (b *entityBridge) IdentifyEntity(filters map[string]interface{}) (*string, *entityprovider.EntityProviderError) {
	if b.client == nil {
		return nil, bridgeEntityNotImplemented()
	}
	clientID, ok := filters["clientId"].(string)
	if !ok || clientID == "" {
		return nil, entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeInvalidRequestFormat, "Invalid request", "clientId filter required",
		)
	}
	ctx := context.Background()
	client, err := b.client.GetOAuthClientByClientID(ctx, clientID)
	if err != nil {
		return nil, &entityprovider.EntityProviderError{
			Code:    entityprovider.ErrorCodeSystemError,
			Message: err.Error(),
		}
	}
	if client == nil {
		return nil, entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeEntityNotFound, "Entity not found", "OAuth client not found",
		)
	}
	id := client.ID
	return &id, nil
}

func (b *entityBridge) SearchEntities(
	_ map[string]interface{},
) ([]*entityprovider.Entity, *entityprovider.EntityProviderError) {
	return nil, bridgeEntityNotImplemented()
}

func (b *entityBridge) GetEntity(entityID string) (*entityprovider.Entity, *entityprovider.EntityProviderError) {
	if b.client == nil {
		return nil, bridgeEntityNotImplemented()
	}
	app, err := b.client.GetApplicationByID(context.Background(), entityID)
	if err != nil {
		return nil, &entityprovider.EntityProviderError{
			Code:    entityprovider.ErrorCodeSystemError,
			Message: err.Error(),
		}
	}
	if app == nil {
		return nil, entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeEntityNotFound, "Entity not found", "Application not found",
		)
	}
	return &entityprovider.Entity{
		ID:       app.ID,
		OUID:     app.OUID,
		Category: entityprovider.EntityCategoryApp,
	}, nil
}

func (b *entityBridge) CreateEntity(_ *entityprovider.Entity, _ json.RawMessage) (
	*entityprovider.Entity, *entityprovider.EntityProviderError,
) {
	return nil, bridgeEntityNotImplemented()
}

func (b *entityBridge) UpdateEntity(_ string, _ *entityprovider.Entity) (
	*entityprovider.Entity, *entityprovider.EntityProviderError,
) {
	return nil, bridgeEntityNotImplemented()
}

func (b *entityBridge) DeleteEntity(_ string) *entityprovider.EntityProviderError {
	return bridgeEntityNotImplemented()
}

func (b *entityBridge) UpdateCredentials(_ string, _ json.RawMessage) *entityprovider.EntityProviderError {
	return bridgeEntityNotImplemented()
}

func (b *entityBridge) UpdateAttributes(_ string, _ json.RawMessage) *entityprovider.EntityProviderError {
	return bridgeEntityNotImplemented()
}

func (b *entityBridge) UpdateSystemAttributes(_ string, _ json.RawMessage) *entityprovider.EntityProviderError {
	return bridgeEntityNotImplemented()
}

func (b *entityBridge) UpdateSystemCredentials(_ string, _ json.RawMessage) *entityprovider.EntityProviderError {
	return bridgeEntityNotImplemented()
}

func (b *entityBridge) ValidateEntityIDs(_ []string) ([]string, *entityprovider.EntityProviderError) {
	return nil, bridgeEntityNotImplemented()
}

func (b *entityBridge) GetEntitiesByIDs(_ []string) ([]entityprovider.Entity, *entityprovider.EntityProviderError) {
	return nil, bridgeEntityNotImplemented()
}

func (b *entityBridge) GetEntityListCount(
	_ entityprovider.EntityCategory, _ map[string]interface{},
) (int, *entityprovider.EntityProviderError) {
	return 0, bridgeEntityNotImplemented()
}

func (b *entityBridge) GetEntityList(
	_ entityprovider.EntityCategory, _, _ int, _ map[string]interface{},
) ([]entityprovider.Entity, *entityprovider.EntityProviderError) {
	return nil, bridgeEntityNotImplemented()
}

func (b *entityBridge) GetTransitiveEntityGroups(entityID string) (
	[]entityprovider.EntityGroup, *entityprovider.EntityProviderError,
) {
	if b.client == nil {
		return nil, bridgeEntityNotImplemented()
	}
	groups, err := b.client.GetTransitiveEntityGroups(context.Background(), entityID)
	if err != nil {
		return nil, &entityprovider.EntityProviderError{
			Code:    entityprovider.ErrorCodeSystemError,
			Message: err.Error(),
		}
	}
	return toEntityGroups(groups), nil
}

func bridgeEntityNotImplemented() *entityprovider.EntityProviderError {
	return entityprovider.NewEntityProviderError(
		entityprovider.ErrorCodeNotImplemented,
		"Method Not Implemented",
		"The method is not implemented by the engine entity bridge.",
	)
}

var _ entityprovider.EntityProviderInterface = (*entityBridge)(nil)
