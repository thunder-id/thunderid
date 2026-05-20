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

package user

import (
	"encoding/json"
	"sort"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// UserGroup represents a group with basic information for user endpoints.
type UserGroup struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	OUID               string `json:"ouId"`
}

// AuthenticateUserResponse represents the response from user authentication
type AuthenticateUserResponse struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	OUID             string `json:"ouId"`
}

// UserGroupListResponse represents the response for listing groups that a user belongs to.
type UserGroupListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	Count        int              `json:"count"`
	Groups       []UserGroup      `json:"groups"`
	Links        []testutils.Link `json:"links"`
}

// OUCreateRequest represents the request body for creating an organization unit.
type OUCreateRequest struct {
	Handle      string  `json:"handle"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Parent      *string `json:"parent,omitempty"`
}

// OUResponse represents an organization unit response.
type OUResponse struct {
	ID          string  `json:"id"`
	Handle      string  `json:"handle"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Parent      *string `json:"parent"`
}

// Equals compares two testutils.User for equality
func Equals(user, expectedUser testutils.User) bool {
	if user.ID != expectedUser.ID || user.OUID != expectedUser.OUID || user.Type != expectedUser.Type {
		return false
	}

	// Compare the Attributes JSON
	var attr1, attr2 map[string]interface{}
	if err := json.Unmarshal(user.Attributes, &attr1); err != nil {
		return false
	}
	if err := json.Unmarshal(expectedUser.Attributes, &attr2); err != nil {
		return false
	}

	return compareStringSlices(getSortedKeys(attr1), getSortedKeys(attr2))
}

// getSortedKeys returns sorted keys from a map
func getSortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// compareStringSlices compares two string slices
func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
