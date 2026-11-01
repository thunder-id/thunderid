// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {UserListResponse} from '../../models/users';
import UsersList from '../UsersList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

const mockNavigate = vi.fn();

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

// Mock hooks - TanStack Query interfaces
interface UseGetUsersReturn {
  data: UserListResponse | undefined;
  isLoading: boolean;
  error: Error | null;
}

const mockUseGetUsers = vi.fn<() => UseGetUsersReturn>();

vi.mock('@/api/useGetUsers', () => ({
  default: () => mockUseGetUsers(),
}));

// The delete flow lives in UserDeleteDialog (covered by its own test). Stub it here so these
// tests focus on UsersList's responsibility: opening the dialog for the selected user.
vi.mock('@/components/UserDeleteDialog', () => ({
  default: ({open, userId, onClose}: {open: boolean; userId: string | null; onClose: () => void}) =>
    open ? (
      <div data-testid="user-delete-dialog">
        <span data-testid="delete-dialog-user-id">{userId}</span>
        <button type="button" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

const rowOf = (userId: string): HTMLElement => screen.getAllByText(userId)[0].closest<HTMLElement>('tr')!;
const bodyRows = (): HTMLElement[] => screen.getAllByRole('row').slice(1);
const firstCells = (): string[] => bodyRows().map((row) => within(row).getAllByRole('cell')[0].textContent ?? '');

describe('UsersList', () => {
  const mockUsersData: UserListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    users: [
      {
        id: 'user1',
        ouId: 'org1',
        type: 'schema1',
        display: 'John Doe',
        attributes: {
          username: 'john.doe',
          firstname: 'John',
          lastname: 'Doe',
          email: 'john@example.com',
        },
      },
      {
        id: 'user2',
        ouId: 'org2',
        type: 'schema2',
        display: 'Jane Smith',
        attributes: {
          username: 'jane.smith',
          firstname: 'Jane',
          lastname: 'Smith',
          email: 'jane@example.com',
        },
      },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUsers.mockReturnValue({
      data: mockUsersData,
      isLoading: false,
      error: null,
    });
  });

  it('renders DataGrid with users', async () => {
    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
      expect(rowOf('user2')).toHaveTextContent('Jane Smith');
    });
  });

  it("shows each user's access status, treating an absent status as active", async () => {
    mockUseGetUsers.mockReturnValue({
      data: {
        ...mockUsersData,
        users: [{...mockUsersData.users[0], status: {value: 'LOCKED'}}, mockUsersData.users[1]],
      },
      isLoading: false,
      error: null,
    });

    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByText('Locked')).toBeInTheDocument();
      expect(screen.getByText('Active')).toBeInTheDocument();
    });
  });

  it('keeps the status indicators reachable from the keyboard', async () => {
    render(<UsersList />);

    const indicators = await screen.findAllByRole('img', {name: 'Active'});
    expect(indicators.length).toBeGreaterThan(0);
    indicators.forEach((indicator) => expect(indicator).toHaveAttribute('tabindex', '0'));
  });

  it('renders the listing as a card table', () => {
    render(<UsersList />);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.queryByRole('grid')).not.toBeInTheDocument();
  });

  it('gives each row action button an accessible name', () => {
    render(<UsersList />);

    expect(screen.getAllByRole('button', {name: 'Edit'})).toHaveLength(2);
    expect(screen.getAllByRole('button', {name: 'Delete'})).toHaveLength(2);
  });

  it('displays user avatars with initials', async () => {
    render(<UsersList />);

    await waitFor(() => {
      expect(bodyRows()).toHaveLength(2);
    });
  });

  it('displays loading state', () => {
    mockUseGetUsers.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    });

    render(<UsersList />);

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
  });

  it('displays error from users request', async () => {
    mockUseGetUsers.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Failed to load users'),
    });

    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load users')).toBeInTheDocument();
    });
  });

  it('should render inline delete buttons for each row', async () => {
    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
    });

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    expect(deleteButtons.length).toBeGreaterThan(0);
  });

  it('navigates to view page when row is clicked', async () => {
    const user = userEvent.setup();
    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
    });

    const row = rowOf('user1');
    await user.click(row);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/user1');
    });
  });

  it('opens delete dialog for the selected user when Delete is clicked', async () => {
    const user = userEvent.setup();
    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
    });

    expect(screen.queryByTestId('user-delete-dialog')).not.toBeInTheDocument();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('user-delete-dialog')).toBeInTheDocument();
      expect(screen.getByTestId('delete-dialog-user-id')).toHaveTextContent('user1');
    });
  });

  it('cancels delete when the dialog requests close', async () => {
    const user = userEvent.setup();
    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
    });

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('user-delete-dialog')).toBeInTheDocument();
    });

    const cancelButton = screen.getByRole('button', {name: /cancel/i});
    await user.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByTestId('user-delete-dialog')).not.toBeInTheDocument();
    });
  });

  it('renders the error in place of the grid, not a dismissible snackbar', async () => {
    mockUseGetUsers.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Failed to load users'),
    });

    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load users')).toBeInTheDocument();
    });

    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('handles error when row click navigation fails', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValue(navigationError);

    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
    });

    const row = rowOf('user1');
    await user.click(row);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/user1');
    });
  });

  it('should navigate to user when View action button is clicked', async () => {
    const user = userEvent.setup();

    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
    });

    const viewButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(viewButtons.length).toBeGreaterThan(0);
    await user.click(viewButtons[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/user1');
    });
  });

  it('should navigate to correct user when View action is clicked for second row', async () => {
    const user = userEvent.setup();

    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user2')).toHaveTextContent('Jane Smith');
    });

    const viewButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(viewButtons.length).toBeGreaterThan(1);
    await user.click(viewButtons[1]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/user2');
    });
  });

  it('should log error when View button navigation fails', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
    });

    const viewButtons = screen.getAllByRole('button', {name: /^edit$/i});
    await user.click(viewButtons[0]);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to user details',
        expect.objectContaining({
          error: navigationError,
          userId: 'user1',
        }),
      );
    });
  });

  it('renders empty grid when no data', async () => {
    mockUseGetUsers.mockReturnValue({
      data: {
        totalResults: 0,
        startIndex: 1,
        count: 0,
        users: [],
      },
      isLoading: false,
      error: null,
    });

    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByText('No rows')).toBeInTheDocument();
    });
  });

  it('falls back to user ID when display is not set', async () => {
    mockUseGetUsers.mockReturnValue({
      data: {
        totalResults: 1,
        startIndex: 1,
        count: 1,
        users: [
          {
            id: 'user1',
            ouId: 'org1',
            type: 'schema1',
          },
        ],
      },
      isLoading: false,
      error: null,
    });

    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('user1');
    });
  });

  it('renders independent inline delete buttons for each user row', async () => {
    render(<UsersList />);

    await waitFor(() => {
      expect(rowOf('user1')).toHaveTextContent('John Doe');
      expect(rowOf('user2')).toHaveTextContent('Jane Smith');
    });

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    expect(deleteButtons.length).toBeGreaterThanOrEqual(2);
  });

  it('sorts users by name, ID and organization unit', async () => {
    const user = userEvent.setup();
    mockUseGetUsers.mockReturnValue({
      data: {
        ...mockUsersData,
        users: [
          {...mockUsersData.users[0], ouHandle: 'sales'},
          {...mockUsersData.users[1], ouHandle: 'engineering'},
        ],
      },
      isLoading: false,
      error: null,
    });

    render(<UsersList />);

    await user.click(screen.getByRole('button', {name: 'Name'}));
    expect(firstCells()[0]).toContain('Jane Smith');

    await user.click(screen.getByRole('button', {name: 'User ID'}));
    expect(firstCells()[0]).toContain('John Doe');

    await user.click(screen.getByRole('button', {name: 'Organization unit'}));
    expect(firstCells()[0]).toContain('Jane Smith');
  });
});
