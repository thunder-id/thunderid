// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {beforeEach, describe, it, expect, vi} from 'vitest';

const {mockUseGetAccountLifecycleFlows, mockNavigate} = vi.hoisted(() => ({
  mockUseGetAccountLifecycleFlows: vi.fn(),
  mockNavigate: vi.fn(),
}));

vi.mock('../../../api/useGetAccountLifecycleFlows', () => ({
  default: (): unknown => mockUseGetAccountLifecycleFlows() as unknown,
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
}));

const {default: AccountSuspensionSection} = await import('../AccountSuspensionSection');

const flow = (id: string, name: string, isReadOnly = false) => ({
  id,
  handle: `${id}-handle`,
  name,
  flowType: 'ADMINISTRATION',
  updatedAt: '2026-10-01T10:30:00Z',
  isReadOnly,
});

const loaded = (flows: ReturnType<typeof flow>[], unconfigured: string[] = []) => ({
  data: {flows, unconfigured},
  isLoading: false,
  error: null,
  refetch: vi.fn(),
});

const row = (name: string): HTMLElement => screen.getByRole('gridcell', {name}).closest<HTMLElement>('[role="row"]')!;

describe('AccountSuspensionSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says what the flows do and what suspension does', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([]));
    renderWithProviders(<AccountSuspensionSection />);

    expect(screen.getByText('Lock and suspension flows')).toBeVisible();
    expect(screen.getByText(/ends existing sessions and tokens/)).toBeVisible();
    expect(screen.getByText(/suspend, unsuspend and unlock users, agents and applications/)).toBeVisible();
  });

  it('lists each flow with the flow list columns', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([flow('f-1', 'User suspend flow')]));
    renderWithProviders(<AccountSuspensionSection />);

    ['Name', 'Type', 'Last Updated', 'Actions'].forEach((name) => {
      expect(screen.getByRole('columnheader', {name})).toBeInTheDocument();
    });
    const cells = within(row('User suspend flow'));
    expect(cells.getByText('ADMINISTRATION')).toBeVisible();
    const updated = new Date('2026-10-01T10:30:00Z').toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
    expect(cells.getByText(updated)).toBeVisible();
  });

  // The flows list deletes from here too; this card only opens flows.
  it('opens an editable flow with Edit and a read-only one with View, and never offers delete', async () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(
      loaded([flow('f-edit', 'Editable flow'), flow('f-view', 'Declarative flow', true)]),
    );
    renderWithProviders(<AccountSuspensionSection />);

    const editable = within(row('Editable flow'));
    expect(editable.queryByRole('button', {name: 'View'})).not.toBeInTheDocument();
    await userEvent.click(editable.getByRole('button', {name: 'Edit'}));
    expect(mockNavigate).toHaveBeenLastCalledWith('/flows/f-edit');

    const declarative = within(row('Declarative flow'));
    expect(declarative.queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();
    await userEvent.click(declarative.getByRole('button', {name: 'View'}));
    expect(mockNavigate).toHaveBeenLastCalledWith('/flows/f-view');

    expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    expect(mockNavigate).toHaveBeenCalledTimes(2);
  });

  it('opens the flow editor when a row is clicked', async () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([flow('f-1', 'User suspend flow')]));
    renderWithProviders(<AccountSuspensionSection />);

    await userEvent.click(screen.getByRole('gridcell', {name: 'User suspend flow'}));
    expect(mockNavigate).toHaveBeenCalledWith('/flows/f-1');
  });

  it('says which operations have no usable flow and links to the flows list', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(
      loaded([flow('f-1', 'User suspend flow')], ['userUnlockFlow', 'applicationUnsuspendFlow']),
    );
    renderWithProviders(<AccountSuspensionSection />);

    const missing = within(screen.getByTestId('account-suspension-unconfigured'));
    expect(missing.getByText('Unlock users: Not configured.')).toBeVisible();
    expect(missing.getByText('Unsuspend applications: Not configured.')).toBeVisible();
    expect(missing.getByRole('link', {name: 'Open flows'})).toHaveAttribute('href', '/flows');
  });

  it('shows no table when no operation has a flow, and no list when every one has', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([], ['userSuspendFlow']));
    const {unmount} = renderWithProviders(<AccountSuspensionSection />);

    expect(screen.queryByRole('grid')).not.toBeInTheDocument();
    expect(screen.getByText('Suspend users: Not configured.')).toBeVisible();
    unmount();

    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([flow('f-1', 'User suspend flow')]));
    renderWithProviders(<AccountSuspensionSection />);

    expect(screen.getByRole('grid')).toBeInTheDocument();
    expect(screen.queryByTestId('account-suspension-unconfigured')).not.toBeInTheDocument();
  });

  it('shows the table loading while the flows resolve', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue({data: undefined, isLoading: true, error: null, refetch: vi.fn()});
    renderWithProviders(<AccountSuspensionSection />);

    expect(document.querySelector('.MuiDataGrid-skeletonLoadingOverlay')).toBeInTheDocument();
    expect(screen.queryByTestId('account-suspension-unconfigured')).not.toBeInTheDocument();
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
    expect(screen.queryByRole('grid')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Refresh'}));
    expect(refetch).toHaveBeenCalled();
  });
});
