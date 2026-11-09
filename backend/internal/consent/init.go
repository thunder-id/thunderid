// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package consent provides the consent persistence and service layer.
package consent

import "github.com/thunder-id/thunderid/internal/system/eventbus"

// Initialize constructs the consent service.
//
// bus is required: the service publishes the audit event of every consent it creates or updates on it.
func Initialize(
	inboundClientProvider InboundClientProvider, bus eventbus.EventBusInterface,
) (ConsentServiceInterface, error) {
	if bus == nil {
		return nil, errNilEventBus
	}
	return newConsentService(inboundClientProvider, bus)
}
