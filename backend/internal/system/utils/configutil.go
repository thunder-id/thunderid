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

// Package utils provides utility functions for server wide operations.
package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const tplLiteralPlaceholderFmt = "__TPL_LITERAL_%d__"

var (
	// Pattern to match Go template variables like {{.Variable}}
	varPattern = regexp.MustCompile(`\{\{\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

	// Pattern to match Go template range patterns like {{- range .ArrayVar}}
	rangePattern = regexp.MustCompile(`\{\{-\s*range\s+\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

	// Pattern to match Go template file path patterns
	filePattern = regexp.MustCompile(`file://(?:"([^"]*)"|([^\s"]+))`)

	// anyBracesPattern matches any single-line {{ ... }} expression (no nested braces).
	anyBracesPattern = regexp.MustCompile(`\{\{[^}]*\}\}`)

	// goTemplateExprPattern matches only the Go template expressions that Thunder config
	// files intentionally use: {{.Var}}, {{.}} (range element), {{- range .Var}}, {{- end}}.
	// Anything not matching this is a non-Go-template literal (e.g. {{ t(key) }}, {{appName}}).
	goTemplateExprPattern = regexp.MustCompile(
		`\{\{` +
			`(?:` +
			`\s*\.\s*[A-Za-z_][A-Za-z0-9_]*\s*` + // {{.Variable}}
			`|\s*\.\s*` + // {{.}}
			`|-\s*range\s+\.\s*[A-Za-z_][A-Za-z0-9_]*\s*` + // {{- range .Var}}
			`|-\s*end\s*` + // {{- end}}
			`)` +
			`\}\}`,
	)
)

// SubstituteFilePaths replaces file path placeholders in the given content with the actual file contents.
//
// Supported patterns:
//  1. file://path/to/file - Unquoted file path (no spaces allowed)
//  2. file://"path/with/spaces" - Quoted file path (spaces allowed)
//  3. file:/relative/path - Relative file path (resolved against base directory)
//
// If a file cannot be read, an error is returned.
func SubstituteFilePaths(content []byte, baseDir string) ([]byte, error) {
	// This runs outside any request, so context.Background() is used (no request trace ID).
	ctx := context.Background()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConfigUtil"))
	isError := false

	out := filePattern.ReplaceAllStringFunc(string(content), func(match string) string {
		sub := filePattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}

		// Group 1: quoted path (file://"path"), Group 2: unquoted path (file://path)
		path := sub[1]
		if path == "" {
			path = sub[2]
		}
		if path == "" {
			logger.Warn(ctx, "Empty file path in placeholder", log.String("placeholder", match))
			return ""
		}

		// Convert relative paths to absolute
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}

		data, err := readFileContent(path)
		if err != nil {
			logger.Error(ctx, "Failed to read file content", log.String("filePath", path), log.Error(err))
			isError = true
			return ""
		}

		return data
	})

	return []byte(out), func() error {
		if isError {
			return fmt.Errorf("one or more file path substitutions failed")
		}
		return nil
	}()
}

// readFileContent reads the content of the file at the given path.
func readFileContent(path string) (string, error) {
	path = filepath.Clean(path)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SubstituteEnvironmentVariables replaces Go template variable placeholders in the given content.
//
// Supported patterns:
//  1. {{.Variable}} - Simple variable substitution from environment variables
//  2. {{- range .ArrayVariable}} - Array iteration using VARIABLE_NAME_0, VARIABLE_NAME_1, ... pattern
//
// If an environment variable is not set, an error is returned.
func SubstituteEnvironmentVariables(content []byte) ([]byte, error) {
	contentStr := string(content)

	// Find all variables referenced in the template
	templateVars := make(map[string]interface{})

	// Extract simple variables
	varMatches := varPattern.FindAllStringSubmatch(contentStr, -1)
	for _, match := range varMatches {
		if len(match) > 1 {
			varName := match[1]
			envValue, exists := os.LookupEnv(varName)
			if !exists {
				return nil, fmt.Errorf("environment variable %s is not set", varName)
			}
			templateVars[varName] = envValue
		}
	}

	// Extract array variables from range statements
	rangeMatches := rangePattern.FindAllStringSubmatch(contentStr, -1)
	for _, match := range rangeMatches {
		if len(match) > 1 {
			varName := match[1]
			arrayElements := buildArrayFromEnvVars(varName)
			templateVars[varName] = arrayElements
		}
	}

	// If no template variables found, return original content
	if len(templateVars) == 0 {
		return content, nil
	}

	// Shield non-Go-template {{ ... }} expressions (e.g. {{ t(key) }}, {{appName}},
	// {{meta(prop)}}) so the Go template parser does not mistake them for function calls.
	shielded, restore := shieldNonGoTemplateExpressions(contentStr)

	// Create and execute the template
	tmpl, err := template.New("config").Parse(shielded)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, templateVars); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// Restore shielded expressions to their original form
	result := buf.String()
	for placeholder, original := range restore {
		result = strings.ReplaceAll(result, placeholder, original)
	}

	return []byte(result), nil
}

// shieldNonGoTemplateExpressions replaces any {{ ... }} pattern that is not a valid
// Go template expression with a unique placeholder string, returning the modified
// content and a map to restore the originals after template execution.
func shieldNonGoTemplateExpressions(content string) (string, map[string]string) {
	restore := make(map[string]string)
	idx := 0

	result := anyBracesPattern.ReplaceAllStringFunc(content, func(match string) string {
		if goTemplateExprPattern.MatchString(match) {
			return match
		}
		placeholder := fmt.Sprintf(tplLiteralPlaceholderFmt, idx)
		idx++
		restore[placeholder] = match
		return placeholder
	})

	return result, restore
}

// buildArrayFromEnvVars builds an array by reading environment variables with indexed suffixes
// starting from VARNAME_0, VARNAME_1, etc., until an empty or non-existent variable is found.
func buildArrayFromEnvVars(varName string) []string {
	var elements []string
	index := 0

	for {
		indexedVarName := fmt.Sprintf("%s_%d", varName, index)
		value, exists := os.LookupEnv(indexedVarName)

		// Stop if the variable doesn't exist or is empty
		if !exists || value == "" {
			break
		}

		elements = append(elements, value)
		index++
	}

	return elements
}
