// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {readProjectStatus} from '../useProjectStatus';

describe('readProjectStatus', () => {
  it('lifts when, who and the rollback out of the project, and reads the last auto-sync run', () => {
    const status = readProjectStatus({
      project: {
        versions: [{hash: 'h6', number: 6, resources: 1}],
        pending: 0,
        required: 0,
        deployedAt: '2026-11-06T13:09:21Z',
        deployedBy: {id: 'u1', name: 'Jerad'},
        rollback: {available: true, toVersion: {hash: 'h4', number: 4}},
      },
      shared: {pending: 3, byType: []},
      needed: {differing: 0},
      autoSync: {
        enabled: true,
        inherited: false,
        lastRun: {at: '2026-11-06T13:10:00Z', version: {hash: 'h6', number: 6}, error: {message: 'Refused'}},
      },
    });

    expect(status.deployedAt).toBe('2026-11-06T13:09:21Z');
    expect(status.deployedBy).toEqual({id: 'u1', name: 'Jerad'});
    expect(status.rollback).toEqual({available: true, toVersion: {hash: 'h4', number: 4}});
    expect(status.project).toEqual({versions: [{hash: 'h6', number: 6, resources: 1}], pending: 0, required: 0});
    expect(status.autoSync?.lastRun).toEqual({at: '2026-11-06T13:10:00Z', version: 'h6', error: 'Refused'});
  });
});
