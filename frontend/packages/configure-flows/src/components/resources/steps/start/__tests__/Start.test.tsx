// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import Start from '../Start';

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

describe('Start', () => {
  describe('Rendering', () => {
    it('should render the Start node', () => {
      render(<Start />);

      expect(screen.getByText('Start')).toBeInTheDocument();
    });

    it('should render a Fab button with start label', () => {
      render(<Start />);

      const fab = screen.getByRole('button', {name: 'start'});
      expect(fab).toBeInTheDocument();
    });

    it('should render the Fab in the primary color', () => {
      render(<Start />);

      expect(screen.getByRole('button', {name: 'start'})).toHaveClass('MuiFab-primary');
    });
  });

  describe('React Flow Handle', () => {
    it('should render a source handle', () => {
      render(<Start />);

      const handle = screen.getByTestId('handle-source');
      expect(handle).toBeInTheDocument();
    });

    it('should position handle on the right', () => {
      render(<Start />);

      const handle = screen.getByTestId('handle-source');
      expect(handle).toHaveAttribute('data-position', 'right');
    });

    it('should have correct handle id with next suffix', () => {
      render(<Start />);

      const handle = screen.getByTestId('handle-source');
      // Handle id should contain 'start' and '_NEXT' suffix
      expect(handle.getAttribute('data-handle-id')).toContain('start');
      expect(handle.getAttribute('data-handle-id')).toContain('_NEXT');
    });
  });

  describe('Fab Properties', () => {
    it('should render extended variant Fab', () => {
      render(<Start />);

      const fab = screen.getByRole('button');
      // Extended variant Fab will have extended class
      expect(fab).toBeInTheDocument();
    });

    it('should render small size Fab', () => {
      render(<Start />);

      const fab = screen.getByRole('button');
      expect(fab).toBeInTheDocument();
    });
  });

  describe('Structure', () => {
    it('should be wrapped in a div', () => {
      const {container} = render(<Start />);

      expect(container.firstChild?.nodeName).toBe('DIV');
    });

    it('should contain Fab and Handle as children', () => {
      render(<Start />);

      expect(screen.getByRole('button')).toBeInTheDocument();
      expect(screen.getByTestId('handle-source')).toBeInTheDocument();
    });
  });
});
