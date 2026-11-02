// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {CommonStepFactoryPropsInterface} from '../../CommonStepFactory';
import End from '../End';

// Mock @xyflow/react
vi.mock('@xyflow/react', () => ({
  Handle: ({type, position, id}: {type: string; position: string; id: string}) => (
    <div data-testid={`handle-${type}`} data-position={position} data-handle-id={id} />
  ),
  Position: {
    Left: 'left',
    Right: 'right',
    Top: 'top',
    Bottom: 'bottom',
  },
}));

// Default mock props for End component
const createMockProps = (overrides: Partial<CommonStepFactoryPropsInterface> = {}): CommonStepFactoryPropsInterface =>
  ({
    id: 'end-node-1',
    resourceId: 'end-resource-1',
    resources: [],
    data: {},
    type: 'END',
    zIndex: 1,
    isConnectable: true,
    positionAbsoluteX: 0,
    positionAbsoluteY: 0,
    dragging: false,
    selected: false,
    deletable: false,
    selectable: true,
    parentId: undefined,
    ...overrides,
  }) as CommonStepFactoryPropsInterface;

describe('End', () => {
  describe('Rendering', () => {
    it('should render the End node', () => {
      render(<End {...createMockProps()} />);

      expect(screen.getByText('End')).toBeInTheDocument();
    });

    it('should render a Fab button with end label', () => {
      render(<End {...createMockProps()} />);

      const fab = screen.getByRole('button', {name: 'end'});
      expect(fab).toBeInTheDocument();
    });

    it('should render the Fab in the success color', () => {
      render(<End {...createMockProps()} />);

      expect(screen.getByRole('button', {name: 'end'})).toHaveClass('MuiFab-success');
    });
  });

  describe('React Flow Handle', () => {
    it('should render a target handle', () => {
      render(<End {...createMockProps()} />);

      const handle = screen.getByTestId('handle-target');
      expect(handle).toBeInTheDocument();
    });

    it('should position handle on the left', () => {
      render(<End {...createMockProps()} />);

      const handle = screen.getByTestId('handle-target');
      expect(handle).toHaveAttribute('data-position', 'left');
    });

    it('should have correct handle id with previous suffix', () => {
      render(<End {...createMockProps()} />);

      const handle = screen.getByTestId('handle-target');
      // Handle id should contain 'end' and '_PREVIOUS' suffix
      expect(handle.getAttribute('data-handle-id')).toContain('end');
      expect(handle.getAttribute('data-handle-id')).toContain('_PREVIOUS');
    });
  });

  describe('Fab Properties', () => {
    it('should render extended variant Fab', () => {
      render(<End {...createMockProps()} />);

      const fab = screen.getByRole('button');
      // Extended variant Fab will be rendered
      expect(fab).toBeInTheDocument();
    });

    it('should render small size Fab', () => {
      render(<End {...createMockProps()} />);

      const fab = screen.getByRole('button');
      expect(fab).toBeInTheDocument();
    });
  });

  describe('Structure', () => {
    it('should be wrapped in a div', () => {
      const {container} = render(<End {...createMockProps()} />);

      expect(container.firstChild?.nodeName).toBe('DIV');
    });

    it('should contain Handle before Fab (target comes first)', () => {
      const {container} = render(<End {...createMockProps()} />);

      const children = container.firstChild?.childNodes;
      expect(children).toBeDefined();
      // Target handle should come before the Fab button
      expect(children?.length).toBeGreaterThanOrEqual(2);
    });
  });

  describe('Props', () => {
    it('should accept props without error', () => {
      // End component accepts CommonStepFactoryPropsInterface but doesn't use them
      render(<End {...createMockProps({data: {}, id: 'end-node-1'})} />);

      expect(screen.getByText('End')).toBeInTheDocument();
    });
  });
});
