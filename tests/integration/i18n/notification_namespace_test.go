// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package i18n

import "net/http"

// These tests lock in the behaviour of the `notification` namespace: a first-class namespace
// hardcoded in backend/internal/system/i18n/mgt/constants.go whose default keys/values are
// seeded from the bootstrap translation resource
// (backend/cmd/server/bootstrap/01-default-resources.yaml). They assert that:
//   1. the bootstrap-seeded defaults resolve over the (public) resolve endpoint, and
//   2. further keys/values can be added to the namespace through the management API,
//      both per-key and in bulk, and round-trip through resolve.
//
// Writes target testLanguage (fr-CA), which SetupTest/TearDownSuite clear, so they never
// disturb the en-US bootstrap seed that the first test reads.

// notificationNamespace mirrors mgt.NotificationNamespace.
const notificationNamespace = "notification"

// TestNotificationNamespaceServesBootstrapSeededDefaults verifies that a key seeded via the
// bootstrap translation resource resolves for the system language.
func (suite *I18nMgtAPITestSuite) TestNotificationNamespaceServesBootstrapSeededDefaults() {
	resolved := suite.resolveLanguage(systemLanguage, notificationNamespace)

	seeded := resolved.Translations[notificationNamespace]
	suite.Require().NotEmpty(seeded, "expected notification namespace to carry bootstrap-seeded defaults")
	suite.Equal("Notification Templates", seeded["templates.list.title"])
}

// TestNotificationNamespaceAcceptsSingleKeyViaAPI adds one new key through the single-key write
// endpoint and reads it back through the single-key resolve endpoint.
func (suite *I18nMgtAPITestSuite) TestNotificationNamespaceAcceptsSingleKeyViaAPI() {
	const key = "templates.editor.actions.preview.label"

	set := suite.setKeyOverride(testLanguage, notificationNamespace, key, "Preview")
	suite.Equal(notificationNamespace, set.Namespace)
	suite.Equal(key, set.Key)

	resolved := suite.resolveKey(testLanguage, notificationNamespace, key)
	suite.Equal("Preview", resolved.Value)
	suite.Equal(notificationNamespace, resolved.Namespace)
}

// TestNotificationNamespaceAcceptsBulkKeysViaAPI adds several keys through the bulk write endpoint
// and reads the whole namespace back through the namespace-filtered resolve endpoint.
func (suite *I18nMgtAPITestSuite) TestNotificationNamespaceAcceptsBulkKeysViaAPI() {
	added := map[string]string{
		"templates.email.fields.subject.label": "Email subject",
		"templates.sms.fields.body.label":      "SMS body",
		"templates.actions.send_test.label":    "Send test",
	}

	suite.setBulkOverrides(testLanguage, map[string]map[string]string{notificationNamespace: added})

	resolved := suite.resolveLanguage(testLanguage, notificationNamespace)
	got := resolved.Translations[notificationNamespace]
	suite.Require().NotEmpty(got, "expected the bulk-written notification keys to resolve")
	for key, value := range added {
		suite.Equal(value, got[key], "key %q should round-trip through the API", key)
	}
}

// TestNotificationNamespaceApiWriteIsRetrievable is an end-to-end guard that a freshly added
// notification key is immediately visible over the resolve endpoint (no restart / re-bootstrap).
func (suite *I18nMgtAPITestSuite) TestNotificationNamespaceApiWriteIsRetrievable() {
	const key = "templates.list.actions.refresh.label"

	suite.Require().Equal(http.StatusNotFound,
		suite.resolveKeyStatus(testLanguage, notificationNamespace, key),
		"key must not exist before it is written")

	suite.setKeyOverride(testLanguage, notificationNamespace, key, "Refresh")

	suite.Equal("Refresh", suite.resolveKey(testLanguage, notificationNamespace, key).Value)
}
