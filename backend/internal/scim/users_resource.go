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
	var m map[string]json.RawMessage
	if err := json.Unmarshal(attrs, &m); err != nil {
		log.GetLogger().Error(ctx,
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
		log.GetLogger().Error(ctx,
			"stripCredentialFields: failed to marshal stripped attributes; "+
				"returning empty object to prevent credential leak",
			log.Error(err))
		return json.RawMessage(`{}`)
	}
	return stripped
}

// buildSCIMUserResource converts a Thunder user.User into a SCIMUser wire response.
func buildSCIMUserResource(
	ctx context.Context, u user.User, extensionURN, baseURL string, credKeys map[string]struct{},
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
		scimUser.CoreAttrs = mapToCoreAttrs(scimUser.Attributes)
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

func userVersionState(u user.User) any {
	return struct {
		Attributes json.RawMessage
	}{
		Attributes: u.Attributes,
	}
}
