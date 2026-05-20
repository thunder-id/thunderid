/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package config provides structures and functions for loading and managing server configurations.
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	urlpath "path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/utils"

	yaml "gopkg.in/yaml.v3"
)

const schemeHTTPS = "https"

// SecurityConfig holds the security-related configuration details.
//
// JWKSCacheTTL controls how long fetched JWKS responses are reused from the in-process
// cache before being re-fetched. It is not specific to trusted_issuer: the same cache
// backs every JWKS consumer in the server (trusted issuer validation, federated OIDC
// authenticators such as Google, etc.), so the setting lives at the security level
// rather than nested under any particular consumer. Value is in seconds; zero disables
// the cache; negative values are rejected at load time.
type SecurityConfig struct {
	JWKSCacheTTL  int                 `yaml:"jwks_cache_ttl" json:"jwks_cache_ttl"`
	TrustedIssuer TrustedIssuerConfig `yaml:"trusted_issuer" json:"trusted_issuer"`
}

// Validate checks the security configuration for correctness, including any nested
// sections that expose their own Validate method.
func (c *SecurityConfig) Validate() error {
	if c.JWKSCacheTTL < 0 {
		return fmt.Errorf("server.security.jwks_cache_ttl must be non-negative (got %d)", c.JWKSCacheTTL)
	}
	return c.TrustedIssuer.Validate()
}

// ServerConfig holds the server configuration details.
type ServerConfig struct {
	Hostname       string         `yaml:"hostname" json:"hostname"`
	Port           int            `yaml:"port" json:"port"`
	HTTPOnly       bool           `yaml:"http_only" json:"http_only"`
	PublicURL      string         `yaml:"public_url" json:"public_url"`
	Identifier     string         `yaml:"identifier" json:"identifier"`
	SecurityConfig SecurityConfig `yaml:"security" json:"security"`
}

// GateClientConfig holds the client configuration details.
type GateClientConfig struct {
	Hostname  string `yaml:"hostname" json:"hostname"`
	Port      int    `yaml:"port" json:"port"`
	Scheme    string `yaml:"scheme" json:"scheme"`
	Path      string `yaml:"path" json:"path"`
	LoginPath string `yaml:"login_path" json:"login_path"`
	ErrorPath string `yaml:"error_path" json:"error_path"`
}

// TLSConfig holds the TLS configuration details.
type TLSConfig struct {
	MinVersion string `yaml:"min_version" json:"min_version"`
	CertFile   string `yaml:"cert_file" json:"cert_file"`
	KeyFile    string `yaml:"key_file" json:"key_file"`
}

// DataSource holds the individual database connection details.
// Type is the only common field; connection parameters live under the
// matching sub-struct (Postgres, SQLite, or Redis).
type DataSource struct {
	Type     string             `yaml:"type" json:"type"`
	Postgres PostgresDataSource `yaml:"postgres" json:"postgres"`
	SQLite   SQLiteDataSource   `yaml:"sqlite" json:"sqlite"`
	Redis    RedisDataSource    `yaml:"redis" json:"redis"`
}

// PostgresDataSource holds PostgreSQL-specific connection details.
type PostgresDataSource struct {
	Hostname          string `yaml:"hostname" json:"hostname"`
	Port              int    `yaml:"port" json:"port"`
	Name              string `yaml:"name" json:"name"`
	Username          string `yaml:"username" json:"username"`
	Password          string `yaml:"password" json:"password"`
	SSLMode           string `yaml:"sslmode" json:"sslmode"`
	MaxOpenConns      int    `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns      int    `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime   int    `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	MaxRetries        int    `yaml:"max_retries" json:"max_retries"`
	MinRetryBackoffMS int    `yaml:"min_retry_backoff_ms" json:"min_retry_backoff_ms"`
	MaxRetryBackoffMS int    `yaml:"max_retry_backoff_ms" json:"max_retry_backoff_ms"`
}

// SQLiteDataSource holds SQLite-specific connection details.
type SQLiteDataSource struct {
	Path              string `yaml:"path" json:"path"`
	Options           string `yaml:"options" json:"options"`
	MaxOpenConns      int    `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns      int    `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime   int    `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	MaxRetries        int    `yaml:"max_retries" json:"max_retries"`
	MinRetryBackoffMS int    `yaml:"min_retry_backoff_ms" json:"min_retry_backoff_ms"`
	MaxRetryBackoffMS int    `yaml:"max_retry_backoff_ms" json:"max_retry_backoff_ms"`
}

// RedisDataSource holds Redis-specific connection details.
type RedisDataSource struct {
	Address           string `yaml:"address" json:"address"`
	Username          string `yaml:"username" json:"username"`
	Password          string `yaml:"password" json:"password"`
	DB                int    `yaml:"db" json:"db"`
	KeyPrefix         string `yaml:"key_prefix" json:"key_prefix"`
	MaxRetries        int    `yaml:"max_retries" json:"max_retries"`
	MinRetryBackoffMS int    `yaml:"min_retry_backoff_ms" json:"min_retry_backoff_ms"`
	MaxRetryBackoffMS int    `yaml:"max_retry_backoff_ms" json:"max_retry_backoff_ms"`
	DialTimeoutMS     int    `yaml:"dial_timeout_ms" json:"dial_timeout_ms"`
	ReadTimeoutMS     int    `yaml:"read_timeout_ms" json:"read_timeout_ms"`
	WriteTimeoutMS    int    `yaml:"write_timeout_ms" json:"write_timeout_ms"`
}

// DatabaseConfig holds the different database configuration details.
type DatabaseConfig struct {
	Config  DataSource `yaml:"config" json:"config"`
	Runtime DataSource `yaml:"runtime" json:"runtime"`
	User    DataSource `yaml:"user" json:"user"`
}

// CacheProperty defines the properties for individual caches.
type CacheProperty struct {
	Name           string `yaml:"name" json:"name"`
	Disabled       bool   `yaml:"disabled" json:"disabled"`
	Size           int    `yaml:"size" json:"size"`
	TTL            int    `yaml:"ttl" json:"ttl"`
	EvictionPolicy string `yaml:"eviction_policy" json:"eviction_policy"`
}

// CacheConfig holds the cache configuration details.
type CacheConfig struct {
	Disabled        bool            `yaml:"disabled" json:"disabled"`
	Type            string          `yaml:"type" json:"type"`
	Size            int             `yaml:"size" json:"size"`
	TTL             int             `yaml:"ttl" json:"ttl"`
	EvictionPolicy  string          `yaml:"eviction_policy" json:"eviction_policy"`
	CleanupInterval int             `yaml:"cleanup_interval" json:"cleanup_interval"`
	Properties      []CacheProperty `yaml:"properties,omitempty" json:"properties,omitempty"`
	Redis           RedisConfig     `yaml:"redis" json:"redis"`
}

// RedisConfig holds the Redis connection configuration.
type RedisConfig struct {
	Address           string `yaml:"address" json:"address"`
	Username          string `yaml:"username" json:"username"`
	Password          string `yaml:"password" json:"password"`
	DB                int    `yaml:"db" json:"db"`
	KeyPrefix         string `yaml:"key_prefix" json:"key_prefix"`
	MaxRetries        int    `yaml:"max_retries" json:"max_retries"`
	MinRetryBackoffMS int    `yaml:"min_retry_backoff_ms" json:"min_retry_backoff_ms"`
	MaxRetryBackoffMS int    `yaml:"max_retry_backoff_ms" json:"max_retry_backoff_ms"`
	DialTimeoutMS     int    `yaml:"dial_timeout_ms" json:"dial_timeout_ms"`
	ReadTimeoutMS     int    `yaml:"read_timeout_ms" json:"read_timeout_ms"`
	WriteTimeoutMS    int    `yaml:"write_timeout_ms" json:"write_timeout_ms"`
}

// JWTConfig holds the JWT configuration details.
type JWTConfig struct {
	Issuer         string `yaml:"issuer" json:"issuer"`
	ValidityPeriod int64  `yaml:"validity_period" json:"validity_period"`
	Audience       string `yaml:"audience" json:"audience"`
	PreferredKeyID string `yaml:"preferred_key_id" json:"preferred_key_id"`
	Leeway         int64  `yaml:"leeway" json:"leeway"`
}

// RefreshTokenConfig holds the refresh token configuration details.
type RefreshTokenConfig struct {
	RenewOnGrant   bool  `yaml:"renew_on_grant" json:"renew_on_grant"`
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
	ExpiresIn  int64 `yaml:"expires_in" json:"expires_in"`
}

// OAuthConfig holds the OAuth configuration details.
type OAuthConfig struct {
	RefreshToken      RefreshTokenConfig      `yaml:"refresh_token" json:"refresh_token"`
	AuthorizationCode AuthorizationCodeConfig `yaml:"authorization_code" json:"authorization_code"`
	DCR               DCRConfig               `yaml:"dcr" json:"dcr"`
	PAR               PARConfig               `yaml:"par" json:"par"`
	AuthClass         AuthClassConfig         `yaml:"auth_class" json:"auth_class"`
	// AllowWildcardRedirectURI enables wildcard pattern matching for redirect URIs.
	// When false (default), only exact redirect URI matching is performed.
	AllowWildcardRedirectURI bool `yaml:"allow_wildcard_redirect_uri" json:"allow_wildcard_redirect_uri"`
}

// FlowConfig holds the configuration details for the flow service.
type FlowConfig struct {
	DefaultAuthFlowHandle    string `yaml:"default_auth_flow_handle" json:"default_auth_flow_handle"`
	UserOnboardingFlowHandle string `yaml:"user_onboarding_flow_handle" json:"user_onboarding_flow_handle"`
	MaxVersionHistory        int    `yaml:"max_version_history" json:"max_version_history"`
	AutoInferRegistration    bool   `yaml:"auto_infer_registration" json:"auto_infer_registration"`
	Store                    string `yaml:"store" json:"store"`
}

// CryptoConfig holds the cryptographic configuration details.
type CryptoConfig struct {
	Encryption      EncryptionConfig      `yaml:"encryption" json:"encryption"`
	PasswordHashing PasswordHashingConfig `yaml:"password_hashing" json:"password_hashing"`
	Keys            []KeyConfig           `yaml:"keys" json:"keys"`
}

// KeyConfig holds the key configuration details.
type KeyConfig struct {
	ID       string `yaml:"id" json:"id"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
}

// EncryptionConfig holds the encryption configuration details.
type EncryptionConfig struct {
	Key string `yaml:"key" json:"key"`
}

// PasswordHashingConfig holds the password hashing configuration details.
type PasswordHashingConfig struct {
	Algorithm string         `yaml:"algorithm" json:"algorithm"`
	Argon2ID  Argon2IDConfig `yaml:"argon2id" json:"argon2id"`
	PBKDF2    PBKDF2Config   `yaml:"pbkdf2" json:"pbkdf2"`
	SHA256    SHA256Config   `yaml:"sha256" json:"sha256"`
}

// Argon2IDConfig holds the Argon2id password hashing configuration details.
type Argon2IDConfig struct {
	Iterations  int `yaml:"iterations" json:"iterations"`
	Memory      int `yaml:"memory" json:"memory"`
	Parallelism int `yaml:"parallelism" json:"parallelism"`
	KeySize     int `yaml:"key_size" json:"key_size"`
	SaltSize    int `yaml:"salt_size" json:"salt_size"`
}

// PBKDF2Config holds the PBKDF2 password hashing configuration details.
type PBKDF2Config struct {
	Iterations int `yaml:"iterations" json:"iterations"`
	KeySize    int `yaml:"key_size" json:"key_size"`
	SaltSize   int `yaml:"salt_size" json:"salt_size"`
}

// SHA256Config holds the SHA256 password hashing configuration details.
type SHA256Config struct {
	SaltSize int `yaml:"salt_size" json:"salt_size"`
}

// CORSConfig holds the configuration details for the CORS middleware.
//
// AllowedOrigins is heterogeneous: each entry is either a bare string (a
// literal origin matched after RFC-6454 canonicalization, with the special
// value "null" denoting the CORS null origin) or an object of the shape
// { regex: "..." } (an RE2 pattern matched against the raw request Origin
// header byte for byte). See the CORS section of
// docs/content/guides/getting-started/configuration.mdx.
type CORSConfig struct {
	AllowedOrigins cors.OriginEntries `yaml:"allowed_origins" json:"allowed_origins"`
}

// Validate checks every allowed-origins entry so configuration errors —
// invalid literals, malformed regexes, the unsupported "*" wildcard — are
// surfaced at server start rather than on the first cross-origin request.
// Installation of the runtime matcher is the server bootstrap's
// responsibility (see cors.InitializeMatcher); this config layer only owns
// YAML validation.
func (c *CORSConfig) Validate() error {
	return cors.Validate(c.AllowedOrigins)
}

// DeclarativeResources holds the configuration details for the declarative resources.
type DeclarativeResources struct {
	Enabled bool `yaml:"enabled" json:"enabled" default:"false"`
}

// ObservabilityConfig holds the observability configuration details.
type ObservabilityConfig struct {
	Enabled     bool                      `yaml:"enabled" json:"enabled"`
	Output      ObservabilityOutputConfig `yaml:"output" json:"output"`
	FailureMode string                    `yaml:"failure_mode" json:"failure_mode"`
}

// ObservabilityOutputConfig holds observability output configuration.
type ObservabilityOutputConfig struct {
	File          ObservabilityFileConfig    `yaml:"file" json:"file"`
	Console       ObservabilityConsoleConfig `yaml:"console" json:"console"`
	OpenTelemetry ObservabilityOTelConfig    `yaml:"opentelemetry" json:"opentelemetry"`
}

// ObservabilityFileConfig captures file sink settings for observability events.
type ObservabilityFileConfig struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`
	FilePath      string        `yaml:"file_path" json:"file_path"`
	Format        string        `yaml:"format" json:"format"`
	BufferSize    int           `yaml:"buffer_size" json:"buffer_size"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	Categories    []string      `yaml:"categories" json:"categories"`
}

// ObservabilityConsoleConfig captures console sink settings for observability events.
type ObservabilityConsoleConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	Format     string   `yaml:"format" json:"format"`
	Categories []string `yaml:"categories" json:"categories"`
}

// ObservabilityOTelConfig holds OpenTelemetry configuration.
type ObservabilityOTelConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	ExporterType   string   `yaml:"exporter_type" json:"exporter_type"`
	OTLPEndpoint   string   `yaml:"otlp_endpoint" json:"otlp_endpoint"`
	ServiceName    string   `yaml:"service_name" json:"service_name"`
	ServiceVersion string   `yaml:"service_version" json:"service_version"`
	Environment    string   `yaml:"environment" json:"environment"`
	SampleRate     float64  `yaml:"sample_rate" json:"sample_rate"`
	Categories     []string `yaml:"categories" json:"categories"`
	// Insecure disables TLS for OTLP (not recommended for production)
	Insecure bool `yaml:"insecure" json:"insecure"`
}

// UserConfig holds the user management configuration details.
type UserConfig struct {
	IndexedAttributes []string `yaml:"indexed_attributes" json:"indexed_attributes"`
	// Store defines the storage mode for users.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// SystemResourceServerConfig holds configuration for the built-in system resource server.
type SystemResourceServerConfig struct {
	Handle     string `yaml:"handle" json:"handle"`
	Identifier string `yaml:"identifier" json:"identifier"`
}

// ResourceConfig holds the resource management configuration details.
type ResourceConfig struct {
	DefaultDelimiter string `yaml:"default_delimiter" json:"default_delimiter"`
	// Store defines the storage mode for resource servers.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store                string                     `yaml:"store" json:"store"`
	SystemResourceServer SystemResourceServerConfig `yaml:"system_resource_server" json:"system_resource_server"`
}

// OrganizationUnitConfig holds the organization unit service configuration.
type OrganizationUnitConfig struct {
	// Store defines the storage mode for organization units.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// IdentityProviderConfig holds the identity provider service configuration.
type IdentityProviderConfig struct {
	// Store defines the storage mode for identity providers.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// ApplicationConfig holds the application service configuration.
type ApplicationConfig struct {
	// Store defines the storage mode for applications.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// EntityTypeConfig holds the entity type service configuration.
type EntityTypeConfig struct {
	// Store defines the storage mode for entity types.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// RoleConfig holds the role service configuration.
type RoleConfig struct {
	// Store defines the storage mode for roles.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// ThemeConfig holds the theme service configuration.
type ThemeConfig struct {
	// Store defines the storage mode for themes.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// LayoutConfig holds the layout service configuration.
type LayoutConfig struct {
	// Store defines the storage mode for layouts.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// TranslationConfig holds the translation service configuration.
type TranslationConfig struct {
	// Store defines the storage mode for translations.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// PasskeyConfig holds the passkey configuration details.
type PasskeyConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
}

// AuthnProviderConfig holds the authentication provider configuration details.
type AuthnProviderConfig struct {
	Type string     `yaml:"type" json:"type"`
	Rest RestConfig `yaml:"rest" json:"rest"`
}

// UserProviderConfig holds the user provider configuration details.
type UserProviderConfig struct {
	Type string `yaml:"type" json:"type"`
}

// EntityProviderConfig holds the entity provider configuration details.
type EntityProviderConfig struct {
	Type string `yaml:"type" json:"type"`
}

// RestConfig holds the REST authentication provider configuration details.
type RestConfig struct {
	BaseURL  string             `yaml:"base_url" json:"base_url"`
	Timeout  int                `yaml:"timeout" json:"timeout"`
	Security RestSecurityConfig `yaml:"security" json:"security"`
}

// RestSecurityConfig holds the REST authentication provider security configuration details.
type RestSecurityConfig struct {
	APIKey string `yaml:"api_key" json:"api_key"`
}

// EmailConfig holds the email configuration details.
type EmailConfig struct {
	SMTP SMTPEmailConfig `yaml:"smtp" json:"smtp"`
}

// SMTPEmailConfig holds the SMTP email configuration details.
type SMTPEmailConfig struct {
	Host                 string `yaml:"host" json:"host"`
	Port                 int    `yaml:"port" json:"port"`
	Username             string `yaml:"username" json:"username"`
	Password             string `yaml:"password" json:"password"`
	FromAddress          string `yaml:"from_address" json:"from_address"`
	EnableStartTLS       *bool  `yaml:"enable_start_tls" json:"enable_start_tls"`
	EnableAuthentication *bool  `yaml:"enable_authentication" json:"enable_authentication"`
}

// ConsentConfig holds the configuration for the consent service integration.
type ConsentConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	BaseURL    string `yaml:"base_url" json:"base_url"`
	Timeout    int    `yaml:"timeout" json:"timeout"`         // HTTP request timeout in seconds. Default: 5
	MaxRetries int    `yaml:"max_retries" json:"max_retries"` // Max retry attempts for transient errors. Default: 3
}

// RequiredClaim defines a claim name and expected value that must be present in the token.
type RequiredClaim struct {
	Claim string `yaml:"claim" json:"claim"`
	Value string `yaml:"value" json:"value"`
}

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
	Issuer         string          `yaml:"issuer" json:"issuer"`
	JWKSURL        string          `yaml:"jwks_url" json:"jwks_url"`
	Audience       string          `yaml:"audience" json:"audience"`
	RequiredClaims []RequiredClaim `yaml:"required_claims" json:"required_claims"`
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
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
		return fmt.Errorf(
			"trusted_issuer.jwks_url must use https (got http://%s); "+
				"http is only allowed for localhost", host)
	default:
		return fmt.Errorf("trusted_issuer.jwks_url must use https scheme (got %q)", parsed.Scheme)
	}
}

// AuthClassConfig holds the ACR-AMR mapping configuration.
type AuthClassConfig struct {
	Amrs   []string            `yaml:"amrs" json:"amrs"`
	AcrAMR map[string][]string `yaml:"acr_amr" json:"acr_amr"`
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

// Config holds the complete configuration details of the server.
type Config struct {
	Server               ServerConfig           `yaml:"server" json:"server"`
	GateClient           GateClientConfig       `yaml:"gate_client" json:"gate_client"`
	TLS                  TLSConfig              `yaml:"tls" json:"tls"`
	Database             DatabaseConfig         `yaml:"database" json:"database"`
	Cache                CacheConfig            `yaml:"cache" json:"cache"`
	JWT                  JWTConfig              `yaml:"jwt" json:"jwt"`
	OAuth                OAuthConfig            `yaml:"oauth" json:"oauth"`
	Flow                 FlowConfig             `yaml:"flow" json:"flow"`
	Crypto               CryptoConfig           `yaml:"crypto" json:"crypto"`
	CORS                 CORSConfig             `yaml:"cors" json:"cors"`
	User                 UserConfig             `yaml:"user" json:"user"`
	DeclarativeResources DeclarativeResources   `yaml:"declarative_resources" json:"declarative_resources"`
	Resource             ResourceConfig         `yaml:"resource" json:"resource"`
	OrganizationUnit     OrganizationUnitConfig `yaml:"organization_unit" json:"organization_unit"`
	IdentityProvider     IdentityProviderConfig `yaml:"identity_provider" json:"identity_provider"`
	Application          ApplicationConfig      `yaml:"application" json:"application"`
	EntityType           EntityTypeConfig       `yaml:"user_type" json:"user_type"`
	Observability        ObservabilityConfig    `yaml:"observability" json:"observability"`
	Passkey              PasskeyConfig          `yaml:"passkey" json:"passkey"`
	AuthnProvider        AuthnProviderConfig    `yaml:"authn_provider" json:"authn_provider"`
	UserProvider         UserProviderConfig     `yaml:"user_provider" json:"user_provider"`
	EntityProvider       EntityProviderConfig   `yaml:"entity_provider" json:"entity_provider"`
	Role                 RoleConfig             `yaml:"role" json:"role"`
	Theme                ThemeConfig            `yaml:"theme" json:"theme"`
	Layout               LayoutConfig           `yaml:"layout" json:"layout"`
	Translation          TranslationConfig      `yaml:"translation" json:"translation"`
	Email                EmailConfig            `yaml:"email" json:"email"`
	Consent              ConsentConfig          `yaml:"consent" json:"consent"`
}

// LoadConfig loads the configurations from the specified YAML file and applies defaults.
func LoadConfig(configPath string, defaultPath string, serverHome string) (*Config, error) {
	var cfg Config

	// Load default configuration if provided
	if defaultPath != "" {
		defaultCfg, err := loadDefaultConfig(defaultPath, serverHome)
		if err != nil {
			return nil, err
		}
		cfg = *defaultCfg
	}

	// Load user configuration
	var userCfg Config
	userCfg, err := loadUserConfig(configPath, serverHome)
	if err != nil {
		return nil, err
	}

	// Merge user configuration with defaults
	mergeConfigs(&cfg, &userCfg)
	// Derive login_path and error_path from path if not explicitly set
	if cfg.GateClient.Path != "" {
		if cfg.GateClient.LoginPath == "" {
			cfg.GateClient.LoginPath = urlpath.Join(cfg.GateClient.Path, "signin")
		}
		if cfg.GateClient.ErrorPath == "" {
			cfg.GateClient.ErrorPath = urlpath.Join(cfg.GateClient.Path, "error")
		}
	}

	// Derive JWT issuer from server config if not set
	if cfg.JWT.Issuer == "" {
		cfg.JWT.Issuer = GetServerURL(&cfg.Server)
	}

	// Default system resource server identifier to "system" if not set.
	if cfg.Resource.SystemResourceServer.Identifier == "" {
		cfg.Resource.SystemResourceServer.Identifier = "system"
	}

	if err := cfg.Server.SecurityConfig.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.CORS.Validate(); err != nil {
		return nil, err
	}

	// Validate ACR-AMR mapping.
	if err := cfg.OAuth.AuthClass.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// loadDefaultConfig loads the default configuration from a JSON file.
func loadDefaultConfig(path string, serverHome string) (*Config, error) {
	var cfg Config
	configPath := filepath.Clean(path)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	data, err = utils.SubstituteFilePaths(data, serverHome)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func loadUserConfig(path string, serverHome string) (Config, error) {
	var cfg Config
	configPath := filepath.Clean(path)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}
	data, err = utils.SubstituteEnvironmentVariables(data)
	if err != nil {
		return Config{}, err
	}
	data, err = utils.SubstituteFilePaths(data, serverHome)
	if err != nil {
		return Config{}, err
	}

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
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

// mergeConfigs merges user configuration into the base configuration.
// Non-zero values from userCfg will override corresponding values in baseCfg.
func mergeConfigs(baseCfg, userCfg *Config) {
	mergeStructs(reflect.ValueOf(baseCfg).Elem(), reflect.ValueOf(userCfg).Elem())
}

// mergeStructs recursively merges struct fields.
func mergeStructs(base, user reflect.Value) {
	if !base.IsValid() || !user.IsValid() {
		return
	}

	switch base.Kind() {
	case reflect.Struct:
		for i := 0; i < base.NumField(); i++ {
			baseField := base.Field(i)
			userField := user.Field(i)
			if baseField.CanSet() && userField.IsValid() {
				// For structs, we need to recursively merge even if the user struct is zero value
				// to ensure defaults are preserved
				if baseField.Kind() == reflect.Struct && userField.Kind() == reflect.Struct {
					mergeStructs(baseField, userField)
				} else {
					// For non-struct fields, only override if user value is non-zero
					if !isZeroValue(userField) {
						baseField.Set(userField)
					}
				}
			}
		}
	case reflect.Slice:
		// For slices, if user has values, use them. Otherwise keep base values
		if user.Len() > 0 {
			base.Set(user)
		}
	case reflect.Map:
		// For maps, merge key-value pairs
		if !user.IsNil() && user.Len() > 0 {
			if base.IsNil() {
				base.Set(reflect.MakeMap(base.Type()))
			}
			for _, key := range user.MapKeys() {
				base.SetMapIndex(key, user.MapIndex(key))
			}
		}
	default:
		// For primitive types, use user value if it's not zero value
		if !isZeroValue(user) {
			base.Set(user)
		}
	}
}

// isZeroValue checks if a reflect.Value represents the zero value for its type.
func isZeroValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Map, reflect.Chan:
		return v.IsNil() || v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}
