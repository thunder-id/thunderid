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

package http

import (
	"crypto/tls"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// GetTLSVersion returns the appropriate TLS version constant based on the provided
// configuration. It defaults to TLS 1.3 if the configured version is not recognized
// or empty.
func GetTLSVersion(config config.Config) uint16 {
	var minTLSVersion uint16
	switch config.TLS.MinVersion {
	case "1.2":
		minTLSVersion = tls.VersionTLS12
	case "1.3":
		minTLSVersion = tls.VersionTLS13
	default:
		minTLSVersion = tls.VersionTLS13 // Default to TLS 1.3 for better security
	}
	return minTLSVersion
}
