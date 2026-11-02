// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {Element} from '../../models/elements';
import updateNestedComponent, {findContainingComponent} from '../updateNestedComponent';

describe('updateNestedComponent', () => {
  it('should update a top-level component', () => {
    const components = [{id: 'stack-1', type: 'STACK', components: []}] as unknown as Element[];

    const result = updateNestedComponent(components, 'stack-1', (target) => ({
      ...target,
      components: [...(target.components ?? []), {id: 'new-1', type: 'BUTTON'} as unknown as Element],
    }));

    expect(result[0].components).toHaveLength(1);
    expect(result[0].components?.[0].id).toBe('new-1');
  });

  it('should update a component nested inside another container', () => {
    const components = [
      {
        id: 'form-1',
        type: 'BLOCK',
        components: [{id: 'stack-1', type: 'STACK', components: []}],
      },
    ] as unknown as Element[];

    const result = updateNestedComponent(components, 'stack-1', (target) => ({
      ...target,
      components: [...(target.components ?? []), {id: 'new-1', type: 'BUTTON'} as unknown as Element],
    }));

    const form = result[0];
    const stack = form.components?.[0];
    expect(stack?.id).toBe('stack-1');
    expect(stack?.components).toHaveLength(1);
    expect(stack?.components?.[0].id).toBe('new-1');
  });

  it('should update a component nested multiple levels deep', () => {
    const components = [
      {
        id: 'form-1',
        type: 'BLOCK',
        components: [
          {
            id: 'stack-1',
            type: 'STACK',
            components: [{id: 'stack-2', type: 'STACK', components: []}],
          },
        ],
      },
    ] as unknown as Element[];

    const result = updateNestedComponent(components, 'stack-2', (target) => ({
      ...target,
      components: [{id: 'new-1', type: 'IMAGE'} as unknown as Element],
    }));

    const innerStack = result[0].components?.[0].components?.[0];
    expect(innerStack?.id).toBe('stack-2');
    expect(innerStack?.components).toEqual([{id: 'new-1', type: 'IMAGE'}]);
  });

  it('should leave the tree unchanged when the target id does not exist', () => {
    const components = [
      {id: 'form-1', type: 'BLOCK', components: [{id: 'stack-1', type: 'STACK', components: []}]},
    ] as unknown as Element[];

    const result = updateNestedComponent(components, 'non-existent', (target) => target);

    expect(result).toEqual(components);
  });

  it('should not modify sibling containers that do not hold the target', () => {
    const components = [
      {id: 'form-1', type: 'BLOCK', components: [{id: 'input-1', type: 'TEXT_INPUT'}]},
      {id: 'stack-1', type: 'STACK', components: [{id: 'button-1', type: 'BUTTON'}]},
    ] as unknown as Element[];

    const result = updateNestedComponent(components, 'stack-1', (target) => ({
      ...target,
      components: [...(target.components ?? []), {id: 'new-1', type: 'IMAGE'} as unknown as Element],
    }));

    const form = result.find((c) => c.id === 'form-1');
    expect(form?.components).toEqual([{id: 'input-1', type: 'TEXT_INPUT'}]);
  });
});

describe('findContainingComponent', () => {
  it('should find the top-level container holding the child', () => {
    const components = [
      {id: 'stack-1', type: 'STACK', components: [{id: 'button-1', type: 'BUTTON'}]},
    ] as unknown as Element[];

    const result = findContainingComponent(components, 'button-1');

    expect(result?.id).toBe('stack-1');
  });

  it('should find a container nested inside another container', () => {
    const components = [
      {
        id: 'form-1',
        type: 'BLOCK',
        components: [{id: 'stack-1', type: 'STACK', components: [{id: 'button-1', type: 'BUTTON'}]}],
      },
    ] as unknown as Element[];

    const result = findContainingComponent(components, 'button-1');

    expect(result?.id).toBe('stack-1');
  });

  it('should return undefined when the child id does not exist', () => {
    const components = [
      {id: 'stack-1', type: 'STACK', components: [{id: 'button-1', type: 'BUTTON'}]},
    ] as unknown as Element[];

    const result = findContainingComponent(components, 'non-existent');

    expect(result).toBeUndefined();
  });

  it('should return undefined when no components have children', () => {
    const components = [{id: 'input-1', type: 'TEXT_INPUT'}] as unknown as Element[];

    const result = findContainingComponent(components, 'input-1');

    expect(result).toBeUndefined();
  });
});
