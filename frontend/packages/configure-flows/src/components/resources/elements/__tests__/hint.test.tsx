// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import {Hint} from '../hint';

// Mock the PlaceholderComponent
vi.mock('../adapters/PlaceholderComponent', () => ({
  default: ({children, value}: {children: React.ReactNode; value: string}) => (
    <div data-testid="placeholder-component" data-value={value}>
      {children}
    </div>
  ),
}));

// Mock the icons - use importOriginal to preserve all exports while mocking specific ones
vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    InfoIcon: () => <svg data-testid="info-icon" />,
  };
});

describe('Hint', () => {
  it('should render hint text', () => {
    render(<Hint hint="This is a helpful hint" />);

    expect(screen.getByText('This is a helpful hint')).toBeInTheDocument();
  });

  it('should render the InfoIcon', () => {
    render(<Hint hint="Test hint" />);

    expect(screen.getByTestId('info-icon')).toBeInTheDocument();
  });

  it('should render the hint value as text', () => {
    render(<Hint hint="Placeholder hint" />);

    expect(screen.getByText('Placeholder hint')).toBeInTheDocument();
  });

  it('should render with empty hint', () => {
    render(<Hint hint="" />);

    expect(screen.getByTestId('info-icon')).toBeInTheDocument();
  });

  it('should render hint with special characters', () => {
    const specialCharsHint = 'Use special characters: &, "test"';
    render(<Hint hint={specialCharsHint} />);

    expect(screen.getByText(specialCharsHint)).toBeInTheDocument();
  });

  it('should render hint with long text', () => {
    const longHint =
      'This is a very long hint text that provides detailed information about the input field and its expected format for the user to understand.';
    render(<Hint hint={longHint} />);

    expect(screen.getByText(longHint)).toBeInTheDocument();
  });
});
