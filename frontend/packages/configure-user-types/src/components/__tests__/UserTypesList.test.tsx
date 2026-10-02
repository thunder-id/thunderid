// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-return */
import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import type * as OxygenUI from '@wso2/oxygen-ui';
import {type ReactElement, type ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type useGetUserTypesHook from '../../api/useGetUserTypes';
import type {UserTypeListResponse, UserTypeListItem} from '../../types/user-types';
import UserTypesList from '../UserTypesList';

const {mockLoggerError, mockUserTypeDeleteDialog} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
  mockUserTypeDeleteDialog: vi.fn(),
}));

const mockNavigate = vi.fn();
const mockRefetchUserTypes = vi.fn();

// Mock the shared delete dialog: its own behavior (usages check, mutation, errors) is
// covered by UserTypeDeleteDialog.test.tsx. Here we only verify UserTypesList wires the
// right props to it and reacts correctly to onClose.
vi.mock('../edit-user-type/UserTypeDeleteDialog', () => ({
  default: (props: {open: boolean; userTypeId: string | null; onClose: () => void}) => {
    mockUserTypeDeleteDialog(props);
    if (!props.open) return null;
    return (
      <div role="dialog" data-testid="user-type-delete-dialog">
        <span>Delete User Type</span>
        <span>{props.userTypeId}</span>
        <button type="button" onClick={props.onClose}>
          Close
        </button>
      </div>
    );
  },
}));

type MockDataGridRow = UserTypeListItem & Record<string, unknown>;

type MockDataGridColumn =
  | {
      field?: string;
      valueGetter?: (_value: unknown, row: MockDataGridRow) => unknown;
      renderCell?: (params: {row: MockDataGridRow; field: string; value: unknown; id: string}) => ReactNode;
    }
  | null
  | undefined;

interface MockDataGridProps {
  rows?: MockDataGridRow[];
  columns?: MockDataGridColumn[];
  loading?: boolean;
  onRowClick?: (params: {row: MockDataGridRow}) => void;
  getRowId?: (row: MockDataGridRow) => string;
}

// Mock @wso2/oxygen-ui to avoid CSS import issues and use ListingTable
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
      Provider: ({children, loading = false}: {children: ReactNode; loading?: boolean}) => (
        <div data-testid="listing-table-provider" data-loading={loading ? 'true' : 'false'}>
          {children}
        </div>
      ),
      Container: ({children}: {children: ReactNode}): ReactElement => children as ReactElement,
      DataGrid: ({
        rows = [],
        columns = [],
        loading = false,
        onRowClick = undefined,
        getRowId = undefined,
      }: MockDataGridProps) => (
        <div data-testid="data-grid" data-loading={loading ? 'true' : 'false'}>
          {rows.map((row: MockDataGridRow) => {
            const rowId = getRowId ? getRowId(row) : String(row.id ?? '');
            return (
              <div
                key={rowId}
                role="row"
                data-testid={`row-${rowId}`}
                onClick={() => onRowClick?.({row})}
                onKeyDown={() => onRowClick?.({row})}
                tabIndex={0}
              >
                {columns.map((column) => {
                  if (!column?.field) return null;

                  const fallbackValue = (row as Record<string, unknown>)[column.field];
                  const value =
                    typeof column.valueGetter === 'function' ? column.valueGetter(undefined, row) : fallbackValue;

                  const params = {
                    row,
                    field: column.field,
                    value,
                    id: rowId,
                  };

                  const content = (typeof column.renderCell === 'function' ? column.renderCell(params) : value) as
                    | ReactNode
                    | null
                    | undefined;

                  if (content === null || content === undefined) {
                    return null;
                  }

                  return (
                    <span key={`${rowId}-${column.field}`} className="MuiDataGrid-cell">
                      {content}
                    </span>
                  );
                })}
              </div>
            );
          })}
        </div>
      ),
      CellIcon: ({primary, icon = undefined}: {primary: string; icon?: ReactNode}) => (
        <>
          {icon}
          <span>{primary}</span>
        </>
      ),
      RowActions: ({children}: {children: ReactNode}): ReactElement => children as ReactElement,
      EmptyState: ({
        title = undefined,
        description = undefined,
        action = undefined,
      }: {
        title?: string;
        description?: string;
        action?: ReactNode;
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
// Mock hooks
type UseGetUserTypesReturn = ReturnType<typeof useGetUserTypesHook>;

const mockUseGetUserTypes = vi.fn<() => UseGetUserTypesReturn>();
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const mockUseGetOrganizationUnits = vi.fn<() => any>();
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const mockRefetchOrganizationUnits = vi.fn() as any;

vi.mock('../../api/useGetUserTypes', () => ({
  default: () => mockUseGetUserTypes(),
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
      {id: 'schema1', name: 'Employee Schema', ouId: 'root-ou', ouHandle: 'root', allowSelfRegistration: false},
      {id: 'schema2', name: 'Contractor Schema', ouId: 'child-ou', ouHandle: 'child', allowSelfRegistration: true},
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
    mockUseGetOrganizationUnits.mockReturnValue({
      data: mockOrganizationUnitsResponse,
      isLoading: false,
      error: null,
      refetch: mockRefetchOrganizationUnits,
    });
  });

  it('renders DataGrid with user types', () => {
    render(<UserTypesList />);

    expect(screen.getByTestId('row-schema1')).toHaveTextContent('Employee Schema');
    expect(screen.getByTestId('row-schema2')).toHaveTextContent('Contractor Schema');
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

    expect(screen.getByTestId('listing-table-provider')).toHaveAttribute('data-loading', 'true');
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
    expect(screen.queryByTestId('data-grid')).not.toBeInTheDocument();
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
    const row = screen.getByTestId('row-schema1');
    await user.click(row);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types/schema1');
    });
  });

  it('opens delete dialog with the clicked row id when Delete is clicked', async () => {
    const user = userEvent.setup();
    render(<UserTypesList />);

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('user-type-delete-dialog')).toBeInTheDocument();
    });
    expect(mockUserTypeDeleteDialog).toHaveBeenLastCalledWith(
      expect.objectContaining({open: true, userTypeId: 'schema1'}),
    );
  });

  it('closes the delete dialog and clears the selection when onClose fires', async () => {
    const user = userEvent.setup();
    render(<UserTypesList />);

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('user-type-delete-dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', {name: /close/i}));

    await waitFor(() => {
      expect(screen.queryByTestId('user-type-delete-dialog')).not.toBeInTheDocument();
    });
    expect(mockUserTypeDeleteDialog).toHaveBeenLastCalledWith(expect.objectContaining({open: false, userTypeId: null}));
  });

  it('navigates when row is clicked', async () => {
    const user = userEvent.setup();
    render(<UserTypesList />);

    const row = screen.getByTestId('row-schema1');
    await user.click(row);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/user-types/schema1');
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

    const grid = screen.getByTestId('data-grid');
    expect(grid).toBeInTheDocument();
    expect(grid).toHaveAttribute('data-loading', 'false');
  });

  it('handles navigation error when row is clicked', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));

    render(<UserTypesList />);

    const row = screen.getByTestId('row-schema1');
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
});
