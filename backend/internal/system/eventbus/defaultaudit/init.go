// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package defaultaudit builds the event bus a server binary publishes audit events on, with the
// default sink that writes them to standard output.
package defaultaudit

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/eventbus/stdoutsink"
)

// stdoutSubscriberID is the stable subscriber ID of the stdout sink.
const stdoutSubscriberID = "audit-stdout"

// Initialize builds the event bus and subscribes the stdout sink when audit is on. With audit off the
// bus has no sink: an event is still validated by the bus, then dropped, so a publisher never checks
// whether audit is on.
func Initialize(cfg config.AuditConfig) (eventbus.EventBusInterface, eventbus.StopperInterface, error) {
	bus, registry, stopper := eventbus.Initialize()
	if !cfg.IsEnabled() {
		return bus, stopper, nil
	}

	if err := audit.Subscribe(registry, stdoutSubscriberID, stdoutsink.Initialize()); err != nil {
		stopper.Stop()
		return nil, nil, fmt.Errorf("failed to subscribe the audit stdout sink: %w", err)
	}
	return bus, stopper, nil
}
