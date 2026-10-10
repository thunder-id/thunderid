// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** One resource of the configuration a gateway runs. */
export interface AppliedResource {
  /** The type of the resource, as an export names it, e.g. `application`. */
  resourceType: string;
  /** The resource's identifier, or its name for one exported without an id. */
  id: string;
  /** What the resource's own read returns. */
  resource: unknown;
  /** The collections its own API serves apart from it, keyed by the path below the resource. */
  parts?: Record<string, unknown>;
}

/** The configuration a gateway runs: every resource of the version it last applied. */
export interface AppliedConfiguration {
  gatewayId: string;
  /** The hash of the version the gateway applied. Absent when nothing was applied. */
  version?: string;
  appliedAt?: string;
  resources: AppliedResource[];
  /** The version's documents that cannot be shown as their resource. */
  skipped?: {resourceType: string; id?: string; code: string; reason: string}[];
}

/** Whether a value is a variable, whose value can be read back, or a secret, whose value never is. */
export type ValueKind = 'variable' | 'secret';

/** A value as a gateway's store lists it. A secret's value is never listed. */
export interface StoredValue {
  name: string;
  value?: string;
  description?: string;
  /** Set on a secret that has a value. */
  exists?: boolean;
}
