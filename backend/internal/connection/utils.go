// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// smsVendorName returns the connection vendor name for a message provider, or false when the
// provider has no registered vendor (such instances are not exposed by /connections).
func smsVendorName(provider ncommon.NotificationProviderType) (string, bool) {
	for _, vendor := range smsBackedVendors {
		if vendor.provider == provider {
			return vendor.name, true
		}
	}
	return "", false
}

// emailVendorName returns the connection vendor name for an email provider, or false when the
// provider has no registered vendor (such instances are not exposed by /connections).
func emailVendorName(provider ncommon.NotificationProviderType) (string, bool) {
	for _, vendor := range emailBackedVendors {
		if vendor.provider == provider {
			return vendor.name, true
		}
	}
	return "", false
}

// idpVendorName returns the connection vendor name for an identity-provider type, or false
// when the type has no registered vendor (such instances are not exposed by /connections).
func idpVendorName(idpType providers.IDPType) (string, bool) {
	for _, vendor := range idpBackedVendors {
		if vendor.idpType == idpType {
			return vendor.name, true
		}
	}
	return "", false
}

// isIDPBackedVendorName reports whether name is a registered IdP-backed vendor's connection
// name (e.g. "google").
func isIDPBackedVendorName(name string) bool {
	for _, vendor := range idpBackedVendors {
		if vendor.name == name {
			return true
		}
	}
	return false
}
