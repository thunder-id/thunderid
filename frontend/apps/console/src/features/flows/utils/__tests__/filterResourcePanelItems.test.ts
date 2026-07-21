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
import filterResourcePanelItems from '../filterResourcePanelItems';
import type {ResourcePanelListItem} from '../toResourcePanelItems';

const createItem = (label: string, overrides: Record<string, unknown> = {}): ResourcePanelListItem => ({
  id: label,
  resource: {
    resourceType: 'STEP',
    category: 'EXECUTOR',
    type: 'TASK_EXECUTION',
    display: {label, showOnResourcePanel: true},
    ...overrides,
  } as unknown as Resource,
});

const createItems = (): ResourcePanelListItem[] => [
  createItem('Generate OTP'),
  createItem('Google'),
  createItem('Text Input', {resourceType: 'ELEMENT', category: 'FIELD', type: 'TEXT_INPUT'}),
];

describe('filterResourcePanelItems', () => {
  it('should return items unchanged for a blank query', () => {
    const items = createItems();

    expect(filterResourcePanelItems(items, '')).toBe(items);
    expect(filterResourcePanelItems(items, '   ')).toBe(items);
  });

  it('should match items by label, case-insensitively', () => {
    const result = filterResourcePanelItems(createItems(), 'GOOGLE');

    expect(result.map((item) => item.resource.display.label)).toEqual(['Google']);
  });

  it('should match items by description', () => {
    const items = [
      createItem('Verify OTP', {
        display: {label: 'Verify OTP', description: 'Validate the one-time passcode', showOnResourcePanel: true},
      }),
    ];

    expect(filterResourcePanelItems(items, 'passcode')).toHaveLength(1);
  });

  it('should match items via capability synonyms', () => {
    expect(filterResourcePanelItems(createItems(), 'mfa').map((item) => item.resource.display.label)).toEqual([
      'Generate OTP',
    ]);
    expect(filterResourcePanelItems(createItems(), 'social').map((item) => item.resource.display.label)).toEqual([
      'Google',
    ]);
  });

  it('should match items by their kind', () => {
    expect(filterResourcePanelItems(createItems(), 'element').map((item) => item.resource.display.label)).toEqual([
      'Text Input',
    ]);
    expect(filterResourcePanelItems(createItems(), 'executor')).toHaveLength(2);
  });

  it('should require every term to match', () => {
    expect(filterResourcePanelItems(createItems(), 'generate otp')).toHaveLength(1);
    expect(filterResourcePanelItems(createItems(), 'generate google')).toHaveLength(0);
  });

  it('should return an empty list when nothing matches', () => {
    expect(filterResourcePanelItems(createItems(), 'zzz-no-match')).toHaveLength(0);
  });
});
