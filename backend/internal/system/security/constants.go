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

const (
	// maxPublicPathLength defines the maximum allowed length for a public path.
	// This prevents potential DoS attacks via excessively long paths (even with safe regex).
	maxPublicPathLength = 4096

	// directAuthHeaderName is the request header carrying the Direct Auth Secret on Direct API requests.
	directAuthHeaderName = "Direct-Auth-Secret"
)
