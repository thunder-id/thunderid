// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package systemapps

// pageDefinition describes a page an app can have and the actions it can offer.
type pageDefinition struct {
	key     string
	actions []string
}

// adminAppPages lists the Admin App's pages, in navigation order.
var adminAppPages = []pageDefinition{
	{key: "users", actions: []string{"invite", "create", "edit", "delete"}},
	{key: "groups", actions: []string{"create", "members", "delete"}},
	{key: "roles", actions: []string{"create", "assign", "delete"}},
	{key: "agents", actions: []string{"create", "delete"}},
}

// accountAppPages lists the Account App's pages, in navigation order.
var accountAppPages = []pageDefinition{
	{key: "profile", actions: []string{"edit"}},
	{key: "security", actions: []string{"password"}},
}

// pageCatalog returns the pages an app type can have.
func pageCatalog(appType AppType) []pageDefinition {
	if appType == AppTypeAccount {
		return accountAppPages
	}
	return adminAppPages
}

// defaultPage returns the configuration a page starts with: shown to every user of the app, with
// every action on except deletion.
func defaultPage(definition pageDefinition) AppPage {
	actions := make(map[string]bool, len(definition.actions))
	for _, action := range definition.actions {
		actions[action] = action != "delete"
	}

	return AppPage{Key: definition.key, Enabled: true, Visibility: PageVisibilityAll, Actions: actions}
}

// defaultAppConfig returns the configuration an app has before anything is set.
func defaultAppConfig(appType AppType) AppConfig {
	catalog := pageCatalog(appType)
	pages := make([]AppPage, 0, len(catalog))
	for _, definition := range catalog {
		pages = append(pages, defaultPage(definition))
	}

	// The Account App reads like a personal account page, with its pages beside the content.
	layout := NavigationLayoutTopBar
	if appType == AppTypeAccount {
		layout = NavigationLayoutSideBar
	}

	return AppConfig{
		Type:       appType,
		Pages:      pages,
		Navigation: AppNavigation{Layout: layout},
		Content:    AppContent{Width: ContentWidthContained},
		Branding:   AppBranding{ShowSecuredBy: true},
	}
}
