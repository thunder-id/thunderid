// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderWithProviders, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {page} from 'vitest/browser';
import type {AuthorizationDetailType} from '../../../models/authorization-detail-type';
import AuthorizationDetailTypesList from '../AuthorizationDetailTypesList';

const mockShowToast = vi.fn();
const mockNavigate = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useToast: () => ({showToast: mockShowToast})};
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

const mockUseGetTypes = vi.fn<(params?: {limit: number; offset: number}) => unknown>();
const mockDelete = vi.fn();

vi.mock('../../../api/useGetAuthorizationDetailTypes', () => ({
  default: (params: {limit: number; offset: number}) => mockUseGetTypes(params),
}));
vi.mock('../../../api/useDeleteAuthorizationDetailType', () => ({
  default: () => ({mutate: mockDelete, isPending: false}),
}));

/** Renders at the given viewport width, and restores the viewport the test started with afterwards. */
async function atViewport(width: number, run: () => Promise<void> | void): Promise<void> {
  const {innerWidth, innerHeight} = window;
  await page.viewport(width, 900);
  try {
    await run();
  } finally {
    await page.viewport(innerWidth, innerHeight);
  }
}

const types: AuthorizationDetailType[] = [
  {
    id: 'type-1',
    type: 'payment_initiation',
    name: 'Payment initiation',
    description: 'Pays once from the account of the user',
    version: 2,
    schema: {type: 'object', properties: {}},
    processor: {endpoint: 'https://bank.example.com/process'},
    reuse: false,
    ouId: 'ou-1',
  },
  {
    id: 'type-2',
    type: 'account_information',
    name: 'Account information',
    version: 1,
    schema: {type: 'object', properties: {}},
    reuse: true,
    ouId: 'ou-1',
  },
];

describe('AuthorizationDetailTypesList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
    mockUseGetTypes.mockReturnValue({
      data: {totalResults: 2, startIndex: 0, count: 2, types},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  it('lists the types with their description, type identifier and version', async () => {
    await atViewport(1280, () => {
      renderWithProviders(<AuthorizationDetailTypesList />);

      expect(screen.getByText('Payment initiation')).toBeInTheDocument();
      expect(screen.getByRole('gridcell', {name: 'Pays once from the account of the user'})).toBeInTheDocument();
      expect(screen.getByText('payment_initiation'), 'tells apart types of the same name').toHaveAttribute(
        'title',
        'payment_initiation',
      );
      expect(getComputedStyle(screen.getByText('payment_initiation')).fontFamily).toMatch(/monospace/);
      expect(screen.getByRole('gridcell', {name: '2'})).toBeInTheDocument();
      expect(screen.queryByText('https://bank.example.com/process'), 'the processor is shown on the type').toBeNull();
      expect(screen.queryByText('Ask every time'), 'the consent setting is shown on the type').toBeNull();
    });
  });

  it('keeps every column on a narrow screen, where the list scrolls sideways', async () => {
    await atViewport(480, () => {
      renderWithProviders(<AuthorizationDetailTypesList />);

      const headers = screen.getAllByRole('columnheader').map((header) => header.textContent);
      expect(headers).toEqual(['Name', 'Description', 'Type identifier', 'Version', 'Actions']);
      const scroller = screen.getByRole('grid').querySelector<HTMLElement>('.MuiDataGrid-virtualScroller');
      expect(scroller).not.toBeNull();
      expect(scroller!.scrollWidth).toBeGreaterThan(scroller!.clientWidth);
    });
  });

  it('shows the shared message when there are no types', () => {
    mockUseGetTypes.mockReturnValue({
      data: {totalResults: 0, startIndex: 0, count: 0, types: []},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderWithProviders(<AuthorizationDetailTypesList />);

    expect(screen.getByText('No rows')).toBeInTheDocument();
  });

  it('opens the edit page of a type from its edit action', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypesList />);

    await user.click(screen.getAllByRole('button', {name: 'Edit'})[1]);
    expect(mockNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types/type-2');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('opens the edit page of a type when its row is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypesList />);

    await user.click(screen.getByText('Payment initiation'));
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types/type-1');
  });

  it('deletes a type after confirmation', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypesList />);

    await user.click(screen.getAllByRole('button', {name: 'Delete'})[0]);
    expect(mockNavigate, 'deleting does not open the type').not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', {name: 'Delete'}));
    expect(mockDelete).toHaveBeenCalledWith('type-1', expect.any(Object));

    const {onError} = mockDelete.mock.calls[0][1] as {onError: (err: Error) => void};
    onError(new Error('boom'));
    expect(await screen.findByText('Failed to delete the intent. Please try again.')).toBeInTheDocument();
  });

  it('moves focus to the list once a deleted type is gone from it', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypesList />);

    await user.click(screen.getAllByRole('button', {name: 'Delete'})[0]);
    await user.click(screen.getByRole('button', {name: 'Delete'}));
    const {onSuccess} = mockDelete.mock.calls[0][1] as {onSuccess: () => void};
    act(() => onSuccess());
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());

    mockUseGetTypes.mockReturnValue({
      data: {totalResults: 1, startIndex: 0, count: 1, types: [types[1]]},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    rerender(<AuthorizationDetailTypesList />);

    await waitFor(() => expect(screen.getByRole('grid')).toContainElement(document.activeElement as HTMLElement));
  });

  it('marks a type defined in declarative configuration as read only, with no edit or delete action', () => {
    mockUseGetTypes.mockReturnValue({
      data: {totalResults: 2, startIndex: 0, count: 2, types: [{...types[0], isReadOnly: true}, types[1]]},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
    renderWithProviders(<AuthorizationDetailTypesList />);

    expect(screen.getAllByRole('button', {name: 'Read only'})).toHaveLength(1);
    expect(screen.queryByRole('button', {name: 'View'})).toBeNull();
    expect(screen.getAllByRole('button', {name: 'Edit'})).toHaveLength(1);
    expect(screen.getAllByRole('button', {name: 'Delete'})).toHaveLength(1);
  });

  it('steps back one page, not more, while the page before an emptied page loads', async () => {
    const user = userEvent.setup();
    const emptied = {totalResults: 20, startIndex: 20, count: 0, types: []};
    let deleted = false;
    mockUseGetTypes.mockImplementation((params) => {
      if (deleted && params?.offset === 20) return {data: emptied, isLoading: false, error: null, refetch: vi.fn()};
      if (deleted && params?.offset === 10) {
        return {data: emptied, isLoading: false, isPlaceholderData: true, error: null, refetch: vi.fn()};
      }
      return {
        data: {totalResults: 25, startIndex: params?.offset ?? 0, count: 2, types},
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      };
    });
    const {rerender} = renderWithProviders(<AuthorizationDetailTypesList />);
    await user.click(screen.getByRole('button', {name: /next page/i}));
    await user.click(screen.getByRole('button', {name: /next page/i}));
    expect(mockUseGetTypes).toHaveBeenLastCalledWith({limit: 10, offset: 20});

    deleted = true;
    rerender(<AuthorizationDetailTypesList />);

    await waitFor(() => expect(mockUseGetTypes).toHaveBeenLastCalledWith({limit: 10, offset: 10}));
  });

  it('shows a load failure in place of the list', () => {
    mockUseGetTypes.mockReturnValue({data: undefined, isLoading: false, error: new Error('down'), refetch: vi.fn()});
    renderWithProviders(<AuthorizationDetailTypesList />);

    expect(screen.getByText('Failed to load intents')).toBeInTheDocument();
  });
});
