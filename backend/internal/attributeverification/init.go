// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize builds the attribute verification service over the entity service, which stores the
// verification, and the entity type service, which declares the contact attributes.
func Initialize(entityService entity.EntityServiceInterface, entityTypeService entitytype.EntityTypeServiceInterface,
	observability providers.ObservabilityProvider) AttributeVerificationServiceInterface {
	return newAttributeVerificationService(entityService, entityTypeService, observability)
}
