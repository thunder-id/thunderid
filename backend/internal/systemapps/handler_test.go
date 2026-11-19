// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package systemapps

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pageKeys(config AppConfig) []string {
	keys := make([]string, 0, len(config.Pages))
	for _, page := range config.Pages {
		keys = append(keys, page.Key)
	}
	return keys
}

func TestDecode_EmptyMeansNothingIsSet(t *testing.T) {
	handler := NewAppConfigHandler(AppTypeAdmin)

	for _, raw := range []json.RawMessage{nil, json.RawMessage("null")} {
		value, err := handler.Decode(raw)
		require.NoError(t, err)
		assert.Nil(t, value)
	}
}

func TestDecode_ParsesAConfiguration(t *testing.T) {
	value, err := NewAppConfigHandler(AppTypeAdmin).Decode(json.RawMessage(
		`{"navigation":{"layout":"SIDE_BAR"},"pages":[{"key":"users","enabled":false,"visibility":"ALL"}]}`))

	require.NoError(t, err)
	config := value.(AppConfig)
	assert.Equal(t, NavigationLayoutSideBar, config.Navigation.Layout)
	assert.False(t, config.Pages[0].Enabled)
}

func TestDecode_RejectsMalformedJSON(t *testing.T) {
	_, err := NewAppConfigHandler(AppTypeAdmin).Decode(json.RawMessage(`{"pages":`))

	assert.Error(t, err)
}

func TestMerge_DefaultsWhenNothingIsSet(t *testing.T) {
	admin := NewAppConfigHandler(AppTypeAdmin).Merge(nil, nil).(AppConfig)
	account := NewAppConfigHandler(AppTypeAccount).Merge(nil, nil).(AppConfig)

	assert.Equal(t, AppTypeAdmin, admin.Type)
	assert.Equal(t, []string{"users", "groups", "roles", "agents"}, pageKeys(admin))
	assert.Equal(t, NavigationLayoutTopBar, admin.Navigation.Layout)
	assert.Equal(t, map[string]bool{"invite": true, "create": true, "edit": true, "delete": false}, admin.Pages[0].Actions)
	assert.Equal(t, ContentWidthContained, admin.Content.Width)
	assert.Equal(t, AppTypeAccount, account.Type)
	assert.Equal(t, []string{"profile", "security"}, pageKeys(account))
	assert.Equal(t, NavigationLayoutSideBar, account.Navigation.Layout)
	assert.Equal(t, map[string]bool{"password": true}, account.Pages[1].Actions)
	assert.Equal(t, ContentWidthContained, account.Content.Width)
}

func TestMerge_PrefersTheWritableLayer(t *testing.T) {
	readOnly := AppConfig{Navigation: AppNavigation{Layout: NavigationLayoutSideBar}}
	writable := AppConfig{Navigation: AppNavigation{Layout: NavigationLayoutTopBar}, Branding: AppBranding{Title: "Acme"}}

	merged := NewAppConfigHandler(AppTypeAdmin).Merge(readOnly, writable).(AppConfig)

	assert.Equal(t, NavigationLayoutTopBar, merged.Navigation.Layout)
	assert.Equal(t, "Acme", merged.Branding.Title)
	assert.Equal(t, AppTypeAdmin, merged.Type)
}

func TestMerge_UsesTheReadOnlyLayerWhenNothingIsWritten(t *testing.T) {
	readOnly := AppConfig{Navigation: AppNavigation{Layout: NavigationLayoutSideBar}}

	merged := NewAppConfigHandler(AppTypeAdmin).Merge(readOnly, nil).(AppConfig)

	assert.Equal(t, NavigationLayoutSideBar, merged.Navigation.Layout)
}

func TestMerge_FillsInPagesAndWidthAConfigurationLeavesOut(t *testing.T) {
	writable := AppConfig{
		Navigation: AppNavigation{Layout: NavigationLayoutTopBar},
		Pages:      []AppPage{{Key: "roles", Enabled: false, Visibility: PageVisibilityAll}},
	}

	merged := NewAppConfigHandler(AppTypeAdmin).Merge(nil, writable).(AppConfig)

	assert.Equal(t, []string{"roles", "users", "groups", "agents"}, pageKeys(merged))
	assert.False(t, merged.Pages[0].Enabled)
	assert.Equal(t, ContentWidthContained, merged.Content.Width)
}

func TestValidate_AcceptsTheDefaults(t *testing.T) {
	for _, appType := range []AppType{AppTypeAdmin, AppTypeAccount} {
		assert.NoError(t, NewAppConfigHandler(appType).Validate(context.Background(), defaultAppConfig(appType), nil, nil))
	}
}

func TestValidate_RejectsAnotherAppsPages(t *testing.T) {
	assert.Error(t, NewAppConfigHandler(AppTypeAccount).Validate(context.Background(), defaultAppConfig(AppTypeAdmin), nil, nil))
}

func TestValidate_RejectsAnUnexpectedType(t *testing.T) {
	assert.Error(t, NewAppConfigHandler(AppTypeAdmin).Validate(context.Background(), "not a config", nil, nil))
}

func TestValidate_RejectsInvalidConfigurations(t *testing.T) {
	valid := func() AppConfig { return defaultAppConfig(AppTypeAdmin) }

	cases := map[string]func(*AppConfig){
		"navigation layout": func(c *AppConfig) { c.Navigation.Layout = "FLOATING" },
		"content width":     func(c *AppConfig) { c.Content.Width = "HALF" },
		"page of another app": func(c *AppConfig) {
			c.Pages = append(c.Pages, AppPage{Key: "profile", Visibility: PageVisibilityAll})
		},
		"repeated page": func(c *AppConfig) { c.Pages = append(c.Pages, c.Pages[0]) },
		"visibility":    func(c *AppConfig) { c.Pages[0].Visibility = "SOMETIMES" },
		"action":        func(c *AppConfig) { c.Pages[0].Actions["password"] = true },
		"theme ID":      func(c *AppConfig) { c.ThemeID = "theme; drop" },
		"logo scheme":   func(c *AppConfig) { c.Branding.Logo = &AppImage{Light: "javascript:alert(1)"} },
		"logo relative": func(c *AppConfig) { c.Branding.Logo = &AppImage{Dark: "/logo.svg"} },
		"favicon":       func(c *AppConfig) { c.Branding.Favicon = &AppImage{Light: "data:image/png;base64,AAAA"} },
		"link url":      func(c *AppConfig) { c.Branding.Links = []AppLink{{Label: "Terms", URL: "javascript:x"}} },
		"link label":    func(c *AppConfig) { c.Branding.Links = []AppLink{{URL: "https://acme.example/terms"}} },
		"many links": func(c *AppConfig) {
			for range 6 {
				c.Branding.Links = append(c.Branding.Links, AppLink{Label: "Terms", URL: "https://acme.example/terms"})
			}
		},
		"language tag": func(c *AppConfig) { c.Languages = &AppLanguages{Menu: true, Offered: []string{"en_US"}} },
		"default not offered": func(c *AppConfig) {
			c.Languages = &AppLanguages{Menu: true, Offered: []string{"en-US"}, Default: "fr"}
		},
		"repeated language": func(c *AppConfig) {
			c.Languages = &AppLanguages{Menu: true, Offered: []string{"en-US", "en-US"}}
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			config := valid()
			mutate(&config)

			assert.Error(t, NewAppConfigHandler(AppTypeAdmin).Validate(context.Background(), config, nil, nil))
		})
	}
}

func TestValidate_AcceptsAThemeID(t *testing.T) {
	config := defaultAppConfig(AppTypeAdmin)
	config.ThemeID = "0198c5e2-7a4b-7c1d-9f00-3a2b1c0d4e5f"

	assert.NoError(t, NewAppConfigHandler(AppTypeAdmin).Validate(context.Background(), config, nil, nil))
}

func TestValidate_AcceptsBrandingURLs(t *testing.T) {
	config := defaultAppConfig(AppTypeAdmin)
	config.Branding.Logo = &AppImage{
		Light: "https://cdn.example.com/admin-logo.svg",
		Dark:  "https://cdn.example.com/admin-logo-dark.svg",
	}
	config.Branding.Favicon = &AppImage{Light: "http://localhost:8090/favicon.png"}

	assert.NoError(t, NewAppConfigHandler(AppTypeAdmin).Validate(context.Background(), config, nil, nil))
}

func TestValidate_AcceptsFooterLinksAndALanguageMenu(t *testing.T) {
	config := defaultAppConfig(AppTypeAdmin)
	config.Branding.Links = []AppLink{
		{Label: "Privacy", URL: "https://acme.example/privacy"},
		{Label: "Terms", URL: "https://acme.example/terms"},
	}
	config.Languages = &AppLanguages{Menu: true, Offered: []string{"en-US", "es", "zh-Hant-TW"}, Default: "en-US"}

	assert.NoError(t, NewAppConfigHandler(AppTypeAdmin).Validate(context.Background(), config, nil, nil))
}
