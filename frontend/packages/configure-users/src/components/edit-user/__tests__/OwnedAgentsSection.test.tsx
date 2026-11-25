// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OwnedAgentListResponse} from '../../../models/users';
import OwnedAgentsSection from '../OwnedAgentsSection';

const {getOwnedAgentsMock, refetchMock, navigateMock} = vi.hoisted(() => ({
  getOwnedAgentsMock:
    vi.fn<() => {data: OwnedAgentListResponse | undefined; isLoading: boolean; error: Error | null}>(),
  refetchMock: vi.fn(),
  navigateMock: vi.fn(),
}));

vi.mock('../../../api/useGetUserOwnedAgents', () => ({
  default: () => ({...getOwnedAgentsMock(), refetch: refetchMock}),
}));

vi.mock('react-router', async () => ({
  ...(await vi.importActual<typeof import('react-router')>('react-router')),
  useNavigate: () => navigateMock,
}));

vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: () => ({}),
}));

// Renders the grid as plain rows, avoiding the DataGrid's cssstyle issues with CSS variables.
vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    DataGrid: {
      ...actual.DataGrid,
      DataGrid: ({
        rows,
        columns,
        loading = false,
        onRowClick = undefined,
        getRowId,
        localeText = undefined,
      }: {
        rows: Record<string, unknown>[];
        columns: {
          field: string;
          renderCell?: (params: {row: Record<string, unknown>}) => React.ReactNode;
          valueGetter?: (value: unknown, row: Record<string, unknown>) => string;
        }[];
        loading?: boolean;
        localeText?: {noRowsLabel?: string};
        onRowClick?: (params: {row: Record<string, unknown>}) => void;
        getRowId: (row: Record<string, unknown>) => string;
      }) => (
        <div role="grid" aria-busy={loading}>
          {rows.length === 0 && <p>{localeText?.noRowsLabel}</p>}
          {rows.map((row) => (
            <div
              key={getRowId(row)}
              role="row"
              onClick={() => onRowClick?.({row})}
              onKeyDown={() => onRowClick?.({row})}
              tabIndex={0}
            >
              {columns.map((col) => (
                <div key={col.field}>
                  {col.renderCell
                    ? col.renderCell({row})
                    : col.valueGetter
                      ? col.valueGetter(undefined, row)
                      : (row[col.field] as string | undefined)}
                </div>
              ))}
            </div>
          ))}
        </div>
      ),
    },
  };
});

describe('OwnedAgentsSection', () => {
  const ownedAgents: OwnedAgentListResponse = {
    totalResults: 2,
    count: 2,
    agents: [
      {id: 'agent-1', display: 'Claude Code', kind: 'personal-assistant'},
      {id: 'agent-2', display: 'Billing Agent'},
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    navigateMock.mockResolvedValue(undefined);
  });

  it("lists the user's agents with their names and IDs", () => {
    getOwnedAgentsMock.mockReturnValue({data: ownedAgents, isLoading: false, error: null});

    render(<OwnedAgentsSection userId="user-1" />);

    expect(screen.getByText('Owned agents')).toBeInTheDocument();
    const rows = screen.getAllByRole('row');
    expect(within(rows[0]).getByText('Claude Code')).toBeInTheDocument();
    expect(within(rows[0]).getByText('agent-1')).toBeInTheDocument();
    expect(within(rows[1]).getByText('Billing Agent')).toBeInTheDocument();
  });

  it('marks only the personal agents with a Personal chip', () => {
    getOwnedAgentsMock.mockReturnValue({data: ownedAgents, isLoading: false, error: null});

    render(<OwnedAgentsSection userId="user-1" />);

    const rows = screen.getAllByRole('row');
    expect(within(rows[0]).getByText('Personal')).toBeInTheDocument();
    expect(within(rows[1]).queryByText('Personal')).not.toBeInTheDocument();
  });

  it("opens the agent's page from the Edit action", async () => {
    const user = userEvent.setup();
    getOwnedAgentsMock.mockReturnValue({data: ownedAgents, isLoading: false, error: null});

    render(<OwnedAgentsSection userId="user-1" />);

    await user.click(within(screen.getAllByRole('row')[1]).getByRole('button', {name: 'Edit'}));

    expect(navigateMock).toHaveBeenCalledTimes(1);
    expect(navigateMock).toHaveBeenCalledWith('/agents/agent-2');
  });

  it("opens the agent's page when its row is clicked", async () => {
    const user = userEvent.setup();
    getOwnedAgentsMock.mockReturnValue({data: ownedAgents, isLoading: false, error: null});

    render(<OwnedAgentsSection userId="user-1" />);

    await user.click(screen.getAllByRole('row')[0]);

    expect(navigateMock).toHaveBeenCalledWith('/agents/agent-1');
  });

  it('says the user owns no agents when the table is empty', () => {
    getOwnedAgentsMock.mockReturnValue({data: {totalResults: 0, count: 0, agents: []}, isLoading: false, error: null});

    render(<OwnedAgentsSection userId="user-1" />);

    expect(screen.getByText("This user doesn't own any agents.")).toBeInTheDocument();
  });

  it('shows the table as loading while the agents load', () => {
    getOwnedAgentsMock.mockReturnValue({data: undefined, isLoading: true, error: null});

    render(<OwnedAgentsSection userId="user-1" />);

    expect(screen.getByRole('grid')).toHaveAttribute('aria-busy', 'true');
  });

  it('shows an inline error with a retry when the agents fail to load', async () => {
    const user = userEvent.setup();
    getOwnedAgentsMock.mockReturnValue({data: undefined, isLoading: false, error: new Error('boom')});

    render(<OwnedAgentsSection userId="user-1" />);

    expect(screen.getByText('Owned agents')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: /retry|refresh/i}));
    expect(refetchMock).toHaveBeenCalled();
  });
});
