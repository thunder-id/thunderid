// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {AgentListResponse} from '../../models/agent';
import AgentsList from '../AgentsList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

// Mock the dependencies
vi.mock('../../api/useGetAgents');
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});
vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
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
      EmptyState: ({
        title = undefined,
        description = undefined,
        action = undefined,
      }: {
        title?: string;
        description?: string;
        action?: React.ReactNode;
      }): React.ReactElement => (
        <div>
          {title && <h2>{title}</h2>}
          {description && <p>{description}</p>}
          {action}
        </div>
      ),
    },
  };
});

// Mock AgentDeleteDialog to avoid MUI dialog issues
vi.mock('../AgentDeleteDialog', () => ({
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

vi.mock('@thunderid/logger/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/logger/react')>()),
  useLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

const {default: useGetAgents} = await import('../../api/useGetAgents');
const {useNavigate} = await import('react-router');
const {useDataGridLocaleText} = await import('@thunderid/hooks');

describe('AgentsList', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  const mockAgentsData: AgentListResponse = {
    totalResults: 2,
    startIndex: 0,
    count: 2,
    agents: [
      {
        id: 'agent-1',
        ouId: 'ou-1',
        ouHandle: 'engineering',
        type: 'default',
        display: 'Test Agent 1',
        description: 'First test agent',
        clientId: 'client_id_1',
      },
      {
        id: 'agent-2',
        ouId: 'ou-2',
        type: 'default',
        display: 'Test Agent 2',
        description: 'Second test agent',
        clientId: 'client_id_2',
      },
    ],
  };

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockLoggerError.mockReset();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    vi.mocked(useDataGridLocaleText).mockReturnValue({});
  });

  const renderComponent = () => render(<AgentsList />);

  it('should render loading state', () => {
    vi.mocked(useGetAgents).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getByRole('grid')).toBeInTheDocument();
  });

  it('should render error state', () => {
    const error = new Error('Failed to load agents');
    vi.mocked(useGetAgents).mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getByText('Failed to load agents')).toBeInTheDocument();
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  });

  it('should render error state with default message when error has no message', () => {
    const error = new Error();
    error.message = '';
    vi.mocked(useGetAgents).mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getByText('Failed to load agents')).toBeInTheDocument();
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  });

  it('should render agents list successfully', () => {
    vi.mocked(useGetAgents).mockReturnValue({
      data: mockAgentsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getByText('Test Agent 1')).toBeInTheDocument();
    expect(screen.getByText('Test Agent 2')).toBeInTheDocument();
    expect(screen.getByText('First test agent')).toBeInTheDocument();
    expect(screen.getByText('Second test agent')).toBeInTheDocument();
  });

  it('should fall back to the agent id when an agent has no display value', () => {
    vi.mocked(useGetAgents).mockReturnValue({
      data: {...mockAgentsData, agents: [{...mockAgentsData.agents[0], display: undefined}]},
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getAllByText('agent-1').length).toBeGreaterThan(0);
    expect(screen.queryByText('Test Agent 1')).not.toBeInTheDocument();
  });

  it('should display agent ids', () => {
    vi.mocked(useGetAgents).mockReturnValue({
      data: mockAgentsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getByText('agent-1')).toBeInTheDocument();
    expect(screen.getByText('agent-2')).toBeInTheDocument();
  });

  it('should display ouHandle when present and ouId otherwise', () => {
    vi.mocked(useGetAgents).mockReturnValue({
      data: mockAgentsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getByText('engineering')).toBeInTheDocument();
    expect(screen.getByText('ou-2')).toBeInTheDocument();
  });

  it('should display "-" when both ouHandle and ouId are missing', () => {
    const dataWithMissingOu: AgentListResponse = {
      ...mockAgentsData,
      agents: [
        {
          ...mockAgentsData.agents[0],
          ouHandle: undefined,
          ouId: undefined as unknown as string,
        },
      ],
    };

    vi.mocked(useGetAgents).mockReturnValue({
      data: dataWithMissingOu,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('should open delete dialog when clicking Delete action', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetAgents).mockReturnValue({
      data: mockAgentsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });
  });

  it('should close delete dialog when cancelled', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetAgents).mockReturnValue({
      data: mockAgentsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

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

  it('should navigate to agent edit page when clicking row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetAgents).mockReturnValue({
      data: mockAgentsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    const rows = screen.getAllByRole('row');
    await user.click(rows[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/agents/agent-1');
    });
  });

  it('should navigate to agent when Edit action button is clicked', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetAgents).mockReturnValue({
      data: mockAgentsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(0);
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/agents/agent-1');
    });
  });

  it('should log error when navigation fails', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    vi.mocked(useGetAgents).mockReturnValue({
      data: mockAgentsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetAgents>);

    renderComponent();

    const rows = screen.getAllByRole('row');
    await user.click(rows[0]);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to agent',
        expect.objectContaining({
          error: navigationError,
          agentId: 'agent-1',
        }),
      );
    });
  });

  it('should handle empty agents list', () => {
    vi.mocked(useGetAgents).mockReturnValue({
      data: {totalResults: 0, startIndex: 0, count: 0, agents: []},
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetAgents>);

    renderComponent();

    expect(screen.getByRole('grid')).toBeInTheDocument();
  });
});
