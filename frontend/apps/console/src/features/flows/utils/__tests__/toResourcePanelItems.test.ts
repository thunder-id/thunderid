/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
