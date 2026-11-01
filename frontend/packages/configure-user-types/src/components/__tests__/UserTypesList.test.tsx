// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-return */
import {render, screen, waitFor, userEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type useDeleteUserTypeHook from '../../api/useDeleteUserType';
import type useGetUserTypesHook from '../../api/useGetUserTypes';
import type {UserTypeListResponse} from '../../types/user-types';
import UserTypesList from '../UserTypesList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

const mockNavigate = vi.fn();
const rowOf = (userTypeId: string): HTMLElement => screen.getAllByText(userTypeId)[0].closest<HTMLElement>('tr')!;
const bodyRows = (): HTMLElement[] => screen.getAllByRole('row').slice(1);
const firstCells = (): string[] => bodyRows().map((row) => within(row).getAllByRole('cell')[0].textContent ?? '');
const mockMutateAsync = vi.fn();
const mockRefetchUserTypes = vi.fn();

// Mock react-router
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));
// Mock hooks
type UseGetUserTypesReturn = ReturnType<typeof useGetUserTypesHook>;
type UseDeleteUserTypeReturn = ReturnType<typeof useDeleteUserTypeHook>;

const mockUseGetUserTypes = vi.fn<() => UseGetUserTypesReturn>();
const mockUseDeleteUserType = vi.fn<() => UseDeleteUserTypeReturn>();
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const mockUseGetOrganizationUnits = vi.fn<() => any>();
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const mockRefetchOrganizationUnits = vi.fn() as any;

vi.mock('../../api/useGetUserTypes', () => ({
  default: () => mockUseGetUserTypes(),
}));

vi.mock('../../api/useDeleteUserType', () => ({
  default: () => mockUseDeleteUserType(),
}));

vi.mock('../../../organization-units/api/useGetOrganizationUnits', () => ({
  default: () => mockUseGetOrganizationUnits(),
}));

describe('UserTypesList', () => {
  const mockUserTypesData: UserTypeListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    types: [
      {
        id: 'schema1',
        handle: 'employee',
        displayName: 'Employee Schema',
        ouId: 'root-ou',
        ouHandle: 'root',
        allowSelfRegistration: false,
      },
      {
        id: 'schema2',
        handle: 'contractor',
        displayName: 'Contractor Schema',
        ouId: 'child-ou',
        ouHandle: 'child',
        allowSelfRegistration: true,
      },
    ],
  };

  const mockOrganizationUnitsResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    organizationUnits: [
      {id: 'root-ou', name: 'Root Organization', handle: 'root', description: null, parent: null},
      {id: 'child-ou', name: 'Child Organization', handle: 'child', description: null, parent: 'root-ou'},
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUserTypes.mockReturnValue({
      data: mockUserTypesData,
      isLoading: false,
      error: null,
      refetch: mockRefetchUserTypes,
    } as unknown as ReturnType<typeof useGetUserTypesHook>);
    mockUseDeleteUserType.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      error: null,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useDeleteUserTypeHook>);
    mockUseGetOrganizationUnits.mockReturnValue({
      data: mockOrganizationUnitsResponse,
      isLoading: false,
      error: null,
      refetch: mockRefetchOrganizationUnits,
    });
  });

  it('renders DataGrid with user types', () => {
    render(<UserTypesList />);

    expect(rowOf('schema1')).toHaveTextContent('Employee Schema');
    expect(rowOf('schema2')).toHaveTextContent('Contractor Schema');
  });

  it('renders the listing as a card table', () => {
    render(<UserTypesList />);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.queryByRole('grid')).not.toBeInTheDocument();
  });

  it('names the edit and delete actions of each row', () => {
    render(<UserTypesList />);

    expect(screen.getAllByRole('button', {name: 'Edit'})).toHaveLength(2);
    expect(screen.getAllByRole('button', {name: 'Delete'})).toHaveLength(2);
  });

  it('names the read-only indicator of a read-only user type', () => {
    mockUseGetUserTypes.mockReturnValue({
      data: {...mockUserTypesData, types: [{...mockUserTypesData.types[0], isReadOnly: true}]},
      isLoading: false,
      error: null,
      refetch: mockRefetchUserTypes,
    } as unknown as ReturnType<typeof useGetUserTypesHook>);

    render(<UserTypesList />);

    expect(screen.getByRole('button', {name: 'Read only'})).toBeInTheDocument();
  });

  it('shows the handle under the display name', () => {
    render(<UserTypesList />);

    expect(rowOf('schema1')).toHaveTextContent('employee');
    expect(rowOf('schema2')).toHaveTextContent('contractor');
  });

  it('shows organization unit names when available', () => {
    render(<UserTypesList />);

    expect(screen.getByText('root')).toBeInTheDocument();
    expect(screen.getByText('child')).toBeInTheDocument();
  });

  it('falls back to organization unit id when ouHandle is missing', () => {
    mockUseGetUserTypes.mockReturnValueOnce({
      data: {
        ...mockUserTypesData,
        types: [{...mockUserTypesData.types[0], ouHandle: undefined}],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetUserTypesHook>);

    render(<UserTypesList />);

    expect(screen.getByText('root-ou')).toBeInTheDocument();
  });

  it('shows no data text when organization unit is not provided', () => {
    mockUseGetUserTypes.mockReturnValueOnce({
      data: {
        ...mockUserTypesData,
        types: [{...mockUserTypesData.types[0], ouId: undefined, ouHandle: undefined}],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetUserTypesHook>);

    render(<UserTypesList />);

    expect(screen.getByText('No data available')).toBeInTheDocument();
  });

  it('displays loading state', () => {
    mockUseGetUserTypes.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as unknown as ReturnType<typeof useGetUserTypesHook>);

    render(<UserTypesList />);

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
  });

  it('renders a read error state in place of the grid', async () => {
    const error = new Error('Failed to load user types');

    mockUseGetUserTypes.mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
      refetch: mockRefetchUserTypes,
    } as unknown as ReturnType<typeof useGetUserTypesHook>);

    render(<UserTypesList />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load user types')).toBeInTheDocument();
    });
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('retries the query when Refresh is clicked on the read error state', async () => {
    const user = userEvent.setup();
    const error = new Error('Failed to load user types');

    mockUseGetUserTypes.mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
      refetch: mockRefetchUserTypes,
    } as unknown as ReturnType<typeof useGetUserTypesHook>);

    render(<UserTypesList />);

    const refreshButton = await screen.findByRole('button', {name: /refresh/i});
    await user.click(refreshButton);

    expect(mockRefetchUserTypes).toHaveBeenCalled();
  });

  it('renders inline delete buttons for each row', () => {
    render(<UserTypesList />);

    // The component uses inline delete buttons instead of a menu
    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    expect(deleteButtons.length).toBeGreaterThan(0);
  });

  it('navigates to edit page when row is clicked', async () => {
    const user = userEvent.setup();
    render(<UserTypesList />);

    // Row click navigates to the user type view page
    const row = rowOf('schema1');
    await user.click(row);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types/schema1');
    });
  });

  it('opens delete dialog when Delete is clicked', async () => {
    const user = userEvent.setup();
    render(<UserTypesList />);

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete user type')).toBeInTheDocument();
      expect(screen.getByText('Are you sure you want to delete this user type?')).toBeInTheDocument();
    });
  });

  it('cancels delete when Cancel button is clicked', async () => {
    const user = userEvent.setup();
    render(<UserTypesList />);

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete user type')).toBeInTheDocument();
    });

    const cancelButton = screen.getByRole('button', {name: /Cancel/i});
    await user.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByText('Delete user type')).not.toBeInTheDocument();
    });
  });

  it('deletes user type when confirmed', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValue(undefined);

    render(<UserTypesList />);

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete user type')).toBeInTheDocument();
    });

    const confirmButton = screen.getByRole('button', {name: /delete/i});
    await user.click(confirmButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith('schema1');
    });
  });

  it('displays delete error in dialog', async () => {
    const user = userEvent.setup();
    const deleteError = new Error('Failed to delete');

    mockUseDeleteUserType.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      error: deleteError,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useDeleteUserTypeHook>);

    render(<UserTypesList />);

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Failed to delete user type. Please try again.')).toBeInTheDocument();
    });
  });

  it('navigates when row is clicked', async () => {
    const user = userEvent.setup();
    render(<UserTypesList />);

    const row = rowOf('schema1');
    await user.click(row);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types/schema1');
    });
  });

  it('displays deleting state on confirm button', async () => {
    const user = userEvent.setup();
    mockUseDeleteUserType.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: true,
      error: null,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useDeleteUserTypeHook>);

    render(<UserTypesList />);

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Loading...')).toBeInTheDocument();
    });
  });

  it('renders empty grid when no data', () => {
    mockUseGetUserTypes.mockReturnValue({
      data: {
        totalResults: 0,
        startIndex: 1,
        count: 0,
        types: [],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetUserTypesHook>);

    render(<UserTypesList />);

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'false');
    expect(screen.getByText('No rows')).toBeInTheDocument();
  });

  it('keeps delete dialog open on delete error', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockRejectedValue(new Error('Delete failed'));

    render(<UserTypesList />);

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByText('Delete user type')).toBeInTheDocument();
    });

    const confirmButton = screen.getByRole('button', {name: /delete/i});
    await user.click(confirmButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith('schema1');
      // Dialog stays open so user can see error and retry
      expect(screen.getByText('Delete user type')).toBeInTheDocument();
    });
  });

  it('handles navigation error when row is clicked', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));

    render(<UserTypesList />);

    const row = rowOf('schema1');
    await user.click(row);

    // Navigation was called but failed - the catch handler silently handles the error
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types/schema1');
    });
  });

  it('should navigate to user type when Edit action button is clicked', async () => {
    const user = userEvent.setup();

    render(<UserTypesList />);

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(0);
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types/schema1');
    });
  });

  it('should navigate to correct user type when Edit action is clicked for second row', async () => {
    const user = userEvent.setup();

    render(<UserTypesList />);

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(1);
    await user.click(editButtons[1]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types/schema2');
    });
  });

  it('should log error when Edit button navigation fails', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    render(<UserTypesList />);

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to user type',
        expect.objectContaining({
          error: navigationError,
          userTypeId: 'schema1',
        }),
      );
    });
  });

  it('sorts user types by name, ID, organization unit and self registration', async () => {
    const user = userEvent.setup();
    render(<UserTypesList />);

    await user.click(screen.getByRole('button', {name: 'Name'}));
    expect(firstCells()[0]).toContain('Contractor Schema');

    await user.click(screen.getByRole('button', {name: 'User type ID'}));
    expect(firstCells()[0]).toContain('Employee Schema');

    await user.click(screen.getByRole('button', {name: 'Organization unit'}));
    expect(firstCells()[0]).toContain('Contractor Schema');
    await user.click(screen.getByRole('button', {name: 'Organization unit'}));
    expect(firstCells()[0]).toContain('Employee Schema');

    await user.click(screen.getByRole('button', {name: 'Self registration'}));
    expect(firstCells()[0]).toContain('Employee Schema');
    await user.click(screen.getByRole('button', {name: 'Self registration'}));
    expect(firstCells()[0]).toContain('Contractor Schema');
  });
});
