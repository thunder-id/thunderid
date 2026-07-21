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

import {render, screen, fireEvent} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import InteractionContext from '../../../context/InteractionContext';
import UIPanelContext from '../../../context/UIPanelContext';
import type {Base} from '../../../models/base';
import {ElementTypes} from '../../../models/elements';
import {ResourceTypes} from '../../../models/resources';

// Import after mocks
import ResourcePropertyPanel from '../ResourcePropertyPanel';

// Use vi.hoisted for mock functions
const {mockDeleteElements} = vi.hoisted(() => ({
  mockDeleteElements: vi.fn().mockResolvedValue({}),
}));

// Mock @xyflow/react
vi.mock('@xyflow/react', async () => {
  const actual = await vi.importActual('@xyflow/react');
  return {
    ...actual,
    useReactFlow: () => ({
      deleteElements: mockDeleteElements,
    }),
  };
});

// Mock ResourceProperties component
vi.mock('../ResourceProperties', () => ({
  default: () => <div data-testid="resource-properties">Resource Properties Content</div>,
}));

describe('ResourcePropertyPanel', () => {
  const mockSetIsOpenResourcePropertiesPanel = vi.fn();
  const mockOnComponentDelete = vi.fn();

  const mockBaseResource: Base = {
    id: 'resource-1',
    type: 'TEXT_INPUT',
    category: 'FIELD',
    resourceType: ResourceTypes.Element,
    version: '1.0.0',
    deprecated: false,
    deletable: true,
    display: {
      label: 'Test Resource',
      image: '',
      showOnResourcePanel: false,
    },
    config: {
      field: {name: '', type: ElementTypes},
      styles: {},
    },
  };

  const defaultUIPanelValue = {
    isResourcePanelOpen: true,
    isResourcePropertiesPanelOpen: false,
    isVersionHistoryPanelOpen: false,
    resourcePropertiesPanelHeading: 'Test Panel Heading' as ReactNode,
    setIsResourcePanelOpen: vi.fn(),
    setIsOpenResourcePropertiesPanel: mockSetIsOpenResourcePropertiesPanel,
    setIsVersionHistoryPanelOpen: vi.fn(),
    setResourcePropertiesPanelHeading: vi.fn(),
    registerCloseValidationPanel: vi.fn(),
  };

  const defaultInteractionValue = {
    lastInteractedResource: mockBaseResource,
    lastInteractedStepId: 'step-1',
    setLastInteractedResource: vi.fn(),
    setLastInteractedStepId: vi.fn(),
    onResourceDropOnCanvas: vi.fn(),
    selectedAttributes: {} as Record<string, never[]>,
    setSelectedAttributes: vi.fn(),
  };

  const createWrapper = (
    uiPanelOverrides: Partial<typeof defaultUIPanelValue> = {},
    interactionOverrides: Partial<typeof defaultInteractionValue> = {},
  ) => {
    const uiPanelValue = {...defaultUIPanelValue, ...uiPanelOverrides};
    const interactionValue = {...defaultInteractionValue, ...interactionOverrides};

    function Wrapper({children}: {children: ReactNode}) {
      return (
        <ReactFlowProvider>
          <UIPanelContext.Provider value={uiPanelValue}>
            <InteractionContext.Provider value={interactionValue}>{children}</InteractionContext.Provider>
          </UIPanelContext.Provider>
        </ReactFlowProvider>
      );
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render panel heading from context', () => {
      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {wrapper: createWrapper()});

      expect(screen.getByText('Test Panel Heading')).toBeInTheDocument();
    });

    it('should render ResourceProperties component', () => {
      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {wrapper: createWrapper()});

      expect(screen.getByTestId('resource-properties')).toBeInTheDocument();
    });

    it('should render delete button when resource is deletable', () => {
      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {wrapper: createWrapper()});

      expect(screen.getByRole('button', {name: 'Delete', hidden: true})).toBeInTheDocument();
    });

    it('should not render delete button when resource is not deletable', () => {
      const nonDeletableResource: Base = {
        ...mockBaseResource,
        deletable: false,
      };

      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {
        wrapper: createWrapper({}, {lastInteractedResource: nonDeletableResource}),
      });

      expect(screen.queryByRole('button', {name: 'Delete', hidden: true})).not.toBeInTheDocument();
    });
  });

  describe('Close Functionality', () => {
    it('should call setIsOpenResourcePropertiesPanel(false) when close button is clicked', () => {
      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {wrapper: createWrapper()});

      // Find the close button (the X icon button) - use hidden: true since drawer has aria-hidden
      const closeButton = screen.getAllByRole('button', {hidden: true})[0];
      fireEvent.click(closeButton);

      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(false);
    });
  });

  describe('Delete Functionality', () => {
    it('should delete step node when resource is a Step', () => {
      const stepResource: Base = {
        ...mockBaseResource,
        resourceType: ResourceTypes.Step,
      };

      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {
        wrapper: createWrapper({}, {lastInteractedResource: stepResource}),
      });

      const deleteButton = screen.getByRole('button', {name: 'Delete', hidden: true});
      fireEvent.click(deleteButton);

      expect(mockDeleteElements).toHaveBeenCalledWith({nodes: [{id: stepResource.id}]});
      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(false);
    });

    it('should call onComponentDelete when resource is not a Step', () => {
      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {wrapper: createWrapper()});

      const deleteButton = screen.getByRole('button', {name: 'Delete', hidden: true});
      fireEvent.click(deleteButton);

      expect(mockOnComponentDelete).toHaveBeenCalledWith('step-1', mockBaseResource);
      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(false);
    });

    it('should not render delete button when lastInteractedResource is null', () => {
      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {
        wrapper: createWrapper({}, {lastInteractedResource: null as unknown as Base}),
      });

      expect(screen.queryByRole('button', {name: 'Delete', hidden: true})).not.toBeInTheDocument();
    });
  });

  describe('Drawer State', () => {
    it('should render drawer when open prop is true', () => {
      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {wrapper: createWrapper()});

      const drawer = document.querySelector('.MuiDrawer-root');
      expect(drawer).toBeInTheDocument();
    });

    it('should render drawer as closed when open prop is false', () => {
      render(<ResourcePropertyPanel open={false} onComponentDelete={mockOnComponentDelete} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('resource-properties')).toBeInTheDocument();
    });
  });

  describe('Edge Cases', () => {
    it('should handle deleteElements rejection gracefully', () => {
      mockDeleteElements.mockRejectedValueOnce(new Error('Delete failed'));

      const stepResource: Base = {
        ...mockBaseResource,
        resourceType: ResourceTypes.Step,
      };

      render(<ResourcePropertyPanel open onComponentDelete={mockOnComponentDelete} />, {
        wrapper: createWrapper({}, {lastInteractedResource: stepResource}),
      });

      const deleteButton = screen.getByRole('button', {name: 'Delete', hidden: true});

      // Should not throw
      expect(() => fireEvent.click(deleteButton)).not.toThrow();
    });
  });
});
