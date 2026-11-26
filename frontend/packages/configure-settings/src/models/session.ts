// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ServerConfigLayers} from './responses';

/**
 * The value of the `session` server-config section, in whole seconds. A field left out takes the
 * section's built-in default.
 *
 * @public
 */
export interface SessionValue {
  /** How long a session lasts without activity. */
  idleTimeoutSeconds?: number;
  /** How long a session lasts after sign-in, however active it is. */
  absoluteTimeoutSeconds?: number;
}

/**
 * One of the session timeouts.
 *
 * @public
 */
export type SessionTimeoutField = 'idleTimeoutSeconds' | 'absoluteTimeoutSeconds';

/**
 * The session timeouts in the order the card shows them.
 *
 * @public
 */
export const SessionTimeoutFields: readonly SessionTimeoutField[] = ['idleTimeoutSeconds', 'absoluteTimeoutSeconds'];

/**
 * The `session` section as the Console reads it.
 *
 * @public
 */
export type SessionConfigResponse = ServerConfigLayers<SessionValue>;
