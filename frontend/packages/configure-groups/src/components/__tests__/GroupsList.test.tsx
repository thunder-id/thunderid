// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import type * as OxygenUI from '@wso2/oxygen-ui';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {GroupListResponse} from '../../models/group';
import GroupsList from '../GroupsList';

interface MockDataGridProps {
  rows?: {id: string; name?: string; [key: string]: unknown}[];
  columns?: {
    field?: string;
    valueGetter?: (value: unknown, row: Record<string, unknown>) => unknown;
    renderCell?: (params: {row: Record<string, unknown>; field: string; value: unknown; id: string}) => React.ReactNode;
  }[];
  loading?: boolean;
  onRowClick?: (params: {row: unknown}, details: unknown, event: unknown) => void;
  getRowId?: (row: {id: string; [key: string]: unknown}) => string;
}

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof OxygenUI>('@wso2/oxygen-ui');
  return {
    ...actual,
    DataGrid: {
      ...(actual.DataGrid ?? {}),
      GridColDef: {},
      GridRenderCellParams: {},
    },
    ListingTable: {
      Provider: ({
        children,
        loading = false,
        variant = undefined,
      }: {
        children: React.ReactNode;
        loading?: boolean;
        variant?: string;
      }) => (
        <div data-testid="listing-table-provider" data-loading={loading ? 'true' : 'false'} data-variant={variant}>
          {children}
        </div>
      ),
      Container: ({children, disablePaper = false}: {children: React.ReactNode; disablePaper?: boolean}) => (
        <div data-testid="listing-table-container" data-disable-paper={disablePaper ? 'true' : undefined}>
          {children}
        </div>
      ),
      CellIcon: ({
        icon,
        primary,
        secondary = undefined,
      }: {
        icon: React.ReactNode;
        primary: React.ReactNode;
        secondary?: React.ReactNode;
      }) => (
        <div data-testid="cell-icon">
          {icon}
          <span>{primary}</span>
          {secondary && <span>{secondary}</span>}
        </div>
      ),
      RowActions: ({children}: {children: React.ReactNode}) => <div data-testid="row-actions">{children}</div>,
      DataGrid: ({
        rows = [],
        columns = [],
        loading = false,
        onRowClick = undefined,
        getRowId = undefined,
      }: MockDataGridProps) => (
        <div data-testid="data-grid" data-loading={loading}>
          {rows.map((row) => {
            const rowId = getRowId ? getRowId(row) : row.id;
            return (
              <div key={rowId} className="MuiDataGrid-row-container">
                <button
                  type="button"
                  className="MuiDataGrid-row"
                  onClick={() => onRowClick?.({row}, {}, {})}
                  data-testid={`row-${rowId}`}
                >
                  {row.name}
                </button>
                {columns.map((column) => {
                  if (!column?.field) return null;
                  const value = column.valueGetter ? column.valueGetter(undefined, row) : row[column.field];
                  const content = column.renderCell
                    ? column.renderCell({row, field: column.field, value, id: String(rowId)})
                    : value;
                  return (
                    <span key={`${rowId}-${column.field}`} className="MuiDataGrid-cell">
                      {content as React.ReactNode}
                    </span>
                  );
                })}
              </div>
            );
          })}
        </div>
      ),
      EmptyState: ({
        title = undefined,
        description = undefined,
        action = undefined,
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

  it('should render groups in the data grid', () => {
    renderWithProviders(<GroupsList />);

    expect(screen.getByTestId('row-g1')).toHaveTextContent('Group One');
    expect(screen.getByTestId('row-g2')).toHaveTextContent('Group Two');
  });

  it('should render the description as a secondary line only when present', () => {
    renderWithProviders(<GroupsList />);

    const nameCells = screen.getAllByTestId('cell-icon');
    expect(nameCells[0]).toHaveTextContent('Group OneFirst group');
    expect(nameCells[1]).toHaveTextContent(/^Group Two$/);
  });

  it('should not render a secondary line for an empty description', () => {
    mockUseGetGroups.mockReturnValue({
      data: {...mockGroupsData, groups: [{id: 'g3', name: 'Group Three', description: '', ouId: 'ou1'}]},
      isLoading: false,
      error: null,
    });

    renderWithProviders(<GroupsList />);

    expect(screen.getByTestId('cell-icon')).toHaveTextContent(/^Group Three$/);
  });

  it('should render the data grid variant inside a paper container', () => {
    renderWithProviders(<GroupsList />);

    expect(screen.getByTestId('listing-table-provider')).toHaveAttribute('data-variant', 'data-grid');
    expect(screen.getByTestId('listing-table-container')).not.toHaveAttribute('data-disable-paper');
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

    expect(screen.getByTestId('listing-table-provider')).toHaveAttribute('data-loading', 'true');
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

    await user.click(screen.getByTestId('row-g1'));

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

    await user.click(screen.getByTestId('row-g1'));

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
