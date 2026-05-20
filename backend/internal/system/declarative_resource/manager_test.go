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

// Package declarativeresource provides functionality for managing declarative resources.
package declarativeresource

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// FileBasedRuntimeManagerTestSuite contains comprehensive tests for the file-based runtime manager.
// The test suite covers:
// - Environment variable substitution
// - Configuration file reading from the declarative resources directory
// - Concurrent file processing
// - Error handling for missing directories and files
// - Edge cases like empty files, binary files, and special characters
// - Comprehensive error scenarios including missing environment variables, permission errors, and file access issues
type FileBasedRuntimeManagerTestSuite struct {
	suite.Suite
	tempDir         string
	originalEnvVars map[string]string
}

func TestFileBasedRuntimeManagerTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedRuntimeManagerTestSuite))
}

func (suite *FileBasedRuntimeManagerTestSuite) SetupSuite() {
	// Initialize minimal config for testing
	testConfig := &config.Config{
		Server: config.ServerConfig{
			Hostname: "localhost",
			Port:     8080,
		},
	}

	// Create temporary thunder home directory
	tempDir := suite.T().TempDir()
	err := config.InitializeServerRuntime(tempDir, testConfig)
	suite.Require().NoError(err, "Failed to initialize server runtime")
}

func (suite *FileBasedRuntimeManagerTestSuite) SetupTest() {
	// Create temp directory for test files
	suite.tempDir = suite.T().TempDir()

	// Store original environment variables
	suite.originalEnvVars = make(map[string]string)
}

func (suite *FileBasedRuntimeManagerTestSuite) TearDownTest() {
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
func (suite *FileBasedRuntimeManagerTestSuite) setEnvVar(key, value string) {
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

// Helper function to create test files in the declarative resources directory
func (suite *FileBasedRuntimeManagerTestSuite) createTestFile(configDir, filename, content string) string {
	serverHome := config.GetServerRuntime().ServerHome
	declarativeDir := filepath.Join(serverHome, "repository", "resources", configDir)
	err := os.MkdirAll(declarativeDir, 0750)
	suite.Require().NoError(err)

	filePath := filepath.Join(declarativeDir, filename)
	err = os.WriteFile(filePath, []byte(content), 0600)
	suite.Require().NoError(err)

	return filePath
}

// Tests for GetConfigs function - Success Cases

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_SingleFile() {
	configDir := "test-configs"
	content := "test config content"
	suite.createTestFile(configDir, "config1.json", content)

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 1)
	suite.Equal(content, string(configs[0]))
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_MultipleFiles() {
	configDir := "multi-configs"
	content1 := "config file 1"
	content2 := "config file 2"
	content3 := "config file 3"

	suite.createTestFile(configDir, "config1.json", content1)
	suite.createTestFile(configDir, "config2.yaml", content2)
	suite.createTestFile(configDir, "config3.properties", content3)

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 3)

	// Since configs are read concurrently, we need to check all contents are present
	configStrings := make([]string, len(configs))
	for i, config := range configs {
		configStrings[i] = string(config)
	}
	suite.Contains(configStrings, content1)
	suite.Contains(configStrings, content2)
	suite.Contains(configStrings, content3)
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_WithEnvironmentVariables() {
	suite.setEnvVar("ConfigValue", "substituted_value")

	configDir := "env-configs"
	content := "value: {{.ConfigValue}}"
	suite.createTestFile(configDir, "config.json", content)

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 1)
	suite.Equal("value: substituted_value", string(configs[0]))
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_EmptyDirectory() {
	configDir := "empty-configs"
	// Create empty directory
	serverHome := config.GetServerRuntime().ServerHome
	declarativeDir := filepath.Join(serverHome, "repository", "resources", configDir)
	err := os.MkdirAll(declarativeDir, 0750)
	suite.Require().NoError(err)

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 0)
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_DirectoryWithSubdirectories() {
	configDir := "mixed-configs"

	// Create a file
	suite.createTestFile(configDir, "config.json", "valid config")

	// Create a subdirectory
	serverHome := config.GetServerRuntime().ServerHome
	declarativeDir := filepath.Join(serverHome, "repository", "resources", configDir)
	subDir := filepath.Join(declarativeDir, "subdir")
	err := os.MkdirAll(subDir, 0750)
	suite.Require().NoError(err)

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 1) // Only the file, not the subdirectory
	suite.Equal("valid config", string(configs[0]))
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_LargeFiles() {
	configDir := "large-configs"

	// Create a large config content
	largeContent := make([]byte, 10000)
	for i := range largeContent {
		largeContent[i] = byte('A' + (i % 26))
	}

	suite.createTestFile(configDir, "large.json", string(largeContent))

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 1)
	suite.Equal(largeContent, configs[0])
}

// Tests for GetConfigs function - Error Cases

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_NonExistentDirectory() {
	configDir := "non-existent-directory"

	configs, err := GetConfigs(configDir)

	suite.Nil(err)
	suite.Len(configs, 0)
}

// Error Scenario Tests

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_FileWithMissingEnvironmentVariable() {
	configDir := "test-error-missing-env"

	// Create a file with a missing environment variable
	content := "database_host: {{.MissingDBHost}}\nport: 5432"
	suite.createTestFile(configDir, "config.yaml", content)

	configs, err := GetConfigs(configDir)

	suite.Error(err)
	suite.Nil(configs)
	suite.Contains(err.Error(), "errors occurred while reading configuration files")
	suite.Contains(err.Error(), "environment variable MissingDBHost is not set")
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_MultipleFilesWithEnvironmentVariableErrors() {
	configDir := "test-error-multiple-files"

	// Create multiple files with missing environment variables
	content1 := "host: {{.MissingHost}}\nport: 8080"
	suite.createTestFile(configDir, "config1.yaml", content1)

	content2 := "database: {{.MissingDB}}\nuser: admin"
	suite.createTestFile(configDir, "config2.yaml", content2)

	content3 := "valid_config: true\nstatic_value: test"
	suite.createTestFile(configDir, "config3.yaml", content3)

	configs, err := GetConfigs(configDir)

	suite.Error(err)
	suite.Nil(configs)
	suite.Contains(err.Error(), "errors occurred while reading configuration files")
	suite.Contains(err.Error(), "environment variable MissingHost is not set")
	suite.Contains(err.Error(), "environment variable MissingDB is not set")
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_MixedSuccessAndFailureFiles() {
	configDir := "test-error-mixed"

	// Create one valid file
	suite.createTestFile(configDir, "valid.yaml", "valid_config: true")

	// Create one file with missing environment variable
	suite.createTestFile(configDir, "invalid.yaml", "host: {{.MissingVar}}")

	configs, err := GetConfigs(configDir)

	// Should fail even if some files are valid
	suite.Error(err)
	suite.Nil(configs)
	suite.Contains(err.Error(), "errors occurred while reading configuration files")
	suite.Contains(err.Error(), "environment variable MissingVar is not set")
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_ReadPermissionError() {
	if runtime.GOOS == "windows" {
		suite.T().Skip("Skipping permission test on Windows")
	}

	configDir := "test-error-permissions"

	// Create a file using the helper method
	filePath := suite.createTestFile(configDir, "config.yaml", "test: value")

	// Remove read permissions
	err := os.Chmod(filePath, 0000)
	suite.NoError(err)

	// Restore permissions after test
	defer func() {
		err := os.Chmod(filePath, 0600)
		suite.Require().NoError(err, "Failed to restore file permissions")
	}()

	configs, err := GetConfigs(configDir)

	suite.Error(err)
	suite.Nil(configs)
	suite.Contains(err.Error(), "errors occurred while reading configuration files")
	suite.Contains(err.Error(), "permission denied")
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_DirectoryReadPermissionError() {
	if runtime.GOOS == "windows" {
		suite.T().Skip("Skipping permission test on Windows")
	}

	configDir := "test-error-dir-permissions"

	// Create a file first using the helper method
	suite.createTestFile(configDir, "config.yaml", "test: value")

	// Get the directory path and remove read permissions
	serverHome := config.GetServerRuntime().ServerHome
	declarativeDir := filepath.Join(serverHome, "repository", "resources", configDir)

	err := os.Chmod(declarativeDir, 0000)
	suite.NoError(err)

	// Restore permissions after test
	defer func() {
		err := os.Chmod(declarativeDir, 0750) // nolint:gosec // Restoring to original permissions
		suite.Require().NoError(err, "Failed to restore directory permissions")
	}()

	configs, err := GetConfigs(configDir)

	suite.Error(err)
	suite.Nil(configs)
	suite.Contains(err.Error(), "permission denied")
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_CorruptedFile() {
	configDir := "test-error-corrupted"

	// Create a file with invalid UTF-8 sequences using the file system directly
	serverHome := config.GetServerRuntime().ServerHome
	declarativeDir := filepath.Join(serverHome, "repository", "resources", configDir)
	err := os.MkdirAll(declarativeDir, 0750)
	suite.Require().NoError(err)

	configFile := filepath.Join(declarativeDir, "corrupted.yaml")
	corruptedData := []byte{0xff, 0xfe, 0xfd} // Invalid UTF-8
	err = os.WriteFile(configFile, corruptedData, 0600)
	suite.NoError(err)

	configs, err := GetConfigs(configDir)

	// Should still succeed as we read files as bytes, but may cause issues with env var substitution
	// depending on the content. In this case, no env vars to substitute, so should succeed
	suite.NoError(err)
	suite.Len(configs, 1)
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_FileWithComplexEnvironmentVariableErrors() {
	configDir := "test-error-complex"

	// Create a file with multiple missing variables in complex patterns
	content := `
database:
  host: {{ .DB_HOST }}
  port: {{ .DB_PORT }}
  credentials:
    username: {{ .DB_USER }}
    password: {{ .DB_PASS }}
redis:
  url: {{ .REDIS_URL }}
logging:
  level: {{ .LOG_LEVEL }}
`
	suite.createTestFile(configDir, "complex.yaml", content)

	configs, err := GetConfigs(configDir)

	suite.Error(err)
	suite.Nil(configs)
	suite.Contains(err.Error(), "errors occurred while reading configuration files")
	// The environment variable substitution fails on the first missing variable it encounters
	// The exact variable that fails first may depend on the order of regex matching, but we expect at least one
	suite.Contains(err.Error(), "environment variable")
	suite.Contains(err.Error(), "is not set")
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_ConcurrentErrorScenarios() {
	configDir := "test-error-concurrent"

	// Create multiple files with different error scenarios
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("config%d.yaml", i)
		content := fmt.Sprintf("missing_var_%d: {{ .MISSING_VAR_%d }}", i, i)
		suite.createTestFile(configDir, filename, content)
	}

	configs, err := GetConfigs(configDir)

	suite.Error(err)
	suite.Nil(configs)
	suite.Contains(err.Error(), "errors occurred while reading configuration files")

	// Should handle multiple concurrent errors properly
	for i := 0; i < 10; i++ {
		expectedError := fmt.Sprintf("environment variable MISSING_VAR_%d is not set", i)
		suite.Contains(err.Error(), expectedError)
	}
}

// Edge Case and Integration Tests

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_ConcurrentAccess() {
	configDir := "concurrent-configs"

	// Create multiple files for concurrent reading
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file%d.json", i)
		content := fmt.Sprintf("config content %d", i)
		suite.createTestFile(configDir, filename, content)
	}

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 10)
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_SpecialCharactersInContent() {
	suite.setEnvVar("SPECIAL_VAR", "special@#$%^&*()_+-=[]{}|;:,.<>?")

	configDir := "special-chars-configs"
	content := `{
  "special": "{{ .SPECIAL_VAR }}",
  "unicode": "Test με unicode 中文 🚀",
  "escaped": "This has \"quotes\" and \n newlines"
}`
	suite.createTestFile(configDir, "special.json", content)

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 1)
	suite.Contains(string(configs[0]), "special@#$%^&*()_+-=[]{}|;:,.<>?")
	suite.Contains(string(configs[0]), "Test με unicode 中文 🚀")
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_DifferentFileExtensions() {
	configDir := "extension-configs"

	// Test various file extensions
	extensions := []string{"json", "yaml", "yml", "xml", "properties", "conf", "cfg", "txt"}
	for _, ext := range extensions {
		filename := fmt.Sprintf("config.%s", ext)
		content := fmt.Sprintf("content for %s file", ext)
		suite.createTestFile(configDir, filename, content)
	}

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, len(extensions))
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_NestedVariableSubstitution() {
	suite.setEnvVar("BASE_URL", "https://api.example.com")
	suite.setEnvVar("API_VERSION", "v1")
	suite.setEnvVar("ENDPOINT", "users")

	configDir := "nested-vars-configs"
	content := `{
  "api": {
    "url": "{{ .BASE_URL }}/{{ .API_VERSION }}/{{ .ENDPOINT }}",
    "timeout": 30
  }
}`
	suite.createTestFile(configDir, "api.json", content)

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 1)
	suite.Contains(string(configs[0]), "https://api.example.com/v1/users")
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_BinaryFiles() {
	configDir := "binary-configs"

	// Create a binary file (non-text content)
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD}
	serverHome := config.GetServerRuntime().ServerHome
	declarativeDir := filepath.Join(serverHome, "repository", "resources", configDir)
	err := os.MkdirAll(declarativeDir, 0750)
	suite.Require().NoError(err)

	filePath := filepath.Join(declarativeDir, "binary.dat")
	err = os.WriteFile(filePath, binaryContent, 0600)
	suite.Require().NoError(err)

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 1)
	suite.Equal(binaryContent, configs[0])
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_EmptyFiles() {
	configDir := "empty-files-configs"

	// Create an empty file
	suite.createTestFile(configDir, "empty.json", "")

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 1)
	suite.Equal("", string(configs[0]))
}

func (suite *FileBasedRuntimeManagerTestSuite) TestGetConfigs_HiddenFiles() {
	configDir := "hidden-files-configs"

	// Create a regular file
	suite.createTestFile(configDir, "normal.json", "normal content")

	// Create a hidden file (starting with .)
	suite.createTestFile(configDir, ".hidden.json", "hidden content")

	configs, err := GetConfigs(configDir)

	suite.NoError(err)
	suite.Len(configs, 2) // Both files should be read
}
