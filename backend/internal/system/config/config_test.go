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

package config

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type ConfigTestSuite struct {
	suite.Suite
	originalEnvVars map[string]string
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (suite *ConfigTestSuite) SetupTest() {
	// Store original environment variables
	suite.originalEnvVars = make(map[string]string)
}

func (suite *ConfigTestSuite) TearDownTest() {
	// Restore original environment variables
	for key, value := range suite.originalEnvVars {
		if value == "" {
			err := os.Unsetenv(key)
			suite.Require().NoError(err, "Failed to unset environment variable")
		} else {
			err := os.Setenv(key, value)
			suite.Require().NoError(err, "Failed to set environment variable")
		}
	}
}

// Helper function to set environment variable and track for cleanup
func (suite *ConfigTestSuite) setEnvVar(key, value string) {
	if _, exists := suite.originalEnvVars[key]; !exists {
		if originalValue, hasOriginal := os.LookupEnv(key); hasOriginal {
			suite.originalEnvVars[key] = originalValue
		} else {
			suite.originalEnvVars[key] = ""
		}
	}
	err := os.Setenv(key, value)
	suite.Require().NoError(err, "Failed to set environment variable")
}

func (suite *ConfigTestSuite) TestLoadConfigWithDefaults() {
	tempDir := suite.T().TempDir()

	dummyCryptoKey := "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"
	cryptoPath := suite.createTempFile(tempDir, "crypto*.key", dummyCryptoKey)
	defaultContent := fmt.Sprintf(`{
  "server": {
    "hostname": "default-host",
    "port": 8080,
    "http_only": false
  },
  "gate_client": {
    "hostname": "default-gate",
    "port": 9080,
    "scheme": "http",
    "login_path": "/default-login",
    "error_path": "/default-error"
  },
  "crypto": {
    "encryption": {
      "key": "file://%q"
    }
  },
  "jwt": {
    "issuer": "default-issuer",
    "validity_period": 7200
  },
  "oauth": {
    "refresh_token": {
      "renew_on_grant": false,
      "validity_period": 86400
    }
  },
  "notification": {
    "otp": {
      "length": 6,
      "use_numeric_only": true,
      "validity_period_seconds": 120
    }
  }
}`, cryptoPath)

	defaultPath := suite.createTempFile(tempDir, "default*.json", defaultContent)

	// Create a partial YAML user configuration file.
	userContent := `
server:
  hostname: "user-host"
  port: 8090

jwt:
  issuer: "user-issuer"
`
	userPath := suite.createTempFile(tempDir, "user*.yaml", userContent)

	// Test loading the configuration with defaults.
	config, err := LoadConfig(userPath, defaultPath, tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)

	// Validate merged configuration values.
	assert.Equal(suite.T(), "user-host", config.Server.Hostname) // User override
	assert.Equal(suite.T(), 8090, config.Server.Port)            // User override
	assert.Equal(suite.T(), false, config.Server.HTTPOnly)       // Default value
	assert.Equal(suite.T(), "default-gate", config.GateClient.Hostname)
	assert.Equal(suite.T(), 9080, config.GateClient.Port)
	assert.Equal(suite.T(), "http", config.GateClient.Scheme)
	assert.Equal(suite.T(), "/default-login", config.GateClient.LoginPath)
	assert.Equal(suite.T(), "/default-error", config.GateClient.ErrorPath)
	assert.Equal(suite.T(), "user-issuer", config.JWT.Issuer)             // User override
	assert.Equal(suite.T(), int64(7200), config.JWT.ValidityPeriod)       // Default value
	assert.Equal(suite.T(), dummyCryptoKey, config.Crypto.Encryption.Key) // Default value
}

func (suite *ConfigTestSuite) TestLoadConfigWithDefaults_NoDefaults() {
	// Create a partial YAML user configuration file.
	userContent := `
server:
  hostname: "user-host"
  port: 8090
database:
  config:
    type: "sqlite"
    sqlite:
      path: "{{.TestVar}}"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`

	tempDir := suite.T().TempDir()
	userFile := suite.createTempFile(tempDir, "user*.yaml", userContent)

	suite.setEnvVar("TestVar", "mysql")

	// Test loading the configuration without defaults (empty defaults path).
	config, err := LoadConfig(userFile, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)

	// Should behave like regular LoadConfig.
	assert.Equal(suite.T(), "user-host", config.Server.Hostname)
	assert.Equal(suite.T(), 8090, config.Server.Port)
	assert.Equal(suite.T(), false, config.Server.HTTPOnly) // Zero value for bool
	assert.Equal(suite.T(), "mysql", config.Database.Config.SQLite.Path)
}

func (suite *ConfigTestSuite) TestLoadConfigGateClientDefaultsToServer() {
	tempDir := suite.T().TempDir()

	// gate_client omitted entirely: hostname/port/scheme derive from the server config.
	userContent := `
server:
  hostname: "user-host"
  port: 8095
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile := suite.createTempFile(tempDir, "user*.yaml", userContent)

	config, err := LoadConfig(userFile, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)

	assert.Equal(suite.T(), "user-host", config.GateClient.Hostname)
	assert.Equal(suite.T(), 8095, config.GateClient.Port)
	assert.Equal(suite.T(), "https", config.GateClient.Scheme)
}

func (suite *ConfigTestSuite) TestLoadConfigGateClientDefaultsToServerPublicURL() {
	tempDir := suite.T().TempDir()

	// gate_client omitted and server exposes a public_url: gate_client derives from the public_url.
	userContent := `
server:
  hostname: "0.0.0.0"
  port: 8090
  public_url: "https://thunderid.local:9443"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile := suite.createTempFile(tempDir, "user*.yaml", userContent)

	config, err := LoadConfig(userFile, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)

	assert.Equal(suite.T(), "thunderid.local", config.GateClient.Hostname)
	assert.Equal(suite.T(), 9443, config.GateClient.Port)
	assert.Equal(suite.T(), "https", config.GateClient.Scheme)
}

func (suite *ConfigTestSuite) TestLoadConfigGateClientDefaultsToServerPublicURLWithoutPort() {
	tempDir := suite.T().TempDir()

	// public_url without an explicit port: gate_client falls back to the scheme's default port.
	userContent := `
server:
  hostname: "0.0.0.0"
  port: 8090
  public_url: "https://thunderid.local"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile := suite.createTempFile(tempDir, "user*.yaml", userContent)

	config, err := LoadConfig(userFile, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)

	assert.Equal(suite.T(), "thunderid.local", config.GateClient.Hostname)
	assert.Equal(suite.T(), 443, config.GateClient.Port)
	assert.Equal(suite.T(), "https", config.GateClient.Scheme)
}

func (suite *ConfigTestSuite) TestLoadConfigGateClientPartialOverride() {
	tempDir := suite.T().TempDir()

	// Only gate_client.path is set: host/port/scheme still inherit from the server config.
	userContent := `
server:
  hostname: "user-host"
  port: 8095
  http_only: true
gate_client:
  path: "/login"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile := suite.createTempFile(tempDir, "user*.yaml", userContent)

	config, err := LoadConfig(userFile, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)

	assert.Equal(suite.T(), "user-host", config.GateClient.Hostname)
	assert.Equal(suite.T(), 8095, config.GateClient.Port)
	assert.Equal(suite.T(), "http", config.GateClient.Scheme)
	assert.Equal(suite.T(), "/login", config.GateClient.Path)
}

func (suite *ConfigTestSuite) TestLoadConfigGateClientDefaultsToServerHTTPPublicURLWithoutPort() {
	tempDir := suite.T().TempDir()

	// http public_url without an explicit port: gate_client falls back to port 80.
	userContent := `
server:
  hostname: "0.0.0.0"
  port: 8090
  public_url: "http://thunderid.local"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile := suite.createTempFile(tempDir, "user*.yaml", userContent)

	config, err := LoadConfig(userFile, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)

	assert.Equal(suite.T(), "thunderid.local", config.GateClient.Hostname)
	assert.Equal(suite.T(), 80, config.GateClient.Port)
	assert.Equal(suite.T(), "http", config.GateClient.Scheme)
}

func (suite *ConfigTestSuite) TestLoadConfigGateClientUnspecifiedHostFails() {
	tempDir := suite.T().TempDir()

	// server binds to 0.0.0.0 with no public_url and no explicit gate_client host: the gate
	// client would resolve to the unreachable bind address, so loading must fail.
	userContent := `
server:
  hostname: "0.0.0.0"
  port: 8090
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile := suite.createTempFile(tempDir, "user*.yaml", userContent)

	config, err := LoadConfig(userFile, "", tempDir)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), config)
	assert.Contains(suite.T(), err.Error(), "unreachable")
}

func (suite *ConfigTestSuite) TestLoadConfigGateClientMalformedPublicURLFails() {
	tempDir := suite.T().TempDir()

	// gate_client derives from a malformed public_url (non-numeric port): parsing must fail loudly
	// rather than silently leaving gate_client empty.
	userContent := `
server:
  hostname: "0.0.0.0"
  port: 8090
  public_url: "https://thunderid.local:notaport"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile := suite.createTempFile(tempDir, "user*.yaml", userContent)

	config, err := LoadConfig(userFile, "", tempDir)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), config)
	assert.Contains(suite.T(), err.Error(), "failed to parse server URL")
}

func (suite *ConfigTestSuite) TestLoadConfigLogLevel() {
	tempDir := suite.T().TempDir()

	notification := `
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`

	// A deployment.yaml with a log level should parse into cfg.Log.Level.
	withLevel := suite.createTempFile(tempDir, "user*.yaml", "log:\n  level: \"debug\"\n"+notification)
	config, err := LoadConfig(withLevel, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)
	assert.Equal(suite.T(), "debug", config.Log.Level)

	// Omitting the log section leaves the level empty.
	withoutLevel := suite.createTempFile(tempDir, "user*.yaml", "server:\n  hostname: \"test\"\n"+notification)
	config, err = LoadConfig(withoutLevel, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)
	assert.Equal(suite.T(), "", config.Log.Level)
}

func (suite *ConfigTestSuite) TestLoadConfigLogOutput() {
	tempDir := suite.T().TempDir()

	notification := `
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`

	logOutput := `
log:
  level: "info"
  output:
    console:
      enabled: true
    file:
      enabled: true
      path: "logs"
      file_name: "thunderid.log"
      format: "json"
      rotation:
        size:
          enabled: true
          max_size_mb: 0.5
        time:
          enabled: true
          interval_days: 2
        max_backups: 7
        max_age_days: 14
        compress: true
`

	userFile := suite.createTempFile(tempDir, "user*.yaml", logOutput+notification)
	config, err := LoadConfig(userFile, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)

	file := config.Log.Output.File
	assert.Equal(suite.T(), boolPtr(true), config.Log.Output.Console.Enabled)
	assert.Equal(suite.T(), boolPtr(true), file.Enabled)
	assert.Equal(suite.T(), "logs", file.Path)
	assert.Equal(suite.T(), "thunderid.log", file.FileName)
	assert.Equal(suite.T(), "json", file.Format)
	assert.Equal(suite.T(), boolPtr(true), file.Rotation.Size.Enabled)
	assert.Equal(suite.T(), floatPtr(0.5), file.Rotation.Size.MaxSizeMB)
	assert.Equal(suite.T(), boolPtr(true), file.Rotation.Time.Enabled)
	assert.Equal(suite.T(), intPtr(2), file.Rotation.Time.IntervalDays)
	assert.Equal(suite.T(), intPtr(7), file.Rotation.MaxBackups)
	assert.Equal(suite.T(), intPtr(14), file.Rotation.MaxAgeDays)
	assert.Equal(suite.T(), boolPtr(true), file.Rotation.Compress)

	// Omitting the output block leaves the file toggle unset (nil), so the default
	// from default.json is preserved on merge.
	withoutOutput := suite.createTempFile(tempDir, "user*.yaml", "log:\n  level: \"info\"\n"+notification)
	config, err = LoadConfig(withoutOutput, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)
	assert.Nil(suite.T(), config.Log.Output.File.Enabled)
}

// TestLogConfigOverrideToZeroValues verifies the presence-based (pointer) merge:
// a deployment.yaml value overrides a default.json default even when it is the
// zero value (false / 0), while an omitted (nil) value keeps the default.
func (suite *ConfigTestSuite) TestLogConfigOverrideToZeroValues() {
	base := &Config{}
	base.Log.Level = "info"
	base.Log.Output.Console.Enabled = boolPtr(true)
	base.Log.Output.File.Rotation.MaxBackups = intPtr(20)

	user := &Config{}
	user.Log.Output.Console.Enabled = boolPtr(false) // explicitly disable console
	user.Log.Output.File.Rotation.MaxBackups = intPtr(0)

	mergeConfigs(base, user)

	assert.Equal(suite.T(), "info", base.Log.Level, "untouched default preserved")
	assert.Equal(suite.T(), boolPtr(false), base.Log.Output.Console.Enabled, "false must override the true default")
	assert.Equal(suite.T(), intPtr(0), base.Log.Output.File.Rotation.MaxBackups, "0 must override the non-zero default")

	// An omitted (nil) user value keeps the base default.
	base2 := &Config{}
	base2.Log.Output.Console.Enabled = boolPtr(true)
	mergeConfigs(base2, &Config{})
	assert.Equal(suite.T(), boolPtr(true), base2.Log.Output.Console.Enabled, "nil override keeps the default")
}

func (suite *ConfigTestSuite) TestLoadConfigWithDefaults_ErrorCases() {
	tempDir := suite.T().TempDir()

	// Test with non-existent user config file.
	_, err := LoadConfig("non-existent.yaml", "", tempDir)
	assert.Error(suite.T(), err)

	// Test with non-existent defaults file.
	userFile := suite.createTempFile(tempDir, "user*.yaml", "server:\n  hostname: test")
	_, err = LoadConfig(userFile, "non-existent-defaults.json", tempDir)
	assert.Error(suite.T(), err)

	// Test with invalid JSON defaults file.
	invalidDefaultsFile := suite.createTempFile(tempDir, "invalid*.json", "invalid json")
	_, err = LoadConfig(userFile, invalidDefaultsFile, tempDir)
	assert.Error(suite.T(), err)
}

func (suite *ConfigTestSuite) TestMergeStructs() {
	// Test merging complex nested structures
	base := &Config{
		Server: engineconfig.ServerConfig{
			Hostname: "base-host",
			Port:     8080,
			HTTPOnly: false,
		},
		GateClient: engineconfig.GateClientConfig{
			Hostname:  "base-gate",
			Port:      9080,
			Scheme:    "http",
			LoginPath: "/base-login",
			ErrorPath: "/base-error",
		},
		JWT: engineconfig.JWTConfig{
			Issuer:         "base-issuer",
			ValidityPeriod: 3600,
		},
		OAuth: engineconfig.OAuthConfig{
			RefreshToken: engineconfig.RefreshTokenConfig{
				RenewOnGrant:   false,
				ValidityPeriod: 7200,
			},
		},
		Cache: engineconfig.CacheConfig{
			Disabled:        false,
			Type:            "memory",
			EvictionPolicy:  "LRU",
			CleanupInterval: 60,
			Properties: []engineconfig.CacheProperty{
				{Name: "base-cache", Size: 100, TTL: 300},
			},
		},
		Database: DatabaseConfig{
			Config: DataSource{
				Type: "postgres",
				Postgres: PostgresDataSource{
					Hostname: "base-config-host",
					Port:     5432,
				},
			},
			RuntimeTransient: DataSource{
				Type: "postgres",
				Postgres: PostgresDataSource{
					Hostname: "base-runtime-host",
					Port:     5432,
				},
			},
		},
	}

	user := &Config{
		Server: engineconfig.ServerConfig{
			Hostname: "user-host", // Override
			Port:     8090,        // Override
			// HTTPOnly: false (zero value, should not override)
		},
		GateClient: engineconfig.GateClientConfig{
			Hostname: "user-gate", // Override
			// Other fields are zero values, should not override
		},
		JWT: engineconfig.JWTConfig{
			Issuer: "user-issuer", // Override
			// ValidityPeriod: 0 (zero value, should not override)
		},
		OAuth: engineconfig.OAuthConfig{
			RefreshToken: engineconfig.RefreshTokenConfig{
				RenewOnGrant: true, // Override
				// ValidityPeriod: 0 (zero value, should not override)
			},
		},
		Cache: engineconfig.CacheConfig{
			Properties: []engineconfig.CacheProperty{
				{Name: "user-cache", Size: 200, TTL: 600},
			}, // Override slice
		},
		Database: DatabaseConfig{
			Config: DataSource{
				Postgres: PostgresDataSource{
					Username: "user-config-username", // Override
				},
				// Other fields are zero values, should not override
			},
		},
	}

	// Apply merge
	mergeConfigs(base, user)

	// Validate merged results
	assert.Equal(suite.T(), "user-host", base.Server.Hostname)                   // Overridden
	assert.Equal(suite.T(), 8090, base.Server.Port)                              // Overridden
	assert.Equal(suite.T(), false, base.Server.HTTPOnly)                         // Not overridden (zero value)
	assert.Equal(suite.T(), "user-gate", base.GateClient.Hostname)               // Overridden
	assert.Equal(suite.T(), 9080, base.GateClient.Port)                          // Not overridden (zero value)
	assert.Equal(suite.T(), "http", base.GateClient.Scheme)                      // Not overridden (zero value)
	assert.Equal(suite.T(), "/base-login", base.GateClient.LoginPath)            // Not overridden (zero value)
	assert.Equal(suite.T(), "/base-error", base.GateClient.ErrorPath)            // Not overridden (zero value)
	assert.Equal(suite.T(), "user-issuer", base.JWT.Issuer)                      // Overridden
	assert.Equal(suite.T(), int64(3600), base.JWT.ValidityPeriod)                // Not overridden (zero value)
	assert.Equal(suite.T(), true, base.OAuth.RefreshToken.RenewOnGrant)          // Overridden
	assert.Equal(suite.T(), int64(7200), base.OAuth.RefreshToken.ValidityPeriod) // Not overridden (zero value)

	// Test slice override
	assert.Len(suite.T(), base.Cache.Properties, 1)
	assert.Equal(suite.T(), "user-cache", base.Cache.Properties[0].Name)
	assert.Equal(suite.T(), 200, base.Cache.Properties[0].Size)
	assert.Equal(suite.T(), 600, base.Cache.Properties[0].TTL)

	// Test nested struct field override
	assert.Equal(suite.T(), "user-config-username", base.Database.Config.Postgres.Username)
	assert.Equal(suite.T(), "postgres", base.Database.Config.Type)                      // Not overridden (zero value)
	assert.Equal(suite.T(), "base-config-host", base.Database.Config.Postgres.Hostname) // Not overridden (zero value)
}

func (suite *ConfigTestSuite) TestMergeStructs_EdgeCases() {
	// Test with invalid/nil values
	var base, user reflect.Value

	// Test with invalid values
	mergeStructs(base, user)
	assert.False(suite.T(), base.IsValid())
	assert.False(suite.T(), user.IsValid())

	// Test with direct map merging (not as struct fields)
	userMapVal := reflect.ValueOf(map[string]string{
		"key1": "user-value1", // Override
		"key3": "user-value3", // New key
	})

	// For direct map merging, create a new map and test
	testMap := make(map[string]string)
	testMap["key1"] = "base-value1"
	testMap["key2"] = "base-value2"

	baseMapReflectVal := reflect.ValueOf(&testMap).Elem()
	mergeStructs(baseMapReflectVal, userMapVal)

	// Validate direct map merging works correctly
	assert.Equal(suite.T(), "user-value1", testMap["key1"]) // Overridden
	assert.Equal(suite.T(), "base-value2", testMap["key2"]) // Preserved
	assert.Equal(suite.T(), "user-value3", testMap["key3"]) // Added

	// Test struct field behavior - maps in struct fields get replaced entirely
	type MapConfig struct {
		StringMap map[string]string
		IntMap    map[string]int
	}

	baseMap := &MapConfig{
		StringMap: map[string]string{
			"key1": "base-value1",
			"key2": "base-value2",
		},
		IntMap: map[string]int{
			"num1": 100,
		},
	}

	userMap := &MapConfig{
		StringMap: map[string]string{
			"key1": "user-value1", // Will replace entire map
			"key3": "user-value3", // New key
		},
		IntMap: map[string]int{
			"num2": 200, // Will replace entire map
		},
	}

	mergeStructs(reflect.ValueOf(baseMap).Elem(), reflect.ValueOf(userMap).Elem())

	// Validate that struct field maps are replaced entirely (current behavior)
	assert.Equal(suite.T(), "user-value1", baseMap.StringMap["key1"])
	assert.Equal(suite.T(), "user-value3", baseMap.StringMap["key3"])
	assert.Equal(suite.T(), "", baseMap.StringMap["key2"]) // Lost because entire map was replaced
	assert.Equal(suite.T(), 200, baseMap.IntMap["num2"])
	assert.Equal(suite.T(), 0, baseMap.IntMap["num1"]) // Lost because entire map was replaced

	// Test with nil map in base
	type NilMapConfig struct {
		NilMap map[string]string
	}

	baseNil := &NilMapConfig{}
	userWithMap := &NilMapConfig{
		NilMap: map[string]string{
			"key": "value",
		},
	}

	mergeStructs(reflect.ValueOf(baseNil).Elem(), reflect.ValueOf(userWithMap).Elem())
	assert.NotNil(suite.T(), baseNil.NilMap)
	assert.Equal(suite.T(), "value", baseNil.NilMap["key"])

	// Test with empty slice override
	type SliceConfig struct {
		Items []string
	}

	baseSlice := &SliceConfig{
		Items: []string{"item1", "item2"},
	}

	userSlice := &SliceConfig{
		Items: []string{}, // Empty slice should not override
	}

	mergeStructs(reflect.ValueOf(baseSlice).Elem(), reflect.ValueOf(userSlice).Elem())
	assert.Len(suite.T(), baseSlice.Items, 2) // Should preserve original
	assert.Equal(suite.T(), "item1", baseSlice.Items[0])
	assert.Equal(suite.T(), "item2", baseSlice.Items[1])

	// Test with nil user map (should not panic)
	type NilUserMapConfig struct {
		TestMap map[string]string
	}

	baseWithMap := &NilUserMapConfig{
		TestMap: map[string]string{"existing": "value"},
	}
	userWithNilMap := &NilUserMapConfig{} // TestMap is nil

	mergeStructs(reflect.ValueOf(baseWithMap).Elem(), reflect.ValueOf(userWithNilMap).Elem())
	assert.Equal(suite.T(), "value", baseWithMap.TestMap["existing"]) // Should be preserved
}

func (suite *ConfigTestSuite) TestIsZeroValue() {
	// Test bool values
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(false)))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(true)))

	// Test int values
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(int(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(int(42))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(int8(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(int8(42))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(int16(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(int16(42))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(int32(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(int32(42))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(int64(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(int64(42))))

	// Test uint values
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(uint(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(uint(42))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(uint8(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(uint8(42))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(uint16(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(uint16(42))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(uint32(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(uint32(42))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(uint64(0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(uint64(42))))

	// Test float values
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(float32(0.0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(float32(3.14))))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(float64(0.0))))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(float64(3.14))))

	// Test string values
	assert.True(suite.T(), isZeroValue(reflect.ValueOf("")))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf("hello")))

	// Test slice values
	var nilSlice []string
	var emptySlice []string = []string{}
	nonEmptySlice := []string{"item"}

	assert.True(suite.T(), isZeroValue(reflect.ValueOf(nilSlice)))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(emptySlice)))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(nonEmptySlice)))

	// Test map values
	var nilMap map[string]string
	emptyMap := make(map[string]string)
	nonEmptyMap := map[string]string{"key": "value"}

	assert.True(suite.T(), isZeroValue(reflect.ValueOf(nilMap)))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(emptyMap)))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(nonEmptyMap)))

	// Test channel values
	var nilChan chan string
	nonNilChan := make(chan string)
	defer close(nonNilChan)

	assert.True(suite.T(), isZeroValue(reflect.ValueOf(nilChan)))
	assert.True(suite.T(), isZeroValue(reflect.ValueOf(nonNilChan))) // Empty channel is zero

	// Test pointer values
	var nilPtr *string
	nonNilPtr := new(string)

	assert.True(suite.T(), isZeroValue(reflect.ValueOf(nilPtr)))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(nonNilPtr)))

	// Test interface values
	var nilInterface interface{}
	var nonNilInterface interface{} = "hello"

	assert.True(suite.T(), isZeroValue(reflect.ValueOf(nilInterface)))
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(nonNilInterface)))

	// Test invalid value
	var invalidValue reflect.Value
	assert.True(suite.T(), isZeroValue(invalidValue))

	// Test struct value (should return false for default case)
	type TestStruct struct {
		Field string
	}
	testStruct := TestStruct{}
	assert.False(suite.T(), isZeroValue(reflect.ValueOf(testStruct)))
}

func (suite *ConfigTestSuite) TestMergeStructs_PrimitiveTypes() {
	type PrimitiveConfig struct {
		StringField  string
		IntField     int
		BoolField    bool
		Float64Field float64
	}

	base := &PrimitiveConfig{
		StringField:  "base-string",
		IntField:     100,
		BoolField:    true,
		Float64Field: 3.14,
	}

	user := &PrimitiveConfig{
		StringField: "user-string", // Override
		// IntField: 0 (zero value, should not override)
		BoolField:    false, // Zero value, should not override
		Float64Field: 2.71,  // Override
	}

	mergeStructs(reflect.ValueOf(base).Elem(), reflect.ValueOf(user).Elem())

	assert.Equal(suite.T(), "user-string", base.StringField) // Overridden
	assert.Equal(suite.T(), 100, base.IntField)              // Not overridden (zero value)
	assert.Equal(suite.T(), true, base.BoolField)            // Not overridden (zero value)
	assert.Equal(suite.T(), 2.71, base.Float64Field)         // Overridden
}

func (suite *ConfigTestSuite) TestMergeStructs_SliceHandling() {
	// Test non-empty slice override
	type SliceConfig struct {
		Items []string
	}

	baseSlice := &SliceConfig{
		Items: []string{"item1", "item2"},
	}

	userSlice := &SliceConfig{
		Items: []string{"new-item1", "new-item2", "new-item3"}, // Non-empty slice should override
	}

	mergeStructs(reflect.ValueOf(baseSlice).Elem(), reflect.ValueOf(userSlice).Elem())
	assert.Len(suite.T(), baseSlice.Items, 3) // Should be overridden
	assert.Equal(suite.T(), "new-item1", baseSlice.Items[0])
	assert.Equal(suite.T(), "new-item2", baseSlice.Items[1])
	assert.Equal(suite.T(), "new-item3", baseSlice.Items[2])
}

func (suite *ConfigTestSuite) TestMergeStructs_UnsettableFields() {
	// Test scenario with unexported/unsettable fields
	type ConfigWithUnexported struct {
		ExportedField   string
		unexportedField string // This field cannot be set via reflection
	}

	base := &ConfigWithUnexported{
		ExportedField:   "base-exported",
		unexportedField: "base-unexported",
	}

	user := &ConfigWithUnexported{
		ExportedField:   "user-exported",
		unexportedField: "user-unexported",
	}

	mergeStructs(reflect.ValueOf(base).Elem(), reflect.ValueOf(user).Elem())

	// Only exported field should be merged
	assert.Equal(suite.T(), "user-exported", base.ExportedField)
	assert.Equal(suite.T(), "base-unexported", base.unexportedField) // Should remain unchanged
}

func (suite *ConfigTestSuite) TestLoadConfig_FileClosingErrors() {
	// Test file close errors - create temporary config that's valid
	// This is harder to test since we can't easily force file.Close() to fail
	// but the code path exists for error handling
	userContent := `
server:
  hostname: "test-host"
  port: 8080
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`

	tempDir := suite.T().TempDir()
	userFile := suite.createTempFile(tempDir, "test-config*.yaml", userContent)

	// Test normal loading - file closing works fine
	config, err := LoadConfig(userFile, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), config)
	assert.Equal(suite.T(), "test-host", config.Server.Hostname)
}

func (suite *ConfigTestSuite) TestLoadConfig_SecurityValidation() {
	// LoadConfig must surface validation errors from engineconfig.SecurityConfig.Validate, not just
	// from individual fields. engineconfig.SecurityConfig.Validate has two error sources — its own
	// JWKSCacheTTL check and the delegated TrustedIssuer.Validate — and both must
	// propagate out of LoadConfig. A positive case ensures a fully-valid security
	// block loads end-to-end without false positives.
	tests := []struct {
		name        string
		content     string
		expectError bool
		errSubstr   string
	}{
		{
			name: "NegativeJWKSCacheTTL",
			content: `
server:
  hostname: "test-host"
  port: 8080
  security:
    jwks_cache_ttl: -1
`,
			expectError: true,
			errSubstr:   "jwks_cache_ttl",
		},
		{
			// Trusted issuer is configured (issuer set) but jwks_url is missing —
			// TrustedIssuer.Validate returns an error. engineconfig.SecurityConfig.Validate must
			// delegate to it, and LoadConfig must surface the error.
			name: "TrustedIssuerMissingJWKSURL",
			content: `
server:
  hostname: "test-host"
  port: 8080
  security:
    trusted_issuer:
      issuer: "https://auth.example.com"
      audience: "https://thunder.example.com"
`,
			expectError: true,
			errSubstr:   "trusted_issuer.jwks_url",
		},
		{
			// Trusted issuer set with insecure http://non-localhost JWKS URL — must
			// be rejected by TrustedIssuer.Validate's HTTPS-only enforcement.
			name: "TrustedIssuerInsecureJWKSURL",
			content: `
server:
  hostname: "test-host"
  port: 8080
  security:
    trusted_issuer:
      issuer: "https://auth.example.com"
      jwks_url: "http://auth.example.com/oauth2/jwks"
      audience: "https://thunder.example.com"
`,
			expectError: true,
			errSubstr:   "https",
		},
		{
			// Fully valid security block with non-zero TTL and a properly configured
			// trusted issuer — must load without error and round-trip the values.
			name: "ValidConfig",
			content: `
server:
  hostname: "test-host"
  port: 8080
  security:
    jwks_cache_ttl: 600
    trusted_issuer:
      issuer: "https://auth.example.com"
      jwks_url: "https://auth.example.com/oauth2/jwks"
      audience: "https://thunder.example.com"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`,
			expectError: false,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			tempDir := suite.T().TempDir()
			userFile := suite.createTempFile(tempDir, "security-validation*.yaml", tc.content)

			cfg, err := LoadConfig(userFile, "", tempDir)
			if tc.expectError {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), cfg)
				if tc.errSubstr != "" {
					assert.Contains(suite.T(), err.Error(), tc.errSubstr)
				}
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), cfg)
				// Round-trip checks: the values we set must actually reach the loaded config.
				assert.Equal(suite.T(), 600, cfg.Server.SecurityConfig.JWKSCacheTTL)
				assert.Equal(suite.T(), "https://auth.example.com",
					cfg.Server.SecurityConfig.TrustedIssuer.Issuer)
				assert.Equal(suite.T(), "https://auth.example.com/oauth2/jwks",
					cfg.Server.SecurityConfig.TrustedIssuer.JWKSURL)
				assert.Equal(suite.T(), "https://thunder.example.com",
					cfg.Server.SecurityConfig.TrustedIssuer.Audience)
			}
		})
	}
}

func (suite *ConfigTestSuite) TestLoadConfig_InvalidYAML() {
	// Test YAML decode error - using a simple syntax error
	invalidYAMLContent := "invalid: yaml: content"

	tempDir := suite.T().TempDir()
	userFile := suite.createTempFile(tempDir, "invalid*.yaml", invalidYAMLContent)

	// Test loading invalid YAML should return error
	_, err := LoadConfig(userFile, "", tempDir)
	assert.Error(suite.T(), err)
}

func (suite *ConfigTestSuite) TestLoadConfig_NotificationValidation() {
	tempDir := suite.T().TempDir()
	userContent := `
server:
  hostname: "test-host"
  port: 8080
notification:
  otp:
    length: 3
    validity_period_seconds: 120
`
	userFile := suite.createTempFile(tempDir, "notification-validation*.yaml", userContent)

	cfg, err := LoadConfig(userFile, "", tempDir)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), cfg)
	assert.Contains(suite.T(), err.Error(), "notification.otp.length")
}

func (suite *ConfigTestSuite) TestLoadConfigWithDerivedIssuer() {
	tempDir := suite.T().TempDir()

	// Case 1: Issuer not set - should derive from server config
	userContent1 := `
server:
  hostname: "auth.example.com"
  port: 443
  http_only: false
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile1 := suite.createTempFile(tempDir, "user1*.yaml", userContent1)

	config1, err := LoadConfig(userFile1, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "https://auth.example.com:443", config1.JWT.Issuer)

	// Case 2: Issuer explicitly set - should use the explicit value
	userContent2 := `
server:
  hostname: "auth.example.com"
  port: 443
jwt:
  issuer: "custom-issuer"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile2 := suite.createTempFile(tempDir, "user2*.yaml", userContent2)

	config2, err := LoadConfig(userFile2, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "custom-issuer", config2.JWT.Issuer)

	// Case 3: PublicURL is set - issuer should use PublicURL
	userContent3 := `
server:
  hostname: "internal-host"
  port: 8090
  public_url: "https://auth.public.com"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile3 := suite.createTempFile(tempDir, "user3*.yaml", userContent3)

	config3, err := LoadConfig(userFile3, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "https://auth.public.com", config3.JWT.Issuer)

	// Case 4: HTTP only mode
	userContent4 := `
server:
  hostname: "localhost"
  port: 8080
  http_only: true
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile4 := suite.createTempFile(tempDir, "user4*.yaml", userContent4)

	config4, err := LoadConfig(userFile4, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "http://localhost:8080", config4.JWT.Issuer)
}

func (suite *ConfigTestSuite) TestLoadConfigWithDerivedPaths() {
	tempDir := suite.T().TempDir()

	// Case 1: Only path is set
	userContent1 := `
gate_client:
  path: "/app"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile1 := suite.createTempFile(tempDir, "user1*.yaml", userContent1)

	config1, err := LoadConfig(userFile1, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "/app", config1.GateClient.Path)
	assert.Equal(suite.T(), "/app/signin", config1.GateClient.LoginPath)
	assert.Equal(suite.T(), "/app/error", config1.GateClient.ErrorPath)

	// Case 2: Path and LoginPath are set
	userContent2 := `
gate_client:
  path: "/app"
  login_path: "/custom/login"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile2 := suite.createTempFile(tempDir, "user2*.yaml", userContent2)

	config2, err := LoadConfig(userFile2, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "/app", config2.GateClient.Path)
	assert.Equal(suite.T(), "/custom/login", config2.GateClient.LoginPath)
	assert.Equal(suite.T(), "/app/error", config2.GateClient.ErrorPath)

	// Case 3: Path and ErrorPath are set
	userContent3 := `
gate_client:
  path: "/app"
  error_path: "/custom/error"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile3 := suite.createTempFile(tempDir, "user3*.yaml", userContent3)

	config3, err := LoadConfig(userFile3, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "/app", config3.GateClient.Path)
	assert.Equal(suite.T(), "/app/signin", config3.GateClient.LoginPath)
	assert.Equal(suite.T(), "/custom/error", config3.GateClient.ErrorPath)

	// Case 4: Path, LoginPath, and ErrorPath are set
	userContent4 := `
gate_client:
  path: "/app"
  login_path: "/custom/login"
  error_path: "/custom/error"
notification:
  otp:
    length: 6
    use_numeric_only: true
    validity_period_seconds: 120
`
	userFile4 := suite.createTempFile(tempDir, "user4*.yaml", userContent4)

	config4, err := LoadConfig(userFile4, "", tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "/app", config4.GateClient.Path)
	assert.Equal(suite.T(), "/custom/login", config4.GateClient.LoginPath)
	assert.Equal(suite.T(), "/custom/error", config4.GateClient.ErrorPath)

	// Case 5: Default config with path, user overrides path
	defaultContent := `{
  "gate_client": {
    "path": "/gate"
  },
  "notification": {
    "otp": {
      "length": 6,
      "use_numeric_only": true,
      "validity_period_seconds": 120
    }
  }
}`
	defaultFile := suite.createTempFile(tempDir, "default*.json", defaultContent)

	userContent5 := `
gate_client:
  path: "/newapp"
`
	userFile5 := suite.createTempFile(tempDir, "user5*.yaml", userContent5)

	config5, err := LoadConfig(userFile5, defaultFile, tempDir)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "/newapp", config5.GateClient.Path)
	assert.Equal(suite.T(), "/newapp/signin", config5.GateClient.LoginPath)
	assert.Equal(suite.T(), "/newapp/error", config5.GateClient.ErrorPath)
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_IsConfigured() {
	assert.False(suite.T(), (&engineconfig.TrustedIssuerConfig{}).IsConfigured())
	assert.False(suite.T(), (&engineconfig.TrustedIssuerConfig{
		JWKSURL:  "https://a/jwks",
		Audience: "https://b",
	}).IsConfigured(),
		"jwks_url and audience without issuer should not activate the feature")
	assert.True(suite.T(), (&engineconfig.TrustedIssuerConfig{Issuer: "https://a"}).IsConfigured())
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_Validate_NotConfigured() {
	// Empty config — feature is off, no validation errors.
	assert.NoError(suite.T(), (&engineconfig.TrustedIssuerConfig{}).Validate())
	// jwks_url/audience set without issuer is also "not configured" and silently ignored.
	cfg := &engineconfig.TrustedIssuerConfig{
		JWKSURL:  "https://auth.example.com/jwks",
		Audience: "https://thunder.example.com",
	}
	assert.NoError(suite.T(), cfg.Validate())
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_Validate_PartiallyConfigured_MissingJWKSURL() {
	cfg := &engineconfig.TrustedIssuerConfig{
		Issuer:   "https://auth.example.com",
		Audience: "https://thunder.example.com",
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "jwks_url")
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_Validate_PartiallyConfigured_MissingAudience() {
	cfg := &engineconfig.TrustedIssuerConfig{
		Issuer:  "https://auth.example.com",
		JWKSURL: "https://auth.example.com/jwks",
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "audience")
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_Validate_HTTPS() {
	cfg := &engineconfig.TrustedIssuerConfig{
		Issuer:   "https://auth.example.com",
		JWKSURL:  "https://auth.example.com/.well-known/jwks.json",
		Audience: "https://thunder.example.com",
	}
	assert.NoError(suite.T(), cfg.Validate())
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_Validate_HTTPRejected() {
	cfg := &engineconfig.TrustedIssuerConfig{
		Issuer:   "https://auth.example.com",
		JWKSURL:  "http://auth.example.com/jwks",
		Audience: "https://thunder.example.com",
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "must use https")
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_Validate_HTTPLocalhostAllowed() {
	hosts := []string{
		"http://localhost:8090/oauth2/jwks",
		"http://127.0.0.1:8090/oauth2/jwks",
		"http://[::1]:8090/oauth2/jwks",
	}
	for _, h := range hosts {
		cfg := &engineconfig.TrustedIssuerConfig{
			Issuer:   "https://auth.example.com",
			JWKSURL:  h,
			Audience: "https://thunder.example.com",
		}
		assert.NoError(suite.T(), cfg.Validate(), "expected %s to be allowed", h)
	}
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_Validate_InvalidScheme() {
	cfg := &engineconfig.TrustedIssuerConfig{
		Issuer:   "https://auth.example.com",
		JWKSURL:  "ftp://auth.example.com/jwks",
		Audience: "https://thunder.example.com",
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "https")
}

func (suite *ConfigTestSuite) TestTrustedIssuerConfig_Validate_InvalidURL() {
	cfg := &engineconfig.TrustedIssuerConfig{
		Issuer:   "https://auth.example.com",
		JWKSURL:  "://bad-url",
		Audience: "https://thunder.example.com",
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
}

func (suite *ConfigTestSuite) TestSecurityConfig_Validate_NegativeJWKSCacheTTL() {
	cfg := &engineconfig.SecurityConfig{
		JWKSCacheTTL: -1,
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "jwks_cache_ttl")
}

func (suite *ConfigTestSuite) TestSecurityConfig_Validate_ZeroJWKSCacheTTL() {
	cfg := &engineconfig.SecurityConfig{
		JWKSCacheTTL: 0,
	}
	err := cfg.Validate()
	assert.NoError(suite.T(), err)
}

func (suite *ConfigTestSuite) TestSecurityConfig_Validate_DelegatesToTrustedIssuer() {
	// A security config with a misconfigured trusted issuer must surface that error
	// through engineconfig.SecurityConfig.Validate, since the parent is now the entry point.
	cfg := &engineconfig.SecurityConfig{
		JWKSCacheTTL: 300,
		TrustedIssuer: engineconfig.TrustedIssuerConfig{
			Issuer:   "https://auth.example.com",
			JWKSURL:  "", // missing, should fail validation
			Audience: "https://thunder.example.com",
		},
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "trusted_issuer.jwks_url")
}

func (suite *ConfigTestSuite) createTempFile(dir, pattern, content string) string {
	tempFile, err := os.CreateTemp(dir, pattern)
	suite.Require().NoError(err, "failed to create temp file")

	_, err = tempFile.WriteString(content)
	suite.Require().NoError(err, "failed to write to temp file")

	err = tempFile.Close()
	suite.Require().NoError(err, "failed to close temp file")

	return tempFile.Name()
}

func (suite *ConfigTestSuite) TestAuthClassValidate_EmptyConfig() {
	cfg := engineconfig.AuthClassConfig{}
	assert.NoError(suite.T(), cfg.Validate())
}

func (suite *ConfigTestSuite) TestAuthClassValidate_ValidMapping() {
	cfg := engineconfig.AuthClassConfig{
		Amrs: []string{"PWD", "OTP"},
		AcrAMR: map[string][]string{
			"urn:thunder:acr:password":       {"PWD"},
			"urn:thunder:acr:generated-code": {"OTP"},
			"urn:thunder:acr:multi":          {"PWD", "OTP"},
		},
	}
	assert.NoError(suite.T(), cfg.Validate())
}

func (suite *ConfigTestSuite) TestAuthClassValidate_EmptyAMRList() {
	cfg := engineconfig.AuthClassConfig{
		Amrs: []string{"PWD"},
		AcrAMR: map[string][]string{
			"urn:thunder:acr:password": {"PWD"},
			"urn:thunder:acr:empty":    {},
		},
	}
	err := cfg.Validate()
	suite.Require().Error(err)
	assert.Contains(suite.T(), err.Error(), "urn:thunder:acr:empty")
	assert.Contains(suite.T(), err.Error(), "empty AMR list")
}

func (suite *ConfigTestSuite) TestAuthClassValidate_UnknownAMRKey() {
	cfg := engineconfig.AuthClassConfig{
		Amrs: []string{"PWD"},
		AcrAMR: map[string][]string{
			"urn:thunder:acr:password": {"PWD"},
			"urn:thunder:acr:otp":      {"NonExistentAMR"},
		},
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "NonExistentAMR")
	assert.Contains(suite.T(), err.Error(), "unknown AMR key")
}

func (suite *ConfigTestSuite) TestAuthClassValidate_NoAMRSection() {
	cfg := engineconfig.AuthClassConfig{
		AcrAMR: map[string][]string{
			"urn:thunder:acr:password": {"PWD"},
		},
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unknown AMR key")
}

func (suite *ConfigTestSuite) TestAuthClassValidate_AcrAMREmptyButAMRPresent() {
	cfg := engineconfig.AuthClassConfig{
		Amrs: []string{"PWD"},
	}
	assert.NoError(suite.T(), cfg.Validate())
}

func (suite *ConfigTestSuite) TestAuthClassValidate_EmptyACRKey() {
	cfg := engineconfig.AuthClassConfig{
		Amrs: []string{"PWD"},
		AcrAMR: map[string][]string{
			"": {"PWD"},
		},
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "ACR value must not be empty")
}

func (suite *ConfigTestSuite) TestAuthClassValidate_EmptyAMREntry() {
	cfg := engineconfig.AuthClassConfig{
		Amrs: []string{"PWD", ""},
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "AMR entry must not be empty")
}

func (suite *ConfigTestSuite) TestAuthClassValidate_EmptyAMRReference() {
	cfg := engineconfig.AuthClassConfig{
		Amrs: []string{"PWD"},
		AcrAMR: map[string][]string{
			"urn:thunder:acr:password": {"PWD", ""},
		},
	}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "references an empty AMR key")
}

func (suite *ConfigTestSuite) TestFlowConfig_ExecutorsYAMLField() {
	const yamlFragment = `
flow:
  max_version_history: 3
  executors:
    - CredentialsAuthExecutor
    - InviteExecutor
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlFragment), &cfg)
	suite.Require().NoError(err)
	suite.Equal([]string{"CredentialsAuthExecutor", "InviteExecutor"}, cfg.Flow.Executors)
	suite.Equal(3, cfg.Flow.MaxVersionHistory)
}

func (suite *ConfigTestSuite) TestOTPConfig_Validate_Defaults() {
	cfg := &OTPConfig{
		Length:                6,
		UseNumericOnly:        true,
		ValidityPeriodSeconds: 120,
	}
	assert.NoError(suite.T(), cfg.Validate())
	assert.Equal(suite.T(), 3, cfg.MaxGenerationAttempts)
}

func (suite *ConfigTestSuite) TestOTPConfig_Validate_MaxGenerationAttemptsBelowMin() {
	cfg := &OTPConfig{Length: 6, ValidityPeriodSeconds: 120, MaxGenerationAttempts: -1}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "notification.otp.max_generation_attempts")
}

func (suite *ConfigTestSuite) TestOTPConfig_Validate_MaxGenerationAttemptsAboveMax() {
	cfg := &OTPConfig{Length: 6, ValidityPeriodSeconds: 120, MaxGenerationAttempts: 11}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "notification.otp.max_generation_attempts")
}

func (suite *ConfigTestSuite) TestOTPConfig_Validate_LengthBelowMin() {
	cfg := &OTPConfig{Length: 3, ValidityPeriodSeconds: 120}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "notification.otp.length")
}

func (suite *ConfigTestSuite) TestOTPConfig_Validate_LengthAboveMax() {
	cfg := &OTPConfig{Length: 11, ValidityPeriodSeconds: 120}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "notification.otp.length")
}

func (suite *ConfigTestSuite) TestOTPConfig_Validate_ValidityBelowMin() {
	cfg := &OTPConfig{Length: 6, ValidityPeriodSeconds: 29}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "notification.otp.validity_period_seconds")
}

func (suite *ConfigTestSuite) TestOTPConfig_Validate_ValidityAboveMax() {
	cfg := &OTPConfig{Length: 6, ValidityPeriodSeconds: 601}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "notification.otp.validity_period_seconds")
}

func (suite *ConfigTestSuite) TestNotificationConfig_Validate_DelegatesToOTP() {
	cfg := &NotificationConfig{OTP: OTPConfig{Length: 3, ValidityPeriodSeconds: 120}}
	err := cfg.Validate()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "notification.otp.length")
}
