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

package cors

import "github.com/thunder-id/thunderid/tests/integration/testutils"

const (
	testServerURL   = testutils.TestServerURL
	serverConfigURL = testServerURL + "/server-config"

	// staticOrigin is configured in resources/deployment.yaml (cors.allowed_origins). It exercises
	// the static, boot-time CORS baseline that the dynamic config is unioned with.
	staticOrigin = "https://static.example.com"
	// dynamicOrigin is set at runtime via PUT /server-config; it is not in deployment.yaml.
	dynamicOrigin = "https://dynamic.example.com"
	// secondDynamicOrigin is a second runtime origin used to verify replace semantics.
	secondDynamicOrigin = "https://second.example.com"
	// regexOrigin matches the regex set at runtime; it is not listed literally anywhere.
	regexOrigin = "https://tenant-7.regex.example"
	// unknownOrigin is never configured and must never be allowed.
	unknownOrigin = "https://unknown.example.com"
)
