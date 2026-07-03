package scim

import (
	"encoding/json"
	"fmt"
)

// SCIMUser is the SCIM wire representation of a ThunderID user resource.
// MarshalJSON embeds the extension attributes under the ThunderID extension URN key
// per RFC 7644 §3.3.
type SCIMUser struct {
	ID           string          `json:"id"`
	Schemas      []string        `json:"schemas"`
	ExtensionURN string          `json:"-"`
	Attributes   json.RawMessage `json:"-"`
	Meta         SCIMMeta        `json:"meta"`
}

// MarshalJSON produces the SCIM wire JSON for a User resource.
// The extension attributes object is keyed by its URN at the top level.
func (u SCIMUser) MarshalJSON() ([]byte, error) {
	type plain struct {
		Schemas []string `json:"schemas"`
		ID      string   `json:"id"`
		Meta    SCIMMeta `json:"meta"`
	}
	base := plain{
		Schemas: u.Schemas,
		ID:      u.ID,
		Meta:    u.Meta,
	}

	baseBytes, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("SCIMUser.MarshalJSON: failed to marshal base: %w", err)
	}

	// No extension attributes — return base object as-is.
	if len(u.Attributes) == 0 || u.ExtensionURN == "" {
		return baseBytes, nil
	}

	// Merge extension attributes under the URN key into the base map.
	var baseMap map[string]json.RawMessage
	if err := json.Unmarshal(baseBytes, &baseMap); err != nil {
		return nil, fmt.Errorf("SCIMUser.MarshalJSON: failed to unmarshal base map: %w", err)
	}
	baseMap[u.ExtensionURN] = u.Attributes

	return json.Marshal(baseMap)
}

// SCIMUserListResponse is the SCIM ListResponse envelope for User resources.
// RFC 7644 §3.4.2
type SCIMUserListResponse struct {
	Schemas      []string   `json:"schemas"`
	TotalResults int        `json:"totalResults"`
	StartIndex   int        `json:"startIndex"`
	ItemsPerPage int        `json:"itemsPerPage"`
	Resources    []SCIMUser `json:"Resources"`
}
