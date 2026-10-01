// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Element as FlowElement} from '../../../../../models/elements';
import StackAdapter, {type StackElement} from '../StackAdapter';

const mockUpdateNodeData = vi.fn();

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    updateNodeData: mockUpdateNodeData,
  }),
}));

vi.mock('../../../../../hooks/useFlowPlugins', () => ({
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

vi.mock('../../../../../utils/generateResourceId', () => ({
  default: (prefix: string) => `${prefix}-generated`,
}));

vi.mock('../../../steps/view/ReorderableElement', () => ({
  default: ({
    element,
    id,
    extraActions,
  }: {
    element: FlowElement;
    id: string;
    index: number;
    extraActions?: ReactNode;
  }) => (
    <div data-testid={`reorderable-element-${id}`}>
      <span data-testid={`element-label-${id}`}>{element.id}</span>
      {extraActions && <div data-testid={`extra-actions-${id}`}>{extraActions}</div>}
    </div>
  ),
}));

vi.mock('../../../../dnd/Droppable', () => ({
  default: ({children, id}: {children: ReactNode; id: string}) => (
    <div data-testid="droppable" data-droppable-id={id}>
      {children}
    </div>
  ),
}));

vi.mock('../../../../dnd/Handle', () => ({
  default: ({children, label, onClick}: {children: ReactNode; label: string; onClick: () => void}) => (
    <button type="button" data-testid={`handle-${label}`} aria-label={label} onClick={onClick}>
      {children}
    </button>
  ),
}));

describe('StackAdapter', () => {
  const createMockElement = (overrides: Partial<StackElement> = {}): StackElement =>
    ({
      id: 'stack-1',
      type: 'BLOCK',
      category: 'BLOCK',
      config: {},
      ...overrides,
    }) as StackElement;

  const createChildElement = (id: string): FlowElement =>
    ({
      id,
      type: 'ELEMENT',
      category: 'FIELD',
      config: {},
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render droppable container', () => {
      const resource = createMockElement();

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('droppable')).toBeInTheDocument();
    });

    it('should render child elements via ReorderableFlowElement', () => {
      const resource = createMockElement({
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('reorderable-element-child-1')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-child-2')).toBeInTheDocument();
    });

    it('should not show a slot placeholder in flex mode, where the add button covers it', () => {
      const resource = createMockElement({components: []});

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('Drop here')).not.toBeInTheDocument();
    });

    it('should not show a slot placeholder when components is undefined', () => {
      const resource = createMockElement({components: undefined});

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('Drop here')).not.toBeInTheDocument();
    });

    it('should not show placeholder when children exist in flex mode', () => {
      const resource = createMockElement({
        components: [createChildElement('child-1')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('Drop here')).not.toBeInTheDocument();
    });
  });

  describe('Flex mode (fewer than two slots)', () => {
    it('should keep the flex layout when items is 1', () => {
      // Every builder-created stack is seeded with items 1, so this is the layout
      // previously authored flows rely on.
      const resource = createMockElement({
        items: 1,
        direction: 'row',
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('reorderable-element-child-1')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-child-2')).toBeInTheDocument();
      // Flex mode never pads with slot placeholders when children exist.
      expect(screen.queryByText('Drop here')).not.toBeInTheDocument();
    });

    it('should not show a slot placeholder when items is 1 and there are no children', () => {
      // A flex stack has no slot structure to convey, and the "Add Component" button
      // already fills the empty state.
      const resource = createMockElement({items: 1, direction: 'row', components: []});

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('Drop here')).not.toBeInTheDocument();
    });

    it('should keep the flex layout when items is absent', () => {
      const resource = createMockElement({
        direction: 'row',
        components: [],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('Drop here')).not.toBeInTheDocument();
    });
  });

  describe('Add component', () => {
    const stackElements: FlowElement[] = [
      {id: 'action', type: 'ACTION', display: {label: 'Button', showOnResourcePanel: true}} as unknown as FlowElement,
      {id: 'text', type: 'TEXT', display: {label: 'Text', showOnResourcePanel: true}} as unknown as FlowElement,
      {
        id: 'input',
        type: 'TEXT_INPUT',
        display: {label: 'Text Input', showOnResourcePanel: true},
      } as unknown as FlowElement,
      {id: 'hidden', type: 'TEXT', display: {label: 'Hidden', showOnResourcePanel: false}} as unknown as FlowElement,
    ];

    it('should render the add component button', () => {
      render(<StackAdapter resource={createMockElement()} stepId="step-1" availableElements={stackElements} />);

      expect(screen.getByTestId('stack-add-component-button')).toBeInTheDocument();
    });

    it('should not render the add button when nothing can be added', () => {
      render(<StackAdapter resource={createMockElement()} stepId="step-1" availableElements={[]} />);

      expect(screen.queryByTestId('stack-add-component-button')).not.toBeInTheDocument();
    });

    it('should offer only stack-compatible elements shown on the resource panel', () => {
      render(<StackAdapter resource={createMockElement()} stepId="step-1" availableElements={stackElements} />);

      fireEvent.click(screen.getByTestId('stack-add-component-button'));

      expect(screen.getByText('Button')).toBeInTheDocument();
      expect(screen.getByText('Text')).toBeInTheDocument();
      // TEXT_INPUT is a form field, and the hidden entry opts out of the panel.
      expect(screen.queryByText('Text Input')).not.toBeInTheDocument();
      expect(screen.queryByText('Hidden')).not.toBeInTheDocument();
    });

    it('should add the chosen element to this stack', () => {
      const onAddElementToForm = vi.fn();
      render(
        <StackAdapter
          resource={createMockElement()}
          stepId="step-1"
          availableElements={stackElements}
          onAddElementToForm={onAddElementToForm}
        />,
      );

      fireEvent.click(screen.getByTestId('stack-add-component-button'));
      fireEvent.click(screen.getByText('Button'));

      expect(onAddElementToForm).toHaveBeenCalledWith(expect.objectContaining({type: 'ACTION'}), 'stack-1');
    });
  });

  describe('Grid mode (multiple slots)', () => {
    it('should show empty placeholder slots for unoccupied grid positions', () => {
      const resource = createMockElement({
        items: 3,
        components: [createChildElement('child-1')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      // 3 items - 1 child = 2 empty slots
      const dropHereTexts = screen.getAllByText('Drop here');
      expect(dropHereTexts).toHaveLength(2);
    });

    it('should show no empty slots when all grid positions are filled', () => {
      const resource = createMockElement({
        items: 2,
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('Drop here')).not.toBeInTheDocument();
    });

    it('should show a placeholder for the trailing cell of a partially filled last row', () => {
      const resource = createMockElement({
        items: 2,
        components: [createChildElement('child-1'), createChildElement('child-2'), createChildElement('child-3')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      // 3 children across 2 columns fill row 1 and half of row 2.
      expect(screen.getAllByText('Drop here')).toHaveLength(1);
    });

    it('should show no empty slots when the last row is exactly filled', () => {
      const resource = createMockElement({
        items: 2,
        components: [
          createChildElement('child-1'),
          createChildElement('child-2'),
          createChildElement('child-3'),
          createChildElement('child-4'),
        ],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('Drop here')).not.toBeInTheDocument();
    });

    it('should fill the last row when children exceed two rows', () => {
      const resource = createMockElement({
        items: 3,
        components: [
          createChildElement('child-1'),
          createChildElement('child-2'),
          createChildElement('child-3'),
          createChildElement('child-4'),
        ],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      // 4 children across 3 columns leave 2 empty cells in row 2.
      expect(screen.getAllByText('Drop here')).toHaveLength(2);
    });
  });

  describe('Move actions for row direction', () => {
    it('should show Move Right but not Move Left for first element', () => {
      const resource = createMockElement({
        direction: 'row',
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      const firstActions = screen.getByTestId('extra-actions-child-1');
      expect(firstActions.querySelector('[data-testid="handle-Move Right"]')).toBeInTheDocument();
      expect(firstActions.querySelector('[data-testid="handle-Move Left"]')).not.toBeInTheDocument();
    });

    it('should show Move Left but not Move Right for last element', () => {
      const resource = createMockElement({
        direction: 'row',
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      const lastActions = screen.getByTestId('extra-actions-child-2');
      expect(lastActions.querySelector('[data-testid="handle-Move Left"]')).toBeInTheDocument();
      expect(lastActions.querySelector('[data-testid="handle-Move Right"]')).not.toBeInTheDocument();
    });

    it('should show both Move Left and Move Right for middle element', () => {
      const resource = createMockElement({
        direction: 'row',
        components: [createChildElement('child-1'), createChildElement('child-2'), createChildElement('child-3')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      const middleActions = screen.getByTestId('extra-actions-child-2');
      expect(middleActions.querySelector('[data-testid="handle-Move Left"]')).toBeInTheDocument();
      expect(middleActions.querySelector('[data-testid="handle-Move Right"]')).toBeInTheDocument();
    });
  });

  describe('Move actions for column direction', () => {
    it('should show Move Down but not Move Up for first element', () => {
      const resource = createMockElement({
        direction: 'column',
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      const firstActions = screen.getByTestId('extra-actions-child-1');
      expect(firstActions.querySelector('[data-testid="handle-Move Down"]')).toBeInTheDocument();
      expect(firstActions.querySelector('[data-testid="handle-Move Up"]')).not.toBeInTheDocument();
    });

    it('should show Move Up but not Move Down for last element', () => {
      const resource = createMockElement({
        direction: 'column',
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      const lastActions = screen.getByTestId('extra-actions-child-2');
      expect(lastActions.querySelector('[data-testid="handle-Move Up"]')).toBeInTheDocument();
      expect(lastActions.querySelector('[data-testid="handle-Move Down"]')).not.toBeInTheDocument();
    });
  });

  describe('Move functionality', () => {
    it('should call updateNodeData when Move Right is clicked', () => {
      const resource = createMockElement({
        direction: 'row',
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      const moveRightButton = screen.getByTestId('handle-Move Right');
      fireEvent.click(moveRightButton);

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
    });

    it('should call updateNodeData when Move Left is clicked', () => {
      const resource = createMockElement({
        direction: 'row',
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      const moveLeftButton = screen.getByTestId('handle-Move Left');
      fireEvent.click(moveLeftButton);

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
    });

    it('should swap elements when move callback is executed', () => {
      const child1 = createChildElement('child-1');
      const child2 = createChildElement('child-2');
      const resource = createMockElement({
        id: 'stack-1',
        direction: 'row',
        components: [child1, child2],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      // Click Move Right on first child
      fireEvent.click(screen.getByTestId('handle-Move Right'));

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));

      // Execute the callback to verify the swap logic
      const updateFn = mockUpdateNodeData.mock.calls[0][1] as (node: {data: unknown}) => unknown;
      const result = updateFn({
        data: {
          components: [
            {
              id: 'stack-1',
              components: [child1, child2],
            },
          ],
        },
      });

      const updatedStack = (result as {components: FlowElement[]}).components[0];
      expect(updatedStack.components![0].id).toBe('child-2');
      expect(updatedStack.components![1].id).toBe('child-1');
    });

    it('should not swap when element is not found in stack', () => {
      const child1 = createChildElement('child-1');
      const child2 = createChildElement('child-2');
      const resource = createMockElement({
        id: 'stack-1',
        direction: 'row',
        components: [child1, child2],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      fireEvent.click(screen.getByTestId('handle-Move Right'));

      const updateFn = mockUpdateNodeData.mock.calls[0][1] as (node: {data: unknown}) => unknown;
      // Pass node data where the stack has different children
      const result = updateFn({
        data: {
          components: [
            {
              id: 'stack-1',
              components: [createChildElement('other-1'), createChildElement('other-2')],
            },
          ],
        },
      });

      // Should remain unchanged since child-1 is not in this stack
      const updatedStack = (result as {components: FlowElement[]}).components[0];
      expect(updatedStack.components![0].id).toBe('other-1');
      expect(updatedStack.components![1].id).toBe('other-2');
    });

    it('should recursively search nested elements for the stack', () => {
      const child1 = createChildElement('child-1');
      const child2 = createChildElement('child-2');
      const resource = createMockElement({
        id: 'stack-1',
        direction: 'row',
        components: [child1, child2],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      fireEvent.click(screen.getByTestId('handle-Move Right'));

      const updateFn = mockUpdateNodeData.mock.calls[0][1] as (node: {data: unknown}) => unknown;
      // Stack is nested inside a parent element
      const result = updateFn({
        data: {
          components: [
            {
              id: 'parent-block',
              components: [
                {
                  id: 'stack-1',
                  components: [child1, child2],
                },
              ],
            },
          ],
        },
      });

      const parentBlock = (result as {components: FlowElement[]}).components[0];
      const updatedStack = parentBlock.components![0];
      expect(updatedStack.components![0].id).toBe('child-2');
      expect(updatedStack.components![1].id).toBe('child-1');
    });
  });

  describe('Single element', () => {
    it('should not show any move actions for a single element', () => {
      const resource = createMockElement({
        direction: 'row',
        components: [createChildElement('child-1')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      const actions = screen.getByTestId('extra-actions-child-1');
      expect(actions.querySelector('[data-testid^="handle-"]')).not.toBeInTheDocument();
    });
  });

  describe('Default direction', () => {
    it('should default to row direction and show left/right actions', () => {
      const resource = createMockElement({
        // No direction specified
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      // Should use row direction by default — Move Right for first, Move Left for last
      expect(screen.getByTestId('handle-Move Right')).toBeInTheDocument();
      expect(screen.getByTestId('handle-Move Left')).toBeInTheDocument();
    });
  });

  describe('Plugin filtering', () => {
    it('should filter components through useFlowPlugins', () => {
      // useFlowPlugins mock returns true for all, so all children render
      const resource = createMockElement({
        components: [createChildElement('child-1'), createChildElement('child-2')],
      });

      render(<StackAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('reorderable-element-child-1')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-child-2')).toBeInTheDocument();
    });
  });
});
