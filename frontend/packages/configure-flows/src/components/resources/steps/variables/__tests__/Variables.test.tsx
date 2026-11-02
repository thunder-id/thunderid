// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import Variables from '../Variables';

// Mock @xyflow/react
vi.mock('@xyflow/react', () => ({
  Handle: ({type, position, id}: {type: string; position: string; id: string | undefined}) => (
    <div data-testid={`handle-${type}`} data-position={position} data-handle-id={id} />
  ),
  Position: {
    Left: 'left',
    Right: 'right',
  },
}));

describe('Variables', () => {
  it('should render a blank box', () => {
    render(<Variables id="vars" />);

    const node = screen.getByTestId('variables-node');
    expect(node).toBeInTheDocument();
    expect(node).toHaveTextContent('');
  });

  it('should take paths in on the left', () => {
    render(<Variables id="vars" />);

    expect(screen.getByTestId('handle-target')).toHaveAttribute('data-position', 'left');
  });

  it('should send the path on from the right, with a handle named after the node', () => {
    render(<Variables id="vars" />);

    const handle = screen.getByTestId('handle-source');
    expect(handle).toHaveAttribute('data-position', 'right');
    expect(handle).toHaveAttribute('data-handle-id', 'vars_NEXT');
  });
});
