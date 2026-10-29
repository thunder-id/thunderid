// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {VerifiableCredentialSummary} from '../../models/vc';
import VerifiableCredentialsList from '../VerifiableCredentialsList';

vi.mock('../../api/useGetVerifiableCredentials', () => ({
  default: vi.fn(),
}));
vi.mock('../CredentialOfferDialog', () => ({
  default: () => null,
}));
vi.mock('../VerifiableCredentialDeleteDialog', () => ({
  default: () => null,
}));
vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: () => ({}),
}));

interface MockColumn {
  field: string;
  renderCell?: (params: {row: VerifiableCredentialSummary}) => React.ReactNode;
}

vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    ListingTable: {
      ...actual.ListingTable,
      Provider: ({children, variant = undefined}: {children: React.ReactNode; variant?: string}) => (
        <div data-testid="listing-table-provider" data-variant={variant}>
          {children}
        </div>
      ),
      Container: ({children, disablePaper = false}: {children: React.ReactNode; disablePaper?: boolean}) => (
        <div data-testid="listing-table-container" data-disable-paper={disablePaper ? 'true' : undefined}>
          {children}
        </div>
      ),
      DataGrid: ({rows, columns}: {rows: VerifiableCredentialSummary[]; columns: MockColumn[]}) => (
        <div role="grid">
          {rows.map((row) => (
            <div key={row.id} role="row">
              {columns.map((col) => (
                <div key={col.field} role="gridcell">
                  {col.renderCell?.({row})}
                </div>
              ))}
            </div>
          ))}
        </div>
      ),
    },
  };
});

const {default: useGetVerifiableCredentials} = await import('../../api/useGetVerifiableCredentials');

const rows: VerifiableCredentialSummary[] = [
  {id: 'a', handle: 'alpha', name: 'Alpha', ouId: 'ou-1', ouHandle: 'engineering', vct: 'vct-a', format: 'dc+sd-jwt'},
  {id: 'b', handle: 'beta', name: 'Beta', ouId: 'ou-2', ouHandle: 'sales', vct: 'vct-b', format: 'dc+sd-jwt'},
  {id: 'c', handle: 'gamma', name: 'Gamma', ouId: 'ou-3', vct: 'vct-c', format: 'dc+sd-jwt'},
];

describe('VerifiableCredentialsList', () => {
  beforeEach(() => {
    vi.mocked(useGetVerifiableCredentials).mockReturnValue({
      data: rows,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetVerifiableCredentials>);
  });

  it('should use the data grid variant inside a paper container', () => {
    render(<VerifiableCredentialsList />);

    expect(screen.getByTestId('listing-table-provider')).toHaveAttribute('data-variant', 'data-grid');
    expect(screen.getByTestId('listing-table-container')).not.toHaveAttribute('data-disable-paper');
  });

  it('should show the handle under the name', async () => {
    render(<VerifiableCredentialsList />);

    const [firstRow] = await screen.findAllByRole('row');
    expect(within(firstRow).getByText('Alpha')).toBeInTheDocument();
    expect(within(firstRow).getByText('alpha')).toBeInTheDocument();
  });

  it('should show a dash when the name is missing', async () => {
    vi.mocked(useGetVerifiableCredentials).mockReturnValue({
      data: [{...rows[0], name: undefined}],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetVerifiableCredentials>);

    render(<VerifiableCredentialsList />);

    const [firstRow] = await screen.findAllByRole('row');
    expect(within(firstRow).getByText('-')).toBeInTheDocument();
  });

  it('should name the row actions', async () => {
    render(<VerifiableCredentialsList />);

    const [firstRow] = await screen.findAllByRole('row');
    expect(within(firstRow).getByRole('button', {name: 'Generate offer'})).toBeInTheDocument();
    expect(within(firstRow).getByRole('button', {name: 'Edit'})).toBeInTheDocument();
    expect(within(firstRow).getByRole('button', {name: 'Delete'})).toBeInTheDocument();
  });
});
