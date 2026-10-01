// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {OAuthAgentConfig} from '../../../../models/agent';
import ClientIdSection from '../ClientIdSection';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

const mockCopy = vi.fn().mockResolvedValue(undefined);

vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useCopyToClipboard: vi.fn(() => ({copied: false, copy: mockCopy})),
}));

describe('ClientIdSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCopy.mockResolvedValue(undefined);
  });

  it('renders the Client ID field when configured', () => {
    const oauth2Config = {clientId: 'client-123'} as OAuthAgentConfig;
    render(<ClientIdSection oauth2Config={oauth2Config} />);

    expect(screen.getByDisplayValue('client-123')).toBeInTheDocument();
  });

  it('renders nothing when there is no client ID', () => {
    const {container} = render(<ClientIdSection oauth2Config={{} as OAuthAgentConfig} />);

    expect(container.firstChild).toBeNull();
  });

  it('copies the client ID when the copy button is clicked', async () => {
    const user = userEvent.setup();
    const oauth2Config = {clientId: 'client-123'} as OAuthAgentConfig;
    render(<ClientIdSection oauth2Config={oauth2Config} />);

    await user.click(screen.getByRole('button', {name: /copy/i}));

    expect(mockCopy).toHaveBeenCalledWith('client-123');
  });
});
