// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {Node} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowBuilderElementConstants from '../../constants/FlowBuilderElementConstants';
import {ElementTypes, BlockTypes} from '../../models/elements';
import type {Element} from '../../models/elements';
import useConfirmPasswordField from '../useConfirmPasswordField';

// Use vi.hoisted to define mocks
const {mockGetNode, mockUpdateNodeData} = vi.hoisted(() => ({
  mockGetNode: vi.fn(),
  mockUpdateNodeData: vi.fn(),
}));

// Store registered handlers for testing
const registeredHandlers: {
  onPropertyChange: ((...args: unknown[]) => boolean)[];
  onPropertyPanelOpen: ((...args: unknown[]) => boolean)[];
  onNodeElementDelete: ((...args: unknown[]) => boolean)[];
} = {
  onPropertyChange: [],
  onPropertyPanelOpen: [],
  onNodeElementDelete: [],
};

// Unsubscribe functions for each registration
const mockUnsubscribes: {
  onPropertyChange: ReturnType<typeof vi.fn>[];
  onPropertyPanelOpen: ReturnType<typeof vi.fn>[];
  onNodeElementDelete: ReturnType<typeof vi.fn>[];
} = {
  onPropertyChange: [],
  onPropertyPanelOpen: [],
  onNodeElementDelete: [],
};

const mockOnPropertyChange = vi.fn().mockImplementation((handler: (...args: unknown[]) => boolean) => {
  registeredHandlers.onPropertyChange.push(handler);
  const unsub = vi.fn();
  mockUnsubscribes.onPropertyChange.push(unsub);
  return unsub;
});

const mockOnPropertyPanelOpen = vi.fn().mockImplementation((handler: (...args: unknown[]) => boolean) => {
  registeredHandlers.onPropertyPanelOpen.push(handler);
  const unsub = vi.fn();
  mockUnsubscribes.onPropertyPanelOpen.push(unsub);
  return unsub;
});

const mockOnNodeElementDelete = vi.fn().mockImplementation((handler: (...args: unknown[]) => boolean) => {
  registeredHandlers.onNodeElementDelete.push(handler);
  const unsub = vi.fn();
  mockUnsubscribes.onNodeElementDelete.push(unsub);
  return unsub;
});

const mockFlowPlugins = {
  onPropertyChange: mockOnPropertyChange,
  emitPropertyChange: vi.fn().mockReturnValue(true),
  onPropertyPanelOpen: mockOnPropertyPanelOpen,
  emitPropertyPanelOpen: vi.fn().mockReturnValue(true),
  onElementFilter: vi.fn().mockReturnValue(vi.fn()),
  emitElementFilter: vi.fn().mockReturnValue(true),
  onEdgeDelete: vi.fn().mockReturnValue(vi.fn()),
  emitEdgeDelete: vi.fn().mockReturnValue(true),
  onNodeDelete: vi.fn().mockReturnValue(vi.fn()),
  emitNodeDelete: vi.fn().mockReturnValue(true),
  onNodeElementDelete: mockOnNodeElementDelete,
  emitNodeElementDelete: vi.fn().mockReturnValue(true),
  onTemplateLoad: vi.fn().mockReturnValue(vi.fn()),
  emitTemplateLoad: vi.fn().mockReturnValue(true),
};

// Mock @xyflow/react
vi.mock('@xyflow/react', async () => {
  const actual = await vi.importActual('@xyflow/react');
  return {
    ...actual,
    useReactFlow: () => ({
      getNode: mockGetNode,
      updateNodeData: mockUpdateNodeData,
    }),
  };
});

// Mock useFlowPlugins - capture handlers for testing
vi.mock('../useFlowPlugins', () => ({
  default: () => mockFlowPlugins,
}));

// Mock generateResourceId
vi.mock('../../utils/generateResourceId', () => ({
  default: vi.fn().mockReturnValue('generated-field-id'),
}));

describe('useConfirmPasswordField', () => {
  const createWrapper = () => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ReactFlowProvider>{children}</ReactFlowProvider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Clear registered handlers
    registeredHandlers.onPropertyChange = [];
    registeredHandlers.onPropertyPanelOpen = [];
    registeredHandlers.onNodeElementDelete = [];
    mockUnsubscribes.onPropertyChange = [];
    mockUnsubscribes.onPropertyPanelOpen = [];
    mockUnsubscribes.onNodeElementDelete = [];
    // Re-wire the capture implementations after clearAllMocks
    mockOnPropertyChange.mockImplementation((handler: (...args: unknown[]) => boolean) => {
      registeredHandlers.onPropertyChange.push(handler);
      const unsub = vi.fn();
      mockUnsubscribes.onPropertyChange.push(unsub);
      return unsub;
    });
    mockOnPropertyPanelOpen.mockImplementation((handler: (...args: unknown[]) => boolean) => {
      registeredHandlers.onPropertyPanelOpen.push(handler);
      const unsub = vi.fn();
      mockUnsubscribes.onPropertyPanelOpen.push(unsub);
      return unsub;
    });
    mockOnNodeElementDelete.mockImplementation((handler: (...args: unknown[]) => boolean) => {
      registeredHandlers.onNodeElementDelete.push(handler);
      const unsub = vi.fn();
      mockUnsubscribes.onNodeElementDelete.push(unsub);
      return unsub;
    });
  });

  describe('Plugin Registration', () => {
    it('should register event handlers on mount', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      // Should register two handlers for onPropertyChange
      expect(mockOnPropertyChange).toHaveBeenCalledWith(expect.any(Function));
      expect(mockOnPropertyPanelOpen).toHaveBeenCalledWith(expect.any(Function));
      expect(mockOnNodeElementDelete).toHaveBeenCalledWith(expect.any(Function));
    });

    it('should call unsubscribe functions on unmount', () => {
      const {unmount} = renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      unmount();

      // All unsubscribe functions should have been called
      mockUnsubscribes.onPropertyChange.forEach((unsub) => expect(unsub).toHaveBeenCalled());
      mockUnsubscribes.onPropertyPanelOpen.forEach((unsub) => expect(unsub).toHaveBeenCalled());
      mockUnsubscribes.onNodeElementDelete.forEach((unsub) => expect(unsub).toHaveBeenCalled());
    });
  });

  describe('addConfirmPasswordField Handler', () => {
    it('should return true for non-password input types', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'text-input-1',
        type: ElementTypes.TextInput,
      } as Element;

      const result = addConfirmPasswordFieldHandler('requireConfirmation', true, element, 'step-1');
      expect(result).toBe(true);
    });

    it('should return true for properties other than requireConfirmation', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      const result = addConfirmPasswordFieldHandler('someOtherProperty', true, element, 'step-1');
      expect(result).toBe(true);
    });

    it('should add confirm password field when requireConfirmation is true', () => {
      const stepNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [{id: 'password-1', type: ElementTypes.PasswordInput}],
            },
          ],
        },
      };

      mockGetNode.mockReturnValue(stepNode);

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      const result = addConfirmPasswordFieldHandler('requireConfirmation', true, element, 'step-1');
      expect(result).toBe(false);
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should remove confirm password field when requireConfirmation is false', () => {
      const stepNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                },
              ],
            },
          ],
        },
      };

      mockGetNode.mockReturnValue(stepNode);

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      const result = addConfirmPasswordFieldHandler('requireConfirmation', false, element, 'step-1');
      expect(result).toBe(false);
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should execute updateNodeData callback correctly when adding confirm field', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      addConfirmPasswordFieldHandler('requireConfirmation', true, element, 'step-1');

      expect(capturedCallback).not.toBeNull();

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [{id: 'password-1', type: ElementTypes.PasswordInput}],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      expect(result['components']).toBeDefined();
    });

    it('should return empty object when node has no components', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      addConfirmPasswordFieldHandler('requireConfirmation', true, element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {},
      };

      const result = capturedCallback!(mockNode);
      expect(result).toEqual({});
    });

    it('should return empty object when removing the confirm field on a node with no components', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      addConfirmPasswordFieldHandler('requireConfirmation', false, element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {},
      };

      const result = capturedCallback!(mockNode);
      expect(result).toEqual({});
    });

    it('should execute callback to add confirm password field after password field', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      addConfirmPasswordFieldHandler('requireConfirmation', true, element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'input-1', type: 'TEXT_INPUT'},
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {id: 'button-1', type: 'BUTTON'},
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      expect(result['components']).toBeDefined();
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      // Confirm password should be inserted after password-1 (at index 2)
      expect(form.components?.length).toBe(4);
      expect((form.components?.[2] as Element & {identifier?: string})?.identifier).toBe(
        FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
      );
    });

    it('should not add confirm field when password is not in a form', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      addConfirmPasswordFieldHandler('requireConfirmation', true, element, 'step-1');

      // Node with components that are not forms
      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'button-1', type: 'BUTTON'},
            {id: 'password-1', type: ElementTypes.PasswordInput},
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      expect(result['components']).toBeDefined();
      // No confirm field should be added since password is not in a form
      expect((result['components'] as Element[]).length).toBe(2);
    });

    it('should execute callback to remove confirm password field when unchecking', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      addConfirmPasswordFieldHandler('requireConfirmation', false, element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                },
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      expect(result['components']).toBeDefined();
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      // Confirm password should be removed
      expect(form.components?.length).toBe(1);
      expect(form.components?.[0].id).toBe('password-1');
    });

    it('should not remove confirm field if not found', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const addConfirmPasswordFieldHandler = handlers?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      addConfirmPasswordFieldHandler('requireConfirmation', false, element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                // No confirm password field
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      expect(result['components']).toBeDefined();
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      // Components should remain unchanged
      expect(form.components?.length).toBe(1);
    });
  });

  describe('addConfirmPasswordFieldProperties Handler', () => {
    it('should return true for non-password input types', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'text-input-1',
        type: ElementTypes.TextInput,
      } as Element;

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'step-1');
      expect(result).toBe(true);
      expect(properties['requireConfirmation']).toBeUndefined();
    });

    it('should return true for password without PASSWORD_IDENTIFIER', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: 'OTHER_IDENTIFIER',
      } as Element & {identifier?: string};

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'step-1');
      expect(result).toBe(true);
    });

    it('should return true without setting requireConfirmation when the node has no components', () => {
      const stepNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {},
      };
      mockGetNode.mockReturnValue(stepNode);

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'step-1');
      expect(result).toBe(true);
      expect(properties['requireConfirmation']).toBe(false);
    });

    it('should add requireConfirmation property for password with PASSWORD_IDENTIFIER', () => {
      const stepNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {
                  id: 'password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
                },
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                  hint: 'Confirm hint',
                  label: 'Confirm Label',
                  placeholder: 'Confirm placeholder',
                },
              ],
            },
          ],
        },
      };

      mockGetNode.mockReturnValue(stepNode);

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'step-1');
      expect(result).toBe(true);
      expect(properties['requireConfirmation']).toBe(true);
      expect(properties['confirmHint']).toBe('Confirm hint');
      expect(properties['confirmLabel']).toBe('Confirm Label');
      expect(properties['confirmPlaceholder']).toBe('Confirm placeholder');
    });

    it('should set requireConfirmation to false when no confirm field exists', () => {
      const stepNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {
                  id: 'password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
                },
              ],
            },
          ],
        },
      };

      mockGetNode.mockReturnValue(stepNode);

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'step-1');
      expect(result).toBe(true);
      expect(properties['requireConfirmation']).toBe(false);
    });

    it('should use resource requireConfirmation value when explicitly set', () => {
      const stepNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {
                  id: 'password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
                },
              ],
            },
          ],
        },
      };

      mockGetNode.mockReturnValue(stepNode);

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
        requireConfirmation: true,
      } as Element & {identifier?: string; requireConfirmation?: boolean};

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'step-1');
      expect(result).toBe(true);
      expect(properties['requireConfirmation']).toBe(true);
    });
  });

  describe('updateConfirmPasswordFieldProperties Handler', () => {
    it('should return true for non-password input types', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'text-input-1',
        type: ElementTypes.TextInput,
      } as Element;

      const result = updatePropertiesHandler('confirmHint', 'New hint', element, 'step-1');
      expect(result).toBe(true);
    });

    it('should update confirm password field properties', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      const result = updatePropertiesHandler('confirmHint', 'New hint', element, 'step-1');
      expect(result).toBe(false);
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should update confirmLabel property', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      const result = updatePropertiesHandler('confirmLabel', 'New label', element, 'step-1');
      expect(result).toBe(false);
    });

    it('should update confirmPlaceholder property', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      const result = updatePropertiesHandler('confirmPlaceholder', 'New placeholder', element, 'step-1');
      expect(result).toBe(false);
    });

    it('should return true for required property (not return false)', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      const result = updatePropertiesHandler('required', true, element, 'step-1');
      expect(result).toBe(true);
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should return true for other properties', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      const result = updatePropertiesHandler('someOtherProperty', 'value', element, 'step-1');
      expect(result).toBe(true);
    });

    it('should execute updateNodeData callback to update confirmHint on confirm field', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      updatePropertiesHandler('confirmHint', 'New hint value', element, 'step-1');

      expect(capturedCallback).not.toBeNull();

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                  hint: 'Old hint',
                },
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      expect(result['components']).toBeDefined();
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      const confirmField = form.components?.find(
        (c: Element) =>
          (c as Element & {identifier?: string}).identifier === FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
      ) as Element & {hint?: string};
      expect(confirmField?.hint).toBe('New hint value');
    });

    it('should execute updateNodeData callback to update confirmLabel on confirm field', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      updatePropertiesHandler('confirmLabel', 'New Label', element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                  label: 'Old Label',
                },
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      const confirmField = form.components?.find(
        (c: Element) =>
          (c as Element & {identifier?: string}).identifier === FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
      ) as Element & {label?: string};
      expect(confirmField?.label).toBe('New Label');
    });

    it('should execute updateNodeData callback to update confirmPlaceholder on confirm field', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      updatePropertiesHandler('confirmPlaceholder', 'New Placeholder', element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                  placeholder: 'Old Placeholder',
                },
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      const confirmField = form.components?.find(
        (c: Element) =>
          (c as Element & {identifier?: string}).identifier === FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
      ) as Element & {placeholder?: string};
      expect(confirmField?.placeholder).toBe('New Placeholder');
    });

    it('should execute updateNodeData callback to update required on confirm field', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      updatePropertiesHandler('required', true, element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                  required: false,
                },
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      const confirmField = form.components?.find(
        (c: Element) =>
          (c as Element & {identifier?: string}).identifier === FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
      ) as Element & {required?: boolean};
      expect(confirmField?.required).toBe(true);
    });

    it('should return empty object when node has no components during update', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      updatePropertiesHandler('confirmHint', 'New hint', element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {},
      };

      const result = capturedCallback!(mockNode);
      expect(result).toEqual({});
    });

    it('should not update non-form components during property update', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const handlers = registeredHandlers.onPropertyChange;
      const updatePropertiesHandler = handlers?.[1];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
      } as Element;

      updatePropertiesHandler('confirmHint', 'New hint', element, 'step-1');

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'button-1', type: 'BUTTON'},
            {id: 'password-1', type: ElementTypes.PasswordInput},
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      // Components should remain unchanged since there's no form
      expect((result['components'] as Element[]).length).toBe(2);
    });
  });

  describe('deleteConfirmPasswordField Handler', () => {
    it('should return true for non-password input types', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const deleteHandler = registeredHandlers.onNodeElementDelete?.[0];

      const element = {
        id: 'text-input-1',
        type: ElementTypes.TextInput,
      } as Element;

      const result = deleteHandler('step-1', element);
      expect(result).toBe(true);
    });

    it('should return true for password without PASSWORD_IDENTIFIER', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const deleteHandler = registeredHandlers.onNodeElementDelete?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: 'OTHER_IDENTIFIER',
      } as Element & {identifier?: string};

      const result = deleteHandler('step-1', element);
      expect(result).toBe(true);
    });

    it('should delete confirm password field when password field is deleted', () => {
      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const deleteHandler = registeredHandlers.onNodeElementDelete?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      const result = deleteHandler('step-1', element);
      expect(result).toBe(true);
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should execute updateNodeData callback correctly when deleting', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const deleteHandler = registeredHandlers.onNodeElementDelete?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      deleteHandler('step-1', element);

      expect(capturedCallback).not.toBeNull();

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                },
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      expect(result['components']).toBeDefined();
    });

    it('should return empty object when node has no components', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const deleteHandler = registeredHandlers.onNodeElementDelete?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      deleteHandler('step-1', element);

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {},
      };

      const result = capturedCallback!(mockNode);
      expect(result).toEqual({});
    });

    it('should execute callback to properly remove confirm password field from form', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const deleteHandler = registeredHandlers.onNodeElementDelete?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      deleteHandler('step-1', element);

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'input-1', type: 'TEXT_INPUT'},
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {
                  id: 'confirm-password-1',
                  type: ElementTypes.PasswordInput,
                  identifier: FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
                },
                {id: 'button-1', type: 'BUTTON'},
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      expect(result['components']).toBeDefined();
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      // Confirm password should be removed
      expect(form.components?.length).toBe(3);
      expect(
        form.components?.find(
          (c: Element) =>
            (c as Element & {identifier?: string}).identifier ===
            FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER,
        ),
      ).toBeUndefined();
    });

    it('should not modify form if no confirm password field exists during delete', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const deleteHandler = registeredHandlers.onNodeElementDelete?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      deleteHandler('step-1', element);

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: BlockTypes.Form,
              components: [
                {id: 'password-1', type: ElementTypes.PasswordInput},
                {id: 'button-1', type: 'BUTTON'},
              ],
            },
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      const components = result['components'] as Element[];
      const form = components[0] as Element & {components?: Element[]};
      // Form components should remain unchanged
      expect(form.components?.length).toBe(2);
    });

    it('should not modify non-form components during delete', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useConfirmPasswordField(), {
        wrapper: createWrapper(),
      });

      const deleteHandler = registeredHandlers.onNodeElementDelete?.[0];

      const element = {
        id: 'password-1',
        type: ElementTypes.PasswordInput,
        identifier: FlowBuilderElementConstants.PASSWORD_IDENTIFIER,
      } as Element & {identifier?: string};

      deleteHandler('step-1', element);

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'button-1', type: 'BUTTON'},
            {id: 'password-1', type: ElementTypes.PasswordInput},
          ],
        },
      };

      const result = capturedCallback!(mockNode);
      // Components should remain unchanged since there's no form
      expect((result['components'] as Element[]).length).toBe(2);
    });
  });
});
