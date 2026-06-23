/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {renderHook} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {Node} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {ElementTypes} from '../../models/elements';
import {StepTypes, ExecutionTypes} from '../../models/steps';
import useStaticContentField from '../useStaticContentField';

// Use vi.hoisted to define mocks that need to be referenced in vi.mock
const {mockGetNode, mockUpdateNodeData} = vi.hoisted(() => ({
  mockGetNode: vi.fn(),
  mockUpdateNodeData: vi.fn(),
}));

// Store registered handlers for testing
const registeredHandlers: {
  onPropertyChange: ((...args: unknown[]) => boolean)[];
  onPropertyPanelOpen: ((...args: unknown[]) => boolean)[];
} = {
  onPropertyChange: [],
  onPropertyPanelOpen: [],
};

const mockUnsubscribes: {
  onPropertyChange: ReturnType<typeof vi.fn>[];
  onPropertyPanelOpen: ReturnType<typeof vi.fn>[];
} = {
  onPropertyChange: [],
  onPropertyPanelOpen: [],
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
  onNodeElementDelete: vi.fn().mockReturnValue(vi.fn()),
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

// Mock useGetFlowBuilderCoreResources
vi.mock('../../api/useGetFlowBuilderCoreResources', () => ({
  default: () => ({
    data: {
      elements: [
        {
          id: 'rich-text-element',
          type: ElementTypes.RichText,
          config: {text: ''},
        },
      ],
    },
    isLoading: false,
    error: null,
  }),
}));

// Mock useFlowPlugins - capture handlers for testing
vi.mock('../useFlowPlugins', () => ({
  default: () => mockFlowPlugins,
}));

// Mock generateResourceId
vi.mock('../../utils/generateResourceId', () => ({
  default: vi.fn().mockReturnValue('generated-id'),
}));

describe('useStaticContentField', () => {
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
    mockUnsubscribes.onPropertyChange = [];
    mockUnsubscribes.onPropertyPanelOpen = [];
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
  });

  describe('Plugin Registration', () => {
    it('should register event handlers on mount', () => {
      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      expect(mockOnPropertyChange).toHaveBeenCalledWith(expect.any(Function));
      expect(mockOnPropertyPanelOpen).toHaveBeenCalledWith(expect.any(Function));
    });

    it('should call unsubscribe functions on unmount', () => {
      const {unmount} = renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      unmount();

      mockUnsubscribes.onPropertyChange.forEach((unsub) => expect(unsub).toHaveBeenCalled());
      mockUnsubscribes.onPropertyPanelOpen.forEach((unsub) => expect(unsub).toHaveBeenCalled());
    });
  });

  describe('addStaticContent Handler', () => {
    it('should return true for non-execution step types', () => {
      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addStaticContentHandler = registeredHandlers.onPropertyChange?.[0];
      expect(addStaticContentHandler).toBeDefined();

      const element = {
        id: 'view-1',
        type: StepTypes.View,
      };

      const result = addStaticContentHandler('enableStaticContent', true, element, 'step-1');
      expect(result).toBe(true);
    });

    it('should return true for properties other than enableStaticContent', () => {
      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addStaticContentHandler = registeredHandlers.onPropertyChange?.[0];

      const element = {
        id: 'execution-1',
        type: StepTypes.Execution,
      };

      const result = addStaticContentHandler('someOtherProperty', true, element, 'step-1');
      expect(result).toBe(true);
    });

    it('should add static content when enableStaticContent is true', () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {
          components: [],
        },
      };

      mockGetNode.mockReturnValue(executionNode);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addStaticContentHandler = registeredHandlers.onPropertyChange?.[0];

      const element = {
        id: 'execution-1',
        type: StepTypes.Execution,
      };

      const result = addStaticContentHandler('enableStaticContent', true, element, 'execution-1');
      expect(result).toBe(false);
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should remove static content when enableStaticContent is false', () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'rich-text-1', type: ElementTypes.RichText}],
        },
      };

      mockGetNode.mockReturnValue(executionNode);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addStaticContentHandler = registeredHandlers.onPropertyChange?.[0];

      const element = {
        id: 'execution-1',
        type: StepTypes.Execution,
      };

      const result = addStaticContentHandler('enableStaticContent', false, element, 'execution-1');
      expect(result).toBe(false);
      expect(mockUpdateNodeData).toHaveBeenCalled();
    });

    it('should call updateNodeData callback correctly when adding content', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addStaticContentHandler = registeredHandlers.onPropertyChange?.[0];

      const element = {
        id: 'execution-1',
        type: StepTypes.Execution,
      };

      addStaticContentHandler('enableStaticContent', true, element, 'execution-1');

      // Execute the captured callback
      const mockNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {components: []},
      };

      expect(capturedCallback).not.toBeNull();
      const result = capturedCallback!(mockNode);
      expect(result.components).toBeDefined();
    });

    it('should call updateNodeData callback correctly when removing content', () => {
      let capturedCallback: ((node: Node) => Record<string, unknown>) | null = null;
      mockUpdateNodeData.mockImplementation((_stepId: string, callback: (node: Node) => Record<string, unknown>) => {
        capturedCallback = callback;
      });

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addStaticContentHandler = registeredHandlers.onPropertyChange?.[0];

      const element = {
        id: 'execution-1',
        type: StepTypes.Execution,
      };

      addStaticContentHandler('enableStaticContent', false, element, 'execution-1');

      // Execute the captured callback
      const mockNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'rich-text-1', type: ElementTypes.RichText}],
        },
      };

      expect(capturedCallback).not.toBeNull();
      const result = capturedCallback!(mockNode);
      expect(result.components).toEqual([]);
    });
  });

  describe('addStaticContentProperties Handler', () => {
    it('should return true when node is not found', () => {
      mockGetNode.mockReturnValue(undefined);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];
      expect(addPropertiesHandler).toBeDefined();

      const resource = {
        id: 'execution-1',
        type: StepTypes.Execution,
        data: {
          action: {
            executor: {name: ExecutionTypes.SMSExecutor},
          },
        },
      };

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'step-1');
      expect(result).toBe(true);
      expect(properties.enableStaticContent).toBeUndefined();
    });

    it('should return true for non-execution step types', () => {
      const viewNode: Node = {
        id: 'view-1',
        type: StepTypes.View,
        position: {x: 0, y: 0},
        data: {},
      };

      mockGetNode.mockReturnValue(viewNode);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'view-1',
        type: StepTypes.View,
      };

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'view-1');
      expect(result).toBe(true);
      expect(properties.enableStaticContent).toBeUndefined();
    });

    it('should not add enableStaticContent property for non-allowed execution types', () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'rich-text-1', type: ElementTypes.RichText}],
        },
      };

      mockGetNode.mockReturnValue(executionNode);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      // SMSExecutor is not in the allowed types list
      const resource = {
        id: 'execution-1',
        type: StepTypes.Execution,
        data: {
          action: {
            executor: {name: ExecutionTypes.SMSExecutor},
          },
        },
      };

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'execution-1');
      expect(result).toBe(true);
      // Property not set because SMSExecutor is not in allowed types
      expect(properties.enableStaticContent).toBeUndefined();
    });

    it('should not add property for non-allowed execution types even without components', () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {
          components: [],
        },
      };

      mockGetNode.mockReturnValue(executionNode);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      // SMSExecutor is not in the allowed types list
      const resource = {
        id: 'execution-1',
        type: StepTypes.Execution,
        data: {
          action: {
            executor: {name: ExecutionTypes.SMSExecutor},
          },
        },
      };

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'execution-1');
      expect(result).toBe(true);
      // Property not set because SMSExecutor is not in allowed types
      expect(properties.enableStaticContent).toBeUndefined();
    });

    it('should return true for MagicLinkExecutor without adding property', () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {
          components: [],
        },
      };

      mockGetNode.mockReturnValue(executionNode);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'execution-1',
        type: StepTypes.Execution,
        data: {
          action: {
            executor: {name: ExecutionTypes.MagicLinkExecutor},
          },
        },
      };

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'execution-1');
      expect(result).toBe(true);
      expect(properties.enableStaticContent).toBeUndefined();
    });

    it('should return true when executor name is not in allowed types', () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {
          components: [],
        },
      };

      mockGetNode.mockReturnValue(executionNode);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'execution-1',
        type: StepTypes.Execution,
        data: {
          action: {
            executor: {name: 'SomeOtherExecutor'},
          },
        },
      };

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'execution-1');
      expect(result).toBe(true);
    });

    it('should return true when executor is undefined', () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {
          components: [],
        },
      };

      mockGetNode.mockReturnValue(executionNode);

      renderHook(() => useStaticContentField(), {
        wrapper: createWrapper(),
      });

      const addPropertiesHandler = registeredHandlers.onPropertyPanelOpen?.[0];

      const resource = {
        id: 'execution-1',
        type: StepTypes.Execution,
        data: {},
      };

      const properties: Record<string, unknown> = {};
      const result = addPropertiesHandler(resource, properties, 'execution-1');
      expect(result).toBe(true);
    });
  });
});
