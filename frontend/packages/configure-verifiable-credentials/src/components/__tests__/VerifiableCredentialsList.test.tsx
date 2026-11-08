// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider, ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {VerifiableCredentialSummary} from '../../models/vc';
import VerifiableCredentialsList from '../VerifiableCredentialsList';

vi.mock('../../api/useGetVerifiableCredentials');
vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: vi.fn(),
}));

vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    OxygenUIThemeProvider: ({children}: {children: React.ReactNode}) => children,
    ListingTable: {
      Provider: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
      Container: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
      RowActions: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
      DataGrid: ({
        rows,
        columns,
        getRowId,
      }: {
        rows: Record<string, unknown>[];
        columns: {field: string; renderCell?: (params: {row: Record<string, unknown>}) => React.ReactElement}[];
        getRowId: (row: Record<string, unknown>) => string;
      }) => (
        <div role="grid">
          {rows.map((row: Record<string, unknown>) => (
            <div key={getRowId(row)} role="row">
              {columns.map((col) => (
                <div key={col.field}>{col.renderCell?.({row})}</div>
              ))}
            </div>
          ))}
        </div>
      ),
    },
  };
});

vi.mock('../CredentialOfferDialog', () => ({
  default: ({open, handle, onClose}: {open: boolean; handle: string | null; onClose: () => void}) => (
    <div data-testid="credential-offer-dialog" data-open={open} data-handle={handle ?? ''}>
      <button type="button" onClick={onClose}>
        Close
      </button>
    </div>
  ),
}));

vi.mock('../VerifiableCredentialDeleteDialog', () => ({
  default: () => null,
}));

const {default: useGetVerifiableCredentials} = await import('../../api/useGetVerifiableCredentials');

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('VerifiableCredentialsList', () => {
  const configurations: VerifiableCredentialSummary[] = [
    {id: 'vc-1', handle: 'lounge-pass', ouId: 'ou-1', ouHandle: 'default', name: 'Lounge Pass', vct: 'v', format: 'f'},
  ];

  beforeEach(() => {
    vi.mocked(useGetVerifiableCredentials).mockReturnValue({
      data: configurations,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useGetVerifiableCredentials>);
  });

  it("lists only the selected project's credential configurations", () => {
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <VerifiableCredentialsList />
      </ProjectScopeProvider>,
    );

    expect(useGetVerifiableCredentials).toHaveBeenCalledWith('project-1');
  });

  it('hides the edit and delete row actions when the admin may only view', () => {
    render(
      <CapabilitiesProvider capabilities={viewOnly}>
        <VerifiableCredentialsList />
      </CapabilitiesProvider>,
    );

    expect(screen.queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
  });

  it('hides the offer action and does not mount the credential offer dialog by default', () => {
    render(<VerifiableCredentialsList />);

    expect(screen.getByRole('button', {name: 'Edit'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Generate offer'})).not.toBeInTheDocument();
    expect(screen.queryByTestId('credential-offer-dialog')).not.toBeInTheDocument();
  });

  it('shows the offer action and opens and closes the credential offer dialog for the row when showWalletQr is set', async () => {
    const user = userEvent.setup();
    render(<VerifiableCredentialsList showWalletQr />);

    const dialog = screen.getByTestId('credential-offer-dialog');
    expect(dialog).toHaveAttribute('data-open', 'false');

    await user.click(screen.getByRole('button', {name: 'Generate offer'}));

    expect(dialog).toHaveAttribute('data-open', 'true');
    expect(dialog).toHaveAttribute('data-handle', 'lounge-pass');

    await user.click(screen.getByRole('button', {name: 'Close'}));

    expect(dialog).toHaveAttribute('data-open', 'false');
    expect(dialog).toHaveAttribute('data-handle', '');
  });
});
