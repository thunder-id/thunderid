// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import * as useDeleteRoleModule from '../../api/useDeleteRole';
import RoleDeleteDialog from '../RoleDeleteDialog';
import type {RoleDeleteDialogProps} from '../RoleDeleteDialog';

// Mock the useDeleteRole hook
vi.mock('../../api/useDeleteRole');

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallbackOrOptions?: string | {defaultValue?: string}) => {
      const translations: Record<string, string> = {
        'delete.title': 'Delete Role',
        'delete.message': 'Are you sure you want to delete this role?',
        'delete.disclaimer':
          'This action cannot be undone. All role assignments and permissions will be permanently removed.',
        'delete.error': 'Failed to delete role. Please try again.',
        'errors.ROL-1013': 'This role is managed declaratively and cannot be edited or deleted.',
        'errors.ROL-1039':
          'A user share still mandates this role. Remove the role from the user shares that mandate it, then delete it again.',
        'common:actions.cancel': 'Cancel',
        'common:actions.delete': 'Delete',
        'common:status.deleting': 'Deleting...',
      };
      if (translations[key] !== undefined) return translations[key];
      if (typeof fallbackOrOptions === 'string') return fallbackOrOptions;
      if (fallbackOrOptions && 'defaultValue' in fallbackOrOptions) return fallbackOrOptions.defaultValue ?? key;
      return key;
    },
  }),
}));

describe('RoleDeleteDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnSuccess = vi.fn();
  const mockMutate = vi.fn();

  const defaultProps: RoleDeleteDialogProps = {
    open: true,
    roleId: 'test-role-id',
    onClose: mockOnClose,
    onSuccess: mockOnSuccess,
  };

  const renderWithProviders = (props = defaultProps) => render(<RoleDeleteDialog {...props} />);

  beforeEach(() => {
    vi.mocked(useDeleteRoleModule.default).mockReturnValue({
      mutate: mockMutate,
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
      status: 'idle',
      submittedAt: 0,
      variables: undefined,
    } as unknown as ReturnType<typeof useDeleteRoleModule.default>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the dialog when open is true', () => {
      renderWithProviders();

      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByText('Delete Role')).toBeInTheDocument();
      expect(screen.getByText('Are you sure you want to delete this role?')).toBeInTheDocument();
    });

    it('should not render dialog content when open is false', () => {
      renderWithProviders({...defaultProps, open: false});

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('should render Cancel and Delete buttons', () => {
      renderWithProviders();

      expect(screen.getByRole('button', {name: 'Cancel'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Delete'})).toBeInTheDocument();
    });

    it('should render warning alert with disclaimer', () => {
      renderWithProviders();

      expect(
        screen.getByText(
          'This action cannot be undone. All role assignments and permissions will be permanently removed.',
        ),
      ).toBeInTheDocument();
    });
  });

  describe('User Interactions', () => {
    it('should call onClose when Cancel button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      await user.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should call onClose when Escape key is pressed', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      await user.keyboard('{Escape}');

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should trigger delete mutation when Delete button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockMutate).toHaveBeenCalledTimes(1);
      expect(mockMutate).toHaveBeenCalledWith('test-role-id', expect.any(Object));
    });

    it('should not trigger delete mutation when roleId is null', () => {
      renderWithProviders({...defaultProps, roleId: null});

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      fireEvent.click(deleteButton);

      expect(mockMutate).not.toHaveBeenCalled();
    });
  });

  describe('Delete Success Flow', () => {
    it('should call onClose and onSuccess callbacks on successful delete', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation(
        (_roleId: string, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onSuccess?.();
        },
      );

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
      expect(mockOnSuccess).toHaveBeenCalledTimes(1);
    });

    it('should work without onSuccess callback', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation(
        (_roleId: string, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onSuccess?.();
        },
      );

      renderWithProviders({...defaultProps, onSuccess: undefined});

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('Delete Error Flow', () => {
    it('should display generic error message when delete fails', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation(
        (_roleId: string, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          options?.onError?.(new Error('Failed to delete role'));
        },
      );

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to delete role. Please try again.')).toBeInTheDocument();
      });

      expect(mockOnClose).not.toHaveBeenCalled();
    });

    it('should tell the user to remove a role from the shares that mandate it', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation(
        (_roleId: string, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
          error.response = {data: {code: 'ROL-1039'}};
          options?.onError?.(error);
        },
      );

      renderWithProviders();

      await user.click(screen.getByRole('button', {name: 'Delete'}));

      await waitFor(() => {
        expect(
          screen.getByText(
            'A user share still mandates this role. Remove the role from the user shares that mandate it, then delete it again.',
          ),
        ).toBeInTheDocument();
      });
      expect(mockOnClose).not.toHaveBeenCalled();
    });

    it('should display mapped error message for a declarative role', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation(
        (_roleId: string, options: {onSuccess?: () => void; onError?: (error: Error) => void}) => {
          const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
          error.response = {data: {code: 'ROL-1013'}};
          options?.onError?.(error);
        },
      );

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(
          screen.getByText('This role is managed declaratively and cannot be edited or deleted.'),
        ).toBeInTheDocument();
      });

      expect(mockOnClose).not.toHaveBeenCalled();
    });
  });

  describe('Loading State', () => {
    it('should disable buttons when delete is pending', () => {
      vi.mocked(useDeleteRoleModule.default).mockReturnValue({
        mutate: mockMutate,
        isPending: true,
        isError: false,
        isSuccess: false,
        error: null,
        data: undefined,
        mutateAsync: vi.fn(),
        reset: vi.fn(),
        context: undefined,
        failureCount: 0,
        failureReason: null,
        isIdle: false,
        isPaused: false,
        status: 'pending',
        submittedAt: 0,
        variables: undefined,
      } as unknown as ReturnType<typeof useDeleteRoleModule.default>);

      renderWithProviders();

      expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Deleting...'})).toBeDisabled();
    });

    it('should show "Deleting..." text on Delete button when pending', () => {
      vi.mocked(useDeleteRoleModule.default).mockReturnValue({
        mutate: mockMutate,
        isPending: true,
        isError: false,
        isSuccess: false,
        error: null,
        data: undefined,
        mutateAsync: vi.fn(),
        reset: vi.fn(),
        context: undefined,
        failureCount: 0,
        failureReason: null,
        isIdle: false,
        isPaused: false,
        status: 'pending',
        submittedAt: 0,
        variables: undefined,
      } as unknown as ReturnType<typeof useDeleteRoleModule.default>);

      renderWithProviders();

      expect(screen.getByText('Deleting...')).toBeInTheDocument();
    });

    it('should not close dialog when Cancel is clicked during pending', () => {
      vi.mocked(useDeleteRoleModule.default).mockReturnValue({
        mutate: mockMutate,
        isPending: true,
        isError: false,
        isSuccess: false,
        error: null,
        data: undefined,
        mutateAsync: vi.fn(),
        reset: vi.fn(),
        context: undefined,
        failureCount: 0,
        failureReason: null,
        isIdle: false,
        isPaused: false,
        status: 'pending',
        submittedAt: 0,
        variables: undefined,
      } as unknown as ReturnType<typeof useDeleteRoleModule.default>);

      renderWithProviders();

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      fireEvent.click(cancelButton);

      expect(mockOnClose).not.toHaveBeenCalled();
    });
  });

  describe('Edge Cases', () => {
    it('should disable Delete button when roleId is null', () => {
      renderWithProviders({...defaultProps, roleId: null});

      expect(screen.getByRole('button', {name: 'Delete'})).toBeDisabled();
    });
  });
});
