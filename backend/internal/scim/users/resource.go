// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// filterAttrsBySchema removes never-returned (credential-flagged) attributes from raw JSON attributes.
// Fails closed by returning an empty JSON object on any parse or marshal error.
func filterAttrsBySchema(
	ctx context.Context, logger log.Logger, attrs json.RawMessage, rawProps map[string]scim.RawPropertyDef,
) json.RawMessage {
	if len(rawProps) == 0 {
		return attrs
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(attrs, &m); err != nil {
		logger.Error(ctx,
			"filterAttrsBySchema: failed to parse user attributes; returning empty object to prevent attribute leak",
			log.Error(err))
		return json.RawMessage(`{}`)
	}
	for key := range m {
		for propName, propDef := range rawProps {
			if !strings.EqualFold(propName, key) {
				continue
			}
			if returned, _ := scim.CredentialCharacteristics(propDef.Credential); returned == scim.ReturnedNever {
				delete(m, key)
			}
			break
		}
	}
	filtered, err := json.Marshal(m)
	if err != nil {
		logger.Error(ctx,
			"filterAttrsBySchema: failed to marshal filtered attributes; "+
				"returning empty object to prevent attribute leak",
			log.Error(err))
		return json.RawMessage(`{}`)
	}
	return filtered
}

// buildSCIMUserResource converts a providers.User into a SCIMUser wire response.
func buildSCIMUserResource(
	ctx context.Context, logger log.Logger, u providers.User, extensionURN, baseURL string,
	rawProps map[string]scim.RawPropertyDef, includeCoreAttrs bool,
) SCIMUser {
	location := fmt.Sprintf("%s%s/Users/%s", baseURL, scim.SCIMBasePath, u.ID)

	scimUser := SCIMUser{
		ID:           u.ID,
		Schemas:      []string{scim.SCIMCoreUserSchemaURN, extensionURN},
		ExtensionURN: extensionURN,
		Meta: scim.SCIMMeta{
			ResourceType: "User",
			Location:     location,
		},
	}

	if len(u.Attributes) > 0 {
		scimUser.Attributes = filterAttrsBySchema(ctx, logger, u.Attributes, rawProps)
		if includeCoreAttrs {
			scimUser.CoreAttrs = mapToCoreAttrs(scimUser.Attributes)
			entAttrs := mapToEnterpriseAttrs(scimUser.Attributes, baseURL)
			if len(entAttrs) > 0 {
				scimUser.EnterpriseAttrs = entAttrs
				scimUser.Schemas = append(scimUser.Schemas, scim.SCIMEnterpriseUserSchemaURN)
			}
		}
	}

	return scimUser
}

// buildSCIMUserListResponse wraps a slice of SCIMUser into the ListResponse envelope.
// startIndex is 1-based per RFC 7644 §3.4.2.
func buildSCIMUserListResponse(users []SCIMUser, totalResults, startIndex, itemsPerPage int) SCIMUserListResponse {
	if users == nil {
		users = []SCIMUser{}
	}
	return SCIMUserListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: totalResults,
		StartIndex:   startIndex,
		ItemsPerPage: itemsPerPage,
		Resources:    users,
	}
}

var alwaysReturnedUserAttrs = map[string]struct{}{"id": {}, "schemas": {}, "meta": {}}

// projectSCIMAttributes prunes a marshaled SCIM resource per RFC 7644 §3.9.
// attributes takes precedence over excludedAttributes; id, schemas, and meta are always retained.
func projectSCIMAttributes(
	resource map[string]interface{}, extensionURN string, attributes, excludedAttributes []string,
) map[string]interface{} {
	keep := len(attributes) > 0
	paths := attributes
	if !keep {
		paths = excludedAttributes
	}

	top := make(map[string]struct{}, len(paths))
	ext := make(map[string]struct{}, len(paths))
	ent := make(map[string]struct{}, len(paths))
	var wholeCoreKept, wholeExtKept, wholeEntKept bool

	corePrefix := strings.ToLower(scim.SCIMCoreUserSchemaURN) + ":"
	entPrefix := strings.ToLower(scim.SCIMEnterpriseUserSchemaURN) + ":"
	var extPrefix string
	if extensionURN != "" {
		extPrefix = strings.ToLower(extensionURN) + ":"
	}

	for _, attr := range paths {
		if strings.EqualFold(attr, scim.SCIMCoreUserSchemaURN) {
			wholeCoreKept = true
			continue
		}
		if strings.EqualFold(attr, scim.SCIMEnterpriseUserSchemaURN) {
			wholeEntKept = true
			continue
		}
		if extensionURN != "" && strings.EqualFold(attr, extensionURN) {
			wholeExtKept = true
			continue
		}

		lattr := strings.ToLower(attr)
		switch {
		case strings.HasPrefix(lattr, corePrefix):
			top[lattr[len(corePrefix):]] = struct{}{}
		case extPrefix != "" && strings.HasPrefix(lattr, extPrefix):
			ext[lattr[len(extPrefix):]] = struct{}{}
		case strings.HasPrefix(lattr, entPrefix):
			ent[lattr[len(entPrefix):]] = struct{}{}
		default:
			// An unqualified attribute name resolves against the core User schema only
			// (RFC 7643 §3.10 default schema). It must never match the custom extension
			// or enterprise objects, which require an explicit schema URN prefix.
			top[lattr] = struct{}{}
		}
	}

	extObj, hasExt := resource[extensionURN].(map[string]interface{})
	hasExt = hasExt && extensionURN != ""

	entObj, hasEnt := resource[scim.SCIMEnterpriseUserSchemaURN].(map[string]interface{})
	hasEnt = hasEnt && entObj != nil

	projected := make(map[string]interface{}, len(resource))
	for k, v := range resource {
		if k == extensionURN || k == scim.SCIMEnterpriseUserSchemaURN {
			continue
		}
		_, always := alwaysReturnedUserAttrs[k]
		_, matched := top[strings.ToLower(k)]
		matched = matched || wholeCoreKept
		if always || matched == keep {
			projected[k] = v
		}
	}
	if hasExt {
		if wholeExtKept {
			if keep {
				projected[extensionURN] = extObj
			}
		} else if kept := filterMap(extObj, ext, keep); len(kept) > 0 {
			projected[extensionURN] = kept
		}
	}
	if hasEnt {
		if wholeEntKept {
			if keep {
				projected[scim.SCIMEnterpriseUserSchemaURN] = entObj
			}
		} else if kept := filterMap(entObj, ent, keep); len(kept) > 0 {
			projected[scim.SCIMEnterpriseUserSchemaURN] = kept
		}
	}
	return projected
}

// filterMap keeps entries present in set (keep=true) or absent from it (keep=false), case-insensitively.
func filterMap(m map[string]interface{}, set map[string]struct{}, keep bool) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		_, in := set[strings.ToLower(k)]
		if in == keep {
			result[k] = v
		}
	}
	return result
}

// projectSCIMUserListResponse rebuilds a list response with attribute projection applied to each resource.
func projectSCIMUserListResponse(
	listResp SCIMUserListResponse, attributes, excludedAttributes []string,
) (map[string]interface{}, error) {
	if len(attributes) == 0 && len(excludedAttributes) == 0 {
		return nil, nil
	}
	shallow := listResp
	shallow.Resources = nil
	raw, err := json.Marshal(shallow)
	if err != nil {
		return nil, fmt.Errorf("projectSCIMUserListResponse: failed to marshal list response: %w", err)
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("projectSCIMUserListResponse: failed to unmarshal list response: %w", err)
	}
	resources := make([]map[string]interface{}, 0, len(listResp.Resources))
	for _, u := range listResp.Resources {
		raw, err := json.Marshal(u)
		if err != nil {
			return nil, fmt.Errorf("projectSCIMUserListResponse: failed to marshal resource: %w", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("projectSCIMUserListResponse: failed to unmarshal resource: %w", err)
		}
		resources = append(resources, projectSCIMAttributes(m, u.ExtensionURN, attributes, excludedAttributes))
	}
	envelope["Resources"] = resources
	return envelope, nil
}

// projectSCIMUserResource prunes a single SCIM User resource per RFC 7644 §3.9.
func projectSCIMUserResource(
	u SCIMUser, attributes, excludedAttributes []string,
) (map[string]interface{}, error) {
	if len(attributes) == 0 && len(excludedAttributes) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(u)
	if err != nil {
		return nil, fmt.Errorf("projectSCIMUserResource: failed to marshal resource: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("projectSCIMUserResource: failed to unmarshal resource: %w", err)
	}
	return projectSCIMAttributes(m, u.ExtensionURN, attributes, excludedAttributes), nil
}
