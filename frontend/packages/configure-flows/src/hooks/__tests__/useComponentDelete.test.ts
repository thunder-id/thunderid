// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access */

import {renderHook, act} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Element} from '../../models/elements';
import useComponentDelete from '../useComponentDelete';

// Mock updateNodeData from @xyflow/react
const mockUpdateNodeData = vi.fn();

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    updateNodeData: mockUpdateNodeData,
  }),
}));

describe('useComponentDelete', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Hook Interface', () => {
    it('should return deleteComponent function', () => {
      const {result} = renderHook(() => useComponentDelete());

      expect(typeof result.current.deleteComponent).toBe('function');
    });

    it('should return stable function reference across renders', () => {
      const {result, rerender} = renderHook(() => useComponentDelete());

      const initialFn = result.current.deleteComponent;

      rerender();

      expect(result.current.deleteComponent).toBe(initialFn);
    });
  });

  describe('deleteComponent', () => {
    it('should call updateNodeData with stepId', () => {
      const {result} = renderHook(() => useComponentDelete());

      const component: Element = {
        id: 'component-1',
        type: 'TEXT_INPUT',
        category: 'INPUT',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', component);
      });

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
    });

    it('should remove component from flat list', () => {
      const {result} = renderHook(() => useComponentDelete());

      const componentToDelete: Element = {
        id: 'component-2',
        type: 'BUTTON',
        category: 'ACTION',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', componentToDelete);
      });

      // Get the updater function passed to updateNodeData
      const updaterFn = mockUpdateNodeData.mock.calls[0][1];

      // Test the updater function with mock node data
      const nodeData = {
        data: {
          components: [
            {id: 'component-1', type: 'TEXT_INPUT', category: 'INPUT'},
            {id: 'component-2', type: 'BUTTON', category: 'ACTION'},
            {id: 'component-3', type: 'TEXT', category: 'TYPOGRAPHY'},
          ],
        },
      };

      const updatedData = updaterFn(nodeData);

      expect(updatedData.components).toHaveLength(2);
      expect(updatedData.components.find((c: Element) => c.id === 'component-2')).toBeUndefined();
      expect(updatedData.components.find((c: Element) => c.id === 'component-1')).toBeDefined();
      expect(updatedData.components.find((c: Element) => c.id === 'component-3')).toBeDefined();
    });

    it('should remove component from nested list recursively', () => {
      const {result} = renderHook(() => useComponentDelete());

      const nestedComponent: Element = {
        id: 'nested-component',
        type: 'BUTTON',
        category: 'ACTION',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', nestedComponent);
      });

      const updaterFn = mockUpdateNodeData.mock.calls[0][1];

      const nodeData = {
        data: {
          components: [
            {
              id: 'container-1',
              type: 'CONTAINER',
              category: 'LAYOUT',
              components: [
                {id: 'nested-component', type: 'BUTTON', category: 'ACTION'},
                {id: 'other-component', type: 'TEXT', category: 'TYPOGRAPHY'},
              ],
            },
            {id: 'component-2', type: 'TEXT_INPUT', category: 'INPUT'},
          ],
        },
      };

      const updatedData = updaterFn(nodeData);

      expect(updatedData.components).toHaveLength(2);
      const container = updatedData.components.find((c: Element) => c.id === 'container-1');
      expect(container.components).toHaveLength(1);
      expect(container.components.find((c: Element) => c.id === 'nested-component')).toBeUndefined();
      expect(container.components.find((c: Element) => c.id === 'other-component')).toBeDefined();
    });

    it('should handle deeply nested components', () => {
      const {result} = renderHook(() => useComponentDelete());

      const deepNestedComponent: Element = {
        id: 'deep-nested',
        type: 'BUTTON',
        category: 'ACTION',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', deepNestedComponent);
      });

      const updaterFn = mockUpdateNodeData.mock.calls[0][1];

      const nodeData = {
        data: {
          components: [
            {
              id: 'level-1',
              type: 'CONTAINER',
              category: 'LAYOUT',
              components: [
                {
                  id: 'level-2',
                  type: 'CONTAINER',
                  category: 'LAYOUT',
                  components: [{id: 'deep-nested', type: 'BUTTON', category: 'ACTION'}],
                },
              ],
            },
          ],
        },
      };

      const updatedData = updaterFn(nodeData);

      const level1 = updatedData.components.find((c: Element) => c.id === 'level-1');
      const level2 = level1.components.find((c: Element) => c.id === 'level-2');
      expect(level2.components).toHaveLength(0);
    });

    it('should handle empty components array', () => {
      const {result} = renderHook(() => useComponentDelete());

      const component: Element = {
        id: 'component-1',
        type: 'BUTTON',
        category: 'ACTION',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', component);
      });

      const updaterFn = mockUpdateNodeData.mock.calls[0][1];

      const nodeData = {
        data: {
          components: [],
        },
      };

      const updatedData = updaterFn(nodeData);

      expect(updatedData.components).toHaveLength(0);
    });

    it('should handle undefined components array', () => {
      const {result} = renderHook(() => useComponentDelete());

      const component: Element = {
        id: 'component-1',
        type: 'BUTTON',
        category: 'ACTION',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', component);
      });

      const updaterFn = mockUpdateNodeData.mock.calls[0][1];

      const nodeData = {
        data: {},
      };

      const updatedData = updaterFn(nodeData);

      expect(updatedData.components).toEqual([]);
    });

    it('should handle null node data', () => {
      const {result} = renderHook(() => useComponentDelete());

      const component: Element = {
        id: 'component-1',
        type: 'BUTTON',
        category: 'ACTION',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', component);
      });

      const updaterFn = mockUpdateNodeData.mock.calls[0][1];

      const updatedData = updaterFn(null);

      expect(updatedData.components).toEqual([]);
    });

    it('should preserve other component properties when deleting', () => {
      const {result} = renderHook(() => useComponentDelete());

      const componentToDelete: Element = {
        id: 'component-2',
        type: 'BUTTON',
        category: 'ACTION',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', componentToDelete);
      });

      const updaterFn = mockUpdateNodeData.mock.calls[0][1];

      const nodeData = {
        data: {
          components: [
            {
              id: 'component-1',
              type: 'TEXT_INPUT',
              category: 'INPUT',
              display: {label: 'Email'},
              config: {placeholder: 'Enter email'},
            },
            {id: 'component-2', type: 'BUTTON', category: 'ACTION'},
          ],
        },
      };

      const updatedData = updaterFn(nodeData);

      const preserved = updatedData.components.find((c: Element) => c.id === 'component-1');
      expect(preserved.display).toEqual({label: 'Email'});
      expect(preserved.config).toEqual({placeholder: 'Enter email'});
    });

    it('should not modify original component objects (immutability)', () => {
      const {result} = renderHook(() => useComponentDelete());

      const componentToDelete: Element = {
        id: 'nested',
        type: 'BUTTON',
        category: 'ACTION',
      } as Element;

      act(() => {
        result.current.deleteComponent('step-1', componentToDelete);
      });

      const updaterFn = mockUpdateNodeData.mock.calls[0][1];

      const originalComponents = [
        {
          id: 'container',
          type: 'CONTAINER',
          category: 'LAYOUT',
          components: [
            {id: 'nested', type: 'BUTTON', category: 'ACTION'},
            {id: 'other', type: 'TEXT', category: 'TYPOGRAPHY'},
          ],
        },
      ];

      const nodeData = {
        data: {
          components: originalComponents,
        },
      };

      const updatedData = updaterFn(nodeData);

      // Original should not be modified
      expect(originalComponents[0].components).toHaveLength(2);
      // Updated should have the component removed
      expect(updatedData.components[0].components).toHaveLength(1);
    });
  });
});
