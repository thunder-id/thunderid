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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import View from '../View';
import type {Element} from '@/features/flows/models/elements';

// Mock i18next
const translations: Record<string, string> = {
  'flows:core.steps.view.addComponent': 'Add Component',
  'flows:core.steps.view.configure': 'Configure',
  'flows:core.steps.view.remove': 'Remove',
  'flows:core.steps.view.noComponentsAvailable': 'No components available',
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => translations[key] || key,
  }),
}));

// Mock @xyflow/react
const mockDeleteElements = vi.fn();
const mockGetNode = vi.fn(() => ({id: 'view-node', data: {}}));
const mockUseNodeId = vi.fn(() => 'view-node-id');

vi.mock('@xyflow/react', () => ({
  Handle: ({type, position, id = ''}: {type: string; position: string; id?: string}) => (
    <div data-testid={`handle-${type}`} data-position={position} data-handle-id={id} />
  ),
  Position: {
    Left: 'left',
    Right: 'right',
    Top: 'top',
    Bottom: 'bottom',
  },
  useNodeId: () => mockUseNodeId(),
  useReactFlow: () => ({
    deleteElements: mockDeleteElements,
    getNode: mockGetNode,
  }),
}));

// Mock useFlowPlugins
vi.mock('@/features/flows/hooks/useFlowPlugins', () => ({
  default: () => ({
    onPropertyChange: vi.fn().mockReturnValue(vi.fn()),
    emitPropertyChange: vi.fn().mockReturnValue(true),
    onPropertyPanelOpen: vi.fn().mockReturnValue(vi.fn()),
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
  }),
}));

// Mock generateResourceId
vi.mock('@/features/flows/utils/generateResourceId', () => ({
  default: (prefix: string) => `${prefix}-generated-id`,
}));

// Mock Droppable
vi.mock('../../../dnd/Droppable', () => ({
  default: ({children, id, accept}: {children: React.ReactNode; id: string; accept: string[]}) => (
    <div data-testid="droppable" data-id={id} data-accept={JSON.stringify(accept)}>
      {children}
    </div>
  ),
}));

// Mock ReorderableViewElement
vi.mock('../ReorderableElement', () => ({
  default: ({element, index}: {element: Element; index: number}) => (
    <div data-testid={`reorderable-element-${index}`} data-element-id={element.id}>
      {element.display?.label || element.type}
    </div>
  ),
}));

describe('View', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNodeId.mockReturnValue('view-node-id');
  });

  describe('Rendering', () => {
    it('should render the View component', () => {
      render(<View />);

      expect(screen.getByText('View')).toBeInTheDocument();
    });

    it('should render with custom heading', () => {
      render(<View heading="Login Form" />);

      expect(screen.getByText('Login Form')).toBeInTheDocument();
    });

    it('should render with flow-builder-step class', () => {
      const {container} = render(<View />);

      expect(container.querySelector('.flow-builder-step')).toBeInTheDocument();
    });

    it('should accept custom className', () => {
      const {container} = render(<View className="custom-view" />);

      expect(container.querySelector('.custom-view')).toBeInTheDocument();
    });
  });

  describe('React Flow Handles', () => {
    it('should render a target handle on the left', () => {
      render(<View />);

      const handle = screen.getByTestId('handle-target');
      expect(handle).toBeInTheDocument();
      expect(handle).toHaveAttribute('data-position', 'left');
    });

    it('should render source handle when enableSourceHandle is true', () => {
      render(<View enableSourceHandle />);

      const handles = screen.getAllByTestId(/handle-/);
      const sourceHandle = handles.find((h) => h.getAttribute('data-testid') === 'handle-source');
      expect(sourceHandle).toBeInTheDocument();
    });

    it('should not render source handle by default', () => {
      render(<View />);

      const sourceHandle = screen.queryByTestId('handle-source');
      expect(sourceHandle).not.toBeInTheDocument();
    });
  });

  describe('Delete Button', () => {
    it('should render delete button when deletable is true', () => {
      render(<View deletable />);

      // Delete button should be present
      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should not render delete button when deletable is false', () => {
      render(<View deletable={false} />);

      // When no components or configure, and not deletable, there may be no buttons
      // The tooltip text won't be visible unless hovered
      expect(screen.queryByText('Remove')).not.toBeInTheDocument();
    });

    it('should call deleteElements when delete button is clicked', () => {
      render(<View deletable />);

      // Find the delete button (last button in the action panel)
      const buttons = screen.getAllByRole('button');
      const deleteButton = buttons[buttons.length - 1];
      fireEvent.click(deleteButton);

      expect(mockDeleteElements).toHaveBeenCalledWith({
        nodes: [{id: 'view-node-id'}],
      });
    });
  });

  describe('Configure Button', () => {
    it('should render configure button when configurable is true', () => {
      const onConfigure = vi.fn();
      render(<View configurable onConfigure={onConfigure} />);

      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should call onConfigure when configure button is clicked', () => {
      const onConfigure = vi.fn();
      render(<View configurable onConfigure={onConfigure} deletable={false} />);

      const buttons = screen.getAllByRole('button');
      // When only configurable, the configure button should be the only one
      fireEvent.click(buttons[0]);

      expect(onConfigure).toHaveBeenCalled();
    });

    it('should not render configure button by default', () => {
      render(<View deletable={false} />);

      const buttons = screen.queryAllByRole('button');
      expect(buttons.length).toBe(0);
    });
  });

  describe('Add Component Menu', () => {
    const mockElements: Element[] = [
      {
        id: 'elem-1',
        type: 'TEXT_INPUT',
        category: 'INPUT',
        resourceType: 'ELEMENT',
        display: {label: 'Text Input', showOnResourcePanel: true},
      },
      {
        id: 'elem-2',
        type: 'BUTTON',
        category: 'ACTION',
        resourceType: 'ELEMENT',
        display: {label: 'Button', showOnResourcePanel: true},
      },
    ] as Element[];

    it('should render add button when availableElements is provided', () => {
      render(<View availableElements={mockElements} deletable={false} />);

      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should open menu when add button is clicked', () => {
      render(<View availableElements={mockElements} deletable={false} />);

      const addButton = screen.getAllByRole('button')[0];
      fireEvent.click(addButton);

      // Menu items should appear
      expect(screen.getByText('Text Input')).toBeInTheDocument();
      expect(screen.getByText('Button')).toBeInTheDocument();
    });

    it('should render a persistent dashed add button below the components', () => {
      render(<View availableElements={mockElements} deletable={false} />);

      expect(screen.getByTestId('view-add-element-button')).toBeInTheDocument();
    });

    it('should not render the dashed add button when no elements are available', () => {
      render(<View availableElements={[]} deletable={false} />);

      expect(screen.queryByTestId('view-add-element-button')).not.toBeInTheDocument();
    });

    it('should show the dashed add button once the available elements load', () => {
      const {rerender} = render(<View availableElements={[]} deletable={false} />);

      expect(screen.queryByTestId('view-add-element-button')).not.toBeInTheDocument();

      rerender(<View availableElements={mockElements} deletable={false} />);

      expect(screen.getByTestId('view-add-element-button')).toBeInTheDocument();
    });

    it('should open the add menu from the dashed add button', () => {
      render(<View availableElements={mockElements} deletable={false} />);

      fireEvent.click(screen.getByTestId('view-add-element-button'));

      expect(screen.getByText('Text Input')).toBeInTheDocument();
      expect(screen.getByText('Button')).toBeInTheDocument();
    });

    it('should call onAddElement when menu item is clicked', () => {
      const onAddElement = vi.fn();
      render(<View availableElements={mockElements} onAddElement={onAddElement} deletable={false} />);

      const addButton = screen.getAllByRole('button')[0];
      fireEvent.click(addButton);

      const menuItem = screen.getByText('Text Input');
      fireEvent.click(menuItem);

      expect(onAddElement).toHaveBeenCalledWith(mockElements[0], 'view-node-id');
    });

    it('should close menu when clicking outside', () => {
      render(<View availableElements={mockElements} deletable={false} />);

      const addButton = screen.getAllByRole('button')[0];
      fireEvent.click(addButton);

      // Menu should be open
      expect(screen.getByText('Text Input')).toBeInTheDocument();

      // Find the menu backdrop/overlay and click it to close
      const menu = screen.getByRole('menu');
      fireEvent.keyDown(menu, {key: 'Escape'});

      // Menu should be closed after escape
      expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    });

    it('should filter out elements with showOnResourcePanel=false', () => {
      const elementsWithHidden: Element[] = [
        ...mockElements,
        {
          id: 'elem-3',
          type: 'HIDDEN',
          category: 'OTHER',
          resourceType: 'ELEMENT',
          display: {label: 'Hidden Element', showOnResourcePanel: false},
        },
      ] as Element[];

      render(<View availableElements={elementsWithHidden} deletable={false} />);

      const addButton = screen.getAllByRole('button')[0];
      fireEvent.click(addButton);

      expect(screen.queryByText('Hidden Element')).not.toBeInTheDocument();
    });
  });

  describe('Components Rendering', () => {
    it('should render components in form group', () => {
      const components: Element[] = [
        {
          id: 'comp-1',
          type: 'TEXT_INPUT',
          category: 'INPUT',
          resourceType: 'ELEMENT',
          display: {label: 'Username'},
        },
        {
          id: 'comp-2',
          type: 'PASSWORD_INPUT',
          category: 'INPUT',
          resourceType: 'ELEMENT',
          display: {label: 'Password'},
        },
      ] as Element[];

      render(<View data={{components}} />);

      // Components are rendered inside the form group
      expect(screen.getByTestId('reorderable-element-0')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-1')).toBeInTheDocument();
    });

    it('should render empty form when no components', () => {
      render(<View data={{components: []}} />);

      // Form group should still be rendered even with no components
      const formGroup = document.querySelector('.MuiFormGroup-root');
      expect(formGroup).toBeInTheDocument();
      expect(screen.queryByTestId('reorderable-element-0')).not.toBeInTheDocument();
    });
  });

  describe('Action Panel Double Click', () => {
    it('should call onActionPanelDoubleClick when action panel is double clicked', () => {
      const onDoubleClick = vi.fn();
      render(<View onActionPanelDoubleClick={onDoubleClick} />);

      const actionPanel = screen.getByTestId('step-action-panel');
      fireEvent.doubleClick(actionPanel);
      expect(onDoubleClick).toHaveBeenCalled();
    });
  });

  describe('Droppable Configuration', () => {
    it('should accept custom droppableAllowedTypes prop', () => {
      // View component accepts droppableAllowedTypes prop
      render(<View droppableAllowedTypes={['CUSTOM_TYPE']} />);

      // Component should render without errors
      expect(screen.getByText('View')).toBeInTheDocument();
    });
  });

  describe('Memoization', () => {
    it('should render correctly on rerender with same props', () => {
      const {rerender} = render(<View heading="Test View" />);

      expect(screen.getByText('Test View')).toBeInTheDocument();

      rerender(<View heading="Test View" />);

      expect(screen.getByText('Test View')).toBeInTheDocument();
    });

    it('should re-render when data prop changes', () => {
      const data1 = {components: []};
      const data2 = {
        components: [
          {id: 'new-comp', type: 'TEXT', category: 'INPUT', resourceType: 'ELEMENT', display: {label: 'New'}},
        ],
      };

      const {rerender} = render(<View data={data1} />);
      expect(screen.queryByTestId('reorderable-element-0')).not.toBeInTheDocument();

      rerender(<View data={data2 as {components: Element[]}} />);
      expect(screen.getByTestId('reorderable-element-0')).toBeInTheDocument();
    });

    it('should re-render when heading prop changes', () => {
      const {rerender} = render(<View heading="Initial Heading" />);
      expect(screen.getByText('Initial Heading')).toBeInTheDocument();

      rerender(<View heading="Updated Heading" />);
      expect(screen.getByText('Updated Heading')).toBeInTheDocument();
    });

    it('should re-render when deletable prop changes', () => {
      const {rerender} = render(<View deletable={false} configurable={false} />);
      expect(screen.queryAllByRole('button')).toHaveLength(0);

      rerender(<View deletable configurable={false} />);
      // Delete button should now be visible
      expect(screen.queryAllByRole('button').length).toBeGreaterThan(0);
    });

    it('should re-render when configurable prop changes', () => {
      const {rerender} = render(<View deletable={false} configurable={false} />);
      expect(screen.queryAllByRole('button')).toHaveLength(0);

      rerender(<View deletable={false} configurable />);
      // Configure button should now be visible
      expect(screen.queryAllByRole('button').length).toBeGreaterThan(0);
    });

    it('should re-render when enableSourceHandle prop changes', () => {
      const {rerender} = render(<View enableSourceHandle={false} />);
      expect(screen.queryByTestId('handle-source')).not.toBeInTheDocument();

      rerender(<View enableSourceHandle />);
      expect(screen.getByTestId('handle-source')).toBeInTheDocument();
    });

    it('should not re-render when only callback props change', () => {
      const onAddElement1 = vi.fn();
      const onAddElement2 = vi.fn();

      const {rerender} = render(<View heading="Same Heading" onAddElement={onAddElement1} />);
      expect(screen.getByText('Same Heading')).toBeInTheDocument();

      // Changing only the callback should not cause visual changes (memoized)
      rerender(<View heading="Same Heading" onAddElement={onAddElement2} />);
      expect(screen.getByText('Same Heading')).toBeInTheDocument();
    });
  });
});
