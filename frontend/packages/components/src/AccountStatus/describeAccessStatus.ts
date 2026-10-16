// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccessStatus, AccessStatusReport, LifecycleCategory} from '@thunderid/types';

type Translate = (key: string, fallback: string, values?: Record<string, string>) => string;

/**
 * Category wording for one hint, with the user's phrasing as the fallback.
 *
 * A category is absent from a map when the user wording already describes it, so a translator has
 * one string to keep rather than three copies of it.
 */
type CategoryHint = Partial<Record<LifecycleCategory, string>> & {fallback: string};

/**
 * What an active record can do. The hold states say what is withheld; this one says nothing is.
 */
const ACTIVE_HINT: CategoryHint = {
  agent: 'This agent can authenticate normally.',
  application: 'This application can authenticate normally.',
  fallback: 'This account can sign in normally.',
};

/**
 * What a suspension withholds, and what takes it off.
 *
 * **No mention of the record staying editable.** It does, and that is the point of the state, but
 * this is the status an operator reads to find out whether the identity can get in. The remediation
 * window is said where it is acted on, which is the suspend confirmation.
 */
const SUSPENDED_HINT: CategoryHint = {
  agent: 'The agent is suspended and cannot authenticate until an administrator unsuspends it.',
  application: 'The application is suspended and cannot authenticate until an administrator unsuspends it.',
  fallback: 'The account is suspended and cannot sign in until an administrator unsuspends it.',
};

/**
 * The line used when the read carried no detail, and so has to be true of both kinds of lock.
 */
const LOCKED_HINT: CategoryHint = {
  agent:
    'The agent is locked and cannot authenticate. A lock from failed attempts lifts on its own; a lock left after a suspension is removed stays until an administrator unlocks it.',
  fallback:
    'The account is locked and cannot sign in. A lock from failed sign-ins lifts on its own; a lock left after a suspension is removed stays until an administrator unlocks it or the user completes self-recovery.',
};

const LOCKED_FAILED_ATTEMPTS_HINT: CategoryHint = {
  agent:
    'The agent is locked after too many failed attempts. It unlocks on its own, or an administrator can unlock it now.',
  fallback:
    'The account is locked after too many failed sign-ins. It unlocks on its own, or an administrator can unlock it now.',
};

const LOCKED_SUSPENSION_RELEASED_HINT: CategoryHint = {
  agent: 'Suspension removed. The agent remains locked until an administrator unlocks it.',
  fallback:
    'Suspension removed. The account remains locked until an administrator unlocks it or the user completes self-recovery.',
};

/**
 * Resolves a hint for the category being rendered.
 *
 * A category with no wording of its own reads the shared key rather than a copy of it, so the two
 * cannot drift apart in a catalog.
 */
function hintFor(base: string, category: LifecycleCategory, wording: CategoryHint, t: Translate): string {
  const specific: string | undefined = wording[category];

  return specific === undefined
    ? t(`common:accountStatus.${base}`, wording.fallback)
    : t(`common:accountStatus.${base}.${category}`, specific);
}

/**
 * What the Console says about one effective status.
 */
export interface AccessStatusDescription {
  value: AccessStatus;
  label: string;
  hint: string;
  /** The operator's note on a suspension, already prefixed. Absent when none was recorded. */
  operatorNote?: string;
}

/**
 * Describes an effective status for the indicator and the banner, so both say the same thing.
 *
 * An absent status reads as active, matching the server: the field is omitted for the ordinary
 * case. An unrecognized value returns null; a build that does not know a status cannot describe it.
 */
export default function describeAccessStatus(
  status: AccessStatusReport | undefined,
  category: LifecycleCategory,
  t: Translate,
): AccessStatusDescription | null {
  const value = status?.value ?? 'ACTIVE';
  const details = status?.details;

  if (value === 'SUSPENDED') {
    // The operator's own note, shown only on authorized management reads, rendered as text.
    return {
      hint: hintFor('suspended.hint', category, SUSPENDED_HINT, t),
      label: t('common:accountStatus.suspended.label', 'Suspended'),
      operatorNote: details?.operatorNote
        ? t('common:accountStatus.suspended.operatorNote', 'Operator note: {{operatorNote}}', {
            operatorNote: details.operatorNote,
          })
        : undefined,
      value,
    };
  }

  if (value === 'LOCKED') {
    // The two locks differ in whether a clock will clear them. Every live lock has to carry an
    // expiry before the hint promises one; a permanent rung among them still holds the account.
    let hint: string = hintFor('locked.hint', category, LOCKED_HINT, t);
    if (details?.reason === 'SUSPENSION_RELEASE_LOCK') {
      hint = hintFor('locked.hint.suspensionReleased', category, LOCKED_SUSPENSION_RELEASED_HINT, t);
    } else if (
      details?.reason === 'FAILED_ATTEMPTS' &&
      (details.lockedScopes?.length ?? 0) > 0 &&
      (details.lockedScopes ?? []).every((locked) => Boolean(locked.expiresAt))
    ) {
      hint = hintFor('locked.hint.failedAttempts', category, LOCKED_FAILED_ATTEMPTS_HINT, t);
    }
    return {hint, label: t('common:accountStatus.locked.label', 'Locked'), value};
  }

  if (value === 'ACTIVE') {
    return {
      hint: hintFor('active.hint', category, ACTIVE_HINT, t),
      label: t('common:accountStatus.active.label', 'Active'),
      value,
    };
  }

  return null;
}
