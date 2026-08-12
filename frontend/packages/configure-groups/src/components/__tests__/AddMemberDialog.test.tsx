// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import type * as OxygenUI from '@wso2/oxygen-ui';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import AddMemberDialog from '../edit-group/members-settings/AddMemberDialog';

interface MockDataGridProps {
  rows?: {id: string; [key: string]: unknown}[];
  loading?: boolean;
  checkboxSelection?: boolean;
  onRowSelectionModelChange?: (model: {type: string; ids: Set<string>}) => void;
}

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof OxygenUI>('@wso2/oxygen-ui');
  return {
    ...actual,
    DataGrid: {
      ...(actual.DataGrid ?? {}),
      DataGrid: ({
        rows = [],
        loading = false,
        checkboxSelection = false,
        onRowSelectionModelChange = undefined,
      }: MockDataGridProps) => (
        <div data-testid="users-grid" data-loading={loading}>
          {rows.map((row) => (
            <div key={row.id} data-testid={`user-${row.id}`}>
              {checkboxSelection && (
                <input
                  type="checkbox"
                  data-testid={`checkbox-${row.id}`}
                  onChange={() => {
                    onRowSelectionModelChange?.({type: 'include', ids: new Set([row.id])});
                  }}
                />
              )}
              {row.id}
            </div>
          ))}
        </div>
      ),
    },
  };
});

interface MockQueryResult {
  data?: Record<string, unknown> | null;
  isLoading?: boolean;
  error?: Error;
}

const mockUseGetUsers = vi.fn<() => MockQueryResult>();
const mockUseGetApplications = vi.fn<() => MockQueryResult>();
const mockUseGetAgents = vi.fn<() => MockQueryResult>();
const mockUseGetAllCandidates = vi.fn();
const mockUseGetAllGroupMembers = vi.fn();
const mockUseGetGroups = vi.fn<() => MockQueryResult>();
vi.mock('../../api/useGetAllCandidates', () => ({
  default: (...args: unknown[]): unknown => mockUseGetAllCandidates(...args),
}));
vi.mock('../../api/useGetAllGroupMembers', () => ({
  default: (...args: unknown[]): unknown => mockUseGetAllGroupMembers(...args),
}));

describe('AddMemberDialog', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    onAdd: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetAllCandidates.mockImplementation(({itemsKey}: {itemsKey: string}) => {
      const responses: Record<string, MockQueryResult> = {
        users: mockUseGetUsers(),
        applications: mockUseGetApplications(),
        agents: mockUseGetAgents(),
        groups: mockUseGetGroups(),
      };
      const response = responses[itemsKey];
      return {
        data: response?.data?.[itemsKey] ?? [],
        isLoading: response?.isLoading ?? false,
        error: response?.error,
      };
    });
    mockUseGetUsers.mockReturnValue({
      data: {
        totalResults: 2,
        startIndex: 0,
        count: 2,
        users: [
          {id: 'u1', ouId: 'ou1', type: 'Person'},
          {id: 'u2', ouId: 'ou2', type: 'Person'},
        ],
      },
      isLoading: false,
    });
    mockUseGetApplications.mockReturnValue({
      data: {
        totalResults: 2,
        count: 2,
        applications: [
          {id: 'a1', name: 'Orders API', description: 'Orders backend'},
          {id: 'a2', name: 'Billing API', description: 'Billing backend'},
        ],
      },
      isLoading: false,
    });
    mockUseGetAgents.mockReturnValue({
      data: {
        totalResults: 2,
        count: 2,
        agents: [
          {id: 'agent-1', name: 'Automation Agent'},
          {id: 'agent-2', name: 'Support Agent'},
        ],
      },
      isLoading: false,
    });
    mockUseGetGroups.mockReturnValue({
      data: {
        totalResults: 3,
        startIndex: 0,
        count: 3,
        groups: [
          {id: 'g1', name: 'Engineering', ouId: 'ou1'},
          {id: 'g2', name: 'Support', ouId: 'ou1'},
          {id: 'g3', name: 'Finance', ouId: 'ou2'},
        ],
      },
      isLoading: false,
    });
    mockUseGetAllGroupMembers.mockReturnValue({
      data: {totalResults: 0, startIndex: 1, count: 0, members: [], links: []},
      isLoading: false,
      refetch: vi.fn(),
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render dialog when open', () => {
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    expect(screen.getByText('Add Member')).toBeInTheDocument();
  });

  it('should not render when closed', () => {
    renderWithProviders(<AddMemberDialog {...defaultProps} open={false} />);

    expect(screen.queryByText('Add Member')).not.toBeInTheDocument();
  });

  it('should render users in the grid', () => {
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    expect(screen.getByTestId('user-u1')).toBeInTheDocument();
    expect(screen.getByTestId('user-u2')).toBeInTheDocument();
  });

  it('should hide members that are already assigned to the group in every tab', async () => {
    const user = userEvent.setup();
    mockUseGetAllGroupMembers.mockReturnValue({
      data: {
        totalResults: 4,
        startIndex: 1,
        count: 4,
        members: [
          {id: 'u1', type: 'user'},
          {id: 'a1', type: 'app'},
          {id: 'agent-1', type: 'agent'},
          {id: 'g1', type: 'group'},
        ],
        links: [],
      },
      isLoading: false,
    });

    renderWithProviders(<AddMemberDialog {...defaultProps} groupId="g1" />);

    expect(screen.queryByTestId('user-u1')).not.toBeInTheDocument();
    expect(screen.getByTestId('user-u2')).toBeInTheDocument();

    await user.click(screen.getByText('Apps'));
    expect(screen.queryByTestId('user-a1')).not.toBeInTheDocument();
    expect(screen.getByTestId('user-a2')).toBeInTheDocument();

    await user.click(screen.getByText('Agents'));
    expect(screen.queryByTestId('user-agent-1')).not.toBeInTheDocument();
    expect(screen.getByTestId('user-agent-2')).toBeInTheDocument();

    await user.click(screen.getByText('Groups'));
    expect(screen.queryByTestId('user-g1')).not.toBeInTheDocument();
    expect(screen.getByTestId('user-g2')).toBeInTheDocument();
  });

  it('should block additions and show retry when current members cannot be loaded', async () => {
    const user = userEvent.setup();
    const refetchMembers = vi.fn();
    mockUseGetAllGroupMembers.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Network error'),
      refetch: refetchMembers,
    });

    renderWithProviders(<AddMemberDialog {...defaultProps} groupId="g1" />);

    expect(screen.getByText('Failed to load current group members. Please try again.')).toBeInTheDocument();
    expect(screen.getByText('Add Selected').closest('button')).toBeDisabled();

    await user.click(screen.getByText('Refresh'));
    expect(refetchMembers).toHaveBeenCalledTimes(1);
  });

  it('should render apps in the grid when Apps tab is selected', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByText('Apps'));

    expect(screen.getByTestId('user-a1')).toBeInTheDocument();
    expect(screen.getByTestId('user-a2')).toBeInTheDocument();
  });

  it('should show loading state', () => {
    mockUseGetUsers.mockReturnValue({
      data: null,
      isLoading: true,
    });
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    expect(screen.getByTestId('users-grid')).toHaveAttribute('data-loading', 'true');
  });

  it('should show no results alert when no users', () => {
    mockUseGetUsers.mockReturnValue({
      data: {totalResults: 0, startIndex: 0, count: 0, users: []},
      isLoading: false,
    });
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    expect(screen.getByText('No users found')).toBeInTheDocument();
  });

  it('should show no results alert when no apps', async () => {
    const user = userEvent.setup();
    mockUseGetApplications.mockReturnValue({
      data: {totalResults: 0, count: 0, applications: []},
      isLoading: false,
    });
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByText('Apps'));
    expect(screen.getByText('No apps found')).toBeInTheDocument();
  });

  it('should disable add button when no selection', () => {
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    const addButton = screen.getByText('Add Selected').closest('button');
    expect(addButton).toBeDisabled();
  });

  it('should enable add button after selecting a user', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByTestId('checkbox-u1'));

    const addButton = screen.getByText('Add Selected').closest('button');
    expect(addButton).not.toBeDisabled();
  });

  it('should call onAdd with selected members', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByTestId('checkbox-u1'));
    await user.click(screen.getByText('Add Selected'));

    expect(defaultProps.onAdd).toHaveBeenCalledWith([{id: 'u1', type: 'user'}]);
  });

  it('should call onAdd with selected app members', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByText('Apps'));
    await user.click(screen.getByTestId('checkbox-a1'));
    await user.click(screen.getByText('Add Selected'));

    expect(defaultProps.onAdd).toHaveBeenCalledWith([{id: 'a1', type: 'app'}]);
  });

  it('should render groups in the grid when Groups tab is selected', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByText('Groups'));

    expect(screen.getByTestId('user-g1')).toBeInTheDocument();
    expect(screen.getByTestId('user-g2')).toBeInTheDocument();
    expect(screen.getByTestId('user-g3')).toBeInTheDocument();
  });

  it('should exclude the edited group from the groups grid', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AddMemberDialog {...defaultProps} excludeGroupId="g2" />);

    await user.click(screen.getByText('Groups'));

    expect(screen.queryByTestId('user-g2')).not.toBeInTheDocument();
    expect(screen.getByTestId('user-g1')).toBeInTheDocument();
    expect(screen.getByTestId('user-g3')).toBeInTheDocument();
  });

  it('should call onAdd with selected group members', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByText('Groups'));
    await user.click(screen.getByTestId('checkbox-g1'));
    await user.click(screen.getByText('Add Selected'));

    expect(defaultProps.onAdd).toHaveBeenCalledWith([{id: 'g1', type: 'group'}]);
  });

  it('should show no results alert when no groups', async () => {
    const user = userEvent.setup();
    mockUseGetGroups.mockReturnValue({
      data: {totalResults: 0, startIndex: 0, count: 0, groups: []},
      isLoading: false,
    });
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByText('Groups'));
    expect(screen.getByText('No groups found')).toBeInTheDocument();
  });

  it('should show error alert when groups fetch fails', async () => {
    const user = userEvent.setup();
    mockUseGetGroups.mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error('Network error'),
    });
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByText('Groups'));

    expect(screen.getByText('Failed to load groups. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('No groups found')).not.toBeInTheDocument();
  });

  it('should call onClose when cancel is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    await user.click(screen.getByText('Cancel'));

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('should load all candidate collections with only the active tab enabled', () => {
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    expect(mockUseGetAllCandidates).toHaveBeenCalledWith({path: '/users', itemsKey: 'users', enabled: true});
    expect(mockUseGetAllCandidates).toHaveBeenCalledWith({
      path: '/applications',
      itemsKey: 'applications',
      enabled: false,
    });
    expect(mockUseGetAllCandidates).toHaveBeenCalledWith({path: '/agents', itemsKey: 'agents', enabled: false});
    expect(mockUseGetAllCandidates).toHaveBeenCalledWith({path: '/groups', itemsKey: 'groups', enabled: false});
  });

  it('should show error alert when users fetch fails', () => {
    mockUseGetUsers.mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error('Network error'),
    });
    renderWithProviders(<AddMemberDialog {...defaultProps} />);

    // Resolved through the i18n catalog, not the raw (unlocalized) error message.
    expect(screen.getByText('Failed to load users. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('No users found')).not.toBeInTheDocument();
  });

  it('should render the add mutation error inside the dialog', () => {
    renderWithProviders(<AddMemberDialog {...defaultProps} error="Failed to add member. Please try again." />);

    expect(screen.getByText('Failed to add member. Please try again.')).toBeInTheDocument();
  });

  it('should call onErrorDismiss when the selection changes', async () => {
    const user = userEvent.setup();
    const onErrorDismiss = vi.fn();
    renderWithProviders(
      <AddMemberDialog
        {...defaultProps}
        error="Failed to add member. Please try again."
        onErrorDismiss={onErrorDismiss}
      />,
    );

    await user.click(screen.getByTestId('checkbox-u1'));

    expect(onErrorDismiss).toHaveBeenCalledTimes(1);
  });

  it('should call onErrorDismiss when switching tabs', async () => {
    const user = userEvent.setup();
    const onErrorDismiss = vi.fn();
    renderWithProviders(
      <AddMemberDialog
        {...defaultProps}
        error="Failed to add member. Please try again."
        onErrorDismiss={onErrorDismiss}
      />,
    );

    await user.click(screen.getByText('Apps'));

    expect(onErrorDismiss).toHaveBeenCalledTimes(1);
  });

  it('should disable actions while the add mutation is submitting', () => {
    renderWithProviders(<AddMemberDialog {...defaultProps} isSubmitting />);

    expect(screen.getByText('Cancel').closest('button')).toBeDisabled();
  });

  it('should not close on Escape while the add mutation is submitting', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(<AddMemberDialog {...defaultProps} onClose={onClose} isSubmitting />);

    await user.keyboard('{Escape}');

    expect(onClose).not.toHaveBeenCalled();
  });

  it('should close on Escape once the add mutation has settled', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(<AddMemberDialog {...defaultProps} onClose={onClose} />);

    await user.keyboard('{Escape}');

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
