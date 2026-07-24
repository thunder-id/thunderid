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

// Package config validates the Thunder ID engine configuration.
package config

import (
	"fmt"
	"net/url"
	"strings"
)

const schemeHTTPS = "https"
const localhost = "localhost"

// Validate checks the security configuration for correctness, including any nested
// sections that expose their own Validate method.
func (c *SecurityConfig) Validate() error {
	if c.JWKSCacheTTL < 0 {
		return fmt.Errorf("server.security.jwks_cache_ttl must be non-negative (got %d)", c.JWKSCacheTTL)
	}
	if err := c.TokenRevocation.Validate(); err != nil {
		return err
	}
	return c.TrustedIssuer.Validate()
}

// Validate checks the token-revocation configuration. It runs only when the feature is enabled: an
// unsupported source is rejected, a negative sync interval is rejected, and a non-positive interval
// otherwise falls back to the default.
func (c *TokenRevocationConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.SyncIntervalSeconds < 0 {
		return fmt.Errorf(
			"server.security.token_revocation.sync_interval_seconds must be non-negative (got %d)",
			c.SyncIntervalSeconds)
	}
	if c.Source != "" && c.Source != tokenRevocationSourceDB {
		return fmt.Errorf(
			"server.security.token_revocation.source %q is not supported (supported: %q)",
			c.Source, tokenRevocationSourceDB)
	}
	return nil
}

// IsConfigured reports whether any DPoP field has been set. When false, callers should
// skip validation: this matches the convention used by TrustedIssuerConfig and keeps
// config-loading tests that omit the dpop section working without surprise failures.
func (c *DPoPConfig) IsConfigured() bool {
	return c.Required || c.IatWindow != 0 || c.Leeway != 0 ||
		c.MaxJTILength != 0 || len(c.AllowedAlgs) > 0
}

// Validate ensures DPoP configuration values are within accepted bounds and the
// allowed_algs list contains only asymmetric JWS algorithms supported for DPoP.
func (c *DPoPConfig) Validate() error {
	if !c.IsConfigured() {
		return nil
	}
	if c.IatWindow <= 0 {
		return fmt.Errorf("oauth.dpop.iat_window must be greater than 0")
	}
	if c.Leeway < 0 {
		return fmt.Errorf("oauth.dpop.leeway must be greater than or equal to 0")
	}
	if c.MaxJTILength <= 0 {
		return fmt.Errorf("oauth.dpop.max_jti_length must be greater than 0")
	}
	if len(c.AllowedAlgs) == 0 {
		return fmt.Errorf("oauth.dpop.allowed_algs must contain at least one algorithm")
	}
	supported := map[string]struct{}{
		"ES256": {}, "ES384": {}, "ES512": {},
		"PS256": {}, "PS384": {}, "PS512": {},
		"RS256": {}, "RS384": {}, "RS512": {},
		"EdDSA": {},
	}
	for _, alg := range c.AllowedAlgs {
		if _, ok := supported[alg]; !ok {
			return fmt.Errorf("oauth.dpop.allowed_algs contains unsupported or symmetric algorithm: %q", alg)
		}
	}
	return nil
}

// IsConfigured reports whether the trusted issuer feature is configured and active.
// Setting issuer is the activation signal; jwks_url and audience are then required.
func (c *TrustedIssuerConfig) IsConfigured() bool {
	return c.Issuer != ""
}

// Validate checks the trusted issuer configuration for correctness.
// When issuer is set, jwks_url and audience must also be set.
// JWKS URL must use HTTPS to prevent MITM attacks on public key retrieval.
// HTTP is allowed only for localhost/127.0.0.1 to support local development and tests.
func (c *TrustedIssuerConfig) Validate() error {
	if !c.IsConfigured() {
		return nil
	}
	if c.JWKSURL == "" {
		return fmt.Errorf("trusted_issuer.jwks_url must be set when trusted_issuer.issuer is set")
	}
	if c.Audience == "" {
		return fmt.Errorf("trusted_issuer.audience must be set when trusted_issuer.issuer is set")
	}

	parsed, err := url.Parse(c.JWKSURL)
	if err != nil {
		return fmt.Errorf("trusted_issuer.jwks_url is not a valid URL: %w", err)
	}
	switch parsed.Scheme {
	case schemeHTTPS:
		return nil
	case "http":
		host := parsed.Hostname()
		if host == localhost || host == "127.0.0.1" || host == "::1" {
			return nil
		}
		return fmt.Errorf(
			"trusted_issuer.jwks_url must use https (got http://%s); "+
				"http is only allowed for localhost", host)
	default:
		return fmt.Errorf("trusted_issuer.jwks_url must use https scheme (got %q)", parsed.Scheme)
	}
}

// Validate checks the ACR-AMR mapping for configuration errors.
func (c *AuthClassConfig) Validate() error {
	amrSet := make(map[string]struct{}, len(c.Amrs))
	for _, amr := range c.Amrs {
		if strings.TrimSpace(amr) == "" {
			return fmt.Errorf("auth_class: AMR entry must not be empty")
		}
		amrSet[amr] = struct{}{}
	}

	if len(c.AcrAMR) == 0 {
		return nil
	}

	for acr, amrKeys := range c.AcrAMR {
		if strings.TrimSpace(acr) == "" {
			return fmt.Errorf("auth_class: ACR value must not be empty")
		}
		if len(amrKeys) == 0 {
			return fmt.Errorf("auth_class: ACR %q has an empty AMR list", acr)
		}
		for _, amrKey := range amrKeys {
			if strings.TrimSpace(amrKey) == "" {
				return fmt.Errorf("auth_class: ACR %q references an empty AMR key", acr)
			}
			if _, ok := amrSet[amrKey]; !ok {
				return fmt.Errorf("auth_class: ACR %q references unknown AMR key %q", acr, amrKey)
			}
		}
	}

	return nil
}

// Validate ensures the token-exchange token-family mode is one of the accepted values. An empty value
// is accepted and treated as the default (no inherited token family).
func (c *TokenExchangeConfig) Validate() error {
	switch c.TokenFamily {
	case "", "none", "inherit":
		return nil
	default:
		return fmt.Errorf("token_exchange: token_family must be empty, \"none\", or \"inherit\", got %q", c.TokenFamily)
	}
}

// GetServerURL constructs the server URL from the server configuration.
// It uses PublicURL if set, otherwise constructs from hostname, port, and scheme.
func GetServerURL(server *ServerConfig) string {
	if server.PublicURL != "" {
		return server.PublicURL
	}
	scheme := schemeHTTPS
	if server.HTTPOnly {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, server.Hostname, server.Port)
}
