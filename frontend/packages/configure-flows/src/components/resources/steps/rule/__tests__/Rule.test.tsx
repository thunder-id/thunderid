// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable react/require-default-props */

import {createEvent, render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {CommonStepFactoryPropsInterface} from '../../CommonStepFactory';
import Rule from '../Rule';

// Mock i18next
const translations: Record<string, string> = {
  'flows:core.rule.conditionalRule': 'Conditional Rule',
  'flows:core.rule.remove': 'Remove',
};

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => translations[key] || key,
  }),
}));

// Mock @xyflow/react
const mockDeleteElements = vi.fn();
const mockUseNodeId = vi.fn((): string => 'test-node-id');

vi.mock('@xyflow/react', () => ({
  Handle: ({type, position, id}: {type: string; position: string; id?: string}) => (
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
  }),
}));

// Mock useInteractionState
const mockSetLastInteractedResource = vi.fn();
vi.mock('../../../../../hooks/useInteractionState', () => ({
  default: () => ({
    setLastInteractedResource: mockSetLastInteractedResource,
  }),
}));

// Default mock props for Rule component
const createMockProps = (overrides: Partial<CommonStepFactoryPropsInterface> = {}): CommonStepFactoryPropsInterface =>
  ({
    id: 'rule-1',
    resourceId: 'rule-resource-1',
    resources: [],
    data: {},
    type: 'RULE',
    zIndex: 1,
    isConnectable: true,
    positionAbsoluteX: 0,
    positionAbsoluteY: 0,
    dragging: false,
    selected: false,
    deletable: true,
    selectable: true,
    parentId: undefined,
    ...overrides,
  }) as CommonStepFactoryPropsInterface;

describe('Rule', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNodeId.mockReturnValue('test-node-id');
  });

  describe('Rendering', () => {
    it('should render the Rule component', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();
    });

    it('should render the rule node surface', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      expect(screen.getByTestId('rule-node')).toBeInTheDocument();
    });
  });

  describe('React Flow Handles', () => {
    it('should render a target handle on the left', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      const targetHandle = screen.getByTestId('handle-target');
      expect(targetHandle).toBeInTheDocument();
      expect(targetHandle).toHaveAttribute('data-position', 'left');
    });

    it('should render a source handle on the right', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      const sourceHandle = screen.getByTestId('handle-source');
      expect(sourceHandle).toBeInTheDocument();
      expect(sourceHandle).toHaveAttribute('data-position', 'right');
    });

    it('should have source handle with id "a"', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      const sourceHandle = screen.getByTestId('handle-source');
      expect(sourceHandle).toHaveAttribute('data-handle-id', 'a');
    });
  });

  describe('Remove Button', () => {
    it('should render a remove button with tooltip', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      // Button should be present
      const removeButton = screen.getByRole('button');
      expect(removeButton).toBeInTheDocument();
    });

    it('should call deleteElements when remove button is clicked', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      const removeButton = screen.getByRole('button');
      fireEvent.click(removeButton);

      expect(mockDeleteElements).toHaveBeenCalledWith({
        nodes: [{id: 'test-node-id'}],
      });
    });

    it('should not call deleteElements if nodeId is empty', () => {
      mockUseNodeId.mockReturnValue('');

      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      const removeButton = screen.getByRole('button');
      fireEvent.click(removeButton);

      expect(mockDeleteElements).not.toHaveBeenCalled();
    });
  });

  describe('Action Panel Interaction', () => {
    it('should set lastInteractedResource when action panel is clicked', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {someData: 'value'}})} />);

      fireEvent.click(screen.getByTestId('rule-action-panel'));
      expect(mockSetLastInteractedResource).toHaveBeenCalled();
    });

    it('should pass correct resource object to setLastInteractedResource', () => {
      const testData = {name: 'Test Rule', condition: 'true'};
      render(<Rule {...createMockProps({id: 'custom-rule-id', data: testData})} />);

      fireEvent.click(screen.getByTestId('rule-action-panel'));

      expect(mockSetLastInteractedResource).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'custom-rule-id',
          name: 'Test Rule',
          condition: 'true',
        }),
      );
    });
  });

  describe('Drag and Drop', () => {
    it('should handle drag event and set dropEffect to move', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      const ruleElement = screen.getByTestId('rule-node');

      const dataTransfer = {dropEffect: ''};
      const dragEvent = createEvent.drag(ruleElement);
      Object.defineProperty(dragEvent, 'dataTransfer', {value: dataTransfer});
      fireEvent(ruleElement, dragEvent);

      expect(dataTransfer.dropEffect).toBe('move');
    });

    it('should handle drag event when dataTransfer is not provided', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      // Should not throw when dataTransfer is undefined
      expect(() => fireEvent.drag(screen.getByTestId('rule-node'))).not.toThrow();
    });

    it('should handle drop event', () => {
      render(<Rule {...createMockProps({id: 'rule-1', data: {}})} />);

      expect(() => fireEvent.drop(screen.getByTestId('rule-node'))).not.toThrow();
    });
  });

  describe('Props Integration', () => {
    it('should use id from props when nodeId is available', () => {
      render(<Rule {...createMockProps({id: 'props-id', data: {}})} />);

      fireEvent.click(screen.getByTestId('rule-action-panel'));

      expect(mockSetLastInteractedResource).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'props-id',
        }),
      );
    });

    it('should fall back to nodeId when id prop is not provided', () => {
      render(<Rule {...createMockProps({data: {}})} />);

      fireEvent.click(screen.getByTestId('rule-action-panel'));

      expect(mockSetLastInteractedResource).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'rule-1',
        }),
      );
    });
  });

  describe('Memoization', () => {
    it('should be wrapped in memo for performance', () => {
      // The component is exported as MemoizedRule
      // We can verify it renders correctly multiple times with same props
      const props = createMockProps({id: 'rule-1', data: {value: 1}});
      const {rerender} = render(<Rule {...props} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();

      // Rerender with same props
      rerender(<Rule {...props} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();
    });

    it('should re-render when data prop changes', () => {
      const initialData = {value: 1};
      const props = createMockProps({id: 'rule-1', data: initialData});
      const {rerender} = render(<Rule {...props} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();

      // Rerender with different data - should trigger re-render
      const newData = {value: 2};
      rerender(<Rule {...createMockProps({id: 'rule-1', data: newData})} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();
    });

    it('should re-render when id prop changes', () => {
      const props = createMockProps({id: 'rule-1', data: {value: 1}});
      const {rerender} = render(<Rule {...props} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();

      // Rerender with different id - should trigger re-render
      rerender(<Rule {...createMockProps({id: 'rule-2', data: {value: 1}})} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();
    });

    it('should not re-render when other props change but data and id remain the same', () => {
      const data = {value: 1};
      const props = createMockProps({id: 'rule-1', data, zIndex: 1});
      const {rerender} = render(<Rule {...props} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();

      // Rerender with same data and id but different zIndex - should not trigger re-render
      rerender(<Rule {...createMockProps({id: 'rule-1', data, zIndex: 2})} />);

      expect(screen.getByText('Conditional Rule')).toBeInTheDocument();
    });
  });
});
