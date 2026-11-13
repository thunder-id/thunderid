// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize builds the attribute verification service over the entity service, which stores the
// verification, the entity type service, which declares the contact attributes, and the authorizer, which
// confines a caller to the users it may update. bus is required: the service publishes the audit event of
// every verification it requests or records on it.
func Initialize(entityService entity.EntityServiceInterface, entityTypeService entitytype.EntityTypeServiceInterface,
	observability providers.ObservabilityProvider, authorizer sysauthz.SystemAuthorizationServiceInterface,
	bus eventbus.EventBusInterface) (AttributeVerificationServiceInterface, error) {
	if bus == nil {
		return nil, errNilEventBus
	}
	return newAttributeVerificationService(entityService, entityTypeService, observability, authorizer, bus), nil
}
