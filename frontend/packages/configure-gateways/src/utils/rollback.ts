// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {useTranslation} from 'react-i18next';
import type {ProjectStatus} from '../models/projectStatus';

/** What a project's rollback on a gateway would do, as its status says it. */
export interface ProjectRollback {
  /** The version the project's resources go back to. */
  to: {hash: string; number?: number};
  /**
   * Whether that version is the one the project runs now: the last deploy brought changes within it, so
   * the rollback undoes that deploy rather than going to an older version.
   */
  undo: boolean;
  /** How many resources the rollback changes, when the status says. */
  changes?: number;
}

/** A project's rollback on a gateway, or undefined when there is nothing to go back to. */
export function projectRollbackOf(status: ProjectStatus | undefined): ProjectRollback | undefined {
  const rollback = status?.rollback;
  if (!rollback?.available || !rollback.toVersion) return undefined;
  const current = status?.project.versions[0];
  const undo = rollback.sameVersion === true || current?.hash === rollback.toVersion.hash;
  return {to: rollback.toVersion, undo, ...(rollback.changes !== undefined ? {changes: rollback.changes} : {})};
}

type Translate = ReturnType<typeof useTranslation<'deploy'>>['t'];

/** What a project's rollback does, for its tooltip: an undo of the last deploy, or a step back. */
export function rollbackHint(t: Translate, rollback: ProjectRollback, version: string, name: string): string {
  const count = rollback.changes;
  if (rollback.undo) {
    return count === undefined
      ? t(
          'card.undoHintUncounted',
          "Puts this project's resources back as they were before the last deploy of {{version}}.",
          {version},
        )
      : t('card.undoHint', {
          version,
          count,
          defaultValue_one:
            "Puts this project's resources back as they were before the last deploy of {{version}} ({{count}} change).",
          defaultValue_other:
            "Puts this project's resources back as they were before the last deploy of {{version}} ({{count}} changes).",
        });
  }
  const hint = t(
    'card.projectRevertHint',
    "Put this project's applications and agents on {{name}} back one step. Shared resources and other projects stay as they are.",
    {name},
  );
  return count === undefined
    ? hint
    : `${hint} ${t('card.rollbackChanges', {
        count,
        defaultValue_one: '({{count}} change)',
        defaultValue_other: '({{count}} changes)',
      })}`;
}
