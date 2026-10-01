// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import StepList from '../StepList';

describe('StepList', () => {
  it('renders each step', () => {
    render(<StepList steps={['First step', 'Second step']} />);
    expect(screen.getByText('First step')).toBeInTheDocument();
    expect(screen.getByText('Second step')).toBeInTheDocument();
  });

  it('numbers steps starting from 1 by default', () => {
    render(<StepList steps={['Step A', 'Step B']} />);
    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('numbers steps from a custom startFrom value', () => {
    render(<StepList steps={['Step A', 'Step B']} startFrom={3} />);
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.queryByText('1')).not.toBeInTheDocument();
  });

  it('renders JSX node steps', () => {
    render(
      <StepList
        steps={[
          <span key="a" data-testid="jsx-step">
            JSX content
          </span>,
        ]}
      />,
    );
    expect(screen.getByTestId('jsx-step')).toBeInTheDocument();
  });
});
