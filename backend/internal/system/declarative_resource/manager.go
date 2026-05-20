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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// GetConfigs reads all configuration files from the specified directory within the resources directory.
func GetConfigs(configDirectoryPath string) ([][]byte, error) {
	logger := log.GetLogger().With(log.String("component", "FileBasedRuntime"))
	serverHome := config.GetServerRuntime().ServerHome
	immutableConfigFilePath := path.Join(serverHome, "repository/resources/")
	absoluteDirectoryPath := filepath.Join(immutableConfigFilePath, configDirectoryPath)
	files, err := os.ReadDir(absoluteDirectoryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return [][]byte{}, nil
		}
		logger.Error("Failed to read configuration directory",
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
					logger.Warn("Failed to read configuration file", log.String("filePath", fileName), log.Error(err))
					configChan <- configResult{content: nil, err: err}
					return
				}
				// Substitute environment variables
				processedContent, err := utils.SubstituteEnvironmentVariables(fileContent)
				if err != nil {
					logger.Warn("Failed to substitute environment variables in configuration file",
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
