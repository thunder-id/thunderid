// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import type * as OxygenUI from '@wso2/oxygen-ui';
import {DataGrid} from '@wso2/oxygen-ui';
import React from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {UserListResponse} from '../../models/users';
import UsersList from '../UsersList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

const mockNavigate = vi.fn();

// Mock DataGrid to avoid CSS import issues
interface MockRow {
  id: string;
  display?: string;
  attributes?: Record<string, unknown>;
  [key: string]: unknown;
}

interface MockDataGridProps {
  rows?: MockRow[];
  columns?: DataGrid.GridColDef<MockRow>[];
  loading?: boolean;
  onRowClick?: (params: {row: MockRow}, event: never, details: never) => void;
  getRowId?: (row: MockRow) => string;
  [key: string]: unknown;
}

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof OxygenUI>('@wso2/oxygen-ui');
  return {
    ...actual,
    DataGrid: {
      ...(actual.DataGrid ?? {}),
      GridColDef: {} as never,
      GridRenderCellParams: {} as never,
    },
    ListingTable: {
      Provider: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
      Container: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
      DataGrid: ({
        rows = [],
        columns = [],
        loading = undefined,
        onRowClick = undefined,
        getRowId = undefined,
      }: MockDataGridProps) => (
        <div data-testid="data-grid" data-loading={loading}>
          {rows.map((row) => {
            const rowId = getRowId ? getRowId(row) : row.id;
            const displayText = row.display ?? rowId;

            return (
              <div key={rowId} className="MuiDataGrid-row-container">
                <button
                  type="button"
                  className="MuiDataGrid-row"
                  onClick={() => {
                    if (onRowClick) {
                      onRowClick({row}, {} as never, {} as never);
                    }
                  }}
                  data-testid={`row-${rowId}`}
                >
                  {displayText}
                </button>
                {columns?.map((column) => {
                  if (column?.field === undefined) return null;

                  let value: unknown;
                  if (typeof column.valueGetter === 'function') {
                    value = column.valueGetter({} as never, row as never, column as never, {} as never);
                  } else if (column.field in row) {
                    value = row[column.field];
                  } else {
                    value = row.attributes?.[column.field];
                  }

                  const params = {
                    row,
                    field: column.field,
                    value,
                    id: rowId,
                    // The real grid hands an unfocused cell tabIndex -1.
                    tabIndex: -1,
                  };

                  const content = typeof column.renderCell === 'function' ? column.renderCell(params as never) : value;

                  if (content === null || content === undefined) {
                    return null;
                  }

                  // Convert content to a renderable format
                  let renderableContent: React.ReactNode;
                  if (typeof content === 'string' || typeof content === 'number' || typeof content === 'boolean') {
                    renderableContent = String(content);
                  } else if (React.isValidElement(content)) {
                    renderableContent = content;
                  } else if (Array.isArray(content)) {
                    renderableContent = JSON.stringify(content);
                  } else if (typeof content === 'object') {
                    renderableContent = JSON.stringify(content);
                  } else {
                    renderableContent = '';
                  }

                  return (
                    <span key={`${rowId}-${column.field}`} className="MuiDataGrid-cell">
                      {renderableContent}
                    </span>
                  );
                })}
              </div>
            );
          })}
        </div>
      ),
      CellIcon: ({primary}: {primary: string}) => <span>{primary}</span>,
      RowActions: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
      EmptyState: ({
        title = '',
        description = '',
        action = null,
      }: {
        title?: string;
        description?: string;
        action?: React.ReactNode;
      }) => (
        <div>
          {title && <div>{title}</div>}
          {description && <div>{description}</div>}
          {action}
        </div>
      ),
    },
  };
});

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
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
      expect(screen.getByTestId('row-user2')).toHaveTextContent('Jane Smith');
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

  it("keeps the status indicators in the grid's roving focus rather than the page tab order", async () => {
    render(<UsersList />);

    const indicators = await screen.findAllByRole('img', {name: 'Active'});
    expect(indicators.length).toBeGreaterThan(0);
    indicators.forEach((indicator) => expect(indicator).toHaveAttribute('tabindex', '-1'));
  });

  it('displays user avatars with initials', async () => {
    render(<UsersList />);

    await waitFor(() => {
      const grid = screen.getByTestId('data-grid');
      expect(grid).toBeInTheDocument();
    });
  });

  it('displays loading state', () => {
    mockUseGetUsers.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    });

    render(<UsersList />);

    const grid = screen.getByTestId('data-grid');
    expect(grid).toBeInTheDocument();
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
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
    });

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    expect(deleteButtons.length).toBeGreaterThan(0);
  });

  it('navigates to view page when row is clicked', async () => {
    const user = userEvent.setup();
    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
    });

    const row = screen.getByTestId('row-user1');
    await user.click(row);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/user1');
    });
  });

  it('opens delete dialog for the selected user when Delete is clicked', async () => {
    const user = userEvent.setup();
    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
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
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
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

    expect(screen.queryByTestId('data-grid')).not.toBeInTheDocument();
  });

  it('handles error when row click navigation fails', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValue(navigationError);

    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
    });

    const row = screen.getByTestId('row-user1');
    await user.click(row);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/user1');
    });
  });

  it('should navigate to user when View action button is clicked', async () => {
    const user = userEvent.setup();

    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
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
      expect(screen.getByTestId('row-user2')).toHaveTextContent('Jane Smith');
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
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
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
      const grid = screen.getByTestId('data-grid');
      expect(grid).toBeInTheDocument();
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
      expect(screen.getByTestId('row-user1')).toHaveTextContent('user1');
    });
  });

  it('renders independent inline delete buttons for each user row', async () => {
    render(<UsersList />);

    await waitFor(() => {
      expect(screen.getByTestId('row-user1')).toHaveTextContent('John Doe');
      expect(screen.getByTestId('row-user2')).toHaveTextContent('Jane Smith');
    });

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    expect(deleteButtons.length).toBeGreaterThanOrEqual(2);
  });
});
