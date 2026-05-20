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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

var fileStoreDirectoryByResourceType = map[string]string{
	resourceTypeApplication:      "applications",
	resourceTypeIdentityProvider: "identity_providers",
	resourceTypeFlow:             "flows",
	resourceTypeOrganizationUnit: "organization_units",
	resourceTypeEntityType:       "user_types",
	resourceTypeRole:             "roles",
	resourceTypeResourceServer:   "resource_servers",
	resourceTypeTheme:            "themes",
	resourceTypeLayout:           "layouts",
	resourceTypeUser:             "users",
	resourceTypeTranslation:      "translations",
}

func deleteFileBackedResource(resourceType, resourceKey string) (string, *serviceerror.ServiceError) {
	directory, ok := fileStoreDirectoryByResourceType[resourceType]
	if !ok {
		return "", serviceerror.CustomServiceError(
			ErrorInvalidImportRequest,
			core.I18nMessage{
				Key:          "error.import.unsupportedResourceType",
				DefaultValue: "unsupported resource type for declarative file management",
			},
		)
	}

	serverHome, err := getThunderHome()
	if err != nil {
		return "", serviceerror.CustomServiceError(serviceerror.InternalServerError,
			core.I18nMessage{Key: "error.import.dynamic", DefaultValue: err.Error()})
	}

	targetDir := filepath.Join(serverHome, "repository", "resources", directory)
	entries, readErr := os.ReadDir(targetDir)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return "", serviceerror.CustomServiceError(ErrorInvalidImportRequest,
				core.I18nMessage{Key: "error.import.delete.dirNotFound", DefaultValue: "resource directory not found"})
		}
		return "", serviceerror.CustomServiceError(serviceerror.InternalServerError,
			core.I18nMessage{Key: "error.import.dynamic", DefaultValue: readErr.Error()})
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		filePath := filepath.Join(targetDir, entry.Name())
		matches, matchErr := fileMatchesResourceKey(filePath, resourceType, resourceKey)
		if matchErr != nil {
			return "", serviceerror.CustomServiceError(serviceerror.InternalServerError,
				core.I18nMessage{Key: "error.import.dynamic", DefaultValue: matchErr.Error()})
		}
		if !matches {
			continue
		}

		if removeErr := os.Remove(filePath); removeErr != nil {
			return "", serviceerror.CustomServiceError(serviceerror.InternalServerError,
				core.I18nMessage{Key: "error.import.dynamic", DefaultValue: removeErr.Error()})
		}
		return entry.Name(), nil
	}

	return "", serviceerror.CustomServiceError(ErrorInvalidImportRequest,
		core.I18nMessage{Key: "error.import.delete.fileNotFound", DefaultValue: "resource file not found"})
}

func extractDocumentIdentity(doc parsedDocument) (string, string) {
	resourceID := getStringField(doc.Node, "id")
	resourceName := getStringField(doc.Node, "name")

	if resourceName == "" {
		switch doc.ResourceType {
		case resourceTypeTheme, resourceTypeLayout:
			resourceName = getStringField(doc.Node, "displayName")
		case resourceTypeTranslation:
			resourceName = getStringField(doc.Node, "language")
		}
	}

	return resourceID, resourceName
}

func getStringField(node *yaml.Node, field string) string {
	if node == nil || node.Kind != yaml.MappingNode {
		return ""
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]
		if key.Value == field && value.Kind == yaml.ScalarNode {
			return strings.TrimSpace(value.Value)
		}
	}

	return ""
}

func fileMatchesResourceKey(filePath, resourceType, resourceKey string) (bool, error) {
	// #nosec G304 -- path is derived from server_home/resource directory and enumerated directory entries.
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	docs, err := parseDocuments(string(content))
	if err != nil {
		return false, err
	}

	for _, doc := range docs {
		if doc.ResourceType != resourceType {
			continue
		}
		if documentMatchesKey(doc, resourceKey) {
			return true, nil
		}
	}

	return false, nil
}

func documentMatchesKey(doc parsedDocument, resourceKey string) bool {
	normalizedKey := strings.TrimSpace(resourceKey)
	if normalizedKey == "" {
		return false
	}

	resourceID, resourceName := extractDocumentIdentity(doc)
	for _, candidate := range []string{
		resourceID,
		resourceName,
		getStringField(doc.Node, "handle"),
		getStringField(doc.Node, "identifier"),
		getStringField(doc.Node, "language"),
	} {
		if strings.TrimSpace(candidate) == normalizedKey {
			return true
		}
	}

	return false
}

func getThunderHome() (serverHome string, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("Server runtime is not initialized")
		}
	}()

	runtime := config.GetServerRuntime()
	if runtime == nil || strings.TrimSpace(runtime.ServerHome) == "" {
		return "", fmt.Errorf("Server runtime is not initialized")
	}

	return runtime.ServerHome, nil
}
