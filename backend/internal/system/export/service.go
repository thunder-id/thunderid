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

package export

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// templateVariablePattern matches only uppercase-only parameter names in the exact forms
// "{{.NAME}}" and "{{- range .NAME}}". This relies on the parameterizer normalizing names
// via toSnakeCase() (which calls strings.ToUpper()) and only emitting those two template
// forms in parameterizer.go. Adding lowercase names or new actions like "if"/"with" will
// require updating this regex.
var templateVariablePattern = regexp.MustCompile(`\{\{\.([A-Z0-9_]+)\}\}|\{\{\-\s*range\s+\.([A-Z0-9_]+)\s*\}\}`)

const (
	formatYAML = "yaml"
	formatJSON = "json"

	resourceTypeApplication        = "application"
	resourceTypeIdentityProvider   = "identity_provider"
	resourceTypeNotificationSender = "notification_sender"
	resourceTypeUserType           = "user_type"
	resourceTypeOU                 = "organization_unit"
	resourceTypeUser               = "user"
	resourceTypeGroup              = "group"
	resourceTypeResourceServer     = "resource_server"
	resourceTypeRole               = "role"
	resourceTypeFlow               = "flow"
	resourceTypeTranslation        = "translation"
	resourceTypeLayout             = "layout"
	resourceTypeTheme              = "theme"
)

// parameterizerInterface defines the interface for template parameterization.
type parameterizerInterface interface {
	ToParameterizedYAML(obj interface{},
		resourceType string, resourceName string,
		rules *declarativeresource.ResourceRules) (string, map[string]string, error)
}

// ExportServiceInterface defines the interface for the export service.
type ExportServiceInterface interface {
	ExportResources(ctx context.Context, request *ExportRequest) (*ExportResponse, *serviceerror.ServiceError)
}

// exportService implements the ExportServiceInterface.
type exportService struct {
	parameterizer parameterizerInterface
	registry      *ResourceExporterRegistry
}

// newExportService creates a new instance of exportService.
func newExportService(
	exporters []declarativeresource.ResourceExporter, param parameterizerInterface,
) ExportServiceInterface {
	// Create registry and register all exporters
	registry := newResourceExporterRegistry()
	for _, exporter := range exporters {
		registry.Register(exporter)
	}

	return &exportService{
		parameterizer: param,
		registry:      registry,
	}
}

// ExportResources exports the specified resources as YAML files.
func (es *exportService) ExportResources(
	ctx context.Context, request *ExportRequest,
) (*ExportResponse, *serviceerror.ServiceError) {
	if request == nil {
		return nil, serviceerror.CustomServiceError(ErrorInvalidRequest, core.I18nMessage{
			Key:          "error.exportservice.nil_request_description",
			DefaultValue: "Export request cannot be nil",
		})
	}

	// Set default options if not provided
	options := request.Options
	if options == nil {
		options = &ExportOptions{
			Format: formatYAML,
		}
	}
	if options.Format == "" {
		options.Format = formatYAML
	}

	var exportFiles []ExportFile
	var exportErrors []declarativeresource.ExportError
	allVariables := make(map[string]string)
	resourceCounts := make(map[string]int)

	// Map resource types to their IDs from the request
	resourceMap := map[string][]string{
		resourceTypeApplication:        request.Applications,
		resourceTypeIdentityProvider:   request.IdentityProviders,
		resourceTypeNotificationSender: request.NotificationSenders,
		resourceTypeUserType:           request.UserTypes,
		resourceTypeOU:                 request.OrganizationUnits,
		resourceTypeUser:               request.Users,
		resourceTypeGroup:              request.Groups,
		resourceTypeResourceServer:     request.ResourceServers,
		resourceTypeRole:               request.Roles,
		resourceTypeFlow:               request.Flows,
		resourceTypeTranslation:        request.Translations,
		resourceTypeLayout:             request.Layouts,
		resourceTypeTheme:              request.Themes,
	}

	// Export resources using the registry
	resourceTypes := make([]string, 0, len(resourceMap))
	for k := range resourceMap {
		resourceTypes = append(resourceTypes, k)
	}
	sort.Strings(resourceTypes)

	for _, resourceType := range resourceTypes {
		resourceIDs := resourceMap[resourceType]
		if len(resourceIDs) == 0 {
			continue
		}

		exporter, exists := es.registry.Get(resourceType)
		if !exists {
			log.GetLogger().Warn("No exporter registered for resource type",
				log.String("resourceType", resourceType))
			continue
		}

		files, vars, errors := es.exportResourcesWithExporter(ctx, exporter, resourceIDs, options)
		exportFiles = append(exportFiles, files...)
		for k, v := range vars {
			allVariables[k] = v
		}
		exportErrors = append(exportErrors, errors...)
		resourceCounts[resourceType] = len(files)
	}

	if len(exportFiles) == 0 {
		return nil, serviceerror.CustomServiceError(ErrorNoResourcesFound, core.I18nMessage{
			Key:          "error.exportservice.no_valid_resources_for_export_description",
			DefaultValue: "No valid resources found for export",
		})
	}

	// Calculate total size
	var totalSize int64
	for i := range exportFiles {
		exportFiles[i].Size = int64(len(exportFiles[i].Content))
		totalSize += exportFiles[i].Size
	}

	envFile := es.generateEnvFile(exportFiles, allVariables)

	totalFilesCount := len(exportFiles)
	if envFile != nil {
		totalFilesCount++
		totalSize += envFile.Size
	}

	summary := &ExportSummary{
		TotalFiles:    totalFilesCount,
		TotalSize:     totalSize,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		ResourceTypes: resourceCounts,
		Errors:        exportErrors,
	}

	return &ExportResponse{
		Files:   exportFiles,
		EnvFile: envFile,
		Summary: summary,
	}, nil
}

// generateEnvFile extracts template variable names from exported files and builds a .env
// payload, populating each entry with its original value where available.
func (es *exportService) generateEnvFile(files []ExportFile, variables map[string]string) *EnvironmentFile {
	variablesSet := make(map[string]struct{})

	for _, file := range files {
		matches := templateVariablePattern.FindAllStringSubmatch(file.Content, -1)
		for _, match := range matches {
			for i := 1; i < len(match); i++ {
				if match[i] == "" {
					continue
				}
				variablesSet[match[i]] = struct{}{}
			}
		}
	}

	if len(variablesSet) == 0 {
		return nil
	}

	varNames := make([]string, 0, len(variablesSet))
	for varName := range variablesSet {
		varNames = append(varNames, varName)
	}
	sort.Strings(varNames)

	var contentBuilder strings.Builder
	for _, varName := range varNames {
		contentBuilder.WriteString(varName)
		contentBuilder.WriteString("=")
		contentBuilder.WriteString(variables[varName])
		contentBuilder.WriteString("\n")
	}

	content := contentBuilder.String()
	return &EnvironmentFile{
		FileName: ".env",
		Content:  content,
		Size:     int64(len(content)),
	}
}

// exportResourcesWithExporter exports resources using a registered exporter.
func (es *exportService) exportResourcesWithExporter(
	ctx context.Context,
	exporter declarativeresource.ResourceExporter,
	resourceIDs []string,
	options *ExportOptions,
) ([]ExportFile, map[string]string, []declarativeresource.ExportError) {
	logger := log.GetLogger().With(log.String("component", "ExportService"))
	resourceType := exporter.GetResourceType()
	exportFiles := make([]ExportFile, 0, len(resourceIDs))
	exportErrors := make([]declarativeresource.ExportError, 0, len(resourceIDs))
	variableValues := make(map[string]string)
	var resourceIDList []string
	if len(resourceIDs) == 1 && resourceIDs[0] == "*" {
		// Export all resources
		ids, err := exporter.GetAllResourceIDs(ctx)
		if err != nil {
			logger.Warn("Failed to get all resources",
				log.String("resourceType", resourceType), log.Any("error", err))
			return []ExportFile{}, variableValues, []declarativeresource.ExportError{}
		}
		resourceIDList = ids
	} else {
		resourceIDList = resourceIDs
	}

	for _, resourceID := range resourceIDList {
		// Get the resource
		resource, _, svcErr := exporter.GetResourceByID(ctx, resourceID)
		if svcErr != nil {
			logger.Warn("Failed to get resource for export",
				log.String("resourceType", resourceType),
				log.String("resourceID", resourceID),
				log.String("error", svcErr.Error.DefaultValue))
			exportErrors = append(exportErrors, declarativeresource.ExportError{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				Error:        svcErr.Error.DefaultValue,
				Code:         svcErr.Code,
			})
			continue
		}

		// Validate resource
		validatedName, exportErr := exporter.ValidateResource(resource, resourceID, logger)
		if exportErr != nil {
			exportErrors = append(exportErrors, *exportErr)
			continue
		}

		// Convert to export format based on options
		var content string
		var fileName string

		if options.Format == formatJSON {
			// Convert to JSON format (could be implemented later)
			logger.Warn("JSON format not yet implemented, falling back to YAML")
			options.Format = formatYAML
		}

		templateContent, vars, err := es.generateTemplateFromStruct(
			resource, exporter.GetParameterizerType(), validatedName, exporter)
		if err != nil {
			logger.Warn("Failed to generate template from struct",
				log.String("resourceType", resourceType),
				log.String("resourceID", resourceID),
				log.String("error", err.Error()))
			exportErrors = append(exportErrors, declarativeresource.ExportError{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				Error:        err.Error(),
				Code:         "TemplateGenerationError",
			})
			continue
		}
		for k, v := range vars {
			variableValues[k] = v
		}
		content = addResourceTypeComment(templateContent, resourceType)

		// Determine file name and folder path based on options
		fileName = es.generateFileName(validatedName, resourceType, resourceID, options)
		folderPath := es.generateFolderPath(resourceType, options)

		// Create export file
		exportFile := ExportFile{
			FileName:     fileName,
			Content:      content,
			FolderPath:   folderPath,
			ResourceType: resourceType,
			ResourceID:   resourceID,
		}
		exportFiles = append(exportFiles, exportFile)
	}

	return exportFiles, variableValues, exportErrors
}

func (es *exportService) generateTemplateFromStruct(data interface{},
	paramResourceType string, resourceName string,
	exporter declarativeresource.ResourceExporter) (string, map[string]string, error) {
	var rules *declarativeresource.ResourceRules
	if pr, ok := exporter.(declarativeresource.PerResourceRuler); ok {
		rules = pr.GetResourceRulesForResource(data)
	} else {
		rules = exporter.GetResourceRules()
	}
	template, vars, err := es.parameterizer.ToParameterizedYAML(
		data, paramResourceType, resourceName, rules)
	if err != nil {
		return "", nil, err
	}
	return template, vars, nil
}

func addResourceTypeComment(content, resourceType string) string {
	commentLine := "# resource_type: " + resourceType
	if strings.HasPrefix(content, commentLine+"\n") || content == commentLine {
		return content
	}
	return commentLine + "\n" + content
}

// sanitizeFileName sanitizes a filename by removing invalid characters.
func sanitizeFileName(name string) string {
	// Replace spaces with underscores and remove special characters
	sanitized := strings.ReplaceAll(name, " ", "_")
	// Remove any characters that are not alphanumeric, hyphens, or underscores
	var result strings.Builder
	for _, char := range sanitized {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' {
			result.WriteRune(char)
		}
	}
	sanitizedName := result.String()
	if sanitizedName == "" {
		sanitizedName = "resource"
	}
	return sanitizedName
}

// generateFileName generates a file name based on naming pattern and options.
// nolint:unparam
func (es *exportService) generateFileName(
	resourceName, resourceType, resourceID string, options *ExportOptions) string {
	// Get file extension based on format
	ext := ".yaml"
	if options.Format == "json" {
		ext = ".json"
	}

	// Use custom naming pattern if provided
	if options.FolderStructure != nil && options.FolderStructure.FileNamingPattern != "" {
		pattern := options.FolderStructure.FileNamingPattern
		pattern = strings.ReplaceAll(pattern, "${name}", sanitizeFileName(resourceName))
		pattern = strings.ReplaceAll(pattern, "${type}", resourceType)
		pattern = strings.ReplaceAll(pattern, "${id}", resourceID)
		return pattern + ext
	}

	// Default naming: sanitized resource name
	return sanitizeFileName(resourceName) + ext
}

// generateFolderPath generates the folder path for a resource based on options.
// nolint:unparam
func (es *exportService) generateFolderPath(resourceType string, options *ExportOptions) string {
	if options.FolderStructure == nil {
		return "" // No folder structure
	}

	// Check for custom structure first
	if options.FolderStructure.CustomStructure != nil {
		if customPath, exists := options.FolderStructure.CustomStructure[resourceType]; exists {
			return customPath
		}
	}

	// Group by type if enabled
	if options.FolderStructure.GroupByType {
		return resourceType + "s" // applications, groups, users, etc.
	}

	return ""
}
