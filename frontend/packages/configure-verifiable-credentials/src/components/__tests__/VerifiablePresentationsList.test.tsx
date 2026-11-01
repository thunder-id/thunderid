// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor, within} from '@thunderid/test-utils';
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
const mockNavigate = vi.fn();
vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
}));
vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: () => ({}),
}));

const {default: useGetVerifiablePresentations} = await import('../../api/useGetVerifiablePresentations');

const rows: VerifiablePresentationSummary[] = [
  {id: 'a', handle: 'alpha', name: 'Alpha', ouId: 'ou-1', ouHandle: 'engineering', vct: 'vct-a', format: 'dc+sd-jwt'},
  {id: 'b', handle: 'beta', name: 'Beta', ouId: 'ou-2', ouHandle: 'sales', vct: 'vct-b', format: 'dc+sd-jwt'},
  {id: 'c', handle: 'gamma', name: 'Gamma', ouId: 'ou-3', vct: 'vct-c', format: 'dc+sd-jwt'},
];

const bodyRows = (): HTMLElement[] => screen.getAllByRole('row').slice(1);

describe('VerifiablePresentationsList', () => {
  beforeEach(() => {
    vi.mocked(useGetVerifiablePresentations).mockReturnValue({
      data: rows,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetVerifiablePresentations>);
  });

  it('should render the list as a card table', () => {
    render(<VerifiablePresentationsList />);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getAllByRole('columnheader')).toHaveLength(3);
    expect(bodyRows()).toHaveLength(3);
  });

  it('should show the handle under the name', () => {
    render(<VerifiablePresentationsList />);

    const [firstRow] = bodyRows();
    expect(within(firstRow).getByText('Alpha')).toBeInTheDocument();
    expect(within(firstRow).getByText('alpha')).toBeInTheDocument();
  });

  it('should show the organization unit handle, its ID, or a dash', () => {
    vi.mocked(useGetVerifiablePresentations).mockReturnValue({
      data: [rows[0], {...rows[1], ouHandle: undefined}, {...rows[2], ouHandle: undefined, ouId: undefined}],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetVerifiablePresentations>);

    render(<VerifiablePresentationsList />);

    const [first, second, third] = bodyRows();
    expect(within(first).getByText('engineering')).toBeInTheDocument();
    expect(within(second).getByText('ou-2')).toBeInTheDocument();
    expect(within(third).getAllByRole('cell')[1]).toHaveTextContent('-');
  });

  it('should show a dash when the name is missing', () => {
    vi.mocked(useGetVerifiablePresentations).mockReturnValue({
      data: [{...rows[0], name: undefined}],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetVerifiablePresentations>);

    render(<VerifiablePresentationsList />);

    const [firstRow] = bodyRows();
    expect(within(firstRow).getAllByRole('cell')[0]).toHaveTextContent('-');
  });

  it('should sort by name and by organization unit', () => {
    render(<VerifiablePresentationsList />);
    const [nameHeader, ouHeader] = screen.getAllByRole('columnheader');

    fireEvent.click(within(nameHeader).getByRole('button'));
    fireEvent.click(within(nameHeader).getByRole('button'));
    expect(nameHeader).toHaveAttribute('aria-sort', 'descending');
    expect(bodyRows()[0]).toHaveTextContent('Gamma');

    fireEvent.click(within(ouHeader).getByRole('button'));
    expect(ouHeader).toHaveAttribute('aria-sort', 'ascending');
    expect(bodyRows()[0]).toHaveTextContent('Alpha');
  });

  it('should open a row when it is clicked or edited', async () => {
    mockNavigate.mockResolvedValue(undefined);
    render(<VerifiablePresentationsList />);

    fireEvent.click(screen.getByText('Beta'));
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith(expect.stringContaining('/b')));

    fireEvent.click(within(bodyRows()[0]).getByRole('button', {name: 'Edit'}));
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith(expect.stringContaining('/a')));
  });

  it('should not open the row from its other actions', () => {
    mockNavigate.mockClear();
    render(<VerifiablePresentationsList />);

    fireEvent.click(within(bodyRows()[0]).getByRole('button', {name: 'Verify'}));
    fireEvent.click(within(bodyRows()[0]).getByRole('button', {name: 'Delete'}));

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('should mark the table busy while loading', () => {
    vi.mocked(useGetVerifiablePresentations).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as unknown as ReturnType<typeof useGetVerifiablePresentations>);

    render(<VerifiablePresentationsList />);

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
  });

  it('should name the row actions', () => {
    render(<VerifiablePresentationsList />);

    const [firstRow] = bodyRows();
    expect(within(firstRow).getByRole('button', {name: 'Verify'})).toBeInTheDocument();
    expect(within(firstRow).getByRole('button', {name: 'Edit'})).toBeInTheDocument();
    expect(within(firstRow).getByRole('button', {name: 'Delete'})).toBeInTheDocument();
  });
});
