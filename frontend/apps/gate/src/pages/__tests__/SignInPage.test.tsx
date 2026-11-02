// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import SignInPage from '../SignInPage';

// Mock the SignIn component
vi.mock('../../components/SignIn/SignIn', () => ({
  default: () => <div data-testid="signin-component">SignIn Component</div>,
}));

describe('SignInPage', () => {
  it('renders without crashing', () => {
    const {container} = render(<SignInPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders SignIn component', () => {
    render(<SignInPage />);
    expect(screen.getByTestId('signin-component')).toBeInTheDocument();
  });

  it('sets the document title', async () => {
    render(<SignInPage />);
    await waitFor(() => expect(document.title).toBe('Sign in | ThunderID'));
  });
});
