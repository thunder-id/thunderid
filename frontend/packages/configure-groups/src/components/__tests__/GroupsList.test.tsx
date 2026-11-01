// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {GroupListResponse} from '../../models/group';
import GroupsList from '../GroupsList';

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockUseGetGroups = vi.fn();
vi.mock('../../api/useGetGroups', () => ({
  default: (...args: unknown[]): unknown => mockUseGetGroups(...args),
}));

const mockDeleteMutate = vi.fn();
const mockDeleteReset = vi.fn();
vi.mock('../../api/useDeleteGroup', () => ({
  default: () => ({
    mutate: mockDeleteMutate,
    reset: mockDeleteReset,
    isPending: false,
  }),
}));

const mockLogger = {error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn()};
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => mockLogger,
}));

const rowOf = (name: string): HTMLElement => screen.getByText(name).closest<HTMLElement>('tr')!;

const bodyCells = (column: number): string[] =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map((row) => within(row).getAllByRole('cell')[column].textContent ?? '');

describe('GroupsList', () => {
  const mockGroupsData: GroupListResponse = {
    totalResults: 2,
    startIndex: 0,
    count: 2,
    groups: [
      {id: 'g1', name: 'Group One', description: 'First group', ouId: 'ou1'},
      {id: 'g2', name: 'Group Two', ouId: 'ou2'},
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetGroups.mockReturnValue({
      data: mockGroupsData,
      isLoading: false,
      error: null,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render groups in a card table', () => {
    renderWithProviders(<GroupsList />);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(rowOf('Group One')).toBeInTheDocument();
    expect(rowOf('Group Two')).toBeInTheDocument();
  });

  it('should render the description as a secondary line only when present', () => {
    renderWithProviders(<GroupsList />);

    expect(bodyCells(0)).toEqual(['Group OneFirst group', 'Group Two']);
  });

  it('should render the organization unit, falling back to the ID and then a dash', () => {
    mockUseGetGroups.mockReturnValue({
      data: {
        ...mockGroupsData,
        groups: [
          {id: 'g1', name: 'Group One', ouId: 'ou1', ouHandle: 'engineering'},
          {id: 'g2', name: 'Group Two', ouId: 'ou2'},
          {id: 'g3', name: 'Group Three'},
        ],
      },
      isLoading: false,
      error: null,
    });
    renderWithProviders(<GroupsList />);

    expect(bodyCells(1)).toEqual(['engineering', 'ou2', '-']);
  });

  it('should sort the groups by name and by organization unit', async () => {
    const user = userEvent.setup();
    mockUseGetGroups.mockReturnValue({
      data: {
        ...mockGroupsData,
        groups: [
          {id: 'g1', name: 'Beta', ouId: 'ou1', ouHandle: 'sales'},
          {id: 'g2', name: 'Alpha', ouId: 'ou2', ouHandle: 'support'},
          {id: 'g3', name: 'Gamma', ouId: 'ou3', ouHandle: 'engineering'},
        ],
      },
      isLoading: false,
      error: null,
    });
    renderWithProviders(<GroupsList />);

    await user.click(screen.getByRole('button', {name: 'Name'}));
    expect(bodyCells(0)).toEqual(['Alpha', 'Beta', 'Gamma']);

    await user.click(screen.getByRole('button', {name: 'Organization unit'}));
    expect(bodyCells(1)).toEqual(['engineering', 'sales', 'support']);
  });

  it('should request the first page of ten groups, then the page the user moves to', async () => {
    const user = userEvent.setup();
    mockUseGetGroups.mockReturnValue({
      data: {...mockGroupsData, totalResults: 30},
      isLoading: false,
      error: null,
    });
    renderWithProviders(<GroupsList />);

    expect(mockUseGetGroups).toHaveBeenLastCalledWith({limit: 10, offset: 0});

    await user.click(screen.getByRole('button', {name: 'Go to next page'}));
    expect(mockUseGetGroups).toHaveBeenLastCalledWith({limit: 10, offset: 10});

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: '25'}));
    expect(mockUseGetGroups).toHaveBeenLastCalledWith({limit: 25, offset: 0});
  });

  it('should show the empty row when there are no groups', () => {
    mockUseGetGroups.mockReturnValue({
      data: {...mockGroupsData, totalResults: 0, count: 0, groups: []},
      isLoading: false,
      error: null,
    });
    renderWithProviders(<GroupsList />);

    expect(screen.getByText('No rows')).toBeInTheDocument();
  });

  it('should not render a secondary line for an empty description', () => {
    mockUseGetGroups.mockReturnValue({
      data: {...mockGroupsData, groups: [{id: 'g3', name: 'Group Three', description: '', ouId: 'ou1'}]},
      isLoading: false,
      error: null,
    });

    renderWithProviders(<GroupsList />);

    expect(bodyCells(0)).toEqual(['Group Three']);
  });

  it('should label the row action buttons', () => {
    mockUseGetGroups.mockReturnValue({
      data: {
        ...mockGroupsData,
        groups: [...mockGroupsData.groups, {id: 'g3', name: 'System Group', ouId: 'ou1', isReadOnly: true}],
      },
      isLoading: false,
      error: null,
    });
    renderWithProviders(<GroupsList />);

    expect(screen.getAllByRole('button', {name: 'Edit'})).toHaveLength(2);
    expect(screen.getAllByRole('button', {name: 'Delete'})).toHaveLength(2);
    expect(screen.getByRole('button', {name: 'Read only'})).toBeInTheDocument();
  });

  it('should show loading state', () => {
    mockUseGetGroups.mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
    });
    renderWithProviders(<GroupsList />);

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
  });

  it('should show error state', () => {
    mockUseGetGroups.mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error('Fetch failed'),
    });
    renderWithProviders(<GroupsList />);

    expect(screen.getByText('Failed to load groups')).toBeInTheDocument();
    // Resolved through the i18n catalog, not the raw (unlocalized) error message.
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  });

  it('should navigate to group on row click', async () => {
    const user = userEvent.setup();
    mockNavigate.mockResolvedValue(undefined);
    renderWithProviders(<GroupsList />);

    await user.click(rowOf('Group One'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/groups/g1');
    });
  });

  it('should refetch groups when the retry action is clicked in the error state', async () => {
    const user = userEvent.setup();
    const mockRefetch = vi.fn();
    mockUseGetGroups.mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error('Fetch failed'),
      refetch: mockRefetch,
    });
    renderWithProviders(<GroupsList />);

    await user.click(screen.getByRole('button', {name: 'Refresh'}));

    expect(mockRefetch).toHaveBeenCalled();
  });

  it('should log an error when navigating to a group on row click fails', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValue(new Error('navigation failed'));
    renderWithProviders(<GroupsList />);

    await user.click(rowOf('Group One'));

    await waitFor(() => {
      expect(mockLogger.error).toHaveBeenCalledWith('Failed to navigate to group', {
        error: expect.any(Error) as Error,
        groupId: 'g1',
      });
    });
  });

  it('should log an error when navigating to a group on edit button click fails', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValue(new Error('navigation failed'));
    renderWithProviders(<GroupsList />);

    const editButtons = screen.getAllByRole('button', {name: /Edit/i});
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockLogger.error).toHaveBeenCalledWith('Failed to navigate to group details', {
        error: expect.any(Error) as Error,
        groupId: 'g1',
      });
    });
  });

  it('should render hover action buttons for each row', () => {
    renderWithProviders(<GroupsList />);

    const editButtons = screen.getAllByRole('button', {name: /Edit/i});
    const deleteButtons = screen.getAllByRole('button', {name: /Delete/i});
    expect(editButtons.length).toBeGreaterThanOrEqual(2);
    expect(deleteButtons.length).toBeGreaterThanOrEqual(2);
  });

  it('should navigate to group on edit button click', async () => {
    const user = userEvent.setup();
    mockNavigate.mockResolvedValue(undefined);
    renderWithProviders(<GroupsList />);

    const editButtons = screen.getAllByRole('button', {name: /Edit/i});
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/groups/g1');
    });
  });

  it('should open delete dialog on delete button click', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupsList />);

    const deleteButtons = screen.getAllByRole('button', {name: /Delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete group')).toBeInTheDocument();
    });
  });

  it('should close delete dialog when cancel is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupsList />);

    const deleteButtons = screen.getAllByRole('button', {name: /Delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete group')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cancel'));

    await waitFor(() => {
      expect(screen.queryByText('Delete group')).not.toBeInTheDocument();
    });
  });
});
