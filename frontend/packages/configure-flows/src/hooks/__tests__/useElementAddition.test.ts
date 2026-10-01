// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, act} from '@testing-library/react';
import type {Node} from '@xyflow/react';
import type {UpdateNodeInternals} from '@xyflow/system';
import type React from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {BlockTypes, ElementCategories, ElementTypes, type Element} from '../../models/elements';
import {StepTypes} from '../../models/steps';
import useElementAddition from '../useElementAddition';

// Mock useGenerateStepElement hook
const mockGenerateStepElement = vi.fn((element: Element) => ({
  ...element,
  id: `generated-${element.id}`,
}));

vi.mock('../useGenerateStepElement', () => ({
  default: () => ({
    generateStepElement: mockGenerateStepElement,
  }),
}));

// Mock generateIdsForResources
vi.mock('../../utils/generateIdsForResources', () => ({
  default: <T>(obj: T): T => {
    if (typeof obj === 'object' && obj !== null && 'id' in obj) {
      return {...obj, id: `generated-form-id`} as T;
    }
    return obj;
  },
}));

// Mock componentMutations
vi.mock('../../utils/componentMutations', () => ({
  INPUT_ELEMENT_TYPES: new Set(['TEXT_INPUT', 'PASSWORD_INPUT', 'EMAIL_INPUT', 'OTP_INPUT', 'CHECKBOX', 'SELECT']),
  mutateComponents: (components: Element[]) => components,
}));

const createMockElement = (overrides: Partial<Element> = {}): Element =>
  ({
    id: 'element-1',
    type: ElementTypes.Action,
    category: ElementCategories.Action,
    version: '1.0.0',
    deprecated: false,
    deletable: true,
    resourceType: 'ELEMENT',
    display: {
      label: 'Element Label',
      image: '',
      showOnResourcePanel: true,
    },
    config: {},
    ...overrides,
  }) as Element;

const createMockViewNode = (overrides: Partial<Node> = {}): Node => ({
  id: 'view-1',
  type: StepTypes.View,
  position: {x: 0, y: 0},
  data: {
    components: [],
  },
  ...overrides,
});

type SetNodesFn = React.Dispatch<React.SetStateAction<Node[]>>;

describe('useElementAddition', () => {
  let mockSetNodes: ReturnType<typeof vi.fn> & SetNodesFn;
  let mockUpdateNodeInternals: ReturnType<typeof vi.fn> & UpdateNodeInternals;

  beforeEach(() => {
    vi.clearAllMocks();
    mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
      if (typeof updater === 'function') {
        return updater([]);
      }
      return updater;
    }) as ReturnType<typeof vi.fn> & SetNodesFn;
    mockUpdateNodeInternals = vi.fn() as ReturnType<typeof vi.fn> & UpdateNodeInternals;
  });

  describe('Hook Interface', () => {
    it('should return handleAddElementToView function', () => {
      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      expect(typeof result.current.handleAddElementToView).toBe('function');
    });

    it('should return handleAddElementToForm function', () => {
      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      expect(typeof result.current.handleAddElementToForm).toBe('function');
    });

    it('should maintain stable function references', () => {
      const {result, rerender} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const initialHandleAddToView = result.current.handleAddElementToView;
      const initialHandleAddToForm = result.current.handleAddElementToForm;

      rerender();

      expect(result.current.handleAddElementToView).toBe(initialHandleAddToView);
      expect(result.current.handleAddElementToForm).toBe(initialHandleAddToForm);
    });
  });

  describe('handleAddElementToView', () => {
    it('should call setNodes when adding an element to view', () => {
      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'button-1', type: ElementTypes.Action});

      act(() => {
        result.current.handleAddElementToView(element, 'view-1');
      });

      expect(mockSetNodes).toHaveBeenCalled();
    });

    it('should call generateStepElement with the element', () => {
      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'button-1', type: ElementTypes.Action});

      act(() => {
        result.current.handleAddElementToView(element, 'view-1');
      });

      expect(mockGenerateStepElement).toHaveBeenCalledWith(element);
    });

    it('should do nothing when view node does not exist', () => {
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'other-view'})];
          return updater(nodes);
        }
        return updater;
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'button-1', type: ElementTypes.Action});

      act(() => {
        result.current.handleAddElementToView(element, 'non-existent-view');
      });

      expect(mockSetNodes).toHaveBeenCalled();
      // The setNodes callback should return unchanged nodes
      const updater = mockSetNodes.mock.calls[0][0] as (nodes: Node[]) => Node[];
      const inputNodes = [createMockViewNode({id: 'other-view'})];
      const resultNodes = updater(inputNodes);
      expect(resultNodes).toEqual(inputNodes);
    });

    it('should add non-input element directly to view components', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'button-1', type: ElementTypes.Action});

      act(() => {
        result.current.handleAddElementToView(element, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      expect((capturedNodes[0].data as {components: Element[]}).components).toHaveLength(1);
      expect((capturedNodes[0].data as {components: Element[]}).components[0].id).toBe('generated-button-1');
    });

    it('should add input element to existing form in view', () => {
      const existingForm = createMockElement({
        id: 'form-1',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        version: '0.1.0',
        components: [],
      });

      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: [existingForm]}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const inputElement = createMockElement({id: 'text-input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToView(inputElement, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      // Should have the form with the new input element inside
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.type === BlockTypes.Form,
      ) as Element & {components: Element[]};
      expect(form).toBeDefined();
      expect(form.components).toHaveLength(1);
      expect(form.components[0].id).toBe('generated-text-input-1');
    });

    it('should add select element to existing form in view', () => {
      const existingForm = createMockElement({
        id: 'form-1',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        version: '0.1.0',
        components: [],
      });

      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: [existingForm]}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const selectElement = createMockElement({id: 'select-1', type: ElementTypes.Select});

      act(() => {
        result.current.handleAddElementToView(selectElement, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.type === BlockTypes.Form,
      ) as Element & {components: Element[]};
      expect(form).toBeDefined();
      expect(form.components).toHaveLength(1);
      expect(form.components[0].id).toBe('generated-select-1');
    });

    it('should create new form when adding input element to view without form', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const inputElement = createMockElement({id: 'text-input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToView(inputElement, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      // Should have created a new form containing the input element
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.type === BlockTypes.Form,
      ) as Element & {components: Element[]};
      expect(form).toBeDefined();
      expect(form.components).toHaveLength(1);
    });

    it('should schedule updateNodeInternals after adding element', async () => {
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          return updater(nodes);
        }
        return undefined;
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'button-1', type: ElementTypes.Action});

      act(() => {
        result.current.handleAddElementToView(element, 'view-1');
      });

      // Wait for queueMicrotask to execute
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('view-1');
    });

    it('should handle view without components property', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'button-1', type: ElementTypes.Action});

      act(() => {
        result.current.handleAddElementToView(element, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      expect(capturedNodes[0].data['components']).toBeDefined();
    });
  });

  describe('handleAddElementToForm', () => {
    it('should call setNodes when adding an element to form', () => {
      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToForm(element, 'form-1');
      });

      expect(mockSetNodes).toHaveBeenCalled();
    });

    it('should call generateStepElement with the element', () => {
      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToForm(element, 'form-1');
      });

      expect(mockGenerateStepElement).toHaveBeenCalledWith(element);
    });

    it('should do nothing when form does not exist in any view', () => {
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          return updater(nodes);
        }
        return updater;
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToForm(element, 'non-existent-form');
      });

      expect(mockSetNodes).toHaveBeenCalled();
      // The setNodes callback should return unchanged nodes
      const updater = mockSetNodes.mock.calls[0][0] as (nodes: Node[]) => Node[];
      const inputNodes = [createMockViewNode({id: 'view-1', data: {components: []}})];
      const resultNodes = updater(inputNodes);
      expect(resultNodes).toEqual(inputNodes);
    });

    it('should add element to the correct form', () => {
      const existingForm = createMockElement({
        id: 'form-1',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        version: '0.1.0',
        components: [],
      });

      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: [existingForm]}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const inputElement = createMockElement({id: 'text-input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToForm(inputElement, 'form-1');
      });

      expect(capturedNodes.length).toBe(1);
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.id === 'form-1',
      ) as Element & {components: Element[]};
      expect(form).toBeDefined();
      expect(form.components).toHaveLength(1);
      expect(form.components[0].id).toBe('generated-text-input-1');
    });

    it('should preserve existing form components when adding new element', () => {
      const existingInput = createMockElement({
        id: 'existing-input',
        type: ElementTypes.TextInput,
        category: ElementCategories.Field,
        version: '1.0.0',
      });

      const existingForm = createMockElement({
        id: 'form-1',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        version: '0.1.0',
        components: [existingInput],
      });

      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: [existingForm]}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const newInputElement = createMockElement({id: 'new-input', type: ElementTypes.PasswordInput});

      act(() => {
        result.current.handleAddElementToForm(newInputElement, 'form-1');
      });

      expect(capturedNodes.length).toBe(1);
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.id === 'form-1',
      ) as Element & {components: Element[]};
      expect(form.components).toHaveLength(2);
      expect(form.components[0].id).toBe('existing-input');
      expect(form.components[1].id).toBe('generated-new-input');
    });

    it('should schedule updateNodeInternals after adding element to form', async () => {
      const existingForm = createMockElement({
        id: 'form-1',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        version: '0.1.0',
        components: [],
      });

      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: [existingForm]}})];
          return updater(nodes);
        }
        return undefined;
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToForm(element, 'form-1');
      });

      // Wait for queueMicrotask to execute
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('view-1');
    });

    it('should not update other nodes when adding element to form', () => {
      const existingForm = createMockElement({
        id: 'form-1',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        version: '0.1.0',
        components: [],
      });

      const otherNode = createMockViewNode({id: 'view-2', data: {components: [{id: 'other-element'}]}});

      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: [existingForm]}}), otherNode];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToForm(element, 'form-1');
      });

      expect(capturedNodes.length).toBe(2);
      // The other node should remain unchanged
      expect(capturedNodes[1].data['components']).toEqual([{id: 'other-element'}]);
    });

    it('should skip non-View nodes when searching for form', () => {
      const existingForm = createMockElement({
        id: 'form-1',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        version: '0.1.0',
        components: [],
      });

      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [
            {
              id: 'execution-node',
              type: 'TASK_EXECUTION',
              position: {x: 0, y: 0},
              data: {components: [existingForm]}, // Form in non-View node should be ignored
            },
            createMockViewNode({id: 'view-1', data: {components: [existingForm]}}),
          ];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const element = createMockElement({id: 'input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToForm(element, 'form-1');
      });

      // The element should be added to the form in the View node, not the execution node
      expect(capturedNodes.length).toBe(2);
      const viewNode = capturedNodes.find((n) => n.id === 'view-1');
      const form = (viewNode?.data as {components: Element[]}).components.find(
        (c: Element) => c.id === 'form-1',
      ) as Element & {components: Element[]};
      expect(form?.components).toHaveLength(1);
    });

    it('should handle form with undefined components property', () => {
      const existingForm = createMockElement({
        id: 'form-1',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        version: '0.1.0',
        // No components property
      });

      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: [existingForm]}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const inputElement = createMockElement({id: 'text-input-1', type: ElementTypes.TextInput});

      act(() => {
        result.current.handleAddElementToForm(inputElement, 'form-1');
      });

      expect(capturedNodes.length).toBe(1);
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.id === 'form-1',
      ) as Element & {components: Element[]};
      expect(form).toBeDefined();
      expect(form.components).toHaveLength(1);
    });
  });

  describe('mutateComponents integration', () => {
    it('should call mutateComponents when adding input element to new form', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      // Use a different input type to ensure full code path coverage
      const inputElement = createMockElement({id: 'password-input-1', type: ElementTypes.PasswordInput});

      act(() => {
        result.current.handleAddElementToView(inputElement, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      // Should have created a new form containing the input element via mutateComponents
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.type === BlockTypes.Form,
      );
      expect(form).toBeDefined();
    });

    it('should call mutateComponents when adding non-input element to view', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      // Use a Resend element type to ensure non-input path is covered
      const resendElement = createMockElement({
        id: 'resend-1',
        type: ElementTypes.Resend,
        category: ElementCategories.Action,
      });

      act(() => {
        result.current.handleAddElementToView(resendElement, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      expect((capturedNodes[0].data as {components: Element[]}).components).toHaveLength(1);
    });

    it('should handle email input type', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const emailInput = createMockElement({id: 'email-input-1', type: ElementTypes.EmailInput});

      act(() => {
        result.current.handleAddElementToView(emailInput, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      // Should have created a new form for this input type
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.type === BlockTypes.Form,
      );
      expect(form).toBeDefined();
    });

    it('should handle OTP input type', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const otpInput = createMockElement({id: 'otp-input-1', type: ElementTypes.OtpInput});

      act(() => {
        result.current.handleAddElementToView(otpInput, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.type === BlockTypes.Form,
      );
      expect(form).toBeDefined();
    });

    it('should handle checkbox input type', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes: Node[] = [createMockViewNode({id: 'view-1', data: {components: []}})];
          capturedNodes = updater(nodes);
        }
      }) as ReturnType<typeof vi.fn> & SetNodesFn;

      const {result} = renderHook(() =>
        useElementAddition({
          setNodes: mockSetNodes,
          updateNodeInternals: mockUpdateNodeInternals,
        }),
      );

      const checkboxInput = createMockElement({id: 'checkbox-1', type: ElementTypes.Checkbox});

      act(() => {
        result.current.handleAddElementToView(checkboxInput, 'view-1');
      });

      expect(capturedNodes.length).toBe(1);
      const form = (capturedNodes[0].data as {components: Element[]}).components.find(
        (c: Element) => c.type === BlockTypes.Form,
      );
      expect(form).toBeDefined();
    });
  });
});
