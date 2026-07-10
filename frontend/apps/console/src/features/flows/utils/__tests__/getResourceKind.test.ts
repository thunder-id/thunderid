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
import getResourceKind, {ResourceKinds} from '../getResourceKind';

const createResource = (overrides: Record<string, unknown> = {}): Resource =>
  ({
    resourceType: 'STEP',
    category: 'WORKFLOW',
    type: 'TASK_EXECUTION',
    display: {label: 'Test', showOnResourcePanel: true},
    ...overrides,
  }) as unknown as Resource;

describe('getResourceKind', () => {
  it('should identify widgets', () => {
    expect(getResourceKind(createResource({resourceType: 'WIDGET'}))).toBe(ResourceKinds.Widget);
  });

  it('should identify elements', () => {
    expect(getResourceKind(createResource({resourceType: 'ELEMENT', category: 'FIELD', type: 'TEXT_INPUT'}))).toBe(
      ResourceKinds.Element,
    );
  });

  it('should identify executors by their category', () => {
    expect(getResourceKind(createResource({category: 'EXECUTOR'}))).toBe(ResourceKinds.Executor);
  });

  it('should identify views by their type', () => {
    expect(getResourceKind(createResource({category: 'INTERFACE', type: 'VIEW'}))).toBe(ResourceKinds.View);
  });

  it('should fall back to step for other step types', () => {
    expect(getResourceKind(createResource({category: 'DECISION', type: 'RULE'}))).toBe(ResourceKinds.Step);
    expect(getResourceKind(createResource({category: 'WORKFLOW', type: 'CALL'}))).toBe(ResourceKinds.Step);
  });
});
