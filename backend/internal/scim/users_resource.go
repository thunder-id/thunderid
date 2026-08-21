// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
)

// stripCredentialFields removes credential-typed keys from raw JSON attributes.
// On any parse or marshal error it fails closed by returning an empty JSON
// object ({}) so that no credential fields can leak into SCIM responses.
func stripCredentialFields(
	ctx context.Context, attrs json.RawMessage, credentialKeys map[string]struct{},
) json.RawMessage {
	if len(credentialKeys) == 0 {
		return attrs
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersServiceLoggerComponentName))

	var m map[string]json.RawMessage
	if err := json.Unmarshal(attrs, &m); err != nil {
		logger.Error(ctx,
			"stripCredentialFields: failed to parse user attributes; returning empty object to prevent credential leak",
			log.Error(err))
		return json.RawMessage(`{}`)
	}
	for key := range credentialKeys {
		delete(m, key)
		for k := range m {
			if strings.EqualFold(k, key) {
				delete(m, k)
			}
		}
	}
	stripped, err := json.Marshal(m)
	if err != nil {
		logger.Error(ctx,
			"stripCredentialFields: failed to marshal stripped attributes; "+
				"returning empty object to prevent credential leak",
			log.Error(err))
		return json.RawMessage(`{}`)
	}
	return stripped
}

// buildSCIMUserResource converts a Thunder user.User into a SCIMUser wire response.
// includeCoreAttrs controls whether the response's mapped core schema fields
// (userName, emails, name, etc.) are populated: callers backing a request
// payload should pass whether that payload carried core attributes, so a
// purely custom-schema request doesn't get core fields mixed into its response.
func buildSCIMUserResource(
	ctx context.Context, u user.User, extensionURN, baseURL string,
	credKeys map[string]struct{}, includeCoreAttrs bool,
) SCIMUser {
	location := fmt.Sprintf("%s%s/Users/%s", baseURL, SCIMBasePath, u.ID)

	scimUser := SCIMUser{
		ID:           u.ID,
		Schemas:      []string{SCIMCoreUserSchemaURN, extensionURN},
		ExtensionURN: extensionURN,
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     location,
			Version:      generateVersion(userVersionState(u)),
		},
	}

	if len(u.Attributes) > 0 {
		scimUser.Attributes = stripCredentialFields(ctx, u.Attributes, credKeys)
		if includeCoreAttrs {
			scimUser.CoreAttrs = mapToCoreAttrs(scimUser.Attributes)
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
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: totalResults,
		StartIndex:   startIndex,
		ItemsPerPage: itemsPerPage,
		Resources:    users,
	}
}

var alwaysReturnedUserAttrs = map[string]struct{}{"id": {}, "schemas": {}, "meta": {}}

// resolveAttributePath splits an optionally URN-qualified attribute path
// (RFC 7644 §3.9) into its bare name and which namespace it's pinned to, if
// any. A bare name is unqualified and may match either namespace.
func resolveAttributePath(attr, extensionURN string) (name string, coreOnly, extensionOnly bool) {
	if extensionURN != "" && strings.HasPrefix(attr, extensionURN+":") {
		return attr[len(extensionURN)+1:], false, true
	}
	if strings.HasPrefix(attr, SCIMCoreUserSchemaURN+":") {
		return attr[len(SCIMCoreUserSchemaURN)+1:], true, false
	}
	return attr, false, false
}

// projectSCIMAttributes prunes a marshaled SCIM resource per RFC 7644 §3.9.
// attributes takes precedence over excludedAttributes; id/schemas/meta are
// always kept. Paths may be bare names, matched against top-level keys and
// keys inside the extension-URN object, or URN-qualified to pin the match to
// one namespace.
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
	for _, attr := range paths {
		name, coreOnly, extOnly := resolveAttributePath(attr, extensionURN)
		lname := strings.ToLower(name)
		if !extOnly {
			top[lname] = struct{}{}
		}
		if !coreOnly {
			ext[lname] = struct{}{}
		}
	}

	extObj, hasExt := resource[extensionURN].(map[string]interface{})
	hasExt = hasExt && extensionURN != ""

	projected := make(map[string]interface{}, len(resource))
	for k, v := range resource {
		if k == extensionURN {
			continue
		}
		_, always := alwaysReturnedUserAttrs[k]
		_, matched := top[strings.ToLower(k)]
		if always || matched == keep {
			projected[k] = v
		}
	}
	if hasExt {
		if kept := filterMap(extObj, ext, keep); len(kept) > 0 {
			projected[extensionURN] = kept
		}
	}
	return projected
}

// filterMap keeps keys present in set (keep true) or absent from it (keep
// false), matching case-insensitively.
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

// projectSCIMUserListResponse rebuilds a list response with each resource's
// attributes pruned. Returns nil, nil if no projection was requested.
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
// Returns nil, nil if no projection was requested.
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

// userVersionState extracts the state of a user that determines its ETag version.
// The ETag covers the user's raw JSON attribute payload.
func userVersionState(u user.User) any {
	return struct {
		Attributes json.RawMessage
	}{
		Attributes: u.Attributes,
	}
}
