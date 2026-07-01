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

// Package testhelpers provides shared test fixtures for backend unit tests.
package testhelpers

import (
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// OAuthConfig returns a minimal OAuth configuration for unit tests.
func OAuthConfig() oauthconfig.Config {
	return oauthconfig.Config{
		DeploymentID:  "test-deployment",
		RuntimeDBType: "sqlite",
		BaseURL:       "https://thunder.io",
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
			Audience:       "https://thunder.io",
			Leeway:         60,
		},
		OAuth: engineconfig.OAuthConfig{
			RefreshToken: engineconfig.RefreshTokenConfig{
				RenewOnGrant:   false,
				ValidityPeriod: 86400,
			},
			AuthorizationCode: engineconfig.AuthorizationCodeConfig{
				ValidityPeriod: 300,
			},
			PAR: engineconfig.PARConfig{
				ExpiresIn: 600,
			},
			DPoP: engineconfig.DPoPConfig{
				AllowedAlgs: []string{"ES256"},
			},
			CIBA: engineconfig.CIBAConfig{
				IDTokenHintMaxAgeDays: 30,
			},
		},
		GateClient: engineconfig.GateClientConfig{
			Scheme:    "https",
			Hostname:  "localhost",
			Port:      3000,
			LoginPath: "/login",
			ErrorPath: "/error",
		},
	}
}
