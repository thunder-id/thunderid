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

package main

import (
	"testing"
	"time"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// maxTokenValidity is the longest token lifetime the Status List subsystem derives its sealed-list
// retention window from; it must be the max of the access- and refresh-token validities.
func TestMaxTokenValidity(t *testing.T) {
	cfg := func(accessSeconds, refreshSeconds int64) oauthconfig.Config {
		return oauthconfig.Config{
			JWT: engineconfig.JWTConfig{ValidityPeriod: accessSeconds},
			OAuth: engineconfig.OAuthConfig{
				RefreshToken: engineconfig.RefreshTokenConfig{ValidityPeriod: refreshSeconds},
			},
		}
	}
	tests := []struct {
		name            string
		access, refresh int64
		want            time.Duration
	}{
		{"refresh is longest", 3600, 86400, 86400 * time.Second},
		{"access is longest", 7200, 3600, 7200 * time.Second},
		{"equal validities", 3600, 3600, 3600 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxTokenValidity(cfg(tt.access, tt.refresh))
			if got != tt.want {
				t.Fatalf("maxTokenValidity = %v, want %v", got, tt.want)
			}
		})
	}
}
