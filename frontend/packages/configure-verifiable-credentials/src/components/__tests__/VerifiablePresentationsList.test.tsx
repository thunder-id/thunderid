// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {VerifiablePresentationSummary} from '../../models/vp';
import VerifiablePresentationsList from '../VerifiablePresentationsList';

vi.mock('../../api/useGetVerifiablePresentations');
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

vi.mock('../VerificationDialog', () => ({
  default: ({open, handle, onClose}: {open: boolean; handle: string | null; onClose: () => void}) => (
    <div data-testid="verification-dialog" data-open={open} data-handle={handle ?? ''}>
      <button type="button" onClick={onClose}>
        Close
      </button>
    </div>
  ),
}));

vi.mock('../VerifiablePresentationDeleteDialog', () => ({
  default: () => null,
}));

const {default: useGetVerifiablePresentations} = await import('../../api/useGetVerifiablePresentations');

describe('VerifiablePresentationsList', () => {
  const definitions: VerifiablePresentationSummary[] = [
    {id: 'vp-1', handle: 'skyline-lounge', ouId: 'ou-1', ouHandle: 'default', name: 'Skyline', vct: 'v', format: 'f'},
  ];

  beforeEach(() => {
    vi.mocked(useGetVerifiablePresentations).mockReturnValue({
      data: definitions,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useGetVerifiablePresentations>);
  });

  it('hides the verify action and does not mount the verification dialog by default', () => {
    render(<VerifiablePresentationsList />);

    expect(screen.getByRole('button', {name: 'Edit'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Verify'})).not.toBeInTheDocument();
    expect(screen.queryByTestId('verification-dialog')).not.toBeInTheDocument();
  });

  it('shows the verify action and opens and closes the verification dialog for the row when showWalletQr is set', async () => {
    const user = userEvent.setup();
    render(<VerifiablePresentationsList showWalletQr />);

    const dialog = screen.getByTestId('verification-dialog');
    expect(dialog).toHaveAttribute('data-open', 'false');

    await user.click(screen.getByRole('button', {name: 'Verify'}));

    expect(dialog).toHaveAttribute('data-open', 'true');
    expect(dialog).toHaveAttribute('data-handle', 'skyline-lounge');

    await user.click(screen.getByRole('button', {name: 'Close'}));

    expect(dialog).toHaveAttribute('data-open', 'false');
    expect(dialog).toHaveAttribute('data-handle', '');
  });
});
