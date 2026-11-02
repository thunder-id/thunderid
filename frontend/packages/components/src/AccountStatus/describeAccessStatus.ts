// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccessStatus, AccessStatusReport} from '@thunderid/types';
import type {TFunction} from 'i18next';
import type {LifecycleCategory} from './lifecycleAction';

/**
 * The effective access statuses a status read reports.
 */
export const AccessStatuses = {
  ACTIVE: 'ACTIVE',
  LOCKED: 'LOCKED',
  SUSPENDED: 'SUSPENDED',
} as const satisfies Record<AccessStatus, AccessStatus>;

/** The lockedMethods entry for a lock on every method. */
const ACCOUNT_METHOD = 'account';

function activeHint(category: LifecycleCategory, t: TFunction): string {
  switch (category) {
    case 'agent':
      return t('common:accountStatus.active.hint.agent', 'This agent can authenticate normally.');
    case 'application':
      return t('common:accountStatus.active.hint.application', 'This application can authenticate normally.');
    default:
      return t('common:accountStatus.active.hint.user', 'This account can sign in normally.');
  }
}

function suspendedHint(category: LifecycleCategory, t: TFunction): string {
  switch (category) {
    case 'agent':
      return t(
        'common:accountStatus.suspended.hint.agent',
        'The agent is suspended and cannot authenticate until an administrator unsuspends it.',
      );
    case 'application':
      return t(
        'common:accountStatus.suspended.hint.application',
        'The application is suspended and cannot authenticate until an administrator unsuspends it.',
      );
    default:
      return t(
        'common:accountStatus.suspended.hint.user',
        'The account is suspended and cannot sign in until an administrator unsuspends it.',
      );
  }
}

// Applications are never locked, so the lock wording is the agent's or the user's.

function lockedHint(category: LifecycleCategory, t: TFunction): string {
  return category === 'agent'
    ? t(
        'common:accountStatus.locked.hint.agent',
        'The agent is locked and cannot authenticate. An administrator can unlock it.',
      )
    : t(
        'common:accountStatus.locked.hint.user',
        'The account is locked and cannot sign in. The user can complete account recovery, or an administrator can unlock the account.',
      );
}

function lockedFailedAttemptsHint(category: LifecycleCategory, t: TFunction): string {
  return category === 'agent'
    ? t(
        'common:accountStatus.locked.hint.failedAttempts.agent',
        'The agent was locked after too many failed authentication attempts. It unlocks automatically at the time shown below. An administrator unlock also removes the lock.',
      )
    : t(
        'common:accountStatus.locked.hint.failedAttempts.user',
        'The account was locked after too many failed sign-in attempts. It unlocks automatically at the time shown below. Account recovery or an administrator unlock also removes the lock.',
      );
}

function lockedFailedAttemptsPermanentHint(category: LifecycleCategory, t: TFunction): string {
  return category === 'agent'
    ? t(
        'common:accountStatus.locked.hint.failedAttemptsPermanent.agent',
        'The agent was locked after too many failed authentication attempts. It stays locked until an administrator unlocks it.',
      )
    : t(
        'common:accountStatus.locked.hint.failedAttemptsPermanent.user',
        'The account was locked after too many failed sign-in attempts. It stays locked until the user completes account recovery or an administrator unlocks it.',
      );
}

function lockedSomeMethodsTitle(category: LifecycleCategory, t: TFunction): string {
  return category === 'agent'
    ? t('common:accountStatus.locked.title.someMethods.agent', 'Authentication locked for some methods')
    : t('common:accountStatus.locked.title.someMethods.user', 'Sign-in locked for some methods');
}

function lockedSomeMethodsHint(category: LifecycleCategory, t: TFunction): string {
  return category === 'agent'
    ? t(
        'common:accountStatus.locked.hint.someMethods.agent',
        'Some authentication methods were locked after too many failed attempts. Other authentication methods are not affected. Each lock below shows when it ends. An administrator unlock removes them.',
      )
    : t(
        'common:accountStatus.locked.hint.someMethods.user',
        'Some sign-in methods were locked after too many failed attempts. Other sign-in methods are not affected. Each lock below shows when it ends. Account recovery or an administrator unlock removes them.',
      );
}

/**
 * The name of a locked sign-in method. Only stored credential and one-time password locks form; any other
 * value is shown as sent.
 */
function methodName(method: string, t: TFunction): string {
  switch (method) {
    case 'credential':
      return t('common:accountStatus.method.credential', 'Password or other stored credential');
    case 'otp':
      return t('common:accountStatus.method.otp', 'One-time password');
    default:
      return method;
  }
}

/**
 * One live lock as a line: the method, unless it is every method, and when the lock ends. A lock on every
 * method with no expiry has no line, since the hint already says how it ends.
 */
function lockLine(method: string, expiresAt: string | undefined, t: TFunction): string | undefined {
  if (method === ACCOUNT_METHOD) {
    return expiresAt
      ? t('common:accountStatus.locked.until', 'Locked until {{expiresAt}}', {
          expiresAt: new Date(expiresAt).toLocaleString(),
        })
      : undefined;
  }
  const name = methodName(method, t);

  return expiresAt
    ? t('common:accountStatus.locked.method.until', '{{method}}: locked until {{expiresAt}}', {
        expiresAt: new Date(expiresAt).toLocaleString(),
        method: name,
      })
    : t('common:accountStatus.locked.method.untilReleased', '{{method}}: locked until released', {method: name});
}

/**
 * What the Console says about one effective status.
 */
export interface AccessStatusDescription {
  value: AccessStatus;
  label: string;
  /** The banner's headline; differs from `label` when only some methods are locked. */
  title: string;
  hint: string;
  /** One line per live lock, saying when it ends. Absent for a status that is not a lock. */
  locks?: string[];
  /** The operator's note on a suspension, already prefixed. Absent when none was recorded. */
  operatorNote?: string;
}

/**
 * Describes an effective status for the indicator and the banner. An absent status reads as active, as
 * the server omits it for the ordinary case; an unknown value returns null.
 */
export default function describeAccessStatus(
  status: AccessStatusReport | undefined,
  category: LifecycleCategory,
  t: TFunction,
): AccessStatusDescription | null {
  const value = status?.value ?? AccessStatuses.ACTIVE;
  const details = status?.details;

  if (value === AccessStatuses.SUSPENDED) {
    const label = t('common:accountStatus.suspended.label', 'Suspended');

    return {
      hint: suspendedHint(category, t),
      label,
      title: label,
      operatorNote: details?.operatorNote
        ? t('common:accountStatus.suspended.operatorNote', 'Operator note: {{operatorNote}}', {
            operatorNote: details.operatorNote,
          })
        : undefined,
      value,
    };
  }

  if (value === AccessStatuses.LOCKED) {
    const label = t('common:accountStatus.locked.label', 'Locked');
    const lockedMethods = details?.lockedMethods ?? [];
    // An `account` entry locks every method; a report without entries reads the same way.
    const wholeAccount = lockedMethods.length === 0 || lockedMethods.some(({method}) => method === ACCOUNT_METHOD);
    const locks = lockedMethods
      .map(({expiresAt, method}) => lockLine(method, expiresAt, t))
      .filter((line): line is string => line !== undefined);

    if (!wholeAccount) {
      return {
        hint: lockedSomeMethodsHint(category, t),
        label,
        locks,
        title: lockedSomeMethodsTitle(category, t),
        value,
      };
    }

    let hint: string = lockedHint(category, t);
    if (details?.reason === 'SUSPENSION_RELEASE_LOCK') {
      // Only a user is left locked by an unsuspend.
      hint = t(
        'common:accountStatus.locked.hint.suspensionReleased.user',
        'Suspension removed. The account remains locked until the user completes account recovery or an administrator unlocks it.',
      );
    } else if (details?.reason === 'FAILED_ATTEMPTS' && lockedMethods.length > 0) {
      // Promise that it unlocks on its own only when every live lock expires.
      hint = lockedMethods.every((locked) => Boolean(locked.expiresAt))
        ? lockedFailedAttemptsHint(category, t)
        : lockedFailedAttemptsPermanentHint(category, t);
    }
    return {hint, label, locks, title: label, value};
  }

  if (value === AccessStatuses.ACTIVE) {
    const label = t('common:accountStatus.active.label', 'Active');

    return {hint: activeHint(category, t), label, title: label, value};
  }

  return null;
}
