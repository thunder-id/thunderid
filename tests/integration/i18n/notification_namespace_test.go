// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package i18n

import "net/http"

// These tests lock in the behaviour of the `notification` namespace: a first-class namespace
// hardcoded in backend/internal/system/i18n/mgt/constants.go. They assert that keys/values can be
// added to the namespace through an import, one or several at a time, and round-trip through resolve.
//
// Writes target testLanguage (fr-CA), which SetupTest/TearDownSuite clear.

// notificationNamespace mirrors mgt.NotificationNamespace.
const notificationNamespace = "notification"

// TestNotificationNamespaceAcceptsSingleKey adds one new key and reads it back through the single-key
// resolve endpoint.
func (suite *I18nMgtAPITestSuite) TestNotificationNamespaceAcceptsSingleKey() {
	const key = "templates.editor.actions.preview.label"

	suite.setOverrides(testLanguage, map[string]map[string]string{notificationNamespace: {key: "Preview"}})

	resolved := suite.resolveKey(testLanguage, notificationNamespace, key)
	suite.Equal("Preview", resolved.Value)
	suite.Equal(notificationNamespace, resolved.Namespace)
}

// TestNotificationNamespaceAcceptsBulkKeys adds several keys and reads the whole namespace back through
// the namespace-filtered resolve endpoint.
func (suite *I18nMgtAPITestSuite) TestNotificationNamespaceAcceptsBulkKeys() {
	added := map[string]string{
		"templates.email.fields.subject.label": "Email subject",
		"templates.sms.fields.body.label":      "SMS body",
		"templates.actions.send_test.label":    "Send test",
	}

	suite.setOverrides(testLanguage, map[string]map[string]string{notificationNamespace: added})

	resolved := suite.resolveLanguage(testLanguage, notificationNamespace)
	got := resolved.Translations[notificationNamespace]
	suite.Require().NotEmpty(got, "expected the bulk-written notification keys to resolve")
	for key, value := range added {
		suite.Equal(value, got[key], "key %q should round-trip through import", key)
	}
}

// TestNotificationNamespaceWriteIsRetrievable is an end-to-end guard that a freshly added notification
// key is immediately visible over the resolve endpoint (no restart / re-bootstrap).
func (suite *I18nMgtAPITestSuite) TestNotificationNamespaceWriteIsRetrievable() {
	const key = "templates.list.actions.refresh.label"

	suite.Require().Equal(http.StatusNotFound,
		suite.resolveKeyStatus(testLanguage, notificationNamespace, key),
		"key must not exist before it is written")

	suite.setOverrides(testLanguage, map[string]map[string]string{notificationNamespace: {key: "Refresh"}})

	suite.Equal("Refresh", suite.resolveKey(testLanguage, notificationNamespace, key).Value)
}
