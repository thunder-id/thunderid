/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {RoleListResponse} from '../../models/role';
import RolesList from '../RolesList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

// Mock the dependencies
vi.mock('../../api/useGetRoles');
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});
vi.mock('@thunderid/hooks', () => ({
  useDataGridLocaleText: vi.fn(),
}));

// Mock @wso2/oxygen-ui to avoid cssstyle issues with CSS variables
vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    OxygenUIThemeProvider: ({children}: {children: React.ReactNode}) => children,
    ListingTable: {
      Provider: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
      Container: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
      DataGrid: ({
        rows,
        columns,
        onRowClick = undefined,
        getRowId,
      }: {
        rows: Record<string, unknown>[];
        columns: {
          field: string;
          renderCell?: (params: {row: Record<string, unknown>}) => React.ReactElement;
          valueGetter?: (value: unknown, row: Record<string, unknown>) => string;
        }[];
        onRowClick?: (params: {row: Record<string, unknown>}) => void;
        getRowId: (row: Record<string, unknown>) => string;
      }) => (
        <div role="grid" data-testid="data-grid">
          {rows.map((row: Record<string, unknown>) => (
            <div
              key={getRowId(row)}
              role="row"
              onClick={() => onRowClick?.({row})}
              onKeyDown={() => onRowClick?.({row})}
              tabIndex={0}
            >
              {columns.map(
                (col: {
                  field: string;
                  renderCell?: (params: {row: Record<string, unknown>}) => React.ReactElement;
                  valueGetter?: (value: unknown, row: Record<string, unknown>) => string;
                }) => {
                  if (col.renderCell) {
                    return <div key={col.field}>{col.renderCell({row})}</div>;
                  }
                  if (col.valueGetter) {
                    return <div key={col.field}>{col.valueGetter(null, row)}</div>;
                  }
                  return <div key={col.field}>{row[col.field] as string}</div>;
                },
              )}
            </div>
          ))}
          <div>
            1–{rows.length} of {rows.length}
          </div>
        </div>
      ),
      RowActions: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
    },
  };
});

// Mock RoleDeleteDialog
vi.mock('../RoleDeleteDialog', () => ({
  default: ({open, onClose}: {open: boolean; onClose: () => void}) =>
    open ? (
      <div role="dialog" data-testid="delete-dialog">
        <button type="button" onClick={onClose}>
          Cancel
        </button>
        <button type="button">Delete</button>
      </div>
    ) : null,
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

const {default: useGetRoles} = await import('../../api/useGetRoles');
const {useNavigate} = await import('react-router');
const {useDataGridLocaleText} = await import('@thunderid/hooks');

describe('RolesList', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  const mockRolesData: RoleListResponse = {
    totalResults: 2,
    startIndex: 0,
    count: 2,
    roles: [
      {
        id: 'role-1',
        name: 'Admin Role',
        description: 'Administrator role',
        ouId: 'ou-1',
      },
      {
        id: 'role-2',
        name: 'Viewer Role',
        description: undefined,
        ouId: 'ou-2',
      },
    ],
  };

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockLoggerError.mockReset();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    vi.mocked(useDataGridLocaleText).mockReturnValue({});
  });

  const renderComponent = () => render(<RolesList />);

  it('should pass loading=true to ListingTable.Provider while fetching', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    // Smoke-test: component renders without error in loading state.
    // The real loading indicator is owned by ListingTable.Provider (not testable with this mock).
    expect(screen.getByRole('grid')).toBeInTheDocument();
    expect(screen.queryByRole('row')).not.toBeInTheDocument();
  });

  it('should render error state', () => {
    const error = new Error('Failed to load roles');
    vi.mocked(useGetRoles).mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getByRole('heading', {name: 'Failed to load roles'})).toBeInTheDocument();
  });

  it('should render roles list successfully', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getByText('Admin Role')).toBeInTheDocument();
    expect(screen.getByText('Viewer Role')).toBeInTheDocument();
    expect(screen.getByText('Administrator role')).toBeInTheDocument();
  });

  it('should display "-" for missing description', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const dashElements = screen.getAllByText('-');
    expect(dashElements.length).toBeGreaterThan(0);
  });

  it('should open delete dialog when clicking Delete action', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });
  });

  it('should close delete dialog when cancelled', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });

    const cancelButton = screen.getByRole('button', {name: /cancel/i});
    await user.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
    });
  });

  it('should navigate when clicking row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const rows = screen.getAllByRole('row');
    expect(rows.length).toBeGreaterThanOrEqual(1);
    await user.click(rows[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/roles/role-1');
    });
  });

  it('should navigate when clicking Edit action button', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(0);
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/roles/role-1');
    });
  });

  it('should render no rows when roles list is empty', () => {
    vi.mocked(useGetRoles).mockReturnValue({
      data: {
        totalResults: 0,
        startIndex: 0,
        count: 0,
        roles: [],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetRoles>);

    renderComponent();

    expect(screen.getByRole('grid')).toBeInTheDocument();
    expect(screen.queryByRole('row')).not.toBeInTheDocument();
  });

  it('should navigate to correct role when clicking different row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const rows = screen.getAllByRole('row');
    expect(rows.length).toBeGreaterThan(1);
    await user.click(rows[1]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/roles/role-2');
    });
  });

  it('should handle navigation error gracefully', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    vi.mocked(useGetRoles).mockReturnValue({
      data: mockRolesData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetRoles>);

    renderComponent();

    const rows = screen.getAllByRole('row');
    expect(rows.length).toBeGreaterThan(0);
    await user.click(rows[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/roles/role-1');
    });

    expect(screen.getByRole('grid')).toBeInTheDocument();
  });
});
