// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package systemapps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
// and uses supported layout, width, visibility and theme ID values.
func (h *AppConfigHandler) Validate(_ context.Context, incoming, _, _ any) error {
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
	errInvalidThemeID          = errors.New("theme ID must be a Design theme ID")
	errInvalidBrandingURL      = errors.New("the logo and favicon must be http or https URLs")
	errInvalidLink             = errors.New("each footer link needs a label of up to 60 characters and an http or https URL")
	errTooManyLinks            = errors.New("an app has at most 5 footer links")
	errInvalidLanguages        = errors.New("languages must be distinct language tags, with the default among them")
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

	if config.ThemeID != "" && !themeIDPattern.MatchString(config.ThemeID) {
		return errInvalidThemeID
	}

	for _, image := range []*AppImage{config.Branding.Logo, config.Branding.Favicon} {
		if image != nil && (!isImageURL(image.Light) || !isImageURL(image.Dark)) {
			return errInvalidBrandingURL
		}
	}

	if err := validateLinks(config.Branding.Links); err != nil {
		return err
	}

	if config.Languages != nil && !isValidLanguages(*config.Languages) {
		return errInvalidLanguages
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

// themeIDPattern matches the IDs Design themes are stored under.
var themeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const maxImageURLLength = 2048

const (
	maxLinks           = 5
	maxLinkLabelLength = 60
	maxLanguages       = 20
)

// languageTagPattern matches BCP 47 language tags such as en, en-US and zh-Hant-TW.
var languageTagPattern = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8}){0,3}$`)

// validateLinks checks an app's footer links: each with a label and an absolute http or https URL.
func validateLinks(links []AppLink) error {
	if len(links) > maxLinks {
		return errTooManyLinks
	}
	for _, link := range links {
		if link.Label == "" || len([]rune(link.Label)) > maxLinkLabelLength || link.URL == "" || !isImageURL(link.URL) {
			return errInvalidLink
		}
	}

	return nil
}

// isValidLanguages checks an app's language menu: distinct language tags, the default among them.
func isValidLanguages(languages AppLanguages) bool {
	if len(languages.Offered) > maxLanguages {
		return false
	}

	seen := make(map[string]bool, len(languages.Offered))
	for _, tag := range languages.Offered {
		if !languageTagPattern.MatchString(tag) || seen[tag] {
			return false
		}
		seen[tag] = true
	}

	return languages.Default == "" || seen[languages.Default]
}

// isImageURL reports whether a branding image is unset or an absolute http or https URL, the only
// kind the apps load into an image or icon.
func isImageURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > maxImageURLLength {
		return false
	}

	parsed, err := url.Parse(value)

	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
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
