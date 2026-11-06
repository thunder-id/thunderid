// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

// Audited event names. A publisher uses these constants, and auditedEvents lists what each may carry.
const (
	// EventUserCreate names an attempt to create a user.
	EventUserCreate = "user.create"
	// EventUserUpdate names an attempt to change a user.
	EventUserUpdate = "user.update"
	// EventUserDelete names an attempt to delete a user.
	EventUserDelete = "user.delete"
	// EventApplicationCreate names an attempt to create an application.
	EventApplicationCreate = "application.create"
	// EventApplicationUpdate names an attempt to change an application.
	EventApplicationUpdate = "application.update"
	// EventApplicationDelete names an attempt to delete an application.
	EventApplicationDelete = "application.delete"
)

const (
	// TargetTypeUser is the target type of an event about a user.
	TargetTypeUser = "user"
	// TargetTypeApplication is the target type of an event about an application.
	TargetTypeApplication = "application"
)

// auditedEvents maps every audited event to the detail keys it may carry, so what is audited, and what
// each event may record, is reviewed in one place. An event not listed here is never written.
var auditedEvents = map[string][]string{
	EventUserCreate: nil,
	EventUserUpdate: nil,
	EventUserDelete: nil,

	EventApplicationCreate: nil,
	EventApplicationUpdate: nil,
	EventApplicationDelete: nil,
}
