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
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

var templateExpressionRegexp = regexp.MustCompile(`\{\{[-\s]*([^{}]+?)[-\s]*\}\}`)
var identifierOrCallRegexp = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*(\([^{}]*\))?$`)

func resolveTemplate(content string, variables map[string]interface{}) (string, error) {
	protectedContent, protectedExpressions := protectLiteralTemplateExpressions(content)

	tmpl, err := template.New("import_content").Option("missingkey=error").Parse(protectedContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse import template: %w", err)
	}

	var data map[string]interface{}
	if variables == nil {
		data = map[string]interface{}{}
	} else {
		data = variables
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to resolve import template variables: %w", err)
	}

	return restoreLiteralTemplateExpressions(buf.String(), protectedExpressions), nil
}

func protectLiteralTemplateExpressions(content string) (string, map[string]string) {
	protectedExpressions := map[string]string{}
	index := 0
	randomSuffix := randomPlaceholderSuffix()

	replaced := templateExpressionRegexp.ReplaceAllStringFunc(content, func(match string) string {
		expression := extractTemplateExpression(match)
		if !shouldProtectLiteralExpression(expression) {
			return match
		}

		key := fmt.Sprintf("__LITERAL_TEMPLATE_EXPR_%s_%d__", randomSuffix, index)
		protectedExpressions[key] = match
		index++
		return key
	})

	return replaced, protectedExpressions
}

func restoreLiteralTemplateExpressions(content string, protectedExpressions map[string]string) string {
	restored := content
	for key, value := range protectedExpressions {
		restored = strings.ReplaceAll(restored, key, value)
	}
	return restored
}

func extractTemplateExpression(match string) string {
	trimmed := strings.TrimSpace(match)
	trimmed = strings.TrimPrefix(trimmed, "{{")
	trimmed = strings.TrimSuffix(trimmed, "}}")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.TrimPrefix(trimmed, "-")
	trimmed = strings.TrimSuffix(trimmed, "-")

	return strings.TrimSpace(trimmed)
}

func shouldProtectLiteralExpression(expression string) bool {
	if expression == "" {
		return false
	}

	if strings.HasPrefix(expression, ".") || strings.HasPrefix(expression, "$") {
		return false
	}

	if isKnownLiteralHelperExpression(expression) {
		return true
	}

	if strings.Contains(expression, "|") || strings.Contains(expression, ":=") || strings.Contains(expression, " ") {
		return false
	}

	switch expression {
	case "range", "if", "with", "template", "block", "define", "end", "else":
		return false
	}

	return identifierOrCallRegexp.MatchString(expression)
}

func isKnownLiteralHelperExpression(expression string) bool {
	fields := strings.Fields(expression)
	if len(fields) == 0 {
		return false
	}

	helper := fields[0]
	helper = strings.TrimSuffix(helper, "(")
	if idx := strings.Index(helper, "("); idx >= 0 {
		helper = helper[:idx]
	}

	switch helper {
	case "t", "meta", "appName":
		return true
	default:
		return false
	}
}

func randomPlaceholderSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}

	return hex.EncodeToString(buf)
}
