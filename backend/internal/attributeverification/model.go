// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// storedContact is an entity's stored contact: the pin of its canonical value and the entity category and
// type the record compares under the entity lock.
type storedContact struct {
	pin        providers.AttributeVerificationPin
	category   providers.EntityCategory
	entityType string
}
