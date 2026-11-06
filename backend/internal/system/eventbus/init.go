// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventbus

// Initialize creates the event bus with the registry and the stopper that only the composition root holds.
func Initialize() (EventBusInterface, RegistryInterface, StopperInterface) {
	return newEventBusService()
}
