// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useCapabilitiesContext from './useCapabilitiesContext';

/**
 * The parts of a record that decide whether it can change here: the server's own `isReadOnly`, and
 * `isOwnedByOU`, which is `false` for a record shared into this organization unit from another.
 *
 * @public
 */
export interface AccessibleRecord {
  isReadOnly?: boolean;
  isOwnedByOU?: boolean;
}

/**
 * What the signed-in admin may do with one record.
 *
 * @public
 */
export interface RecordAccess {
  /** The record can't be edited here: the server locks it, it is shared in, or the admin may not update. */
  readOnly: boolean;
  /** The record can be deleted here: the server doesn't lock it, it is owned here, and the admin may delete. */
  canDelete: boolean;
  /**
   * The record's runtime state (active, suspended) can change here: it is owned here and the admin may
   * update. The server's `isReadOnly` doesn't block this, since a declarative record's state isn't declared.
   */
  canChangeState: boolean;
}

/**
 * Whether a record is locked here whatever the admin may do: the server marks it read-only, or it is
 * shared into this organization unit from another.
 *
 * @public
 */
export function isRecordLocked(record: AccessibleRecord | null | undefined): boolean {
  return Boolean(record?.isReadOnly) || record?.isOwnedByOU === false;
}

/**
 * Decides whether the signed-in admin may edit, delete, or change the state of a record of `resource` (`app`, `role`, ...).
 * A record that isn't loaded yet is treated as editable, so call it before any early return.
 *
 * @public
 */
export default function useRecordAccess(resource: string, record: AccessibleRecord | null | undefined): RecordAccess {
  const {can} = useCapabilitiesContext();
  const locked = isRecordLocked(record);

  return {
    readOnly: locked || !can(resource, 'update'),
    canDelete: !locked && can(resource, 'delete'),
    canChangeState: record?.isOwnedByOU !== false && can(resource, 'update'),
  };
}
