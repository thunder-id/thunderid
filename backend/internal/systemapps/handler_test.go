// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package systemapps

import (
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
	assert.Equal(t, []string{"profile", "security"}, pageKeys(account))
	assert.Equal(t, NavigationLayoutSideBar, account.Navigation.Layout)
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
		handler := NewAppConfigHandler(appType)

		assert.NoError(t, handler.Validate(defaultAppConfig(appType), nil, nil))
	}
}

func TestValidate_RejectsAnUnexpectedType(t *testing.T) {
	assert.Error(t, NewAppConfigHandler(AppTypeAdmin).Validate("not a config", nil, nil))
}

func TestValidate_RejectsInvalidConfigurations(t *testing.T) {
	radius := 64
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
		"theme color": func(c *AppConfig) {
			c.AppTheme = &AppTheme{ColorSchemes: AppThemeColorSchemes{Light: AppThemeColors{Primary: "red;}"}}}
		},
		"theme font": func(c *AppConfig) { c.AppTheme = &AppTheme{FontFamily: "x; background: url(a)"} },
		"theme radius": func(c *AppConfig) {
			c.AppTheme = &AppTheme{BorderRadius: &radius}
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			config := valid()
			mutate(&config)

			assert.Error(t, NewAppConfigHandler(AppTypeAdmin).Validate(config, nil, nil))
		})
	}
}

func TestValidate_AcceptsAPlainAppTheme(t *testing.T) {
	radius := 8
	config := defaultAppConfig(AppTypeAccount)
	config.AppTheme = &AppTheme{
		BorderRadius: &radius,
		FontFamily:   "'Inter', sans-serif",
		ColorSchemes: AppThemeColorSchemes{
			Light: AppThemeColors{Primary: "#0f766e", Background: "rgb(255, 255, 255)"},
			Dark:  AppThemeColors{Primary: "#2DD4BF"},
		},
	}

	assert.NoError(t, NewAppConfigHandler(AppTypeAccount).Validate(config, nil, nil))
}
