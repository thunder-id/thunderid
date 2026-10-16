// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {beforeEach, describe, it, expect, vi} from 'vitest';

const {mockUseGetAccountLifecycleFlows} = vi.hoisted(() => ({
  mockUseGetAccountLifecycleFlows: vi.fn(),
}));

vi.mock('../../../api/useGetAccountLifecycleFlows', () => ({
  default: (): unknown => mockUseGetAccountLifecycleFlows() as unknown,
}));

const {default: AccountSuspensionSection} = await import('../AccountSuspensionSection');

const loaded = (data: Record<string, string>) => ({data, isLoading: false, error: null, refetch: vi.fn()});

const group = (name: string): HTMLElement => screen.getByRole('region', {name});

describe('AccountSuspensionSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says what suspension does and where it is started', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded({}));
    renderWithProviders(<AccountSuspensionSection />);

    expect(screen.getByText('Account suspension')).toBeVisible();
    expect(screen.getByText(/ends existing sessions and tokens/)).toBeVisible();
    expect(screen.getByText(/from each user, agent or application page/)).toBeVisible();
  });

  it('links each configured flow to the flow editor', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(
      loaded({userSuspendFlow: 'f-us', agentUnlockFlow: 'f-al', applicationUnsuspendFlow: 'f-ap'}),
    );
    renderWithProviders(<AccountSuspensionSection />);

    expect(within(group('Users')).getByRole('link', {name: 'Suspend flow'})).toHaveAttribute('href', '/flows/f-us');
    expect(within(group('Agents')).getByRole('link', {name: 'Unlock flow'})).toHaveAttribute('href', '/flows/f-al');
    expect(within(group('Applications')).getByRole('link', {name: 'Unsuspend flow'})).toHaveAttribute(
      'href',
      '/flows/f-ap',
    );
  });

  // An application is never locked, so it has no unlock flow to show.
  it('lists suspend, unsuspend and unlock for users and agents, and no unlock for applications', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded({}));
    renderWithProviders(<AccountSuspensionSection />);

    ['Users', 'Agents'].forEach((name) => {
      expect(within(group(name)).getByText(/Suspend flow: Not configured/)).toBeVisible();
      expect(within(group(name)).getByText(/Unsuspend flow: Not configured/)).toBeVisible();
      expect(within(group(name)).getByText(/Unlock flow: Not configured/)).toBeVisible();
    });
    expect(within(group('Applications')).queryByText(/Unlock flow/)).not.toBeInTheDocument();
  });

  it('says a purpose with no usable flow is not configured and links to the flows list', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded({userSuspendFlow: 'f-us'}));
    renderWithProviders(<AccountSuspensionSection />);

    const users = group('Users');
    expect(within(users).queryByRole('link', {name: 'Unlock flow'})).not.toBeInTheDocument();
    expect(within(users).getByText(/Unlock flow: Not configured/)).toBeVisible();
    within(users)
      .getAllByRole('link', {name: 'Open flows'})
      .forEach((link) => expect(link).toHaveAttribute('href', '/flows'));
  });

  it('shows a progress indicator while the flows resolve', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue({data: undefined, isLoading: true, error: null, refetch: vi.fn()});
    renderWithProviders(<AccountSuspensionSection />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
    expect(screen.queryByRole('region', {name: 'Users'})).not.toBeInTheDocument();
  });

  it('reports a failed read in place and retries it', async () => {
    const refetch = vi.fn().mockResolvedValue(undefined);
    mockUseGetAccountLifecycleFlows.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('boom'),
      refetch,
    });
    renderWithProviders(<AccountSuspensionSection />);

    expect(screen.getByText('Failed to load the account lifecycle flows.')).toBeVisible();
    expect(screen.queryByText('boom')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Refresh'}));
    expect(refetch).toHaveBeenCalled();
  });
});
