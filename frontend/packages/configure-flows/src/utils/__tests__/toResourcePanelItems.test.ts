// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {Resource} from '../../models/resources';
import toResourcePanelItems from '../toResourcePanelItems';

const createResource = (label: string, overrides: Record<string, unknown> = {}): Resource =>
  ({
    resourceType: 'STEP',
    category: 'WORKFLOW',
    type: 'TASK_EXECUTION',
    display: {label, showOnResourcePanel: true},
    ...overrides,
  }) as unknown as Resource;

describe('toResourcePanelItems', () => {
  it('should wrap resources with section-prefixed ids', () => {
    const items = toResourcePanelItems([createResource('Rule')], 'steps');

    expect(items).toHaveLength(1);
    expect(items[0].id).toBe('steps-STEP-TASK_EXECUTION-rule');
    expect(items[0].resource.display.label).toBe('Rule');
  });

  it('should exclude resources hidden from the resource panel', () => {
    const items = toResourcePanelItems(
      [createResource('Visible'), createResource('Hidden', {display: {label: 'Hidden', showOnResourcePanel: false}})],
      'steps',
    );

    expect(items.map((item) => item.resource.display.label)).toEqual(['Visible']);
  });

  it('should assign unique ids to resources sharing a type and label', () => {
    const items = toResourcePanelItems([createResource('Identify User'), createResource('Identify User')], 'executors');

    const ids = items.map((item) => item.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('should return an empty list for undefined input', () => {
    expect(toResourcePanelItems(undefined, 'widgets')).toEqual([]);
  });
});
