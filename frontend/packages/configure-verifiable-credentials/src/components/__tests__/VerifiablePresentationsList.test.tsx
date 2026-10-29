// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {VerifiablePresentationSummary} from '../../models/vp';
import VerifiablePresentationsList from '../VerifiablePresentationsList';

vi.mock('../../api/useGetVerifiablePresentations', () => ({
  default: vi.fn(),
}));
vi.mock('../VerificationDialog', () => ({
  default: () => null,
}));
vi.mock('../VerifiablePresentationDeleteDialog', () => ({
  default: () => null,
}));
vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: () => ({}),
}));

interface MockColumn {
  field: string;
  renderCell?: (params: {row: VerifiablePresentationSummary}) => React.ReactNode;
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
      DataGrid: ({rows, columns}: {rows: VerifiablePresentationSummary[]; columns: MockColumn[]}) => (
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

const {default: useGetVerifiablePresentations} = await import('../../api/useGetVerifiablePresentations');

const rows: VerifiablePresentationSummary[] = [
  {id: 'a', handle: 'alpha', name: 'Alpha', ouId: 'ou-1', ouHandle: 'engineering', vct: 'vct-a', format: 'dc+sd-jwt'},
  {id: 'b', handle: 'beta', name: 'Beta', ouId: 'ou-2', ouHandle: 'sales', vct: 'vct-b', format: 'dc+sd-jwt'},
  {id: 'c', handle: 'gamma', name: 'Gamma', ouId: 'ou-3', vct: 'vct-c', format: 'dc+sd-jwt'},
];

describe('VerifiablePresentationsList', () => {
  beforeEach(() => {
    vi.mocked(useGetVerifiablePresentations).mockReturnValue({
      data: rows,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetVerifiablePresentations>);
  });

  it('should use the data grid variant inside a paper container', () => {
    render(<VerifiablePresentationsList />);

    expect(screen.getByTestId('listing-table-provider')).toHaveAttribute('data-variant', 'data-grid');
    expect(screen.getByTestId('listing-table-container')).not.toHaveAttribute('data-disable-paper');
  });

  it('should show the handle under the name', async () => {
    render(<VerifiablePresentationsList />);

    const [firstRow] = await screen.findAllByRole('row');
    expect(within(firstRow).getByText('Alpha')).toBeInTheDocument();
    expect(within(firstRow).getByText('alpha')).toBeInTheDocument();
  });

  it('should show a dash when the name is missing', async () => {
    vi.mocked(useGetVerifiablePresentations).mockReturnValue({
      data: [{...rows[0], name: undefined}],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetVerifiablePresentations>);

    render(<VerifiablePresentationsList />);

    const [firstRow] = await screen.findAllByRole('row');
    expect(within(firstRow).getByText('-')).toBeInTheDocument();
  });

  it('should name the row actions', async () => {
    render(<VerifiablePresentationsList />);

    const [firstRow] = await screen.findAllByRole('row');
    expect(within(firstRow).getByRole('button', {name: 'Verify'})).toBeInTheDocument();
    expect(within(firstRow).getByRole('button', {name: 'Edit'})).toBeInTheDocument();
    expect(within(firstRow).getByRole('button', {name: 'Delete'})).toBeInTheDocument();
  });
});
