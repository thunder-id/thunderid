// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package event provides event models and types for the analytics system.
package event

import (
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// NewEvent creates a new Event with required fields.
// Additional data should be added using WithData().
func NewEvent(traceID string, eventType string, component string) *providers.Event {
	eventID, err := utils.GenerateUUIDv7()
	if err != nil {
		return &providers.Event{}
	}

	return &providers.Event{
		TraceID:          traceID,
		EventID:          eventID,
		Type:             eventType,
		Timestamp:        time.Now(),
		Component:        component,
		Status:           providers.StatusInProgress,
		ServerIdentifier: serverIdentifier(),
		Data:             make(map[string]interface{}),
	}
}

// serverIdentifier returns the identifier of the deployment that generates events, or an empty
// string when the server runtime is not initialized.
func serverIdentifier() string {
	if !config.IsServerRuntimeInitialized() {
		return ""
	}
	return config.GetServerRuntime().Config.Server.Identifier
}
