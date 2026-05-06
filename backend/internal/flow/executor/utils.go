/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package executor

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/flow/common"
)

// GetUserAttribute extracts a specific attribute value from a user entity's JSON attributes.
func GetUserAttribute(user *entityprovider.Entity, attributeKey string) (string, error) {
	if user == nil || len(user.Attributes) == 0 {
		return "", errors.New("user entity or attributes are empty")
	}

	var attrs map[string]interface{}
	if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
		return "", errors.New("failed to parse user attributes")
	}

	if val, ok := attrs[attributeKey]; ok {
		if strVal, isString := val.(string); isString && strVal != "" {
			return strVal, nil
		}
	}

	return "", fmt.Errorf("attribute '%s' not found or is empty", attributeKey)
}

// upsertInputs merges incoming inputs into existing: replaces entries with a matching
// Identifier in-place, appends entries that are not yet present.
func upsertInputs(existing []common.Input, incoming []common.Input) []common.Input {
	idxMap := make(map[string]int, len(existing))
	for i, inp := range existing {
		idxMap[inp.Identifier] = i
	}
	for _, inp := range incoming {
		if idx, exists := idxMap[inp.Identifier]; exists {
			existing[idx] = inp
		} else {
			existing = append(existing, inp)
		}
	}
	return existing
}
