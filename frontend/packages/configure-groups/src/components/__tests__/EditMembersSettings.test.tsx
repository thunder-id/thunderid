// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {Group} from '../../models/group';
import EditMembersSettings from '../edit-group/members-settings/EditMembersSettings';

vi.mock('../edit-group/members-settings/ManageMembersSection', () => ({
  default: ({
    headerAction,
    onRemoveMember,
  }: {
    groupId: string;
    onRemoveMember: (member: {id: string; type: string}) => void;
    headerAction?: React.ReactNode;
  }) => (
    <div data-testid="manage-members-section">
      {headerAction && <div data-testid="header-action">{headerAction}</div>}
      <button type="button" data-testid="remove-member-btn" onClick={() => onRemoveMember({id: 'u1', type: 'user'})}>
        Remove
      </button>
    </div>
  ),
}));

vi.mock('../edit-group/members-settings/AddMemberDialog', () => ({
  default: ({
    open,
    onClose,
    onAdd,
    error,
  }: {
    open: boolean;
    onClose: () => void;
    onAdd: (members: {id: string; type: string}[]) => void;
    error?: string | null;
    isSubmitting?: boolean;
  }) =>
    open ? (
      <div data-testid="add-member-dialog">
        {error && <div role="alert">{error}</div>}
        <button type="button" onClick={onClose}>
          Close
        </button>
        <button type="button" onClick={() => onAdd([{id: 'u1', type: 'user'}])}>
          Add
        </button>
      </div>
    ) : null,
}));

const mockAddMutate = vi.fn();
vi.mock('../../api/useAddGroupMembers', () => ({
  default: () => ({mutate: mockAddMutate}),
}));

const mockRemoveMutate = vi.fn();
vi.mock('../../api/useRemoveGroupMembers', () => ({
  default: () => ({mutate: mockRemoveMutate}),
}));

describe('EditMembersSettings', () => {
  const mockGroup: Group = {
    id: 'g1',
    name: 'Test Group',
    ouId: 'ou1',
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render ManageMembersSection', () => {
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    expect(screen.getByTestId('manage-members-section')).toBeInTheDocument();
  });

  it('should render Add Member button in header action', () => {
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    expect(screen.getByText('Add Member')).toBeInTheDocument();
  });

  it('should open add member dialog when button clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByText('Add Member'));

    await waitFor(() => {
      expect(screen.getByTestId('add-member-dialog')).toBeInTheDocument();
    });
  });

  it('should call addGroupMembers when members are added', async () => {
    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByText('Add Member'));

    await waitFor(() => {
      expect(screen.getByTestId('add-member-dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Add'));

    expect(mockAddMutate).toHaveBeenCalledWith(
      {groupId: 'g1', members: [{id: 'u1', type: 'user'}]},
      expect.objectContaining({onSuccess: expect.any(Function) as unknown, onError: expect.any(Function) as unknown}),
    );
  });

  it('should close dialog when close is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByText('Add Member'));
    await waitFor(() => {
      expect(screen.getByTestId('add-member-dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Close'));

    await waitFor(() => {
      expect(screen.queryByTestId('add-member-dialog')).not.toBeInTheDocument();
    });
  });

  it('should close dialog on successful add', async () => {
    mockAddMutate.mockImplementation((_data: unknown, opts: {onSuccess: () => void}) => {
      opts.onSuccess();
    });

    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByText('Add Member'));
    await waitFor(() => {
      expect(screen.getByTestId('add-member-dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Add'));

    await waitFor(() => {
      expect(screen.queryByTestId('add-member-dialog')).not.toBeInTheDocument();
    });
  });

  it('should show error when add fails', async () => {
    mockAddMutate.mockImplementation((_data: unknown, opts: {onError: (err: Error) => void}) => {
      opts.onError(new Error('Add failed'));
    });

    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByText('Add Member'));
    await waitFor(() => {
      expect(screen.getByTestId('add-member-dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Add'));

    await waitFor(() => {
      expect(screen.getByText('Failed to add member. Please try again.')).toBeInTheDocument();
    });
  });

  it('should show mapped error message when a selected member no longer exists', async () => {
    mockAddMutate.mockImplementation((_data: unknown, opts: {onError: (err: Error) => void}) => {
      const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
      error.response = {data: {code: 'GRP-1007'}};
      opts.onError(error);
    });

    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByText('Add Member'));
    await waitFor(() => {
      expect(screen.getByTestId('add-member-dialog')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Add'));

    await waitFor(() => {
      expect(
        screen.getByText('One or more selected members no longer exist. Refresh and try again.'),
      ).toBeInTheDocument();
    });
  });

  it('should call removeGroupMembers when remove button is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByTestId('remove-member-btn'));

    expect(mockRemoveMutate).toHaveBeenCalledWith(
      {groupId: 'g1', member: {id: 'u1', type: 'user'}},
      expect.objectContaining({onSuccess: expect.any(Function) as unknown, onError: expect.any(Function) as unknown}),
    );
  });

  it('should clear the remove error on a successful remove retry', async () => {
    mockRemoveMutate.mockImplementationOnce((_data: unknown, opts: {onError: (err: Error) => void}) => {
      opts.onError(new Error('Some error'));
    });

    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByTestId('remove-member-btn'));
    await waitFor(() => {
      expect(screen.getByText('Failed to remove member. Please try again.')).toBeInTheDocument();
    });

    mockRemoveMutate.mockImplementation((_data: unknown, opts: {onSuccess: () => void}) => {
      opts.onSuccess();
    });

    await user.click(screen.getByTestId('remove-member-btn'));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  it('should not clear an unrelated add error still shown in the open dialog on a successful remove', async () => {
    mockAddMutate.mockImplementation((_data: unknown, opts: {onError: (err: Error) => void}) => {
      opts.onError(new Error('Some error'));
    });
    mockRemoveMutate.mockImplementation((_data: unknown, opts: {onSuccess: () => void}) => {
      opts.onSuccess();
    });

    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByText('Add Member'));
    await waitFor(() => {
      expect(screen.getByTestId('add-member-dialog')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Add'));
    await waitFor(() => {
      expect(screen.getByText('Failed to add member. Please try again.')).toBeInTheDocument();
    });

    await user.click(screen.getByTestId('remove-member-btn'));

    await waitFor(() => {
      expect(mockRemoveMutate).toHaveBeenCalled();
    });
    // The add dialog's own error is unrelated to the remove action and stays visible.
    expect(screen.getByText('Failed to add member. Please try again.')).toBeInTheDocument();
  });

  it('should show error when remove fails', async () => {
    mockRemoveMutate.mockImplementation((_data: unknown, opts: {onError: (err: Error) => void}) => {
      opts.onError(new Error('Remove failed'));
    });

    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByTestId('remove-member-btn'));

    await waitFor(() => {
      expect(screen.getByText('Failed to remove member. Please try again.')).toBeInTheDocument();
    });
  });

  it('should close error alert when dismiss is clicked', async () => {
    mockRemoveMutate.mockImplementation((_data: unknown, opts: {onError: (err: Error) => void}) => {
      opts.onError(new Error('Remove failed'));
    });

    const user = userEvent.setup();
    renderWithProviders(<EditMembersSettings group={mockGroup} />);

    await user.click(screen.getByTestId('remove-member-btn'));
    await waitFor(() => {
      expect(screen.getByText('Failed to remove member. Please try again.')).toBeInTheDocument();
    });

    const closeAlertButton = screen.getByRole('button', {name: /close/i});
    await user.click(closeAlertButton);

    await waitFor(() => {
      expect(screen.queryByText('Failed to remove member. Please try again.')).not.toBeInTheDocument();
    });
  });
});
