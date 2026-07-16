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

package security

import "testing"

// ref builds a claims map carrying a status.status_list reference with the given idx and uri.
func ref(idx interface{}, uri string) map[string]interface{} {
	return map[string]interface{}{
		claimStatus: map[string]interface{}{
			claimStatusList: map[string]interface{}{claimStatusListIdx: idx, claimStatusListURI: uri},
		},
	}
}

func TestExtractStatusReference(t *testing.T) {
	tests := []struct {
		name          string
		claims        map[string]interface{}
		wantURI       string
		wantIdx       int64
		wantMalformed bool
	}{
		{"float64 idx", ref(float64(42), "https://i/statuslists/a"), "https://i/statuslists/a", 42, false},
		{"int64 idx", ref(int64(7), "https://i/statuslists/a"), "https://i/statuslists/a", 7, false},
		{"zero idx", ref(float64(0), "https://i/statuslists/a"), "https://i/statuslists/a", 0, false},
		{"absent status claim", map[string]interface{}{}, "", 0, false},
		{"status not an object", map[string]interface{}{claimStatus: "revoked"}, "", 0, false},
		{"status_list absent", map[string]interface{}{claimStatus: map[string]interface{}{}}, "", 0, false},
		{"empty uri", ref(float64(1), ""), "", 0, true},
		{"negative idx", ref(float64(-1), "https://i/statuslists/a"), "", 0, true},
		{"fractional idx", ref(float64(3.5), "https://i/statuslists/a"), "", 0, true},
		{"non-numeric idx", ref("nope", "https://i/statuslists/a"), "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, idx, malformed := extractStatusReference(tt.claims)
			if uri != tt.wantURI || idx != tt.wantIdx || malformed != tt.wantMalformed {
				t.Fatalf("extractStatusReference = (%q, %d, %v), want (%q, %d, %v)",
					uri, idx, malformed, tt.wantURI, tt.wantIdx, tt.wantMalformed)
			}
		})
	}
}
