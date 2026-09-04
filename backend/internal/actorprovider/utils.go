// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package actorprovider

import (
	"context"
	"encoding/json"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// BuildApplication assembles the runtime application view read from engineCtx.Application.
// Entity-agnostic: works for any actor with an inbound-client row.
func BuildApplication(
	ctx context.Context, provider providers.ActorProvider, actorID string,
) (*providers.Application, *tidcommon.ServiceError) {
	client, svcErr := provider.GetInboundClientByID(ctx, actorID)
	if svcErr != nil {
		return nil, svcErr
	}
	if client == nil {
		return nil, &ErrorActorNotFound
	}

	entity, entityErr := provider.GetActor(actorID)
	if entityErr != nil && entityErr.Code != ErrorEntityNotFound.Code {
		return nil, &tidcommon.InternalServerError
	}

	return assembleApplication(client, entity), nil
}

// assembleApplication maps inbound-client and actor records into the application model.
func assembleApplication(
	client *providers.InboundClient, entity *providers.Entity,
) *providers.Application {
	app := &providers.Application{
		ID: client.ID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:            client.AuthFlowID,
			SignOutFlowID:         client.SignOutFlowID,
			Assertion:             client.Assertion,
			LoginConsent:          client.LoginConsent,
			AllowedUserTypes:      client.AllowedUserTypes,
			AllowedAgentTypes:     client.AllowedAgentTypes,
			SubjectAttribute:      client.SubjectAttribute,
			PasskeyAllowedOrigins: client.PasskeyAllowedOrigins,
		},
	}

	entityAttrs := readEntitySystemAttributes(entity)
	if name, ok := entityAttrs["name"].(string); ok {
		app.Name = name
	}
	if metadata, ok := client.Properties["metadata"].(map[string]interface{}); ok {
		app.Metadata = metadata
	}

	if clientID, _ := entityAttrs["clientId"].(string); clientID != "" {
		app.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: clientID,
				},
			},
		}
	}

	return app
}

// BuildApplicationMetadata composes display metadata from inbound-client properties and actor records.
func BuildApplicationMetadata(
	id string, entity *providers.Entity, props map[string]interface{},
) *ApplicationMetadata {
	meta := &ApplicationMetadata{ID: id}
	if entity != nil && len(entity.SystemAttributes) > 0 {
		var attrs map[string]interface{}
		if err := json.Unmarshal(entity.SystemAttributes, &attrs); err == nil && attrs != nil {
			if name, ok := attrs["name"].(string); ok {
				meta.Name = name
			}
			if desc, ok := attrs["description"].(string); ok {
				meta.Description = desc
			}
		}
	}
	if props != nil {
		if v, ok := props["logo_url"].(string); ok {
			meta.LogoURL = v
		}
		if v, ok := props["url"].(string); ok {
			meta.URL = v
		}
		if v, ok := props["tos_uri"].(string); ok {
			meta.TosURI = v
		}
		if v, ok := props["policy_uri"].(string); ok {
			meta.PolicyURI = v
		}
	}
	return meta
}

// readEntitySystemAttributes unmarshals system attributes from an actor record.
func readEntitySystemAttributes(entity *providers.Entity) map[string]interface{} {
	if entity == nil || len(entity.SystemAttributes) == 0 {
		return map[string]interface{}{}
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(entity.SystemAttributes, &attrs); err != nil || attrs == nil {
		return map[string]interface{}{}
	}
	return attrs
}
