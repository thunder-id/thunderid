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

/* eslint-disable react/require-default-props, jsx-a11y/no-static-element-interactions */

import {render, screen, fireEvent} from '@testing-library/react';
import React, {type ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowConfigContext, {type FlowConfigContextProps} from '../../../context/FlowConfigContext';
import {EdgeStyleTypes} from '../../../models/steps';

// Import after mocks are set up
import BaseEdge from '../BaseEdge';

// Use vi.hoisted to define mocks that need to be referenced in vi.mock
const {mockDeleteElements, mockGetNodes, mockUseStore} = vi.hoisted(() => ({
  mockDeleteElements: vi.fn().mockResolvedValue({}),
  mockGetNodes: vi.fn(() => []),
  mockUseStore: vi.fn(() => 'stable-obstacles-key'),
}));

// Mock the calculateEdgePath utility
vi.mock('../../../utils/calculateEdgePath', () => ({
  calculateEdgePath: vi.fn(() => ({
    path: 'M 0,0 L 100,0 L 100,100 L 200,100',
    centerX: 100,
    centerY: 50,
  })),
}));

interface MockBaseEdgeProps {
  id: string;
  path: string;
  style?: React.CSSProperties;
  interactionWidth?: number;
  markerEnd?: string;
  markerStart?: string;
}

interface MockEdgeLabelRendererProps {
  children: React.ReactNode;
}

// Mock @xyflow/react
vi.mock('@xyflow/react', () => ({
  BaseEdge: ({id, path, style, interactionWidth, markerEnd, markerStart}: MockBaseEdgeProps) => (
    <path
      data-testid={`base-edge-${id}`}
      d={path}
      style={style}
      data-interaction-width={interactionWidth}
      data-marker-end={markerEnd}
      data-marker-start={markerStart}
    />
  ),
  EdgeLabelRenderer: ({children}: MockEdgeLabelRendererProps) => (
    <div data-testid="edge-label-renderer">{children}</div>
  ),
  useReactFlow: () => ({
    deleteElements: mockDeleteElements,
    getNodes: mockGetNodes,
  }),
  useStore: mockUseStore,
  getBezierPath: vi.fn(() => ['M 0,0 C 50,0 50,100 200,100', 100, 50, 0, 0]),
  getSmoothStepPath: vi.fn(() => ['M 0,0 L 100,0 L 100,100 L 200,100', 100, 50, 0, 0]),
  Position: {
    Left: 'left',
    Right: 'right',
    Top: 'top',
    Bottom: 'bottom',
  },
}));

interface MockBoxProps {
  children?: React.ReactNode;
  onClick?: () => void;
  onKeyDown?: (e: React.KeyboardEvent) => void;
  onMouseEnter?: () => void;
  onMouseLeave?: () => void;
  sx?: Record<string, unknown>;
  role?: string;
  tabIndex?: number;
  'aria-label'?: string;
}

// Mock @wso2/oxygen-ui
vi.mock('@wso2/oxygen-ui', () => ({
  Box: ({children, onClick, onKeyDown, onMouseEnter, onMouseLeave, role, ...props}: MockBoxProps) => (
    <div
      data-testid="box-component"
      onClick={onClick}
      onKeyDown={onKeyDown}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      role={role ?? (onClick ? 'button' : undefined)}
      {...props}
    >
      {children}
    </div>
  ),
}));

interface MockXIconProps {
  size?: number;
  style?: React.CSSProperties;
}

// Mock @wso2/oxygen-ui-icons-react
vi.mock('@wso2/oxygen-ui-icons-react', () => ({
  XIcon: ({size, style}: MockXIconProps) => <span data-testid="x-icon" data-size={size} style={style} />,
}));

describe('BaseEdge', () => {
  const defaultFlowConfigValue: FlowConfigContextProps = {
    ElementFactory: () => null,
    ResourceProperties: () => null,
    flowCompletionConfigs: {},
    setFlowCompletionConfigs: vi.fn(),
    isVerboseMode: false,
    setIsVerboseMode: vi.fn(),
    edgeStyle: EdgeStyleTypes.SmoothStep,
    setEdgeStyle: vi.fn(),
    flowNodeTypes: {},
    flowEdgeTypes: {},
    setFlowNodeTypes: vi.fn(),
    setFlowEdgeTypes: vi.fn(),
    flowNodes: [],
    setFlowNodes: vi.fn(),
  };

  const createWrapper = (overrides: Partial<FlowConfigContextProps> = {}) => {
    const flowConfigValue = {...defaultFlowConfigValue, ...overrides};

    function Wrapper({children}: {children: ReactNode}) {
      return <FlowConfigContext.Provider value={flowConfigValue}>{children}</FlowConfigContext.Provider>;
    }
    return Wrapper;
  };

  // Mock Position values that match our mocked @xyflow/react Position enum
  const defaultProps = {
    id: 'edge-1',
    source: 'node-1',
    target: 'node-2',
    sourceX: 100,
    sourceY: 100,
    targetX: 300,
    targetY: 100,
    sourcePosition: 'right',
    targetPosition: 'left',
  } as unknown as React.ComponentProps<typeof BaseEdge>;

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseStore.mockReturnValue('stable-obstacles-key');
  });

  describe('Rendering', () => {
    it('should render the edge path', () => {
      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('base-edge-edge-1')).toBeInTheDocument();
    });

    it('should render EdgeLabelRenderer', () => {
      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('edge-label-renderer')).toBeInTheDocument();
    });

    it('should render with calculated path', () => {
      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      const edge = screen.getByTestId('base-edge-edge-1');
      expect(edge).toHaveAttribute('d', 'M 0,0 L 100,0 L 100,100 L 200,100');
    });

    it('should render invisible hover detection path', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      const hoverPath = container.querySelector('path[stroke="transparent"]');
      expect(hoverPath).toBeInTheDocument();
      expect(hoverPath).toHaveAttribute('stroke-width', '20');
    });
  });

  describe('Label', () => {
    it('should render label when provided', () => {
      render(<BaseEdge {...defaultProps} label={<span data-testid="edge-label">Test Label</span>} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByTestId('edge-label')).toBeInTheDocument();
    });

    it('should not render label container when label is not provided', () => {
      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Label container should not have the label content
      expect(screen.queryByTestId('edge-label')).not.toBeInTheDocument();
    });
  });

  describe('Hover Behavior', () => {
    it('should show delete button on hover', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Initially, delete button should not be visible
      expect(screen.queryByTestId('x-icon')).not.toBeInTheDocument();

      // Hover over the edge group
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Delete button should now be visible
      expect(screen.getByTestId('x-icon')).toBeInTheDocument();
    });

    it('should hide delete button when mouse leaves', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      const group = container.querySelector('g');

      // Hover over
      fireEvent.mouseEnter(group!);
      expect(screen.getByTestId('x-icon')).toBeInTheDocument();

      // Mouse leave
      fireEvent.mouseLeave(group!);
      expect(screen.queryByTestId('x-icon')).not.toBeInTheDocument();
    });

    it('should increase stroke width on hover', () => {
      const {container} = render(<BaseEdge {...defaultProps} style={{stroke: 'blue'}} />, {
        wrapper: createWrapper(),
      });

      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      const edge = screen.getByTestId('base-edge-edge-1');
      expect(edge).toHaveStyle('stroke-width: 3');
    });
  });

  describe('Delete Functionality', () => {
    it('should show the delete button while the edge is selected, without hover', () => {
      render(<BaseEdge {...defaultProps} selected />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByRole('button', {name: 'Delete edge'})).toBeInTheDocument();
    });

    it('should not show the delete button when neither hovered nor selected', () => {
      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      expect(screen.queryByRole('button', {name: 'Delete edge'})).not.toBeInTheDocument();
    });

    it('should call deleteElements when delete button is clicked', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Click delete button
      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      fireEvent.click(deleteButton);

      expect(mockDeleteElements).toHaveBeenCalledWith({edges: [{id: 'edge-1'}]});
    });

    it('should stop event propagation on delete click', () => {
      const parentClickHandler = vi.fn();
      const {container} = render(
        // eslint-disable-next-line jsx-a11y/click-events-have-key-events
        <div onClick={parentClickHandler}>
          <BaseEdge {...defaultProps} />
        </div>,
        {
          wrapper: createWrapper(),
        },
      );

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Click delete button
      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      fireEvent.click(deleteButton);

      // Parent click handler should not be called
      expect(parentClickHandler).not.toHaveBeenCalled();
    });

    it('should handle keyboard Enter key to delete', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Press Enter on delete button
      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      fireEvent.keyDown(deleteButton, {key: 'Enter'});

      expect(mockDeleteElements).toHaveBeenCalledWith({edges: [{id: 'edge-1'}]});
    });

    it('should handle keyboard Space key to delete', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Press Space on delete button
      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      fireEvent.keyDown(deleteButton, {key: ' '});

      expect(mockDeleteElements).toHaveBeenCalledWith({edges: [{id: 'edge-1'}]});
    });

    it('should not delete on other key presses', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Press other key on delete button
      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      fireEvent.keyDown(deleteButton, {key: 'Tab'});

      expect(mockDeleteElements).not.toHaveBeenCalled();
    });

    it('should not show delete button when deletable is false', () => {
      const {container} = render(<BaseEdge {...defaultProps} deletable={false} />, {
        wrapper: createWrapper(),
      });

      // Hover over the edge
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Delete button should not be visible
      expect(screen.queryByRole('button', {name: 'Delete edge'})).not.toBeInTheDocument();
    });
  });

  describe('Edge Styles', () => {
    it('should apply custom style prop', () => {
      render(<BaseEdge {...defaultProps} style={{stroke: 'red', strokeDasharray: '5,5'}} />, {
        wrapper: createWrapper(),
      });

      const edge = screen.getByTestId('base-edge-edge-1');
      expect(edge).toHaveStyle('stroke: red');
    });

    it('should use edge style from context', () => {
      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Bezier}),
      });

      expect(screen.getByTestId('base-edge-edge-1')).toBeInTheDocument();
    });

    it('should pass interaction width to base edge', () => {
      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      const edge = screen.getByTestId('base-edge-edge-1');
      expect(edge).toHaveAttribute('data-interaction-width', '20');
    });
  });

  describe('Markers', () => {
    it('should pass markerEnd to base edge', () => {
      render(<BaseEdge {...defaultProps} markerEnd="url(#arrow)" />, {
        wrapper: createWrapper(),
      });

      const edge = screen.getByTestId('base-edge-edge-1');
      expect(edge).toHaveAttribute('data-marker-end', 'url(#arrow)');
    });

    it('should pass markerStart to base edge', () => {
      render(<BaseEdge {...defaultProps} markerStart="url(#arrow-start)" />, {
        wrapper: createWrapper(),
      });

      const edge = screen.getByTestId('base-edge-edge-1');
      expect(edge).toHaveAttribute('data-marker-start', 'url(#arrow-start)');
    });
  });

  describe('Accessibility', () => {
    it('should have accessible delete button with aria-label', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      expect(deleteButton).toHaveAttribute('aria-label', 'Delete edge');
    });

    it('should have tabIndex on delete button', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      expect(deleteButton).toHaveAttribute('tabIndex', '0');
    });
  });

  describe('Delete Button Styling', () => {
    it('should render X icon with correct size', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      const xIcon = screen.getByTestId('x-icon');
      expect(xIcon).toHaveAttribute('data-size', '16');
    });

    it('should render X icon with white color', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      const xIcon = screen.getByTestId('x-icon');
      // Check for white color in either format (named or RGB)
      expect(xIcon).toHaveStyle({color: 'rgb(255, 255, 255)'});
    });
  });

  describe('Error Handling', () => {
    it('should handle deleteElements rejection gracefully', () => {
      mockDeleteElements.mockRejectedValueOnce(new Error('Delete failed'));

      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover to show delete button
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Click delete button - should not throw
      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      expect(() => fireEvent.click(deleteButton)).not.toThrow();
    });
  });

  describe('Label Hover Effects', () => {
    it('should maintain hover state when mouse enters label', () => {
      const {container} = render(
        <BaseEdge {...defaultProps} label={<span data-testid="edge-label">Test Label</span>} />,
        {
          wrapper: createWrapper(),
        },
      );

      // First hover over the edge group
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Then hover over the label box
      const boxes = screen.getAllByTestId('box-component');
      // First box is the label container
      const labelBox = boxes[0];
      fireEvent.mouseEnter(labelBox);

      // Should still show delete button (hover state maintained)
      expect(screen.getByTestId('x-icon')).toBeInTheDocument();
    });

    it('should update hover state when mouse leaves label', () => {
      const {container} = render(
        <BaseEdge {...defaultProps} label={<span data-testid="edge-label">Test Label</span>} />,
        {
          wrapper: createWrapper(),
        },
      );

      // Hover over the edge group
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      const boxes = screen.getAllByTestId('box-component');
      const labelBox = boxes[0];

      // Enter and leave label
      fireEvent.mouseEnter(labelBox);
      fireEvent.mouseLeave(labelBox);

      // Leave the group too
      fireEvent.mouseLeave(group!);

      // Delete button should be hidden
      expect(screen.queryByTestId('x-icon')).not.toBeInTheDocument();
    });
  });

  describe('Delete Button Hover Effects', () => {
    it('should maintain hover state when mouse enters delete button', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // First hover over the edge group
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      // Then hover over the delete button itself
      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      fireEvent.mouseEnter(deleteButton);

      // Should still show delete button
      expect(screen.getByTestId('x-icon')).toBeInTheDocument();
    });

    it('should update hover state when mouse leaves delete button', () => {
      const {container} = render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      // Hover over the edge group
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});

      // Enter and leave delete button
      fireEvent.mouseEnter(deleteButton);
      fireEvent.mouseLeave(deleteButton);

      // Leave the group too
      fireEvent.mouseLeave(group!);

      // Delete button should be hidden
      expect(screen.queryByTestId('x-icon')).not.toBeInTheDocument();
    });

    it('should show delete button when hovering over label then delete button', () => {
      const {container} = render(<BaseEdge {...defaultProps} label={<span>Label</span>} />, {
        wrapper: createWrapper(),
      });

      // Hover over the edge group
      const group = container.querySelector('g');
      fireEvent.mouseEnter(group!);

      const boxes = screen.getAllByTestId('box-component');
      // Hover over label first
      fireEvent.mouseEnter(boxes[0]);

      // Then hover over delete button
      const deleteButton = screen.getByRole('button', {name: 'Delete edge'});
      fireEvent.mouseEnter(deleteButton);

      // Delete button should still be visible
      expect(screen.getByTestId('x-icon')).toBeInTheDocument();
    });
  });

  describe('Dragging Performance', () => {
    it('should use smart routing when no node is being dragged', async () => {
      const {calculateEdgePath} = await import('../../../utils/calculateEdgePath');

      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      expect(calculateEdgePath).toHaveBeenCalled();
    });

    it('should skip smart routing and use the built-in step path while a node is dragged', async () => {
      const {calculateEdgePath} = await import('../../../utils/calculateEdgePath');
      const {getSmoothStepPath} = await import('@xyflow/react');
      mockUseStore.mockReturnValue('dragging');

      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper(),
      });

      expect(calculateEdgePath).not.toHaveBeenCalled();
      expect(getSmoothStepPath).toHaveBeenCalled();
    });

    it('should use the built-in bezier path while dragging with the bezier edge style', async () => {
      const {calculateEdgePath} = await import('../../../utils/calculateEdgePath');
      const {getBezierPath} = await import('@xyflow/react');
      mockUseStore.mockReturnValue('dragging');

      render(<BaseEdge {...defaultProps} />, {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Bezier}),
      });

      expect(calculateEdgePath).not.toHaveBeenCalled();
      expect(getBezierPath).toHaveBeenCalled();
    });
  });
});
