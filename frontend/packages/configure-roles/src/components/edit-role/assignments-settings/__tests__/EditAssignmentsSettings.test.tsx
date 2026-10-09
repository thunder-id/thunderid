// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import * as useAddRoleAssignmentsModule from '../../../../api/useAddRoleAssignments';
import * as useRemoveRoleAssignmentsModule from '../../../../api/useRemoveRoleAssignments';
import type {RoleAssignment} from '../../../../models/role';
import EditAssignmentsSettings from '../EditAssignmentsSettings';

vi.mock('../../../../api/useAddRoleAssignments');
vi.mock('../../../../api/useRemoveRoleAssignments');

vi.mock('../ManageAssignmentsSection', () => ({
  default: ({
    onRemoveAssignment,
    headerAction,
    activeAssignmentTab,
    onAssignmentTabChange,
  }: {
    roleId: string;
    onRemoveAssignment: (a: RoleAssignment) => void;
    headerAction?: React.ReactNode;
    activeAssignmentTab: number;
    onAssignmentTabChange: (tab: number) => void;
  }) => (
    <div data-testid="manage-section">
      {headerAction}
      <button type="button" onClick={() => onRemoveAssignment({id: 'user-1', type: 'user'})} data-testid="remove-btn">
        Remove
      </button>
      <button type="button" onClick={() => onAssignmentTabChange(1)} data-testid="switch-tab-btn">
        Switch Tab
      </button>
      <span data-testid="active-tab">{activeAssignmentTab}</span>
    </div>
  ),
}));

vi.mock('../AddAssignmentDialog', () => ({
  default: ({
    open,
    onClose,
    onAdd,
    initialTab,
    error,
  }: {
    open: boolean;
    roleId: string;
    onClose: () => void;
    onAdd: (assignments: RoleAssignment[]) => void;
    initialTab?: number;
    error?: string | null;
    isSubmitting?: boolean;
  }) =>
    open ? (
      <div data-testid="add-dialog" role="dialog">
        <span data-testid="initial-tab">{initialTab}</span>
        {error && <div role="alert">{error}</div>}
        <button type="button" onClick={() => onAdd([{id: 'user-3', type: 'user'}])} data-testid="confirm-add">
          Confirm Add
        </button>
        <button type="button" onClick={onClose} data-testid="close-dialog">
          Cancel
        </button>
      </div>
    ) : null,
}));

describe('EditAssignmentsSettings', () => {
  const mockAddMutate = vi.fn();
  const mockRemoveMutate = vi.fn();

  const baseMutationState = {
    isPending: false,
    isError: false,
    isSuccess: false,
    error: null,
    data: undefined,
    mutateAsync: vi.fn(),
    reset: vi.fn(),
    context: undefined,
    failureCount: 0,
    failureReason: null,
    isIdle: true,
    isPaused: false,
    status: 'idle' as const,
    submittedAt: 0,
    variables: undefined,
  };

  const renderComponent = () => render(<EditAssignmentsSettings roleId="role-1" />);

  beforeEach(() => {
    vi.mocked(useAddRoleAssignmentsModule.default).mockReturnValue({
      ...baseMutationState,
      mutate: mockAddMutate,
    } as unknown as ReturnType<typeof useAddRoleAssignmentsModule.default>);

    vi.mocked(useRemoveRoleAssignmentsModule.default).mockReturnValue({
      ...baseMutationState,
      mutate: mockRemoveMutate,
    } as unknown as ReturnType<typeof useRemoveRoleAssignmentsModule.default>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render ManageAssignmentsSection', () => {
      renderComponent();

      expect(screen.getByTestId('manage-section')).toBeInTheDocument();
    });

    it('should render Add Assignment button in headerAction', () => {
      renderComponent();

      expect(screen.getByRole('button', {name: 'Add'})).toBeInTheDocument();
    });

    it('should not render AddAssignmentDialog initially', () => {
      renderComponent();

      expect(screen.queryByTestId('add-dialog')).not.toBeInTheDocument();
    });

    it('should not render error alert initially', () => {
      renderComponent();

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  describe('Add Dialog Interactions', () => {
    it('should open AddAssignmentDialog when Add Assignment button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Add'}));

      expect(screen.getByTestId('add-dialog')).toBeInTheDocument();
    });

    it('should pass activeAssignmentTab as initialTab to dialog', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByTestId('switch-tab-btn'));
      await user.click(screen.getByRole('button', {name: 'Add'}));

      expect(screen.getByTestId('initial-tab')).toHaveTextContent('1');
    });

    it('should call addRoleAssignments.mutate when dialog confirms', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Add'}));
      await user.click(screen.getByTestId('confirm-add'));

      expect(mockAddMutate).toHaveBeenCalledTimes(1);
      expect(mockAddMutate).toHaveBeenCalledWith(
        {roleId: 'role-1', assignments: [{id: 'user-3', type: 'user'}]},
        expect.any(Object),
      );
    });

    it('should close dialog on successful add', async () => {
      const user = userEvent.setup();
      mockAddMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onSuccess?.();
        },
      );

      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Add'}));
      await user.click(screen.getByTestId('confirm-add'));

      await waitFor(() => {
        expect(screen.queryByTestId('add-dialog')).not.toBeInTheDocument();
      });
    });

    it('should show error alert on add failure', async () => {
      const user = userEvent.setup();
      mockAddMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onError?.(new Error('Network error'));
        },
      );

      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Add'}));
      await user.click(screen.getByTestId('confirm-add'));

      await waitFor(() => {
        expect(screen.getByText('Failed to add assignment. Please try again.')).toBeInTheDocument();
      });
    });

    it('should show mapped error message when a selected assignee no longer exists', async () => {
      const user = userEvent.setup();
      mockAddMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
          error.response = {data: {code: 'ROL-1007'}};
          options?.onError?.(error);
        },
      );

      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Add'}));
      await user.click(screen.getByTestId('confirm-add'));

      await waitFor(() => {
        expect(
          screen.getByText('One or more selected assignees no longer exist. Refresh and try again.'),
        ).toBeInTheDocument();
      });
    });

    it('should close dialog when Cancel is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Add'}));
      expect(screen.getByTestId('add-dialog')).toBeInTheDocument();

      await user.click(screen.getByTestId('close-dialog'));

      await waitFor(() => {
        expect(screen.queryByTestId('add-dialog')).not.toBeInTheDocument();
      });
    });
  });

  describe('Remove Assignment', () => {
    it('should call removeRoleAssignments.mutate when remove is triggered', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByTestId('remove-btn'));

      expect(mockRemoveMutate).toHaveBeenCalledTimes(1);
      expect(mockRemoveMutate).toHaveBeenCalledWith(
        {roleId: 'role-1', assignment: {id: 'user-1', type: 'user'}},
        expect.any(Object),
      );
    });

    it('should clear the remove error on a successful remove retry', async () => {
      const user = userEvent.setup();

      mockRemoveMutate.mockImplementationOnce(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onError?.(new Error('Some error'));
        },
      );

      renderComponent();

      await user.click(screen.getByTestId('remove-btn'));

      await waitFor(() => {
        expect(screen.getByText('Failed to remove assignment. Please try again.')).toBeInTheDocument();
      });

      mockRemoveMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onSuccess?.();
        },
      );

      await user.click(screen.getByTestId('remove-btn'));

      await waitFor(() => {
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      });
    });

    it('should not clear an unrelated add error still shown in the open dialog on a successful remove', async () => {
      const user = userEvent.setup();

      mockAddMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onError?.(new Error('Some error'));
        },
      );
      mockRemoveMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onSuccess?.();
        },
      );

      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Add'}));
      await user.click(screen.getByTestId('confirm-add'));

      await waitFor(() => {
        expect(screen.getByText('Failed to add assignment. Please try again.')).toBeInTheDocument();
      });

      await user.click(screen.getByTestId('remove-btn'));

      await waitFor(() => {
        expect(mockRemoveMutate).toHaveBeenCalled();
      });
      // The add dialog's own error is unrelated to the remove action and stays visible.
      expect(screen.getByText('Failed to add assignment. Please try again.')).toBeInTheDocument();
    });

    it('should show error alert on remove failure', async () => {
      const user = userEvent.setup();
      mockRemoveMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onError?.(new Error('Remove failed'));
        },
      );

      renderComponent();

      await user.click(screen.getByTestId('remove-btn'));

      await waitFor(() => {
        expect(screen.getByText('Failed to remove assignment. Please try again.')).toBeInTheDocument();
      });
    });

    it('should show mapped error message when the role no longer exists on remove', async () => {
      const user = userEvent.setup();
      mockRemoveMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
          error.response = {data: {code: 'ROL-1003'}};
          options?.onError?.(error);
        },
      );

      renderComponent();

      await user.click(screen.getByTestId('remove-btn'));

      await waitFor(() => {
        expect(screen.getByText('This role no longer exists. It may have already been deleted.')).toBeInTheDocument();
      });
    });
  });

  describe('Error Handling', () => {
    it('should clear error alert when close button is clicked', async () => {
      const user = userEvent.setup();
      mockRemoveMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onError?.(new Error('Some error'));
        },
      );

      renderComponent();

      await user.click(screen.getByTestId('remove-btn'));

      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
      });

      const closeButton = screen.getByRole('alert').querySelector('button');
      if (closeButton) {
        await user.click(closeButton);
      }

      await waitFor(() => {
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      });
    });

    it('should clear the add error and close the dialog on a successful add', async () => {
      const user = userEvent.setup();

      mockAddMutate.mockImplementationOnce(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onError?.(new Error('Previous error'));
        },
      );

      renderComponent();

      await user.click(screen.getByRole('button', {name: 'Add'}));
      await user.click(screen.getByTestId('confirm-add'));

      await waitFor(() => {
        expect(screen.getByText('Failed to add assignment. Please try again.')).toBeInTheDocument();
      });

      mockAddMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onSuccess?.();
        },
      );

      await user.click(screen.getByTestId('confirm-add'));

      await waitFor(() => {
        expect(screen.queryByTestId('add-dialog')).not.toBeInTheDocument();
      });
    });

    it('should not clear an unrelated remove error on a successful add', async () => {
      const user = userEvent.setup();

      mockRemoveMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onError?.(new Error('Previous error'));
        },
      );
      mockAddMutate.mockImplementation(
        (_variables: unknown, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onSuccess?.();
        },
      );

      renderComponent();

      await user.click(screen.getByTestId('remove-btn'));

      await waitFor(() => {
        expect(screen.getByText('Failed to remove assignment. Please try again.')).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', {name: 'Add'}));
      await user.click(screen.getByTestId('confirm-add'));

      await waitFor(() => {
        expect(screen.queryByTestId('add-dialog')).not.toBeInTheDocument();
      });
      // The tab's remove error is unrelated to the add action and stays visible.
      expect(screen.getByText('Failed to remove assignment. Please try again.')).toBeInTheDocument();
    });
  });
});
