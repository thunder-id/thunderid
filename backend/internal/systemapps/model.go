// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package systemapps holds the configuration of the system apps (the Admin App and the Account App),
// which is stored as a server-config section per app.
package systemapps

// AppType identifies a system app: an app the Console builds and data planes serve.
type AppType string

const (
	// AppTypeAdmin is the Admin App, where the admins of an organization unit manage its users,
	// groups, roles and agents.
	AppTypeAdmin AppType = "ADMIN"
	// AppTypeAccount is the Account App, the self-service app for end users.
	AppTypeAccount AppType = "ACCOUNT"
)

// PageVisibility decides who sees a page of an app.
type PageVisibility string

const (
	// PageVisibilityAll shows the page to every admin of the organization unit.
	PageVisibilityAll PageVisibility = "ALL"
	// PageVisibilityByPermission shows the page only to admins whose token carries the page's view
	// permission.
	PageVisibilityByPermission PageVisibility = "BY_PERMISSION"
)

// NavigationLayout decides how an app's pages are navigated.
type NavigationLayout string

const (
	// NavigationLayoutTopBar places the pages in a tab row under the top bar.
	NavigationLayoutTopBar NavigationLayout = "TOP_BAR"
	// NavigationLayoutSideBar places the pages in a side bar.
	NavigationLayoutSideBar NavigationLayout = "SIDE_BAR"
)

// ContentWidth decides how wide an app's pages are.
type ContentWidth string

const (
	// ContentWidthContained centers the pages in a column of limited width.
	ContentWidthContained ContentWidth = "CONTAINED"
	// ContentWidthFull lets the pages use the full width of the window.
	ContentWidthFull ContentWidth = "FULL"
)

// AppPage is the configuration of one page of an app.
type AppPage struct {
	Key        string          `json:"key" yaml:"key"`
	Enabled    bool            `json:"enabled" yaml:"enabled"`
	Visibility PageVisibility  `json:"visibility" yaml:"visibility"`
	Actions    map[string]bool `json:"actions" yaml:"actions"`
}

// AppNavigation is how an app's pages are navigated.
type AppNavigation struct {
	Layout NavigationLayout `json:"layout" yaml:"layout"`
}

// AppContent is how an app lays out its pages.
type AppContent struct {
	Width ContentWidth `json:"width" yaml:"width"`
}

// AppBranding is how an app presents itself.
type AppBranding struct {
	LogoURL       string `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty"`
	Title         string `json:"title,omitempty" yaml:"title,omitempty"`
	ShowSecuredBy bool   `json:"showSecuredBy" yaml:"showSecuredBy"`
}

// AppThemeColors are the colors of one color scheme of an app's theme. A color left empty keeps
// the app's own.
type AppThemeColors struct {
	Primary       string `json:"primary,omitempty" yaml:"primary,omitempty"`
	Background    string `json:"background,omitempty" yaml:"background,omitempty"`
	Surface       string `json:"surface,omitempty" yaml:"surface,omitempty"`
	TextPrimary   string `json:"textPrimary,omitempty" yaml:"textPrimary,omitempty"`
	TextSecondary string `json:"textSecondary,omitempty" yaml:"textSecondary,omitempty"`
	Border        string `json:"border,omitempty" yaml:"border,omitempty"`
}

// AppThemeColorSchemes are an app theme's colors for each color scheme.
type AppThemeColorSchemes struct {
	Light AppThemeColors `json:"light" yaml:"light"`
	Dark  AppThemeColors `json:"dark" yaml:"dark"`
}

// AppTheme is how an app and the SDK components it is built from look. It applies to the app
// only: its sign-in screens take the Gate theme of the app's application.
type AppTheme struct {
	ColorSchemes AppThemeColorSchemes `json:"colorSchemes" yaml:"colorSchemes"`
	// BorderRadius is the corner radius of the app's controls, in pixels. Unset keeps the app's own.
	BorderRadius *int   `json:"borderRadius,omitempty" yaml:"borderRadius,omitempty"`
	FontFamily   string `json:"fontFamily,omitempty" yaml:"fontFamily,omitempty"`
}

// AppConfig is the configuration of an app: the value of its server-config section.
type AppConfig struct {
	Type       AppType       `json:"type" yaml:"type"`
	Pages      []AppPage     `json:"pages" yaml:"pages"`
	Navigation AppNavigation `json:"navigation" yaml:"navigation"`
	Content    AppContent    `json:"content" yaml:"content"`
	AppTheme   *AppTheme     `json:"appTheme,omitempty" yaml:"appTheme,omitempty"`
	Branding   AppBranding   `json:"branding" yaml:"branding"`
}
