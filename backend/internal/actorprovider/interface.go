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

// Package actorprovider resolves inbound actors (applications, agents, and similar entities)
// for runtime layers that need a unified view of inbound-client and backing records.
package actorprovider

import (
	"context"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// ActorProviderInterface resolves inbound actors and exposes their OAuth and membership data.
// It also embeds entityprovider.EntityResolverInterface so an actor provider can satisfy the
// narrow entity-resolution contract that flow executors depend on — letting an embedding
// application supply a single actor provider for both actor and entity resolution.
type ActorProviderInterface interface {
	entityprovider.EntityResolverInterface

	GetOAuthClientByClientID(
		ctx context.Context, clientID string,
	) (*inboundmodel.OAuthClient, *serviceerror.ServiceError)
	GetOAuthProfileByID(
		ctx context.Context, id string,
	) (*inboundmodel.OAuthProfile, *serviceerror.ServiceError)
	GetInboundClientByID(
		ctx context.Context, id string,
	) (*inboundmodel.InboundClient, *serviceerror.ServiceError)
	GetActor(actorID string) (*entityprovider.Entity, *entityprovider.EntityProviderError)
	GetActorGroups(actorID string) ([]entityprovider.EntityGroup, *entityprovider.EntityProviderError)
}
