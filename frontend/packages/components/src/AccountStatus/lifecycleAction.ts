// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {LifecycleOperation} from '@thunderid/types';

/**
 * What the Actions menu offers: one operation, or a release and an unlock in one step.
 */
export type LifecycleAction = LifecycleOperation | 'unsuspendAndUnlock';

/**
 * The operations each action runs, in order. Unlock runs after unsuspend, which writes the
 * post-suspension lock that unlock then clears.
 */
export const LIFECYCLE_ACTION_STEPS: Record<LifecycleAction, LifecycleOperation[]> = {
  suspend: ['suspend'],
  unlock: ['unlock'],
  unsuspend: ['unsuspend'],
  unsuspendAndUnlock: ['unsuspend', 'unlock'],
};

/**
 * A multi-step action that failed after an earlier step had already changed the record.
 */
export class LifecycleActionIncompleteError extends Error {
  constructor(cause: unknown) {
    super(cause instanceof Error ? cause.message : String(cause), {cause});
    this.name = 'LifecycleActionIncompleteError';
  }
}
