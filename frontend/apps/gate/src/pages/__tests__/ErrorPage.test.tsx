// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import ErrorPage from '../ErrorPage';

// Mock the Error component
vi.mock('../../components/Error/Error', () => ({
  default: () => <div data-testid="error-component">Error Component</div>,
}));

describe('ErrorPage', () => {
  it('renders without crashing', () => {
    const {container} = render(<ErrorPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders Error component', () => {
    render(<ErrorPage />);
    expect(screen.getByTestId('error-component')).toBeInTheDocument();
  });

  it('sets the document title', async () => {
    render(<ErrorPage />);
    await waitFor(() => expect(document.title).toBe('Error | ThunderID'));
  });
});
