// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act} from 'react';
import {createRoot} from 'react-dom/client';
import {describe, expect, it} from 'vitest';
import CapabilitiesProvider from '../CapabilitiesProvider';
import useRecordAccess, {isRecordLocked, type AccessibleRecord, type RecordAccess} from '../useRecordAccess';

function Probe({record}: {record: AccessibleRecord | undefined}) {
  return <output>{JSON.stringify(useRecordAccess('role', record))}</output>;
}

function accessFor(record: AccessibleRecord | undefined, granted: string[]): RecordAccess {
  const container = document.createElement('div');
  const root = createRoot(container);
  act(() => {
    root.render(
      <CapabilitiesProvider
        capabilities={{can: (resource, action = 'view') => granted.includes(`${resource}:${action}`), isLoading: false}}
      >
        <Probe record={record} />
      </CapabilitiesProvider>,
    );
  });
  const access = JSON.parse(container.textContent ?? '') as RecordAccess;
  act(() => root.unmount());
  return access;
}

describe('useRecordAccess', () => {
  it('allows editing and deleting an owned record when the admin may update and delete', () => {
    expect(accessFor({}, ['role:update', 'role:delete'])).toEqual({
      readOnly: false,
      canDelete: true,
      canChangeState: true,
    });
  });

  it('follows the capabilities for an owned record', () => {
    expect(accessFor({}, [])).toEqual({readOnly: true, canDelete: false, canChangeState: false});
    expect(accessFor({}, ['role:delete'])).toEqual({readOnly: true, canDelete: true, canChangeState: false});
  });

  it('locks a record the server marks read-only', () => {
    expect(accessFor({isReadOnly: true}, ['role:update', 'role:delete'])).toEqual({
      readOnly: true,
      canDelete: false,
      canChangeState: true,
    });
  });

  it('locks a record shared in from another organization unit', () => {
    expect(accessFor({isOwnedByOU: false}, ['role:update', 'role:delete'])).toEqual({
      readOnly: true,
      canDelete: false,
      canChangeState: false,
    });
  });

  it('treats a record that is not loaded as owned and unlocked', () => {
    expect(accessFor(undefined, ['role:update', 'role:delete'])).toEqual({
      readOnly: false,
      canDelete: true,
      canChangeState: true,
    });
  });

  it('reports a record as locked when the server marks it read-only or it is shared in', () => {
    expect(isRecordLocked({isReadOnly: true})).toBe(true);
    expect(isRecordLocked({isOwnedByOU: false})).toBe(true);
    expect(isRecordLocked({isOwnedByOU: true})).toBe(false);
    expect(isRecordLocked(undefined)).toBe(false);
  });
});
