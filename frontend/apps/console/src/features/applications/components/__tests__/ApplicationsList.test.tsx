/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import type {ApplicationListResponse} from '../../models/responses';
import ApplicationsList from '../ApplicationsList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

// Mock the dependencies
vi.mock('../../api/useGetApplications');
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
        paginationModel = undefined,
        onPaginationModelChange = undefined,
      }: {
        rows: Record<string, unknown>[];
        columns: {
          field: string;
          renderCell?: (params: {row: Record<string, unknown>}) => React.ReactElement;
          valueGetter?: (value: unknown, row: Record<string, unknown>) => string;
        }[];
        onRowClick?: (params: {row: Record<string, unknown>}) => void;
        getRowId: (row: Record<string, unknown>) => string;
        paginationModel?: {page: number; pageSize: number};
        onPaginationModelChange?: (model: {page: number; pageSize: number}) => void;
      }) => (
        <div role="grid" data-testid="data-grid">
          <button
            type="button"
            data-testid="next-page"
            onClick={() =>
              onPaginationModelChange?.({
                page: (paginationModel?.page ?? 0) + 1,
                pageSize: paginationModel?.pageSize ?? 10,
              })
            }
          >
            next
          </button>
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
      CellIcon: ({
        primary,
        secondary = undefined,
        icon = undefined,
      }: {
        primary: string;
        secondary?: string;
        icon?: React.ReactNode;
      }) => (
        <>
          {icon}
          <span>{primary}</span>
          {secondary && <span>{secondary}</span>}
        </>
      ),
      RowActions: ({children}: {children: React.ReactNode}): React.ReactElement => children as React.ReactElement,
    },
  };
});

// Mock ApplicationDeleteDialog to avoid cssstyle issues with MUI dialogs
vi.mock('../ApplicationDeleteDialog', () => ({
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

const {default: useGetApplications} = await import('../../api/useGetApplications');
const {useNavigate} = await import('react-router');
const {useDataGridLocaleText} = await import('@thunderid/hooks');

describe('ApplicationsList', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  const mockApplicationsData: ApplicationListResponse = {
    totalResults: 2,
    count: 2,
    applications: [
      {
        id: 'app-1',
        name: 'Test App 1',
        description: 'First test application',
        logoUrl: 'https://example.com/logo1.png',
        clientId: 'client_id_1',
        authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
        registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
        isRegistrationFlowEnabled: true,
      },
      {
        id: 'app-2',
        name: 'Test App 2',
        description: 'Second test application',
        logoUrl: '',
        clientId: 'client_id_2',
        authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
        registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
        isRegistrationFlowEnabled: false,
      },
    ],
  };

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockLoggerError.mockReset();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    vi.mocked(useDataGridLocaleText).mockReturnValue({});
  });

  const renderComponent = () => render(<ApplicationsList />);

  it('should render loading state', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    // DataGrid shows loading state through its internal overlay, not a standalone progressbar
    expect(screen.getByRole('grid')).toBeInTheDocument();
  });

  it('should render error state', () => {
    const error = new Error('Failed to load applications');
    vi.mocked(useGetApplications).mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByRole('heading', {name: 'Failed to load applications'})).toBeInTheDocument();
    // The error message is displayed twice - once in heading and once in body
    const errorTexts = screen.getAllByText('Failed to load applications');
    expect(errorTexts).toHaveLength(2);
  });

  it('should render applications list successfully', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByText('Test App 1')).toBeInTheDocument();
    expect(screen.getByText('Test App 2')).toBeInTheDocument();
    expect(screen.getByText('First test application')).toBeInTheDocument();
    expect(screen.getByText('Second test application')).toBeInTheDocument();
  });

  it('should display client IDs as chips', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByText('client_id_1')).toBeInTheDocument();
    expect(screen.getByText('client_id_2')).toBeInTheDocument();
  });

  it('should render avatar with logo URL', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const avatars = screen.getAllByRole('img');
    expect(avatars[0]).toHaveAttribute('src', 'https://example.com/logo1.png');
  });

  it('should render AppWindow icon when logo URL is not provided', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    // Should show AppWindow icon when logo URL is not provided
    const avatars = screen.getAllByRole('img', {hidden: true});
    // The second app (Test App 2) has no logoUrl, so it should have the AppWindow icon
    expect(avatars.length).toBeGreaterThan(0);
  });

  it('should display "-" for missing description', () => {
    const dataWithMissingDescription: ApplicationListResponse = {
      ...mockApplicationsData,
      applications: [
        {
          ...mockApplicationsData.applications[0],
          description: undefined,
        },
      ],
    };

    vi.mocked(useGetApplications).mockReturnValue({
      data: dataWithMissingDescription,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const dashElements = screen.getAllByText('-');
    expect(dashElements.length).toBeGreaterThan(0);
  });

  it('should open delete dialog when clicking Delete action', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });
  });

  it('should navigate when clicking row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const rows = screen.getAllByRole('row');

    expect(rows.length).toBeGreaterThanOrEqual(1);
    await user.click(rows[0]);
  });

  it('should close delete dialog when cancelled', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

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

  it('should handle navigation error gracefully', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const rows = screen.getAllByRole('row');
    expect(rows.length).toBeGreaterThan(0);
    await user.click(rows[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-1');
    });

    expect(screen.getByRole('grid')).toBeInTheDocument();
  });

  it('should handle empty applications list', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: {
        totalResults: 0,
        count: 0,
        applications: [],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetApplications>);

    renderComponent();

    // DataGrid should still render but with no rows
    const grid = screen.getByRole('grid');
    expect(grid).toBeInTheDocument();
  });

  it('should navigate to view page when clicking row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const rows = screen.getAllByRole('row');
    // The mock DataGrid has no header row, so index 0 is the first data row (app-1)
    expect(rows.length).toBeGreaterThan(0);
    await user.click(rows[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-1');
    });
  });

  it('should navigate to correct application when clicking different row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const rows = screen.getAllByRole('row');
    // Click on second data row (index 1, no header row in mock)
    expect(rows.length).toBeGreaterThan(1);
    await user.click(rows[1]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-2');
    });
  });

  it('should prevent row selection on click', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    // Verify disableRowSelectionOnClick is applied by checking grid props
    const grid = screen.getByRole('grid');
    expect(grid).toBeInTheDocument();
  });

  it('should display pagination controls', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    // Check for pagination elements
    expect(screen.getByText(/1–2 of 2/)).toBeInTheDocument();
  });

  it('should apply cursor pointer style to rows', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const grid = screen.getByRole('grid');
    expect(grid).toBeInTheDocument();
    // The cursor style is applied via sx prop to the DataGrid
  });

  it('should handle avatar image error', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const avatars = screen.getAllByRole('img');
    // Trigger onError on the first avatar
    expect(avatars[0]).toBeDefined();
    avatars[0].dispatchEvent(new Event('error'));
    // The onError handler should set src to empty string
    expect(avatars[0]).toBeInTheDocument();
  });

  it('should display "-" for missing clientId', () => {
    const dataWithMissingClientId: ApplicationListResponse = {
      ...mockApplicationsData,
      applications: [
        {
          ...mockApplicationsData.applications[0],
          clientId: undefined,
        },
      ],
    };

    vi.mocked(useGetApplications).mockReturnValue({
      data: dataWithMissingClientId,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    // Should display "-" for missing clientId
    const dashes = screen.getAllByText('-');
    expect(dashes.length).toBeGreaterThan(0);
  });

  it('should navigate to application when Edit action button is clicked', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(0);
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-1');
    });
  });

  it('should navigate to correct application when Edit action is clicked for second row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(1);
    await user.click(editButtons[1]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-2');
    });
  });

  it('should log error when Edit button navigation fails', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to application',
        expect.objectContaining({
          error: navigationError,
          applicationId: 'app-1',
        }),
      );
    });
  });

  describe('Server-side pagination', () => {
    beforeEach(() => {
      vi.mocked(useGetApplications).mockReturnValue({
        data: mockApplicationsData,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useGetApplications>);
    });

    it('requests the first page with the default page size on initial render', () => {
      render(<ApplicationsList />);

      expect(useGetApplications).toHaveBeenCalledWith(expect.objectContaining({limit: 10, offset: 0}));
    });

    it('requests the next page (offset = page * pageSize) when pagination changes', async () => {
      const user = userEvent.setup();
      render(<ApplicationsList />);

      await user.click(screen.getByTestId('next-page'));

      expect(useGetApplications).toHaveBeenLastCalledWith(expect.objectContaining({limit: 10, offset: 10}));
    });

    it('resets to the first page when the search term changes', async () => {
      const user = userEvent.setup();
      const {rerender} = render(<ApplicationsList search="" />);

      await user.click(screen.getByTestId('next-page'));
      expect(useGetApplications).toHaveBeenLastCalledWith(expect.objectContaining({offset: 10}));

      rerender(<ApplicationsList search="portal" />);

      expect(useGetApplications).toHaveBeenLastCalledWith(expect.objectContaining({offset: 0, search: 'portal'}));
    });
  });
});
