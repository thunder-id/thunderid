// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import SignOutPage from '../SignOutPage';

// Mock the SignOut component
vi.mock('../../components/SignOut/SignOut', () => ({
  default: () => <div data-testid="signout-component">SignOut Component</div>,
}));

describe('SignOutPage', () => {
  it('renders without crashing', () => {
    const {container} = render(<SignOutPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders SignOut component', () => {
    render(<SignOutPage />);
    expect(screen.getByTestId('signout-component')).toBeInTheDocument();
  });

  it('sets the document title', async () => {
    render(<SignOutPage />);
    await waitFor(() => expect(document.title).toBe('Sign out | ThunderID'));
  });
});
