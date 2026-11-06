// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {ProjectStatus} from '../../models/projectStatus';
import {projectRollbackOf, rollbackHint} from '../rollback';

const status = (rollback: ProjectStatus['rollback']): ProjectStatus => ({
  project: {versions: [{hash: 'h13', number: 13, resources: 2}], pending: 0, required: 0},
  shared: {pending: 0, byType: []},
  needed: {differing: 0},
  rollback,
});

// Fills a default text the way the console's translation does, picking the plural form by the count.
const t = ((key: string, second?: unknown, third?: unknown): string => {
  const options = (typeof second === 'object' ? second : third) as Record<string, unknown> | undefined;
  let text = typeof second === 'string' ? second : key;
  if (options && typeof options['count'] === 'number') {
    text = String(options['count'] === 1 ? options['defaultValue_one'] : options['defaultValue_other']);
  }
  return text.replace(/{{(\w+)}}/g, (_match: string, name: string) => {
    const value = options?.[name];
    return typeof value === 'string' || typeof value === 'number' ? String(value) : '';
  });
}) as unknown as Parameters<typeof rollbackHint>[0];

describe('projectRollbackOf', () => {
  it('has no rollback when the status offers none, or names no version', () => {
    expect(projectRollbackOf(undefined)).toBeUndefined();
    expect(projectRollbackOf(status({available: false, toVersion: {hash: 'h12'}}))).toBeUndefined();
    expect(projectRollbackOf(status({available: true}))).toBeUndefined();
  });

  it('is an undo when it stays on the version the project runs, said or seen', () => {
    expect(projectRollbackOf(status({available: true, toVersion: {hash: 'h13', number: 13}}))).toEqual({
      to: {hash: 'h13', number: 13},
      undo: true,
    });
    expect(
      projectRollbackOf(status({available: true, toVersion: {hash: 'h12'}, sameVersion: true, changes: 3})),
    ).toEqual({to: {hash: 'h12'}, undo: true, changes: 3});
  });

  it('goes back to an older version otherwise', () => {
    expect(projectRollbackOf(status({available: true, toVersion: {hash: 'h12', number: 12}, changes: 1}))).toEqual({
      to: {hash: 'h12', number: 12},
      undo: false,
      changes: 1,
    });
  });
});

describe('rollbackHint', () => {
  it('says an undo puts the resources back as they were before the last deploy, with how many when known', () => {
    expect(rollbackHint(t, {to: {hash: 'h13'}, undo: true, changes: 3}, 'v13', 'Dev')).toBe(
      "Puts this project's resources back as they were before the last deploy of v13 (3 changes).",
    );
    expect(rollbackHint(t, {to: {hash: 'h13'}, undo: true}, 'v13', 'Dev')).toBe(
      "Puts this project's resources back as they were before the last deploy of v13.",
    );
  });

  it('says a step back reaches only the project, with how many when known', () => {
    expect(rollbackHint(t, {to: {hash: 'h12'}, undo: false}, 'v12', 'Dev')).toBe(
      "Put this project's applications and agents on Dev back one step. Shared resources and other projects stay as they are.",
    );
    expect(rollbackHint(t, {to: {hash: 'h12'}, undo: false, changes: 1}, 'v12', 'Dev')).toMatch(/\(1 change\)$/);
  });
});
