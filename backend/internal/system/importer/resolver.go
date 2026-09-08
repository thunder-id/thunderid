// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
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

	// A variable the caller did not supply is filled from the environment, the same way a
	// declarative resource read from disk is. A deployment holds its own credentials as environment
	// variables, so a configuration can travel without them and still resolve where it lands.
	data := map[string]interface{}{}
	for name, value := range variables {
		data[name] = value
	}
	for _, name := range templateVariableNames(protectedContent) {
		if _, supplied := data[name]; supplied {
			continue
		}
		if value, set := os.LookupEnv(name); set {
			data[name] = value
		}
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

// templateVariableNames returns the plain variable names a template refers to, so the ones the
// caller left out can be looked for in the environment. Anything that is not a bare field
// reference, such as a function call or a range, is left to the template engine.
func templateVariableNames(content string) []string {
	seen := map[string]bool{}
	names := []string{}
	for _, match := range templateExpressionRegexp.FindAllStringSubmatch(content, -1) {
		expression := strings.TrimSpace(match[1])
		if !strings.HasPrefix(expression, ".") {
			continue
		}
		name := strings.TrimPrefix(expression, ".")
		if name == "" || strings.ContainsAny(name, ". (") || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
