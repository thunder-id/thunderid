// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-explicit-any */
import {CapabilitiesProvider} from '@thunderid/contexts';
import {render, screen, userEvent, within} from '@thunderid/test-utils';
import type * as OxygenUI from '@wso2/oxygen-ui';
import {type ReactElement, type ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {AgentTypeListItem} from '../../models/agent-type';
import AgentTypesList from '../AgentTypesList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

const mockNavigate = vi.fn();
const mockRefetch = vi.fn();
const mockMutate = vi.fn();

type MockDataGridRow = AgentTypeListItem & Record<string, unknown>;

interface MockDataGridColumn {
  field?: string;
  headerName?: string;
  renderCell?: (params: {row: MockDataGridRow; field: string; value: unknown; id: string}) => ReactNode;
}

interface MockDataGridProps {
  rows?: MockDataGridRow[];
  columns?: MockDataGridColumn[];
  onRowClick?: (params: {row: MockDataGridRow}) => void;
  getRowId?: (row: MockDataGridRow) => string;
}

// Render the grid as plain rows so cells and row actions can be queried without MUI X.
vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof OxygenUI>('@wso2/oxygen-ui');
  return {
    ...actual,
    ListingTable: {
      Provider: ({children, loading = false}: {children: ReactNode; loading?: boolean}) => (
        <div data-testid="listing-table-provider" data-loading={loading ? 'true' : 'false'}>
          {children}
        </div>
      ),
      Container: ({children}: {children: ReactNode}): ReactElement => children as ReactElement,
      DataGrid: ({rows = [], columns = [], onRowClick = undefined, getRowId = undefined}: MockDataGridProps) => (
        <div role="grid">
          <div role="row">
            {columns.map((column) => (
              <span role="columnheader" key={column.field}>
                {column.headerName}
              </span>
            ))}
          </div>
          {rows.map((row) => {
            const rowId = getRowId ? getRowId(row) : row.id;
            return (
              <div
                key={rowId}
                role="row"
                data-testid={`row-${rowId}`}
                onClick={() => onRowClick?.({row})}
                onKeyDown={() => onRowClick?.({row})}
                tabIndex={0}
              >
                {columns.map((column) => (
                  <span role="gridcell" key={`${rowId}-${column.field}`}>
                    {column.renderCell?.({row, field: column.field ?? '', value: row[column.field ?? ''], id: rowId})}
                  </span>
                ))}
              </div>
            );
          })}
        </div>
      ),
      RowActions: ({children}: {children: ReactNode}): ReactElement => children as ReactElement,
      EmptyState: actual.ListingTable.EmptyState,
    },
  };
});

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: mockLoggerError, info: vi.fn(), warn: vi.fn(), debug: vi.fn()}),
}));

const mockUseGetAllAgentTypes = vi.fn<() => any>();
vi.mock('../../api/useGetAllAgentTypes', () => ({
  default: () => mockUseGetAllAgentTypes(),
}));

vi.mock('../../api/useDeleteAgentType', () => ({
  default: () => ({mutate: mockMutate, isPending: false, error: null, reset: vi.fn()}),
}));

const agentTypes: AgentTypeListItem[] = [
  {id: 'type-1', handle: 'default', displayName: 'Default', ouId: 'root-ou', ouHandle: 'default'},
  {id: 'type-2', handle: 'assistant', displayName: 'Assistant', ouId: 'child-ou'},
  {id: 'type-3', handle: 'managed', displayName: 'Managed', ouId: 'root-ou', ouHandle: 'default', isReadOnly: true},
];

// Allows everything except changing agent types, so a check against the wrong resource shows the actions.
const agentTypesViewOnly = {
  can: (resource: string, action = 'view') => resource !== 'agenttype' || action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('AgentTypesList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
    mockUseGetAllAgentTypes.mockReturnValue({data: agentTypes, isLoading: false, error: null, refetch: mockRefetch});
  });

  it('should show the name, handle, ID, and organization unit columns', () => {
    render(<AgentTypesList />);

    const headers = screen.getAllByRole('columnheader').map((header) => header.textContent);
    expect(headers).toEqual(['Name', 'Agent type ID', 'Organization unit', 'Actions']);

    const row = within(screen.getByTestId('row-type-1'));
    expect(row.getByText('Default', {selector: 'p, span'})).toBeInTheDocument();
    expect(row.getByText('type-1')).toBeInTheDocument();
    expect(row.getAllByText('default')).toHaveLength(2);
  });

  // A display name can be a translation template. One with no translation falls back to the handle.
  it('should resolve a translation template display name, and fall back to the handle', () => {
    mockUseGetAllAgentTypes.mockReturnValue({
      data: [{id: 'type-4', handle: 'assistant', displayName: '{{t(custom:missingAgentType)}}', ouId: 'child-ou'}],
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    });

    render(<AgentTypesList />);

    const row = within(screen.getByTestId('row-type-4'));
    expect(row.getAllByText('assistant')).toHaveLength(2);
    expect(row.queryByText('{{t(custom:missingAgentType)}}')).not.toBeInTheDocument();
  });

  it('should fall back to the organization unit ID when the handle is missing', () => {
    render(<AgentTypesList />);

    expect(within(screen.getByTestId('row-type-2')).getByText('child-ou')).toBeInTheDocument();
  });

  it('should report the loading state to the table', () => {
    mockUseGetAllAgentTypes.mockReturnValue({data: undefined, isLoading: true, error: null, refetch: mockRefetch});

    render(<AgentTypesList />);

    expect(screen.getByTestId('listing-table-provider')).toHaveAttribute('data-loading', 'true');
  });

  it('should open the agent type when a row is clicked', async () => {
    const user = userEvent.setup();
    render(<AgentTypesList />);

    await user.click(screen.getByTestId('row-type-2'));

    expect(mockNavigate).toHaveBeenCalledWith('/agent-types/type-2');
  });

  it('should open the agent type from the edit action', async () => {
    const user = userEvent.setup();
    render(<AgentTypesList />);

    await user.click(within(screen.getByTestId('row-type-1')).getByRole('button', {name: 'Edit'}));

    expect(mockNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith('/agent-types/type-1');
  });

  it('should log a failed navigation', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));
    render(<AgentTypesList />);

    await user.click(screen.getByTestId('row-type-1'));

    await vi.waitFor(() => expect(mockLoggerError).toHaveBeenCalled());
  });

  it('should confirm before deleting from the row action', async () => {
    const user = userEvent.setup();
    render(<AgentTypesList />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    await user.click(within(screen.getByTestId('row-type-2')).getByRole('button', {name: 'Delete'}));

    const dialog = screen.getByRole('dialog', {name: 'Delete agent type'});
    expect(mockNavigate).not.toHaveBeenCalled();

    await user.click(within(dialog).getByRole('button', {name: 'Delete'}));

    expect(mockMutate).toHaveBeenCalledWith('type-2', expect.any(Object));
  });

  it('should show read-only types without edit or delete actions', () => {
    render(<AgentTypesList />);

    const row = within(screen.getByTestId('row-type-3'));
    expect(row.getByRole('button', {name: 'Read only'})).toBeInTheDocument();
    expect(row.queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();
    expect(row.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
  });

  it('should hide the edit and delete actions when the admin may only view agent types', () => {
    render(
      <CapabilitiesProvider capabilities={agentTypesViewOnly}>
        <AgentTypesList />
      </CapabilitiesProvider>,
    );

    const row = within(screen.getByTestId('row-type-2'));
    expect(row.getByText('Assistant', {selector: 'p, span'})).toBeInTheDocument();
    expect(row.queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();
    expect(row.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
  });

  it('should render the read failure in place of the table with a retry', async () => {
    const user = userEvent.setup();
    mockUseGetAllAgentTypes.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network down'),
      refetch: mockRefetch,
    });

    render(<AgentTypesList />);

    expect(screen.getByText('Failed to load agent types')).toBeInTheDocument();
    expect(screen.queryByRole('grid')).not.toBeInTheDocument();
    expect(screen.queryByText('Network down')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: /retry|refresh/i}));

    expect(mockRefetch).toHaveBeenCalled();
  });

  it('should render an empty table when there are no agent types', () => {
    mockUseGetAllAgentTypes.mockReturnValue({data: [], isLoading: false, error: null, refetch: mockRefetch});

    render(<AgentTypesList />);

    expect(screen.queryByTestId(/^row-/)).not.toBeInTheDocument();
  });
});
