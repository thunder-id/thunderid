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

// Package serverconfig provides the server-wide configuration store and service layer.
package serverconfig

// ConfigName identifies a server-wide configuration section.
type ConfigName string

const (
	// ConfigNameCORS is the configuration key for runtime-mutable CORS allowed origins.
	ConfigNameCORS ConfigName = "cors"
)

// supportedConfigNames lists all the supported server configuration names.
var supportedConfigNames = []ConfigName{
	ConfigNameCORS,
}

// IsValid reports whether the config name is one of the supported values.
func (n ConfigName) IsValid() bool {
	for _, supported := range supportedConfigNames {
		if n == supported {
			return true
		}
	}
	return false
}
