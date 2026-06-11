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

package thememgt

import "encoding/json"

// Theme represents a theme configuration.
type Theme struct {
	ID          string          `json:"id" yaml:"id,omitempty"`
	Handle      string          `json:"handle" yaml:"handle"`
	DisplayName string          `json:"displayName" yaml:"displayName"`
	Description string          `json:"description" yaml:"description,omitempty"`
	Theme       json.RawMessage `json:"theme" yaml:"theme"`
	CreatedAt   string          `json:"createdAt" yaml:"createdAt,omitempty"`
	UpdatedAt   string          `json:"updatedAt" yaml:"updatedAt,omitempty"`
	IsReadOnly  bool            `json:"isReadOnly" yaml:"isReadOnly"`
}

// ThemeListItem represents a theme item in the list response.
type ThemeListItem struct {
	ID                 string `json:"id"`
	Handle             string `json:"handle"`
	DisplayName        string `json:"displayName"`
	Description        string `json:"description"`
	DefaultColorScheme string `json:"defaultColorScheme"`
	PrimaryColor       string `json:"primaryColor"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	IsReadOnly         bool   `json:"isReadOnly"`
}

// themeColorSchemeInfo is a minimal struct for extracting color info from theme JSON.
type themeColorSchemeInfo struct {
	DefaultColorScheme string `json:"defaultColorScheme"`
	ColorSchemes       map[string]struct {
		Palette struct {
			Primary struct {
				Main string `json:"main"`
			} `json:"primary"`
		} `json:"palette"`
	} `json:"colorSchemes"`
}

// extractThemeColorInfo extracts the defaultColorScheme and primaryColor from a raw theme JSON blob.
// Returns empty strings on any parse or navigation failure so the list response degrades gracefully.
func extractThemeColorInfo(themeJSON json.RawMessage) (defaultColorScheme, primaryColor string) {
	if len(themeJSON) == 0 {
		return "", ""
	}
	var info themeColorSchemeInfo
	if err := json.Unmarshal(themeJSON, &info); err != nil {
		return "", ""
	}
	defaultColorScheme = info.DefaultColorScheme
	if defaultColorScheme == "" {
		return "", ""
	}
	scheme, ok := info.ColorSchemes[defaultColorScheme]
	if !ok {
		return defaultColorScheme, ""
	}
	return defaultColorScheme, scheme.Palette.Primary.Main
}

// CreateThemeRequest represents the request body for creating a theme configuration.
type CreateThemeRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description"`
	Theme       json.RawMessage `json:"theme"`
}

// CreateThemeRequestWithID represents the service-level request for creating a theme, including an optional ID.
type CreateThemeRequestWithID struct {
	ID          string          `json:"id,omitempty" yaml:"id,omitempty"`
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description"`
	Theme       json.RawMessage `json:"theme"`
}

// UpdateThemeRequest represents the request body for updating a theme configuration.
type UpdateThemeRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description"`
	Theme       json.RawMessage `json:"theme"`
}

// themeRequestWithID represents the request structure for creating a theme from file-based config.
type themeRequestWithID struct {
	ID          string      `yaml:"id"`
	Handle      string      `yaml:"handle"`
	DisplayName string      `yaml:"displayName"`
	Description string      `yaml:"description,omitempty"`
	Theme       interface{} `yaml:"theme"`
}

// ThemeListResponse represents the response for listing theme configurations with pagination.
type ThemeListResponse struct {
	TotalResults int             `json:"totalResults"`
	StartIndex   int             `json:"startIndex"`
	Count        int             `json:"count"`
	Themes       []ThemeListItem `json:"themes"`
	Links        []LinkResponse  `json:"links"`
}

// LinkResponse represents a pagination link.
type LinkResponse struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

// Link represents a pagination link.
type Link struct {
	Href string
	Rel  string
}

// ThemeList represents the result of listing theme configurations.
type ThemeList struct {
	TotalResults int
	StartIndex   int
	Count        int
	Themes       []Theme
	Links        []Link
}

// ThemeUsage represents a single resource that references the theme.
type ThemeUsage struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

// ThemeUsagesSummary holds a grouped count of usages by resource type.
// A nil value means the count could not be determined.
type ThemeUsagesSummary struct {
	Applications *int `json:"applications"`
}

// ThemeUsagesResponse is the paginated response for the usages endpoint.
// TotalResults is nil when usage data is unavailable.
type ThemeUsagesResponse struct {
	TotalResults *int               `json:"totalResults"`
	StartIndex   int                `json:"startIndex"`
	Count        int                `json:"count"`
	Summary      ThemeUsagesSummary `json:"summary"`
	Usages       []ThemeUsage       `json:"usages"`
	Links        []LinkResponse     `json:"links"`
}
