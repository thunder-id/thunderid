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

package security

import (
	"fmt"
	"regexp"
	"strings"
)

// compiledAPIPermission holds the pre-compiled regex form of a single apiPermissionEntry.
type compiledAPIPermission struct {
	re         *regexp.Regexp
	permission string
}

// compiledPlaneRoute holds the pre-compiled regex form of a single planeRoute.
type compiledPlaneRoute struct {
	re    *regexp.Regexp
	plane Plane
}

// compilePlaneRoutes compiles the management-route plane classification into regex form.
// It returns an error if any pattern is invalid.
func compilePlaneRoutes(routes []planeRoute) ([]compiledPlaneRoute, error) {
	compiled := make([]compiledPlaneRoute, 0, len(routes))
	for _, r := range routes {
		re, err := compilePathPattern(r.pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, compiledPlaneRoute{re: re, plane: r.plane})
	}
	return compiled, nil
}

// compilePathPattern compiles a single glob-style path pattern into a regular expression.
// It returns an error if the pattern is invalid.
//
// Supported syntax:
//   - "*"  matches exactly one path segment (no slashes).
//   - "**" matches zero or more path segments; only valid as the suffix after "/" (e.g., "/a/**").
func compilePathPattern(pattern string) (*regexp.Regexp, error) {
	var regexPattern string

	if strings.Contains(pattern, "**") {
		// Ensure "**" is only used as a suffix "/**"
		if !strings.HasSuffix(pattern, "/**") {
			return nil,
				fmt.Errorf("invalid pattern: recursive wildcard '**' is only allowed as a suffix: %s", pattern)
		}
		// Ensure "**" appears only once
		if strings.Count(pattern, "**") > 1 {
			return nil, fmt.Errorf("invalid pattern: recursive wildcard '**' can only appear once: %s", pattern)
		}
		base := strings.TrimSuffix(pattern, "/**")
		baseRegex := regexp.QuoteMeta(base)
		baseRegex = strings.ReplaceAll(baseRegex, "\\*", "[^/]+")
		regexPattern = "^" + baseRegex + "(?:/.*)?$"
	} else {
		// Normal pattern (no recursive wildcards)
		regexPattern = regexp.QuoteMeta(pattern)
		regexPattern = strings.ReplaceAll(regexPattern, "\\*", "[^/]+")
		regexPattern = "^" + regexPattern + "$"
	}

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("error compiling path pattern regex for pattern %s: %w", pattern, err)
	}
	return re, nil
}

// compilePathPatterns compiles a slice of glob-style path patterns into regular expressions.
// It returns an error if any pattern is invalid.
func compilePathPatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := compilePathPattern(pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

// compileAPIPermissions compiles a slice of apiPermissionEntry values into their regex form.
// It returns an error if any pattern is invalid.
func compileAPIPermissions(entries []apiPermissionEntry) ([]compiledAPIPermission, error) {
	compiled := make([]compiledAPIPermission, 0, len(entries))
	for _, entry := range entries {
		re, err := compilePathPattern(entry.pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, compiledAPIPermission{re: re, permission: entry.permission})
	}
	return compiled, nil
}
