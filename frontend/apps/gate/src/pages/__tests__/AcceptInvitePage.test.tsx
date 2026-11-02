// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import AcceptInvitePage from '../AcceptInvitePage';

// Mock the AcceptInviteBox component
vi.mock('../../components/AcceptInvite/AcceptInviteBox', () => ({
  default: () => <div data-testid="accept-invite-box">AcceptInviteBox Component</div>,
}));

describe('AcceptInvitePage', () => {
  it('renders without crashing', () => {
    const {container} = render(<AcceptInvitePage />);
    expect(container).toBeInTheDocument();
  });

  it('renders AcceptInviteBox component', () => {
    render(<AcceptInvitePage />);
    expect(screen.getByTestId('accept-invite-box')).toBeInTheDocument();
  });

  it('renders main element', () => {
    render(<AcceptInvitePage />);
    const main = screen.getByRole('main');
    expect(main).toBeInTheDocument();
  });

  it('sets the document title', async () => {
    render(<AcceptInvitePage />);
    await waitFor(() => expect(document.title).toBe('Accept invite | ThunderID'));
  });
});
