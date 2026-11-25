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
	// Logo is the logo the app shows in its header. Unset, the app shows the logo of the
	// application it signs in with.
	Logo *AppImage `json:"logo,omitempty" yaml:"logo,omitempty"`
	// Favicon is the icon the browser shows for the app. Unset, the app shows its own.
	Favicon       *AppImage `json:"favicon,omitempty" yaml:"favicon,omitempty"`
	Title         string    `json:"title,omitempty" yaml:"title,omitempty"`
	ShowSecuredBy bool      `json:"showSecuredBy" yaml:"showSecuredBy"`
	// Links are the pages the app links to at the foot of every page, such as privacy and terms.
	Links []AppLink `json:"links,omitempty" yaml:"links,omitempty"`
}

// AppLink is a link at the foot of an app's pages.
type AppLink struct {
	Label string `json:"label" yaml:"label"`
	URL   string `json:"url" yaml:"url"`
}

// AppLanguages is the language menu of an app.
type AppLanguages struct {
	// Menu shows a menu in the app's header to switch the app's language.
	Menu bool `json:"menu" yaml:"menu"`
	// Offered are the language tags the menu lists, such as en-US.
	Offered []string `json:"offered,omitempty" yaml:"offered,omitempty"`
	// Default is the language the app opens in, until its user picks another. It is one of Offered.
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
}

// AppImage is a branding image for each color scheme, as http or https URLs. Dark, when unset,
// takes the light one.
type AppImage struct {
	Light string `json:"light,omitempty" yaml:"light,omitempty"`
	Dark  string `json:"dark,omitempty" yaml:"dark,omitempty"`
}

// AppConfig is the configuration of an app: the value of its server-config section.
type AppConfig struct {
	Type       AppType       `json:"type" yaml:"type"`
	Pages      []AppPage     `json:"pages" yaml:"pages"`
	Navigation AppNavigation `json:"navigation" yaml:"navigation"`
	Content    AppContent    `json:"content" yaml:"content"`
	// ThemeID is the Design theme the app takes. Unset, the app follows the Gate theme of the
	// application it signs in with.
	ThemeID  string      `json:"themeId,omitempty" yaml:"themeId,omitempty"`
	Branding AppBranding `json:"branding" yaml:"branding"`
	// Languages is the app's language menu. Unset, the app has none.
	Languages *AppLanguages `json:"languages,omitempty" yaml:"languages,omitempty"`
}
