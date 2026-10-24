// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package systemflow is the contract between the services that change what a principal may do and
// the flow engine that revokes what such a change invalidates. A service runs the system flow for its
// operation before it writes, whichever entry point asked for the change: an administration flow, a
// management API, the import API or an internal caller. The package holds no implementation, so a
// service depends on it without depending on the engine, and the engine on it without depending on
// the service.
package systemflow

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Operation names a change that must run its system flow before it is written.
type Operation string

const (
	// ApplicationDelete removes an application: the tokens issued to it are revoked and the sessions
	// it took part in are detached from it before the record goes.
	ApplicationDelete Operation = "application.delete"
	// ApplicationSecretRotate replaces an application's client secret: the tokens issued to it
	// before the change are revoked, so a secret that leaked takes what it issued with it. Run before
	// the write, it revokes what was issued until then; run again after, it moves that line past the
	// write, so a token issued with the old secret in between is revoked too.
	ApplicationSecretRotate Operation = "application.secret_rotate"
)

// Exposure says who may start a system flow. It is a property of the flow's definition, so a flow
// that must be reachable from outside says so in the build, and the rest stay internal by default.
type Exposure string

const (
	// ExposureInternal is the default: only a service starts the flow, in process, after it has
	// authorized the change itself. The flow execution API refuses it.
	ExposureInternal Exposure = "internal"
	// ExposureAPI marks a flow the flow execution API may start as well, behind the permission its
	// definition names, since a system flow has no node that could check the caller itself. No
	// built-in flow is marked so yet, and the API does not serve the mark yet.
	ExposureAPI Exposure = "api"
)

// TargetApplication is the target key naming the application an operation acts on. It is the input
// the application steps of a system flow read, so the two must agree.
const TargetApplication = "targetApplicationId"

// Runner runs an operation's system flow to completion, in process, with the target the change acts
// on. A flow that fails leaves the change unwritten: the caller returns the error and writes nothing.
type Runner interface {
	Run(ctx context.Context, op Operation, target map[string]string) *tidcommon.ServiceError
}

// noRevocation is the runner for a plane that issues no tokens and holds no sessions, so a change
// there invalidates nothing. It is chosen explicitly: a service with no runner at all refuses the
// change, so a server that forgets the wiring fails closed rather than deleting with artifacts left
// valid.
type noRevocation struct{}

// NoRevocation returns a Runner for a plane that issues no tokens and holds no sessions. The control
// plane is one: its applications are configuration, and the gateway a version is applied to revokes
// when its own import removes them.
func NoRevocation() Runner {
	return noRevocation{}
}

// Run does nothing: there is nothing on this plane to revoke.
func (noRevocation) Run(context.Context, Operation, map[string]string) *tidcommon.ServiceError {
	return nil
}
