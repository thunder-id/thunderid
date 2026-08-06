// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package actorprovider

import (
	"context"
	"encoding/json"
	"sort"

	"golang.org/x/text/language"

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

// BuildApplicationMetadata composes display metadata from inbound-client properties and actor
// records, using lang to resolve a per-language client name when one is available.
func BuildApplicationMetadata(
	id string, entity *providers.Entity, props map[string]interface{}, lang string,
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
			if localized := resolveLocalizedName(attrs["nameLangMap"], lang); localized != "" {
				meta.Name = localized
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

// resolveLocalizedName picks nameLangMap[lang], falling back to its base language then to
// English. Returns "" if none match, leaving the caller's default name untouched.
func resolveLocalizedName(nameLangMap interface{}, lang string) string {
	raw, ok := nameLangMap.(map[string]interface{})
	if !ok || len(raw) == 0 {
		return ""
	}

	requested, err := language.Parse(lang)
	if err != nil {
		return ""
	}
	if v := matchByBaseLanguage(raw, requested); v != "" {
		return v
	}
	if en, err := language.Parse("en"); err == nil {
		return matchByBaseLanguage(raw, en)
	}
	return ""
}

// matchByBaseLanguage prefers an exact tag match (e.g. "fr-CA" over "fr"), else falls back
// to a base-language match. Sorted iteration keeps the fallback deterministic.
func matchByBaseLanguage(raw map[string]interface{}, want language.Tag) string {
	codes := make([]string, 0, len(raw))
	for code := range raw {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	wantBase, _ := want.Base()
	baseMatch := ""
	for _, code := range codes {
		v, ok := raw[code].(string)
		if !ok || v == "" {
			continue
		}
		tag, err := language.Parse(code)
		if err != nil {
			continue
		}
		if tag.String() == want.String() {
			return v
		}
		if baseMatch == "" {
			if base, _ := tag.Base(); base.String() == wantBase.String() {
				baseMatch = v
			}
		}
	}
	return baseMatch
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
