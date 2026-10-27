// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package systemapps

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// AppConfigHandler decodes, validates, and merges the server-config section of one system app. It
// implements serverconfig.ServerConfigHandlerInterface structurally so this package does not need to
// import serverconfig.
type AppConfigHandler struct {
	appType AppType
}

// NewAppConfigHandler creates the server-config handler for the given app.
func NewAppConfigHandler(appType AppType) *AppConfigHandler {
	return &AppConfigHandler{appType: appType}
}

// Decode parses a raw section value into an AppConfig. Empty input yields nil: nothing is set, and the
// app's defaults apply.
func (h *AppConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var config AppConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate checks that a configuration names only the app's own pages and actions, each page once,
// and uses supported layout, width, visibility and theme values.
func (h *AppConfigHandler) Validate(incoming, _, _ any) error {
	config, ok := incoming.(AppConfig)
	if !ok {
		return fmt.Errorf("%s: unexpected config type %T", h.appType, incoming)
	}

	return validateAppConfig(config, pageCatalog(h.appType))
}

// Merge returns the effective configuration: the writable layer when set, else the read-only one,
// else the app's defaults. The result always names every page of the app.
func (h *AppConfigHandler) Merge(readOnly, writable any) any {
	config := defaultAppConfig(h.appType)
	if value, ok := writable.(AppConfig); ok {
		config = value
	} else if value, ok := readOnly.(AppConfig); ok {
		config = value
	}

	config.Type = h.appType
	if config.Content.Width == "" {
		config.Content.Width = ContentWidthContained
	}

	return withMissingPages(config, pageCatalog(h.appType))
}

var (
	errInvalidNavigationLayout = errors.New("navigation layout must be TOP_BAR or SIDE_BAR")
	errInvalidContentWidth     = errors.New("content width must be CONTAINED or FULL")
	errInvalidAppTheme         = errors.New("the app theme has a value that is not a plain color or font list")
	errInvalidVisibility       = errors.New("page visibility must be ALL or BY_PERMISSION")
)

// validateAppConfig checks a configuration against the pages its app can have.
func validateAppConfig(config AppConfig, catalog []pageDefinition) error {
	switch config.Navigation.Layout {
	case NavigationLayoutTopBar, NavigationLayoutSideBar:
	default:
		return errInvalidNavigationLayout
	}

	switch config.Content.Width {
	case "", ContentWidthContained, ContentWidthFull:
	default:
		return errInvalidContentWidth
	}

	if config.AppTheme != nil && !isValidAppTheme(*config.AppTheme) {
		return errInvalidAppTheme
	}

	seen := make(map[string]bool, len(config.Pages))
	for _, page := range config.Pages {
		definition, ok := findPage(catalog, page.Key)
		if !ok || seen[page.Key] {
			return fmt.Errorf("unknown or repeated page %q", page.Key)
		}
		seen[page.Key] = true

		switch page.Visibility {
		case PageVisibilityAll, PageVisibilityByPermission:
		default:
			return errInvalidVisibility
		}

		for action := range page.Actions {
			if !offersAction(definition, action) {
				return fmt.Errorf("page %q offers no action %q", page.Key, action)
			}
		}
	}

	return nil
}

// The apps apply theme values as CSS custom properties, so only plain colors and font lists pass.
var (
	appThemeColorPattern = regexp.MustCompile(
		`^(#[0-9a-fA-F]{3,4}|#[0-9a-fA-F]{6}|#[0-9a-fA-F]{8}|rgba?\(\s*[0-9.%,\s/]+\))$`)
	appThemeFontPattern = regexp.MustCompile(`^[a-zA-Z0-9 ,'"\-]{1,200}$`)
)

const maxAppThemeBorderRadius = 32

// isValidAppTheme checks every value an app theme sets.
func isValidAppTheme(theme AppTheme) bool {
	if theme.BorderRadius != nil && (*theme.BorderRadius < 0 || *theme.BorderRadius > maxAppThemeBorderRadius) {
		return false
	}
	if theme.FontFamily != "" && !appThemeFontPattern.MatchString(theme.FontFamily) {
		return false
	}

	for _, colors := range []AppThemeColors{theme.ColorSchemes.Light, theme.ColorSchemes.Dark} {
		for _, color := range []string{
			colors.Primary, colors.Background, colors.Surface, colors.TextPrimary, colors.TextSecondary, colors.Border,
		} {
			if color != "" && !appThemeColorPattern.MatchString(color) {
				return false
			}
		}
	}

	return true
}

// withMissingPages appends, with their defaults, the catalog pages a configuration does not name,
// so a configuration set before a page existed still describes every page.
func withMissingPages(config AppConfig, catalog []pageDefinition) AppConfig {
	named := make(map[string]bool, len(config.Pages))
	for _, page := range config.Pages {
		named[page.Key] = true
	}

	pages := append([]AppPage{}, config.Pages...)
	for _, definition := range catalog {
		if !named[definition.key] {
			pages = append(pages, defaultPage(definition))
		}
	}

	config.Pages = pages
	return config
}

func findPage(catalog []pageDefinition, key string) (pageDefinition, bool) {
	for _, definition := range catalog {
		if definition.key == key {
			return definition, true
		}
	}

	return pageDefinition{}, false
}

func offersAction(definition pageDefinition, action string) bool {
	for _, candidate := range definition.actions {
		if candidate == action {
			return true
		}
	}

	return false
}
