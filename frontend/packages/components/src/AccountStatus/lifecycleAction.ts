// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {TFunction} from 'i18next';

/**
 * The lifecycle operations an operator can run against an identity. Locking is automatic only.
 */
export type LifecycleOperation = 'unlock' | 'suspend' | 'unsuspend';

/**
 * The identity categories the lifecycle operations apply to.
 */
export type LifecycleCategory = 'user' | 'agent' | 'application';

/**
 * What the Actions menu offers: one operation, or a release and an unlock in one step.
 */
export type LifecycleAction = LifecycleOperation | 'unsuspendAndUnlock';

/**
 * The operations each action runs, in order. Unlock runs last to clear the lock unsuspend leaves.
 */
export const LIFECYCLE_ACTION_STEPS: Record<LifecycleAction, LifecycleOperation[]> = {
  suspend: ['suspend'],
  unlock: ['unlock'],
  unsuspend: ['unsuspend'],
  unsuspendAndUnlock: ['unsuspend', 'unlock'],
};

/**
 * The name of an action, used by the Actions menu and the confirm button.
 */
export function lifecycleActionLabel(action: LifecycleAction, t: TFunction): string {
  switch (action) {
    case 'unlock':
      return t('common:accountStatus.unlock.action', 'Unlock');
    case 'unsuspend':
      return t('common:accountStatus.unsuspend.action', 'Unsuspend');
    case 'unsuspendAndUnlock':
      return t('common:accountStatus.unsuspendAndUnlock.action', 'Unsuspend and unlock');
    default:
      return t('common:accountStatus.suspend.action', 'Suspend');
  }
}

/**
 * A multi-step action that failed after an earlier step had already changed the record.
 */
export class LifecycleActionIncompleteError extends Error {
  constructor(cause: unknown) {
    super('A later step of the lifecycle action failed', {cause});
    this.name = 'LifecycleActionIncompleteError';
  }
}
