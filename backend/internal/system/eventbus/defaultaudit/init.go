// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package defaultaudit subscribes the default audit sink, which writes audit events to standard output,
// on the event bus of a server binary.
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

// Initialize subscribes the stdout sink to audit events on the bus that registry belongs to, when audit
// is on. With audit off nothing subscribes: an audit event is still validated by the bus, then dropped,
// so a publisher never checks whether audit is on.
func Initialize(cfg config.AuditConfig, registry eventbus.RegistryInterface) error {
	if !cfg.IsEnabled() {
		return nil
	}
	if err := audit.Subscribe(registry, stdoutSubscriberID, stdoutsink.Initialize()); err != nil {
		return fmt.Errorf("failed to subscribe the audit stdout sink: %w", err)
	}
	return nil
}
