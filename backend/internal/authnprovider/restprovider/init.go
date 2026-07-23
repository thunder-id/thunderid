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

// Package restprovider implements an authentication provider that delegates to an external service over REST.
package restprovider

import (
	"errors"
	"time"

	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	systemhttp "github.com/thunder-id/thunderid/internal/system/http"
)

// Name is the name of the built-in REST authn provider.
const Name = "rest"

// Initialize builds the REST authentication provider from its config block. It
// validates that base_url is set and applies defaults for the request timeout and
// correlation-ID header. Enablement is the caller's concern.
func Initialize(cfg config.RestConfig) (provider.AuthnProviderInterface, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("base_url is required when the rest authn provider is enabled")
	}
	timeout := 10 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}
	correlationIDHeader := cfg.CorrelationIDHeader
	if correlationIDHeader == "" {
		correlationIDHeader = serverconst.CorrelationIDHeaderName
	}
	httpClient := systemhttp.NewHTTPClientWithTimeout(timeout)
	return newRestAuthnProvider(cfg.BaseURL, cfg.Security.APIKey, correlationIDHeader, httpClient), nil
}
