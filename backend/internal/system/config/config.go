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
	"net"
	"net/url"
	"os"
	urlpath "path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
	"github.com/thunder-id/thunderid/internal/system/utils"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	yaml "gopkg.in/yaml.v3"
)

// TLSConfig holds the TLS configuration details.
type TLSConfig struct {
	MinVersion string `yaml:"min_version" json:"min_version"`
	CertFile   string `yaml:"cert_file"   json:"cert_file"`
	KeyFile    string `yaml:"key_file"    json:"key_file"`
}

// DataSource holds the individual database connection details.
// Type is the only common field; connection parameters live under the
// matching sub-struct (Postgres, SQLite, or Redis).
type DataSource struct {
	Type     string             `yaml:"type"     json:"type"`
	Postgres PostgresDataSource `yaml:"postgres" json:"postgres"`
	SQLite   SQLiteDataSource   `yaml:"sqlite"   json:"sqlite"`
	Redis    RedisDataSource    `yaml:"redis"    json:"redis"`
}

// PostgresDataSource holds PostgreSQL-specific connection details.
type PostgresDataSource struct {
	Hostname          string `yaml:"hostname"             json:"hostname"`
	Port              int    `yaml:"port"                 json:"port"`
	Name              string `yaml:"name"                 json:"name"`
	Username          string `yaml:"username"             json:"username"`
	Password          string `yaml:"password"             json:"password"`
	SSLMode           string `yaml:"sslmode"              json:"sslmode"`
	MaxOpenConns      int    `yaml:"max_open_conns"       json:"max_open_conns"`
	MaxIdleConns      int    `yaml:"max_idle_conns"       json:"max_idle_conns"`
	ConnMaxLifetime   int    `yaml:"conn_max_lifetime"    json:"conn_max_lifetime"`
	MaxRetries        int    `yaml:"max_retries"          json:"max_retries"`
	MinRetryBackoffMS int    `yaml:"min_retry_backoff_ms" json:"min_retry_backoff_ms"`
	MaxRetryBackoffMS int    `yaml:"max_retry_backoff_ms" json:"max_retry_backoff_ms"`
}

// SQLiteDataSource holds SQLite-specific connection details.
type SQLiteDataSource struct {
	Path              string `yaml:"path"                 json:"path"`
	Options           string `yaml:"options"              json:"options"`
	MaxOpenConns      int    `yaml:"max_open_conns"       json:"max_open_conns"`
	MaxIdleConns      int    `yaml:"max_idle_conns"       json:"max_idle_conns"`
	ConnMaxLifetime   int    `yaml:"conn_max_lifetime"    json:"conn_max_lifetime"`
	MaxRetries        int    `yaml:"max_retries"          json:"max_retries"`
	MinRetryBackoffMS int    `yaml:"min_retry_backoff_ms" json:"min_retry_backoff_ms"`
	MaxRetryBackoffMS int    `yaml:"max_retry_backoff_ms" json:"max_retry_backoff_ms"`
}

// RedisDataSource holds Redis-specific connection details.
type RedisDataSource struct {
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

// DatabaseConfig holds the different database configuration details.
type DatabaseConfig struct {
	Config            DataSource `yaml:"config"             json:"config"`
	RuntimeTransient  DataSource `yaml:"runtime_transient"  json:"runtime_transient"`
	Entity            DataSource `yaml:"entity"             json:"entity"`
	RuntimePersistent DataSource `yaml:"runtime_persistent" json:"runtime_persistent"`
}

// NotificationConfig holds the notification configuration details.
type NotificationConfig struct {
	OTP OTPConfig `yaml:"otp" json:"otp"`
}

// Validate checks the notification configuration for correctness.
func (c *NotificationConfig) Validate() error {
	return c.OTP.Validate()
}

// OTPConfig holds the OTP generation configuration details.
type OTPConfig struct {
	Length                int  `yaml:"length"                  json:"length"`
	UseNumericOnly        bool `yaml:"use_numeric_only"        json:"use_numeric_only"`
	ValidityPeriodSeconds int  `yaml:"validity_period_seconds" json:"validity_period_seconds"`
}

// Validate ensures OTP configuration values are within accepted bounds.
func (c *OTPConfig) Validate() error {
	if c.Length < 4 || c.Length > 10 {
		return fmt.Errorf("notification.otp.length must be in [4, 10] (got %d)", c.Length)
	}
	if c.ValidityPeriodSeconds < 30 || c.ValidityPeriodSeconds > 600 {
		return fmt.Errorf("notification.otp.validity_period_seconds must be in [30, 600] (got %d)",
			c.ValidityPeriodSeconds)
	}
	return nil
}

// CryptoConfig holds the cryptographic configuration details.
type CryptoConfig struct {
	Encryption      engineconfig.EncryptionConfig `yaml:"encryption"       json:"encryption"`
	PasswordHashing PasswordHashingConfig         `yaml:"password_hashing" json:"password_hashing"`
	Keys            []engineconfig.KeyConfig      `yaml:"keys"             json:"keys"`
}

// PasswordHashingConfig holds the password hashing configuration details.
type PasswordHashingConfig struct {
	Algorithm string         `yaml:"algorithm" json:"algorithm"`
	Argon2ID  Argon2IDConfig `yaml:"argon2id"  json:"argon2id"`
	PBKDF2    PBKDF2Config   `yaml:"pbkdf2"    json:"pbkdf2"`
	SHA256    SHA256Config   `yaml:"sha256"    json:"sha256"`
}

// Argon2IDConfig holds the Argon2id password hashing configuration details.
type Argon2IDConfig struct {
	Iterations  int `yaml:"iterations"  json:"iterations"`
	Memory      int `yaml:"memory"      json:"memory"`
	Parallelism int `yaml:"parallelism" json:"parallelism"`
	KeySize     int `yaml:"key_size"    json:"key_size"`
	SaltSize    int `yaml:"salt_size"   json:"salt_size"`
}

// PBKDF2Config holds the PBKDF2 password hashing configuration details.
type PBKDF2Config struct {
	Iterations int `yaml:"iterations" json:"iterations"`
	KeySize    int `yaml:"key_size"   json:"key_size"`
	SaltSize   int `yaml:"salt_size"  json:"salt_size"`
}

// SHA256Config holds the SHA256 password hashing configuration details.
type SHA256Config struct {
	SaltSize int `yaml:"salt_size" json:"salt_size"`
}

// UserConfig holds the user management configuration details.
type UserConfig struct {
	IndexedAttributes []string `yaml:"indexed_attributes" json:"indexed_attributes"`
	// Store defines the storage mode for users.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store"              json:"store"`
}

// PasskeyConfig holds the passkey configuration details.
type PasskeyConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
}

// AttestationConfig holds engine-level platform attestation configuration shared across
// applications.
type AttestationConfig struct {
	Apple AppleAttestationConfig `yaml:"apple" json:"apple"`
}

// AppleAttestationConfig holds the engine-level Apple App Attest settings. RootCertificate is the
// PEM-encoded Apple "App Attestation Root CA" certificate used as the trust anchor when verifying
// attestation certificate chains.
type AppleAttestationConfig struct {
	RootCertificate string `yaml:"root_certificate" json:"root_certificate"`
}

// OpenID4VPConfig holds the OpenID4VP verifier engine configuration. Engine
// defaults (client_id_scheme, signing key, base URLs, response advertisement, trust
// anchors) live at the top level; presentation definitions are managed at runtime
// via the management API and stored in configdb, not in static configuration.
type OpenID4VPConfig struct {
	// Store defines the storage mode for presentation definitions.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "declarative"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
	// ClientIDScheme selects how the verifier's client_id is determined.
	// Supported values: "x509_hash" (SHA-256 thumbprint of signing cert leaf),
	// "x509_san_dns" (first DNS SAN of signing cert leaf), "redirect_uri" (response URI).
	ClientIDScheme             string               `yaml:"client_id_scheme" json:"client_id_scheme"`
	SigningKeyID               string               `yaml:"signing_key_id" json:"signing_key_id"`
	ResultRedirectURI          string               `yaml:"result_redirect_uri" json:"result_redirect_uri"`
	RequestAudience            string               `yaml:"request_audience" json:"request_audience"`
	EphemeralKeyID             string               `yaml:"ephemeral_key_id" json:"ephemeral_key_id"`
	ResponseEncValues          []string             `yaml:"response_enc_values" json:"response_enc_values"`
	RequestValiditySeconds     int                  `yaml:"request_validity_seconds" json:"request_validity_seconds"`
	StateTTLSeconds            int                  `yaml:"state_ttl_seconds" json:"state_ttl_seconds"`
	LeewaySeconds              int                  `yaml:"leeway_seconds" json:"leeway_seconds"`
	KeyBindingMaxAgeSeconds    int                  `yaml:"key_binding_max_age_seconds" json:"key_binding_max_age_seconds"`     //nolint:lll
	ResultTokenValiditySeconds int                  `yaml:"result_token_validity_seconds" json:"result_token_validity_seconds"` //nolint:lll
	RegistrationCertFile       string               `yaml:"registration_cert_file" json:"registration_cert_file"`
	TrustedAnchors             []TrustedAnchorEntry `yaml:"trusted_anchors" json:"trusted_anchors"` //nolint:lll
	EnforceKeyBinding          bool                 `yaml:"enforce_key_binding" json:"enforce_key_binding"`
}

// TrustedAnchorEntry is a trust anchor (root CA) whose PEM certificate roots the
// X.509 chains presented by credential issuers (via the x5c header). Trust
// anchors are configured once at the OpenID4VP engine level and shared by every
// presentation definition.
type TrustedAnchorEntry struct {
	Name     string `yaml:"name" json:"name"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
}

// OpenID4VCIConfig holds the OpenID4VCI credential issuer engine configuration.
// Engine defaults (issuer identifier, signing key, base URL, authorization
// servers) live here; credential configurations are managed via the management
// API and stored in configdb.
type OpenID4VCIConfig struct {
	CredentialIssuer          string   `yaml:"credential_issuer" json:"credential_issuer"`
	BaseURL                   string   `yaml:"base_url" json:"base_url"`
	SigningKeyID              string   `yaml:"signing_key_id" json:"signing_key_id"`
	AuthorizationServers      []string `yaml:"authorization_servers" json:"authorization_servers"`
	NonceTTLSeconds           int      `yaml:"nonce_ttl_seconds" json:"nonce_ttl_seconds"`
	ProofMaxAgeSeconds        int      `yaml:"proof_max_age_seconds" json:"proof_max_age_seconds"`
	CredentialValiditySeconds int      `yaml:"credential_validity_seconds" json:"credential_validity_seconds"` //nolint:lll
	BatchSize                 int      `yaml:"batch_size" json:"batch_size"`
	EnforceScope              bool     `yaml:"enforce_scope" json:"enforce_scope"`
	// Store defines the storage mode for credential configurations.
	// One of: "mutable", "declarative", "composite". Empty inherits the global
	// declarative_resources setting.
	Store string `yaml:"store" json:"store"`
}

// AuthnProviderConfig holds the authentication provider configuration details.
type AuthnProviderConfig struct {
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
	Enabled bool `yaml:"enabled" json:"enabled"`
	// CredentialTypes lists the credential keys routed to the REST provider.
	CredentialTypes     []string           `yaml:"credential_types" json:"credential_types"`
	BaseURL             string             `yaml:"base_url" json:"base_url"`
	Timeout             int                `yaml:"timeout" json:"timeout"`
	CorrelationIDHeader string             `yaml:"correlation_id_header" json:"correlation_id_header"`
	Security            RestSecurityConfig `yaml:"security" json:"security"`
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
	Host                 string `yaml:"host"                  json:"host"`
	Port                 int    `yaml:"port"                  json:"port"`
	Username             string `yaml:"username"              json:"username"`
	Password             string `yaml:"password"              json:"password"`
	FromAddress          string `yaml:"from_address"          json:"from_address"`
	EnableStartTLS       *bool  `yaml:"enable_start_tls"      json:"enable_start_tls"`
	EnableAuthentication *bool  `yaml:"enable_authentication" json:"enable_authentication"`
}

// DeclarativeResources holds the configuration details for the declarative resources.
type DeclarativeResources struct {
	Enabled bool `yaml:"enabled" json:"enabled" default:"false"`
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

	// GoogleBaseURL overrides the scheme+host of Google's OAuth/OIDC endpoints (path preserved).
	// Empty (the default) means the real Google endpoints are used. Intended for test
	// environments that redirect the flow to a local mock server; leave empty in production.
	GoogleBaseURL string `yaml:"google_base_url" json:"google_base_url"`

	// GitHubBaseURL overrides the scheme+host of GitHub's OAuth endpoints (path preserved).
	// Empty (the default) means the real GitHub endpoints are used. Intended for test
	// environments that redirect the flow to a local mock server; leave empty in production.
	GitHubBaseURL string `yaml:"github_base_url" json:"github_base_url"`
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

// ServerConfigConfig holds the server configuration store settings.
type ServerConfigConfig struct {
	// Store defines the storage mode for server config sections.
	// Valid values: "mutable", "declarative", "composite" (hybrid mode)
	// If not specified, falls back to global DeclarativeResources.Enabled setting:
	//   - If DeclarativeResources.Enabled = true: behaves as "composite"
	//   - If DeclarativeResources.Enabled = false: behaves as "mutable"
	Store string `yaml:"store" json:"store"`
}

// AgentConfig holds the agent service configuration.
type AgentConfig struct {
	// Store defines the storage mode for agents.
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

// GroupConfig holds the group service configuration.
type GroupConfig struct {
	// Store defines the storage mode for groups.
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

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string          `yaml:"level"  json:"level"`
	Output LogOutputConfig `yaml:"output" json:"output"`
	Access LogAccessConfig `yaml:"access" json:"access"`
}

// LogAccessConfig holds the access log settings.
type LogAccessConfig struct {
	// ExcludePaths lists extra path prefixes whose requests are served without an access log line.
	// The Gate and Console frontend prefixes are always excluded in addition to these.
	ExcludePaths []string `yaml:"exclude_paths" json:"exclude_paths"`
}

// LogOutputConfig holds the log output destinations.
type LogOutputConfig struct {
	Console LogConsoleConfig `yaml:"console" json:"console"`
	File    LogFileConfig    `yaml:"file"    json:"file"`
}

// LogConsoleConfig holds the console (stdout) output settings.
//
// Toggle and value fields in the log configuration use pointers so that a value
// present in deployment.yaml overrides the default.json default even when it is
// the zero value (for example console output explicitly disabled with `false`).
// A nil pointer means "not set", in which case the merged default is kept.
type LogConsoleConfig struct {
	Enabled *bool `yaml:"enabled" json:"enabled"`
}

// BuildOutputOptions resolves the log configuration into the logger's output
// options: it resolves a relative file path under serverHome and applies the
// default rotation values when a trigger is enabled without an explicit value.
// File paths are only resolved when file output is enabled, so a disabled file
// output performs no path work and never requires a writable filesystem.
func (c LogConfig) BuildOutputOptions(serverHome string) log.OutputOptions {
	fileCfg := c.Output.File
	fileEnabled := derefBool(fileCfg.Enabled)

	filePath := ""
	if fileEnabled {
		dir := fileCfg.Path
		if dir == "" {
			dir = "logs"
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(serverHome, dir)
		}
		name := fileCfg.FileName
		if name == "" {
			name = "thunderid.log"
		}
		filePath = filepath.Join(dir, name)
	}

	var maxSizeMB float64
	if derefBool(fileCfg.Rotation.Size.Enabled) {
		maxSizeMB = derefFloat(fileCfg.Rotation.Size.MaxSizeMB, rollingfile.DefaultMaxSizeMB)
		if maxSizeMB <= 0 {
			maxSizeMB = rollingfile.DefaultMaxSizeMB
		}
	}

	intervalDays := 0
	if derefBool(fileCfg.Rotation.Time.Enabled) {
		intervalDays = derefInt(fileCfg.Rotation.Time.IntervalDays, rollingfile.DefaultIntervalDays)
		if intervalDays <= 0 {
			intervalDays = rollingfile.DefaultIntervalDays
		}
	}

	return log.OutputOptions{
		ConsoleEnabled: derefBool(c.Output.Console.Enabled),
		FileEnabled:    fileEnabled,
		Format:         fileCfg.Format,
		File: rollingfile.Config{
			Path:         filePath,
			MaxSizeMB:    maxSizeMB,
			IntervalDays: intervalDays,
			MaxBackups:   derefInt(fileCfg.Rotation.MaxBackups, 0),
			MaxAgeDays:   derefInt(fileCfg.Rotation.MaxAgeDays, 0),
			Compress:     derefBool(fileCfg.Rotation.Compress),
		},
	}
}

// derefBool returns *p when p is non-nil, otherwise false (the disabled default
// for every log toggle; an explicit default lives in default.json).
func derefBool(p *bool) bool {
	return p != nil && *p
}

// derefInt returns *p when p is non-nil, otherwise def.
func derefInt(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

// derefFloat returns *p when p is non-nil, otherwise def.
func derefFloat(p *float64, def float64) float64 {
	if p != nil {
		return *p
	}
	return def
}

// LogFileConfig holds the file output settings.
type LogFileConfig struct {
	Enabled  *bool             `yaml:"enabled"   json:"enabled"`
	Path     string            `yaml:"path"      json:"path"`
	FileName string            `yaml:"file_name" json:"file_name"`
	Format   string            `yaml:"format"    json:"format"`
	Rotation LogRotationConfig `yaml:"rotation"  json:"rotation"`
}

// LogRotationConfig holds the file rotation and retention settings.
type LogRotationConfig struct {
	Size       LogSizeRotationConfig `yaml:"size"         json:"size"`
	Time       LogTimeRotationConfig `yaml:"time"         json:"time"`
	MaxBackups *int                  `yaml:"max_backups"  json:"max_backups"`
	MaxAgeDays *int                  `yaml:"max_age_days" json:"max_age_days"`
	Compress   *bool                 `yaml:"compress"     json:"compress"`
}

// LogSizeRotationConfig holds the size-based rotation trigger settings.
type LogSizeRotationConfig struct {
	Enabled   *bool    `yaml:"enabled"     json:"enabled"`
	MaxSizeMB *float64 `yaml:"max_size_mb" json:"max_size_mb"`
}

// LogTimeRotationConfig holds the time-based rotation trigger settings.
type LogTimeRotationConfig struct {
	Enabled      *bool `yaml:"enabled"       json:"enabled"`
	IntervalDays *int  `yaml:"interval_days" json:"interval_days"`
}

// Config holds the complete configuration details of the server.
type Config struct {
	Server               engineconfig.ServerConfig        `yaml:"server"                json:"server"`
	Log                  LogConfig                        `yaml:"log"                   json:"log"`
	GateClient           engineconfig.GateClientConfig    `yaml:"gate_client"           json:"gate_client"`
	TLS                  TLSConfig                        `yaml:"tls"                   json:"tls"`
	Database             DatabaseConfig                   `yaml:"database"              json:"database"`
	Cache                engineconfig.CacheConfig         `yaml:"cache"                 json:"cache"`
	JWT                  engineconfig.JWTConfig           `yaml:"jwt"                   json:"jwt"`
	OAuth                engineconfig.OAuthConfig         `yaml:"oauth"                 json:"oauth"`
	Flow                 engineconfig.FlowConfig          `yaml:"flow"                  json:"flow"`
	Crypto               CryptoConfig                     `yaml:"crypto"                json:"crypto"`
	User                 UserConfig                       `yaml:"user"                  json:"user"`
	DeclarativeResources DeclarativeResources             `yaml:"declarative_resources" json:"declarative_resources"`
	Resource             engineconfig.ResourceConfig      `yaml:"resource"              json:"resource"`
	OrganizationUnit     OrganizationUnitConfig           `yaml:"organization_unit"     json:"organization_unit"`
	IdentityProvider     IdentityProviderConfig           `yaml:"identity_provider"     json:"identity_provider"`
	Application          ApplicationConfig                `yaml:"application"           json:"application"`
	ServerConfig         ServerConfigConfig               `yaml:"server_config" json:"server_config"`
	Agent                AgentConfig                      `yaml:"agent"                 json:"agent"`
	EntityType           EntityTypeConfig                 `yaml:"user_type"             json:"user_type"`
	Observability        engineconfig.ObservabilityConfig `yaml:"observability"         json:"observability"`
	Passkey              PasskeyConfig                    `yaml:"passkey"               json:"passkey"`
	Attestation          AttestationConfig                `yaml:"attestation"           json:"attestation"`
	OpenID4VP            OpenID4VPConfig                  `yaml:"openid4vp"             json:"openid4vp"`
	OpenID4VCI           OpenID4VCIConfig                 `yaml:"openid4vci"            json:"openid4vci"`
	AuthnProvider        AuthnProviderConfig              `yaml:"authn_provider"        json:"authn_provider"`
	UserProvider         UserProviderConfig               `yaml:"user_provider"         json:"user_provider"`
	EntityProvider       EntityProviderConfig             `yaml:"entity_provider"       json:"entity_provider"`
	Group                GroupConfig                      `yaml:"group"                 json:"group"`
	Role                 RoleConfig                       `yaml:"role"                  json:"role"`
	Theme                ThemeConfig                      `yaml:"theme"                 json:"theme"`
	Layout               LayoutConfig                     `yaml:"layout"                json:"layout"`
	Translation          TranslationConfig                `yaml:"translation"           json:"translation"`
	Email                EmailConfig                      `yaml:"email"                 json:"email"`
	Notification         NotificationConfig               `yaml:"notification"          json:"notification"`
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

	// Default gate_client to the server's own URL when not explicitly configured, so the gate only
	// needs configuring when it is hosted separately from the server.
	if cfg.GateClient.Hostname == "" || cfg.GateClient.Port == 0 || cfg.GateClient.Scheme == "" {
		serverURL, err := url.Parse(engineconfig.GetServerURL(&cfg.Server))
		if err != nil {
			return nil, fmt.Errorf("failed to parse server URL for gate_client derivation: %w", err)
		}
		if cfg.GateClient.Scheme == "" {
			cfg.GateClient.Scheme = serverURL.Scheme
		}
		if cfg.GateClient.Hostname == "" {
			cfg.GateClient.Hostname = serverURL.Hostname()
		}
		if cfg.GateClient.Port == 0 {
			if portStr := serverURL.Port(); portStr != "" {
				if port, perr := strconv.Atoi(portStr); perr == nil {
					cfg.GateClient.Port = port
				}
			} else if serverURL.Scheme == "http" {
				cfg.GateClient.Port = 80
			} else {
				cfg.GateClient.Port = 443
			}
		}
	}

	// The resolved gate client host must be reachable by a browser. A bind-all address
	// (0.0.0.0 or ::) produces broken login/error redirects. This happens when server.hostname
	// is a bind address and neither server.public_url nor gate_client.hostname is configured,
	// so fail fast with actionable guidance.
	if isBindAllHost(cfg.GateClient.Hostname) {
		return nil, fmt.Errorf("gate client hostname resolved to an unreachable bind-all address %q; "+
			"set server.public_url (or gate_client.hostname) to a browser-reachable host",
			cfg.GateClient.Hostname)
	}

	// Derive login_path and error_path from path if not explicitly set
	if cfg.GateClient.Path != "" {
		if cfg.GateClient.LoginPath == "" {
			cfg.GateClient.LoginPath = urlpath.Join(cfg.GateClient.Path, "signin")
		}
		if cfg.GateClient.SignOutPath == "" {
			cfg.GateClient.SignOutPath = urlpath.Join(cfg.GateClient.Path, "signout")
		}
		if cfg.GateClient.ErrorPath == "" {
			cfg.GateClient.ErrorPath = urlpath.Join(cfg.GateClient.Path, "error")
		}
		if cfg.GateClient.CallbackPath == "" {
			cfg.GateClient.CallbackPath = urlpath.Join(cfg.GateClient.Path, "callback")
		}
	}

	// Derive JWT issuer from server config if not set
	if cfg.JWT.Issuer == "" {
		cfg.JWT.Issuer = engineconfig.GetServerURL(&cfg.Server)
	}

	if err := cfg.Server.SecurityConfig.Validate(); err != nil {
		return nil, err
	}

	// Validate ACR-AMR mapping.
	if err := cfg.OAuth.AuthClass.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.OAuth.DPoP.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.OAuth.TokenExchange.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Notification.Validate(); err != nil {
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
func GetServerURL(server *engineconfig.ServerConfig) string {
	return engineconfig.GetServerURL(server)
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

// isBindAllHost reports whether host is an unspecified/bind-all IP address (0.0.0.0 or ::).
// Such an address is valid to bind a listener to but can never be reached by a browser, so it
// is invalid as a redirect target. Empty hosts and hostnames such as "localhost" are not
// treated as bind-all.
func isBindAllHost(host string) bool {
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsUnspecified()
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
