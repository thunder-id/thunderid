// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import SignUpPage from '../SignUpPage';

// Mock the SignUp component
vi.mock('../../components/SignUp/SignUp', () => ({
  default: () => <div data-testid="signup-component">SignUp Component</div>,
}));

describe('SignUpPage', () => {
  it('renders without crashing', () => {
    const {container} = render(<SignUpPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders SignUp component', () => {
    render(<SignUpPage />);
    expect(screen.getByTestId('signup-component')).toBeInTheDocument();
  });

  it('sets the document title', async () => {
    render(<SignUpPage />);
    await waitFor(() => expect(document.title).toBe('Sign up | ThunderID'));
  });
});
