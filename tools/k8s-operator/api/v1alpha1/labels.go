// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// InstanceRefLabel is the metadata.labels key a ThunderIDResource (any spec.resource_type — application,
// flow, theme, organization_unit, role, user_type, user, group,
// translation, etc..) uses to reference its owning ThunderIDInstance, instead of a spec
// field. Keeping it out of spec means adding a new resource_type never requires touching the
// operator's schema.
//
// An EnvironmentValues uses the same label to name the ThunderIDInstance that should mount its Secret:
// see findEnvironment in env_secret.go. Unlike a ThunderIDResource, the label is optional on an
// EnvironmentValues — its own reconcile (syncing spec.env into the Secret) doesn't need an owning
// instance at all, the label only controls whether some ThunderIDInstance auto-discovers it.
const InstanceRefLabel = "thunderid.io/instance"

// EnvironmentValuesAdoptLabel is the metadata.labels key a pre-existing, hand-written Secret must
// carry — set to the exact name of the EnvironmentValues that should manage it — before that
// EnvironmentValues's reconciler will touch it. Without a matching label, a Secret that already
// exists and was never previously written by this EnvironmentValues (no managedKeysAnnotation) is
// left alone and the reconcile fails instead: matching by Secret name alone would let anyone who
// can create an EnvironmentValues overwrite keys in any same-named Secret in the namespace,
// including ones they have no RBAC permission to edit directly. See upsertSecret in
// environment_controller.go.
const EnvironmentValuesAdoptLabel = "thunderid.io/environment-values"
