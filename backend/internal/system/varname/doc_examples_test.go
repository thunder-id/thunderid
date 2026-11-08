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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package varname

import "testing"

// The examples in docs/content/guides/resource-export.mdx are pinned here, so a change to the naming
// rule fails a test rather than leaving the guide quietly wrong.
func TestDocumentedExamples(t *testing.T) {
	cases := []struct{ resourceType, resourceID, field, want string }{
		{"application", idAlice, "ClientId", "APPLICATION__0198F3A17C2E7B109A445D3E8F21C6B7__CLIENT_ID"},
		{"agent", idAlice, "ClientId", "AGENT__0198F3A17C2E7B109A445D3E8F21C6B7__CLIENT_ID"},
		{"user", idAlice, "password", "USER__0198F3A17C2E7B109A445D3E8F21C6B7__PASSWORD"},
		{"resource_server", idAlice, "identifier", "RESOURCE_SERVER__0198F3A17C2E7B109A445D3E8F21C6B7__IDENTIFIER"},
	}
	for _, c := range cases {
		if got := DeriveVariableName(c.resourceType, c.resourceID, c.field); got != c.want {
			t.Errorf("DeriveVariableName(%q, %q, %q) = %q, want %q",
				c.resourceType, c.resourceID, c.field, got, c.want)
		}
	}
}
