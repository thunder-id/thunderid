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

package client

import (
	"fmt"
	"strings"
)

// parseHTTPHeaders parses a comma-separated string of HTTP headers into a map.
func parseHTTPHeaders(headersString string) (map[string]string, error) {
	headers := make(map[string]string)
	if strings.TrimSpace(headersString) == "" {
		return headers, nil
	}
	for _, header := range strings.Split(headersString, ",") {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		} else {
			return nil, fmt.Errorf("invalid HTTP header format: %s", header)
		}
	}
	return headers, nil
}
