// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {Element as FlowElement} from '../../../../../models/elements';
import CustomAdapter from '../CustomAdapter';

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    PuzzleIcon: ({size = 24}: {size?: number} = {}) => <svg data-testid="puzzle-icon" data-size={size} />,
  };
});

describe('CustomAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> = {}): FlowElement =>
    ({
      id: 'custom-1',
      type: 'CUSTOM',
      category: 'MISCELLANEOUS',
      ...overrides,
    }) as FlowElement;

  it('should render the puzzle icon', () => {
    render(<CustomAdapter resource={createMockElement()} />);

    expect(screen.getByTestId('puzzle-icon')).toBeInTheDocument();
  });

  it('should render the "Custom" label', () => {
    render(<CustomAdapter resource={createMockElement()} />);

    expect(screen.getByText('Custom')).toBeInTheDocument();
  });

  it('should display the resource identifier', () => {
    render(<CustomAdapter resource={createMockElement({id: 'my-custom-element'})} />);

    expect(screen.getByText(/my-custom-element/)).toBeInTheDocument();
  });

  it('should pass size 20 to the puzzle icon', () => {
    render(<CustomAdapter resource={createMockElement()} />);

    expect(screen.getByTestId('puzzle-icon')).toHaveAttribute('data-size', '20');
  });
});
