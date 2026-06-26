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

// Package config holds Thunder ID engine configuration types shared across packages.
package config

import (
	"time"
)

// TrustedIssuerConfig holds configuration for trusted external issuer authentication.
// Setting Issuer activates the feature: the server trusts tokens carrying that iss claim
// and validates them via the external authentication server's JWKS endpoint. When Issuer
// is set, JWKSURL and Audience are required and the server fails to start if either is
// missing.
//
// Per RFC 9068 §2.2 and RFC 8707, the Audience field must be set to this server's own
// identifier (typically its public URL). The frontend must include a matching "resource"
// parameter in the authorization request so the auth server sets the token's "aud" claim
// to this server's identifier.
//
// RequiredClaims enforces that incoming tokens contain specific claims with expected values.
// Each entry specifies a claim name and the value it must hold. If any required claim is
// missing or does not match, the token is rejected.
type TrustedIssuerConfig struct {
	Issuer         string          `yaml:"issuer"          json:"issuer"`
	JWKSURL        string          `yaml:"jwks_url"        json:"jwks_url"`
	Audience       string          `yaml:"audience"        json:"audience"`
	RequiredClaims []RequiredClaim `yaml:"required_claims" json:"required_claims"`
}

// SecurityConfig holds the security-related configuration details.
//
// JWKSCacheTTL controls how long fetched JWKS responses are reused from the in-process
// cache before being re-fetched. It is not specific to trusted_issuer: the same cache
// backs every JWKS consumer in the server (trusted issuer validation, federated OIDC
// authenticators such as Google, etc.), so the setting lives at the security level
// rather than nested under any particular consumer. Value is in seconds; zero disables
// the cache; negative values are rejected at load time.
type SecurityConfig struct {
	JWKSCacheTTL           int                 `yaml:"jwks_cache_ttl"           json:"jwks_cache_ttl"`
	TrustedIssuer          TrustedIssuerConfig `yaml:"trusted_issuer"           json:"trusted_issuer"`
	SystemPermissionPrefix string              `yaml:"system_permission_prefix" json:"system_permission_prefix"`
}

// KeyConfig holds the key configuration details.
type KeyConfig struct {
	ID       string `yaml:"id"        json:"id"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file"  json:"key_file"`
}

// CacheProperty defines the properties for individual caches.
type CacheProperty struct {
	Name           string `yaml:"name"            json:"name"`
	Disabled       bool   `yaml:"disabled"        json:"disabled"`
	Size           int    `yaml:"size"            json:"size"`
	TTL            int    `yaml:"ttl"             json:"ttl"`
	EvictionPolicy string `yaml:"eviction_policy" json:"eviction_policy"`
}

// CacheConfig holds the cache configuration details.
type CacheConfig struct {
	Disabled        bool            `yaml:"disabled"             json:"disabled"`
	Type            string          `yaml:"type"                 json:"type"`
	Size            int             `yaml:"size"                 json:"size"`
	TTL             int             `yaml:"ttl"                  json:"ttl"`
	EvictionPolicy  string          `yaml:"eviction_policy"      json:"eviction_policy"`
	CleanupInterval int             `yaml:"cleanup_interval"     json:"cleanup_interval"`
	Properties      []CacheProperty `yaml:"properties,omitempty" json:"properties,omitempty"`
	Redis           RedisConfig     `yaml:"redis"                json:"redis"`
}

// RedisConfig holds the Redis connection configuration.
type RedisConfig struct {
	Address           string `yaml:"address"              json:"address"`
	Username          string `yaml:"username"             json:"username"`
	Password          string `yaml:"password"             json:"password"`
	DB                int    `yaml:"db"                   json:"db"`
	KeyPrefix         string `yaml:"key_prefix"           json:"key_prefix"`
	MaxRetries        int    `yaml:"max_retries"          json:"max_retries"`
	MinRetryBackoffMS int    `yaml:"min_retry_backoff_ms" json:"min_retry_backoff_ms"`
	MaxRetryBackoffMS int    `yaml:"max_retry_backoff_ms" json:"max_retry_backoff_ms"`
	DialTimeoutMS     int    `yaml:"dial_timeout_ms"      json:"dial_timeout_ms"`
	ReadTimeoutMS     int    `yaml:"read_timeout_ms"      json:"read_timeout_ms"`
	WriteTimeoutMS    int    `yaml:"write_timeout_ms"     json:"write_timeout_ms"`
}

// ServerConfig holds the server configuration details.
type ServerConfig struct {
	Hostname       string         `yaml:"hostname"   json:"hostname"`
	Port           int            `yaml:"port"       json:"port"`
	HTTPOnly       bool           `yaml:"http_only"  json:"http_only"`
	PublicURL      string         `yaml:"public_url" json:"public_url"`
	Identifier     string         `yaml:"identifier" json:"identifier"`
	SecurityConfig SecurityConfig `yaml:"security"   json:"security"`
}

// GateClientConfig holds the client configuration details.
type GateClientConfig struct {
	Hostname     string `yaml:"hostname"      json:"hostname"`
	Port         int    `yaml:"port"          json:"port"`
	Scheme       string `yaml:"scheme"        json:"scheme"`
	Path         string `yaml:"path"          json:"path"`
	LoginPath    string `yaml:"login_path"    json:"login_path"`
	ErrorPath    string `yaml:"error_path"    json:"error_path"`
	CallbackPath string `yaml:"callback_path" json:"callback_path"`
}

// EncryptionConfig holds the encryption configuration details.
type EncryptionConfig struct {
	Key string `yaml:"key" json:"key"`
}

// JWTConfig holds the JWT configuration details.
type JWTConfig struct {
	Issuer         string `yaml:"issuer"           json:"issuer"`
	ValidityPeriod int64  `yaml:"validity_period"  json:"validity_period"`
	Audience       string `yaml:"audience"         json:"audience"`
	PreferredKeyID string `yaml:"preferred_key_id" json:"preferred_key_id"`
	Leeway         int64  `yaml:"leeway"           json:"leeway"`
}

// AuthClassConfig holds the ACR-AMR mapping configuration.
type AuthClassConfig struct {
	Amrs   []string            `yaml:"amrs"    json:"amrs"`
	AcrAMR map[string][]string `yaml:"acr_amr" json:"acr_amr"`
}

// RefreshTokenConfig holds the refresh token configuration details.
type RefreshTokenConfig struct {
	RenewOnGrant   bool  `yaml:"renew_on_grant"  json:"renew_on_grant"`
	ValidityPeriod int64 `yaml:"validity_period" json:"validity_period"`
}

// AuthorizationCodeConfig holds the authorization code configuration details.
type AuthorizationCodeConfig struct {
	ValidityPeriod int64 `yaml:"validity_period" json:"validity_period"`
}

// DCRConfig holds the Dynamic Client Registration configuration.
type DCRConfig struct {
	Insecure bool `yaml:"insecure" json:"insecure"`
}

// PARConfig holds the Pushed Authorization Request (RFC 9126) configuration.
type PARConfig struct {
	RequirePAR bool  `yaml:"require_par" json:"require_par"`
	ExpiresIn  int64 `yaml:"expires_in"  json:"expires_in"`
}

// DPoPConfig holds the OAuth 2.0 DPoP configuration.
type DPoPConfig struct {
	Required     bool     `yaml:"required"       json:"required"`
	IatWindow    int      `yaml:"iat_window"     json:"iat_window"`
	Leeway       int      `yaml:"leeway"         json:"leeway"`
	AllowedAlgs  []string `yaml:"allowed_algs"   json:"allowed_algs"`
	MaxJTILength int      `yaml:"max_jti_length" json:"max_jti_length"`
}

// CIBAConfig holds the CIBA configuration.
type CIBAConfig struct {
	IDTokenHintMaxAgeDays int `yaml:"id_token_hint_max_age_days" json:"id_token_hint_max_age_days"`
}

// OAuthConfig holds the OAuth configuration details.
type OAuthConfig struct {
	RefreshToken      RefreshTokenConfig      `yaml:"refresh_token"               json:"refresh_token"`
	AuthorizationCode AuthorizationCodeConfig `yaml:"authorization_code"          json:"authorization_code"`
	DCR               DCRConfig               `yaml:"dcr"                         json:"dcr"`
	PAR               PARConfig               `yaml:"par"                         json:"par"`
	DPoP              DPoPConfig              `yaml:"dpop"                        json:"dpop"`
	AuthClass         AuthClassConfig         `yaml:"auth_class"                  json:"auth_class"`
	CIBA              CIBAConfig              `yaml:"ciba"                        json:"ciba"`
	// AllowWildcardRedirectURI enables wildcard pattern matching for redirect URIs.
	// When false (default), only exact redirect URI matching is performed.
	AllowWildcardRedirectURI bool `yaml:"allow_wildcard_redirect_uri" json:"allow_wildcard_redirect_uri"`
}

// FlowConfig holds the configuration details for the flow service.
type FlowConfig struct {
	DefaultAuthFlowHandle    string `yaml:"default_auth_flow_handle"    json:"default_auth_flow_handle"`
	UserOnboardingFlowHandle string `yaml:"user_onboarding_flow_handle" json:"user_onboarding_flow_handle"`
	MaxVersionHistory        int    `yaml:"max_version_history"         json:"max_version_history"`
	AutoInferRegistration    bool   `yaml:"auto_infer_registration"     json:"auto_infer_registration"`
	Store                    string `yaml:"store"                       json:"store"`
	// Executors lists built-in executor names to register (e.g. CredentialsAuthExecutor).
	// When empty, all built-in executors are registered. When set, only listed executors
	// are available; omit only executors you intentionally disable on this node.
	Executors []string `yaml:"executors"                   json:"executors"`
	// Interceptors lists built-in interceptor names to register (e.g. CaptchaInterceptor).
	// When empty, all built-in interceptors are registered. When set, only listed interceptors
	// are available; omit only interceptors you intentionally disable on this node.
	Interceptors []string `yaml:"interceptors"                json:"interceptors"`
}

// ConsentConfig holds the configuration for the consent service integration.
type ConsentConfig struct {
	Enabled    bool   `yaml:"enabled"     json:"enabled"`
	BaseURL    string `yaml:"base_url"    json:"base_url"`
	Timeout    int    `yaml:"timeout"     json:"timeout"`     // HTTP request timeout in seconds. Default: 5
	MaxRetries int    `yaml:"max_retries" json:"max_retries"` // Max retry attempts for transient errors. Default: 3
}

// RequiredClaim defines a claim name and expected value that must be present in the token.
type RequiredClaim struct {
	Claim string `yaml:"claim" json:"claim"`
	Value string `yaml:"value" json:"value"`
}

// ResourceConfig holds the resource management configuration details.
type ResourceConfig struct {
	DefaultDelimiter string `yaml:"default_delimiter" json:"default_delimiter"`
	// Store defines the storage mode for resource servers.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store"             json:"store"`
}

// ObservabilityConfig holds the observability configuration details.
type ObservabilityConfig struct {
	Enabled     bool                      `yaml:"enabled"      json:"enabled"`
	Output      ObservabilityOutputConfig `yaml:"output"       json:"output"`
	FailureMode string                    `yaml:"failure_mode" json:"failure_mode"`
}

// ObservabilityOutputConfig holds observability output configuration.
type ObservabilityOutputConfig struct {
	File          ObservabilityFileConfig    `yaml:"file"          json:"file"`
	Console       ObservabilityConsoleConfig `yaml:"console"       json:"console"`
	OpenTelemetry ObservabilityOTelConfig    `yaml:"opentelemetry" json:"opentelemetry"`
}

// ObservabilityFileConfig captures file sink settings for observability events.
type ObservabilityFileConfig struct {
	Enabled       bool          `yaml:"enabled"        json:"enabled"`
	FilePath      string        `yaml:"file_path"      json:"file_path"`
	Format        string        `yaml:"format"         json:"format"`
	BufferSize    int           `yaml:"buffer_size"    json:"buffer_size"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	Categories    []string      `yaml:"categories"     json:"categories"`
}

// ObservabilityConsoleConfig captures console sink settings for observability events.
type ObservabilityConsoleConfig struct {
	Enabled    bool     `yaml:"enabled"    json:"enabled"`
	Format     string   `yaml:"format"     json:"format"`
	Categories []string `yaml:"categories" json:"categories"`
}

// ObservabilityOTelConfig holds OpenTelemetry configuration.
type ObservabilityOTelConfig struct {
	Enabled        bool     `yaml:"enabled"         json:"enabled"`
	ExporterType   string   `yaml:"exporter_type"   json:"exporter_type"`
	OTLPEndpoint   string   `yaml:"otlp_endpoint"   json:"otlp_endpoint"`
	ServiceName    string   `yaml:"service_name"    json:"service_name"`
	ServiceVersion string   `yaml:"service_version" json:"service_version"`
	Environment    string   `yaml:"environment"     json:"environment"`
	SampleRate     float64  `yaml:"sample_rate"     json:"sample_rate"`
	Categories     []string `yaml:"categories"      json:"categories"`
	// Insecure disables TLS for OTLP (not recommended for production)
	Insecure bool `yaml:"insecure"        json:"insecure"`
}
