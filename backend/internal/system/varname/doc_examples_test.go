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
	cases := []struct{ resourceType, resourceName, field, want string }{
		{"application", "My App", "ClientId", "APPLICATION_MY_APP_CLIENT_ID"},
		{"application", "My-App", "ClientId", "APPLICATION_MY_APP_CLIENT_ID"},
		{"application", "My App-", "ClientId", "APPLICATION_MY_APP_CLIENT_ID"},
		{"agent", "My App", "ClientId", "AGENT_MY_APP_CLIENT_ID"},
		{"user", "alice@example.com", "password", "USER_ALICE_EXAMPLE_COM_PASSWORD"},
		{"application", "2FA App", "ClientId", "APPLICATION_2FA_APP_CLIENT_ID"},
	}
	for _, c := range cases {
		if got := DeriveVariableName(c.resourceType, c.resourceName, c.field); got != c.want {
			t.Errorf("DeriveVariableName(%q, %q, %q) = %q, want %q",
				c.resourceType, c.resourceName, c.field, got, c.want)
		}
	}
}
