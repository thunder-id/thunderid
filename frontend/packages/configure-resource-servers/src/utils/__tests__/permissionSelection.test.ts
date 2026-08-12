// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {ResourcePermissions} from '../../models/resource-server';
import {
  isPermissionSelected,
  togglePermission,
  mergePermissions,
  removePermissions,
  ancestorPermissions,
  getSubtreeSelectionState,
  arePermissionsEqual,
} from '../permissionSelection';

describe('permissionSelection utils', () => {
  const base: ResourcePermissions[] = [
    {resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:create']},
    {resourceServerId: 'rs-2', permissions: ['payments:refund']},
  ];

  describe('isPermissionSelected', () => {
    it('returns true when the permission exists for the server', () => {
      expect(isPermissionSelected(base, 'rs-1', 'bookings:create')).toBe(true);
    });

    it('returns false for an unknown permission or server', () => {
      expect(isPermissionSelected(base, 'rs-1', 'bookings:delete')).toBe(false);
      expect(isPermissionSelected(base, 'rs-9', 'bookings')).toBe(false);
    });
  });

  describe('ancestorPermissions', () => {
    it('lists every enclosing resource permission', () => {
      expect(ancestorPermissions('bookings:reservations:update', ':')).toEqual(['bookings', 'bookings:reservations']);
    });

    it('returns nothing for a flat permission', () => {
      expect(ancestorPermissions('list', ':')).toEqual([]);
    });

    it('honours the resource server delimiter', () => {
      expect(ancestorPermissions('flights-delete', '-')).toEqual(['flights']);
      expect(ancestorPermissions('flights-delete', ':')).toEqual([]);
    });

    it('returns nothing when the delimiter is unknown', () => {
      expect(ancestorPermissions('bookings:create', '')).toEqual([]);
    });
  });

  describe('togglePermission', () => {
    it('adds a permission for a new server', () => {
      const result = togglePermission([], 'rs-1', 'bookings', ':');
      expect(result).toEqual([{resourceServerId: 'rs-1', permissions: ['bookings']}]);
    });

    it('adds a permission to an existing server entry', () => {
      const result = togglePermission(base, 'rs-2', 'payments:charge', ':');
      expect(result.find((e) => e.resourceServerId === 'rs-2')?.permissions).toEqual([
        'payments:refund',
        'payments:charge',
      ]);
    });

    it('removes a permission that is already selected', () => {
      const result = togglePermission(base, 'rs-1', 'bookings', ':');
      expect(result.find((e) => e.resourceServerId === 'rs-1')?.permissions).toEqual(['bookings:create']);
    });

    it('also removes the resource permissions that would still confer it', () => {
      const list = [
        {
          resourceServerId: 'rs-1',
          permissions: ['bookings', 'bookings:reservations', 'bookings:reservations:update', 'payments:refund'],
        },
      ];
      const result = togglePermission(list, 'rs-1', 'bookings:reservations:update', ':');
      expect(result[0].permissions).toEqual(['payments:refund']);
    });

    it('strips ancestors under a non-colon delimiter', () => {
      const list = [{resourceServerId: 'rs-1', permissions: ['flights', 'flights-create', 'flights-delete']}];
      const result = togglePermission(list, 'rs-1', 'flights-delete', '-');
      expect(result[0].permissions).toEqual(['flights-create']);
    });

    it('leaves resources whose permission merely shares a prefix', () => {
      const list = [{resourceServerId: 'rs-1', permissions: ['bookings-archive', 'bookings:create']}];
      const result = togglePermission(list, 'rs-1', 'bookings:create', ':');
      expect(result[0].permissions).toEqual(['bookings-archive']);
    });

    it('does not strip ancestors when adding a permission', () => {
      const list = [{resourceServerId: 'rs-1', permissions: ['bookings']}];
      const result = togglePermission(list, 'rs-1', 'bookings:create', ':');
      expect(result[0].permissions).toEqual(['bookings', 'bookings:create']);
    });

    it('drops the server entry entirely when its last permission is removed', () => {
      const result = togglePermission(base, 'rs-2', 'payments:refund', ':');
      expect(result.find((e) => e.resourceServerId === 'rs-2')).toBeUndefined();
      expect(result).toHaveLength(1);
    });

    it('does not mutate the input', () => {
      const snapshot: ResourcePermissions[] = JSON.parse(JSON.stringify(base)) as ResourcePermissions[];
      togglePermission(base, 'rs-1', 'bookings', ':');
      expect(base).toEqual(snapshot);
    });
  });

  describe('mergePermissions', () => {
    it('appends entries for servers not present in the base', () => {
      const result = mergePermissions(base, [{resourceServerId: 'rs-3', permissions: ['notify:send']}]);
      expect(result).toHaveLength(3);
      expect(result.find((e) => e.resourceServerId === 'rs-3')?.permissions).toEqual(['notify:send']);
    });

    it('merges and de-duplicates permissions for an existing server', () => {
      const result = mergePermissions(base, [{resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:delete']}]);
      expect(result.find((e) => e.resourceServerId === 'rs-1')?.permissions).toEqual([
        'bookings',
        'bookings:create',
        'bookings:delete',
      ]);
    });

    it('does not mutate the inputs', () => {
      const snapshot: ResourcePermissions[] = JSON.parse(JSON.stringify(base)) as ResourcePermissions[];
      mergePermissions(base, [{resourceServerId: 'rs-1', permissions: ['x']}]);
      expect(base).toEqual(snapshot);
    });
  });

  describe('removePermissions', () => {
    it('removes a subset of permissions from the server entry', () => {
      const result = removePermissions(base, 'rs-1', ['bookings']);
      expect(result.find((e) => e.resourceServerId === 'rs-1')?.permissions).toEqual(['bookings:create']);
    });

    it('drops the server entry entirely when all its permissions are removed', () => {
      const result = removePermissions(base, 'rs-2', ['payments:refund']);
      expect(result.find((e) => e.resourceServerId === 'rs-2')).toBeUndefined();
      expect(result).toHaveLength(1);
    });

    it('returns the original list when the server is not found', () => {
      const result = removePermissions(base, 'rs-99', ['bookings']);
      expect(result).toBe(base);
    });

    it('does not mutate the input', () => {
      const snapshot: ResourcePermissions[] = JSON.parse(JSON.stringify(base)) as ResourcePermissions[];
      removePermissions(base, 'rs-1', ['bookings']);
      expect(base).toEqual(snapshot);
    });
  });
});

describe('getSubtreeSelectionState', () => {
  const list = [{resourceServerId: 'rs-1', permissions: ['bookings', 'bookings:create']}];

  it('returns all when every subtree permission is selected', () => {
    expect(getSubtreeSelectionState(list, 'rs-1', ['bookings', 'bookings:create'])).toBe('all');
  });

  it('returns some when only part of the subtree is selected', () => {
    expect(getSubtreeSelectionState(list, 'rs-1', ['bookings', 'bookings:create', 'bookings:delete'])).toBe('some');
  });

  it('returns none when nothing in the subtree is selected', () => {
    expect(getSubtreeSelectionState(list, 'rs-1', ['payments:refund'])).toBe('none');
  });

  it('returns none for an empty subtree', () => {
    expect(getSubtreeSelectionState(list, 'rs-1', [])).toBe('none');
  });

  it('scopes matching to the given resource server', () => {
    expect(getSubtreeSelectionState(list, 'rs-2', ['bookings'])).toBe('none');
  });
});

describe('arePermissionsEqual', () => {
  it('treats reordered servers and permissions as equal', () => {
    const a = [
      {resourceServerId: 'rs-1', permissions: ['x', 'y']},
      {resourceServerId: 'rs-2', permissions: ['z']},
    ];
    const b = [
      {resourceServerId: 'rs-2', permissions: ['z']},
      {resourceServerId: 'rs-1', permissions: ['y', 'x']},
    ];
    expect(arePermissionsEqual(a, b)).toBe(true);
  });

  it('detects differing permissions', () => {
    expect(
      arePermissionsEqual(
        [{resourceServerId: 'rs-1', permissions: ['x']}],
        [{resourceServerId: 'rs-1', permissions: ['x', 'y']}],
      ),
    ).toBe(false);
  });

  it('detects differing server sets', () => {
    expect(arePermissionsEqual([{resourceServerId: 'rs-1', permissions: ['x']}], [])).toBe(false);
  });

  it('treats two empty lists as equal', () => {
    expect(arePermissionsEqual([], [])).toBe(true);
  });
});
