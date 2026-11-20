// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributecache

import "encoding/json"

// withSessionGrants returns the attributes to store: any attribute named sessionGrantsKey is dropped,
// and grants, when present, are stored under it.
func withSessionGrants(attributes map[string]interface{}, grants *SessionGrants) map[string]interface{} {
	stored := make(map[string]interface{}, len(attributes)+1)
	for key, value := range attributes {
		if key != sessionGrantsKey {
			stored[key] = value
		}
	}
	if grants != nil {
		stored[sessionGrantsKey] = grants
	}
	return stored
}

// splitSessionGrants removes the session grants stored under sessionGrantsKey from attributes and
// returns them, or nil when there are none.
func splitSessionGrants(attributes map[string]interface{}) (*SessionGrants, error) {
	value, ok := attributes[sessionGrantsKey]
	if !ok {
		return nil, nil
	}
	delete(attributes, sessionGrantsKey)
	if value == nil {
		return nil, nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var grants SessionGrants
	if err := json.Unmarshal(encoded, &grants); err != nil {
		return nil, err
	}

	return &grants, nil
}
