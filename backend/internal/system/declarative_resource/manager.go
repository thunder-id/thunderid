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

package declarativeresource

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// GetConfigsFromFile reads documents for the given resourceType from a single multi-document YAML file.
// Documents are matched by a "resource_type: <type>" field.
// Environment variable substitution is applied per-document so that non-variable content (e.g.
// UI template expressions like {{ t(...) }}) in other documents does not interfere.
func GetConfigsFromFile(filePath, resourceType string) ([][]byte, error) {
	cleanPath := filepath.Clean(filePath)
	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read resources file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			// Declarative resource files are loaded at startup, outside any request.
			log.GetLogger().Warn(context.Background(), "Failed to close resources file", log.Error(cerr))
		}
	}()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read resources file: %w", err)
	}

	docs := splitYAMLDocuments(fileContent)
	results := make([][]byte, 0, len(docs))
	for _, doc := range docs {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		if !documentMatchesResourceType(doc, resourceType) {
			continue
		}
		processed, err := utils.SubstituteEnvironmentVariables(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to substitute environment variables in resources file: %w", err)
		}
		results = append(results, processed)
	}
	return results, nil
}

// splitYAMLDocuments splits a multi-document YAML byte slice on "---" document separators.
func splitYAMLDocuments(content []byte) [][]byte {
	var docs [][]byte
	var current []byte

	for _, line := range bytes.Split(content, []byte("\n")) {
		if bytes.Equal(bytes.TrimSpace(line), []byte("---")) && bytes.HasPrefix(line, []byte("---")) {
			if len(bytes.TrimSpace(current)) > 0 {
				docs = append(docs, current)
			}
			current = nil
		} else {
			current = append(current, line...)
			current = append(current, '\n')
		}
	}
	if len(bytes.TrimSpace(current)) > 0 {
		docs = append(docs, current)
	}
	return docs
}

// documentMatchesResourceType checks whether a YAML document chunk declares the given resource type
// via a "resource_type: <type>" field.
// We scan line-by-line rather than parsing the whole document to avoid YAML parse errors caused
// by unquoted Go template expressions (e.g. {{.VAR}}) that appear in other fields.
func documentMatchesResourceType(doc []byte, resourceType string) bool {
	prefix := []byte("resource_type:")
	for _, line := range bytes.Split(doc, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, prefix) {
			value := bytes.TrimSpace(bytes.TrimPrefix(trimmed, prefix))
			value = bytes.Trim(value, `"'`)
			if string(value) == resourceType {
				return true
			}
		}
	}
	return false
}

// GetConfigsFromRootDir scans the config/resources/ directory for YAML files (*.yaml, *.yml)
// that sit directly in that directory (not in subdirectories) and returns all documents whose
// resource_type header matches resourceType.  Returns nil, nil when no matching YAML files exist
// so callers can fall through to directory-based loading.
func GetConfigsFromRootDir(resourceType string) ([][]byte, error) {
	serverHome := config.GetServerRuntime().ServerHome
	rootDir := filepath.Join(serverHome, "config", "resources")

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read resources directory: %w", err)
	}

	var results [][]byte
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		docs, err := GetConfigsFromFile(filepath.Join(rootDir, name), resourceType)
		if err != nil {
			return nil, err
		}
		results = append(results, docs...)
	}
	return results, nil
}

// GetConfigs reads all configuration files from the specified directory within the resources directory.
func GetConfigs(configDirectoryPath string) ([][]byte, error) {
	// Declarative config files are loaded at startup, outside any request,
	// so context.Background() is used (no request trace ID to propagate).
	ctx := context.Background()
	logger := log.GetLogger().With(log.String("component", "FileBasedRuntime"))
	serverHome := config.GetServerRuntime().ServerHome
	immutableConfigFilePath := path.Join(serverHome, "config/resources/")
	absoluteDirectoryPath := filepath.Join(immutableConfigFilePath, configDirectoryPath)
	files, err := os.ReadDir(absoluteDirectoryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return [][]byte{}, nil
		}
		logger.Error(ctx, "Failed to read configuration directory",
			log.String("path", absoluteDirectoryPath), log.Error(err))
		return nil, err
	}

	// Count non-directory files
	var fileCount int
	for _, file := range files {
		if !file.IsDir() {
			fileCount++
		}
	}

	configs := make([][]byte, 0, fileCount)
	if fileCount == 0 {
		return configs, nil
	}

	// Use channels to collect results from goroutines
	type configResult struct {
		content []byte
		err     error
	}
	configChan := make(chan configResult)
	var wg sync.WaitGroup

	for _, file := range files {
		if !file.IsDir() {
			wg.Add(1)
			go func(fileName string) {
				defer wg.Done()
				filePath := filepath.Join(absoluteDirectoryPath, fileName)
				filePath = filepath.Clean(filePath)
				// #nosec G304 -- File path is controlled and within a trusted directory
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					logger.Warn(ctx, "Failed to read configuration file",
						log.String("filePath", fileName), log.Error(err))
					configChan <- configResult{content: nil, err: err}
					return
				}
				// Substitute environment variables
				processedContent, err := utils.SubstituteEnvironmentVariables(fileContent)
				if err != nil {
					logger.Warn(ctx, "Failed to substitute environment variables in configuration file",
						log.String("filePath", fileName), log.Error(err))
					configChan <- configResult{content: nil, err: err}
					return
				}

				configChan <- configResult{content: processedContent, err: nil}
			}(file.Name())
		}
	}

	// Wait for all goroutines to complete and close the channel
	go func() {
		wg.Wait()
		close(configChan)
	}()

	// Collect results from the channel
	var errors []error
	for result := range configChan {
		if result.err != nil {
			errors = append(errors, result.err)
			continue
		}
		configs = append(configs, result.content)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("errors occurred while reading configuration files: %v", errors)
	}

	return configs, nil
}
