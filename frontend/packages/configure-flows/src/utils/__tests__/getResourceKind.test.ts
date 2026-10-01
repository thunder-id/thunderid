// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
