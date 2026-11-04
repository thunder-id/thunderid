// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {AdministrationModes, AdministrationProvider, type AdministrationConfig} from '@thunderid/contexts';
import {renderWithProviders} from '@thunderid/test-utils';
import {beforeEach, describe, it, expect, vi} from 'vitest';

const {mockUseGetAccountLifecycleFlows, mockNavigate} = vi.hoisted(() => ({
  mockUseGetAccountLifecycleFlows: vi.fn(),
  mockNavigate: vi.fn(),
}));

vi.mock('../../../api/useGetAccountLifecycleFlows', () => ({
  default: (...args: unknown[]): unknown => mockUseGetAccountLifecycleFlows(...args) as unknown,
}));

// A marker label, so the test can see the grid use the Console's translated locale text.
vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: () => ({columnHeaderSortIconLabel: 'Sort this column'}),
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
    expect(screen.getByText(/invalidates existing sessions and tokens/)).toBeVisible();
    expect(
      screen.getByText(/suspend and unsuspend users, agents, and applications, and unlock users and agents\./),
    ).toBeVisible();
  });

  describe('administration mode', () => {
    const renderUnder = (administration: AdministrationConfig) =>
      renderWithProviders(
        <AdministrationProvider administration={administration}>
          <AccountSuspensionSection />
        </AdministrationProvider>,
      );

    it('lists every lifecycle operation when each runs as a flow', () => {
      mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([]));
      renderWithProviders(<AccountSuspensionSection />);

      expect(mockUseGetAccountLifecycleFlows).toHaveBeenLastCalledWith([
        'agentSuspendFlow',
        'agentUnlockFlow',
        'agentUnsuspendFlow',
        'applicationSuspendFlow',
        'applicationUnsuspendFlow',
        'userSuspendFlow',
        'userUnlockFlow',
        'userUnsuspendFlow',
      ]);
    });

    // The record pages offer only these, so a flow for any other operation could not be used.
    it('lists only the operations that run as flows', () => {
      mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([flow('f-1', 'User suspend flow')]));
      renderUnder({
        features: {agents: {mode: AdministrationModes.FLOW}, users: {operations: {suspend: AdministrationModes.FLOW}}},
        mode: AdministrationModes.NATIVE,
      });

      expect(mockUseGetAccountLifecycleFlows).toHaveBeenLastCalledWith([
        'agentSuspendFlow',
        'agentUnlockFlow',
        'agentUnsuspendFlow',
        'userSuspendFlow',
      ]);
      expect(screen.getByRole('grid')).toBeInTheDocument();
    });

    it('says in one sentence that there are no flows when no operation runs as one', () => {
      mockUseGetAccountLifecycleFlows.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });
      renderUnder({mode: AdministrationModes.NATIVE});

      expect(mockUseGetAccountLifecycleFlows).toHaveBeenLastCalledWith([]);
      expect(
        screen.getByText(
          'Suspend, unsuspend, and unlock do not run through flows in this console, so there are no flows to manage.',
        ),
      ).toBeVisible();
      expect(screen.queryByRole('grid')).not.toBeInTheDocument();
      expect(screen.queryByTestId('account-suspension-unconfigured')).not.toBeInTheDocument();
    });
  });

  it('lists each flow with the flow list columns', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([flow('f-1', 'User suspend flow')]));
    renderWithProviders(<AccountSuspensionSection />);

    ['Name', 'Type', 'Last updated', 'Actions'].forEach((name) => {
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

  it('labels the grid with the Console locale text', () => {
    mockUseGetAccountLifecycleFlows.mockReturnValue(loaded([flow('f-1', 'User suspend flow')]));
    renderWithProviders(<AccountSuspensionSection />);

    expect(screen.getAllByLabelText('Sort this column').length).toBeGreaterThan(0);
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
