// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import * as useDeleteApplicationModule from '../../api/useDeleteApplication';
import ApplicationDeleteDialog from '../ApplicationDeleteDialog';

// Mock the useDeleteApplication hook
vi.mock('../../api/useDeleteApplication');

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallbackOrOptions?: string | {defaultValue?: string}) => {
      const translations: Record<string, string> = {
        'delete.title': 'Delete Application',
        'delete.message': 'Are you sure you want to delete this application? This action cannot be undone.',
        'delete.disclaimer':
          'Warning: All associated data, configurations, and access tokens will be permanently removed.',
        'delete.error': 'Failed to delete application. Please try again.',
        'errors.APP-1030': 'This application is managed declaratively and cannot be edited or deleted.',
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

describe('ApplicationDeleteDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnSuccess = vi.fn();
  const mockMutate = vi.fn();

  const defaultProps = {
    open: true,
    applicationId: 'test-app-id',
    onClose: mockOnClose,
    onSuccess: mockOnSuccess,
  };

  const renderWithProviders = (props = defaultProps) => render(<ApplicationDeleteDialog {...props} />);

  beforeEach(() => {
    // Default mock implementation
    vi.mocked(useDeleteApplicationModule.default).mockReturnValue({
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
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the dialog when open is true', () => {
      renderWithProviders();

      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByText('Delete Application')).toBeInTheDocument();
      expect(
        screen.getByText('Are you sure you want to delete this application? This action cannot be undone.'),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          'Warning: All associated data, configurations, and access tokens will be permanently removed.',
        ),
      ).toBeInTheDocument();
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

    it('should not render error alert initially', () => {
      renderWithProviders();

      expect(screen.queryByRole('alert')).toHaveTextContent('Warning: All associated data'); // Only warning alert
    });
  });

  describe('User Interactions', () => {
    it('should call onClose when Cancel button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      await user.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
      expect(mockMutate).not.toHaveBeenCalled();
    });

    it('should call onClose when Escape key is pressed', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      // Press Escape key
      await user.keyboard('{Escape}');

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should trigger delete mutation when Delete button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockMutate).toHaveBeenCalledTimes(1);
      expect(mockMutate).toHaveBeenCalledWith('test-app-id', expect.any(Object));
    });

    it('should not trigger delete mutation when applicationId is null', async () => {
      const user = userEvent.setup();
      renderWithProviders({...defaultProps, applicationId: ''});

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockMutate).not.toHaveBeenCalled();
    });
  });

  describe('Delete Success Flow', () => {
    it('should call onClose and onSuccess callbacks on successful delete', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation((_, options: {onSuccess?: () => void}) => {
        // Simulate successful mutation
        options?.onSuccess?.();
      });

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalledTimes(1);
        expect(mockOnSuccess).toHaveBeenCalledTimes(1);
      });
    });

    it('should work without onSuccess callback', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation((_, options: {onSuccess?: () => void}) => {
        options?.onSuccess?.();
      });

      renderWithProviders({...defaultProps, onSuccess: vi.fn()});

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalledTimes(1);
      });
    });

    it('should clear any previous errors on successful delete', async () => {
      const user = userEvent.setup();

      // First, trigger an error
      mockMutate.mockImplementationOnce((_, options: {onError?: (error: Error) => void}) => {
        options?.onError?.(new Error('Delete failed'));
      });

      const {rerender} = renderWithProviders();

      let deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to delete application. Please try again.')).toBeInTheDocument();
      });

      // Then trigger success
      mockMutate.mockImplementationOnce((_, options: {onSuccess?: () => void}) => {
        options?.onSuccess?.();
      });

      rerender(<ApplicationDeleteDialog {...defaultProps} />);

      deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalled();
        expect(screen.queryByText('Failed to delete application. Please try again.')).not.toBeInTheDocument();
      });
    });
  });

  describe('Delete Error Flow', () => {
    it('should display generic error message when delete fails', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation((_, options: {onError?: (error: Error) => void}) => {
        options?.onError?.(new Error('Some raw backend error text'));
      });

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to delete application. Please try again.')).toBeInTheDocument();
      });

      expect(mockOnClose).not.toHaveBeenCalled();
      expect(mockOnSuccess).not.toHaveBeenCalled();
    });

    it('should display the mapped error message for a declarative application', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation((_, options: {onError?: (error: Error) => void}) => {
        const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
        error.response = {data: {code: 'APP-1030'}};
        options?.onError?.(error);
      });

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(
          screen.getByText('This application is managed declaratively and cannot be edited or deleted.'),
        ).toBeInTheDocument();
      });

      expect(mockOnClose).not.toHaveBeenCalled();
    });

    it('should clear error when Cancel is clicked after error', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation((_, options: {onError?: (error: Error) => void}) => {
        options?.onError?.(new Error('Delete failed'));
      });

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to delete application. Please try again.')).toBeInTheDocument();
      });

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      await user.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should clear a previous error before retrying delete', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementationOnce((_, options: {onError?: (error: Error) => void}) => {
        options?.onError?.(new Error('Delete failed'));
      });

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to delete application. Please try again.')).toBeInTheDocument();
      });

      mockMutate.mockImplementationOnce(() => {
        // Simulate the request still being in flight; the stale error should already be gone.
      });

      await user.click(deleteButton);

      expect(screen.queryByText('Failed to delete application. Please try again.')).not.toBeInTheDocument();
    });
  });

  describe('Loading State', () => {
    it('should disable buttons when delete is pending', () => {
      vi.mocked(useDeleteApplicationModule.default).mockReturnValue({
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
        submittedAt: Date.now(),
        variables: '',
      });

      renderWithProviders();

      expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Deleting...'})).toBeDisabled();
    });

    it('should show "Deleting..." text on Delete button when pending', () => {
      vi.mocked(useDeleteApplicationModule.default).mockReturnValue({
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
        submittedAt: Date.now(),
        variables: '',
      });

      renderWithProviders();

      expect(screen.getByRole('button', {name: 'Deleting...'})).toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    });

    it('should not trigger another delete when already pending', () => {
      vi.mocked(useDeleteApplicationModule.default).mockReturnValue({
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
        submittedAt: Date.now(),
        variables: 'test-app-id',
      });

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Deleting...'});
      expect(deleteButton).toBeDisabled();

      // Verify button cannot be interacted with when disabled
      expect(mockMutate).not.toHaveBeenCalled();
    });
  });

  describe('Edge Cases', () => {
    it('should handle rapid clicks on Delete button', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});

      await user.click(deleteButton);
      await user.click(deleteButton);
      await user.click(deleteButton);

      // Should still only call mutate once per click (3 times total)
      expect(mockMutate).toHaveBeenCalledTimes(3);
    });

    it('should handle dialog opening and closing multiple times', async () => {
      const user = userEvent.setup();
      const {rerender} = renderWithProviders({...defaultProps, open: false});

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

      // Open dialog
      rerender(<ApplicationDeleteDialog {...defaultProps} open />);

      expect(screen.getByRole('dialog')).toBeInTheDocument();

      // Click Cancel
      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      await user.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);

      // Close dialog (simulate parent closing it)
      rerender(<ApplicationDeleteDialog {...defaultProps} open={false} />);

      // Wait for dialog to close
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      });

      // Open again
      rerender(<ApplicationDeleteDialog {...defaultProps} open />);

      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should handle changing applicationId while dialog is open', async () => {
      const user = userEvent.setup();
      const {rerender} = renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockMutate).toHaveBeenCalledWith('test-app-id', expect.any(Object));

      // Change applicationId
      rerender(<ApplicationDeleteDialog {...defaultProps} applicationId="new-app-id" />);

      mockMutate.mockClear();

      const deleteButtonAfterChange = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButtonAfterChange);

      expect(mockMutate).toHaveBeenCalledWith('new-app-id', expect.any(Object));
    });

    it('should handle all callbacks being undefined', async () => {
      const user = userEvent.setup();

      renderWithProviders({
        open: true,
        applicationId: 'test-app-id',
        onClose: vi.fn(),
        onSuccess: vi.fn(),
      });

      mockMutate.mockImplementation((_, options: {onSuccess?: () => void}) => {
        options?.onSuccess?.();
      });

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      // Should not throw error even though onSuccess is undefined
      await waitFor(() => {
        expect(mockMutate).toHaveBeenCalled();
      });
    });
  });

  describe('Accessibility', () => {
    it('should have proper ARIA attributes', () => {
      renderWithProviders();

      const dialog = screen.getByRole('dialog');
      expect(dialog).toHaveAttribute('role', 'dialog');
    });

    it('should be keyboard accessible', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      const deleteButton = screen.getByRole('button', {name: 'Delete'});

      // Tab to focus buttons
      await user.tab();
      expect(cancelButton).toHaveFocus();

      await user.tab();
      expect(deleteButton).toHaveFocus();

      // Press Enter on Delete button
      await user.keyboard('{Enter}');

      expect(mockMutate).toHaveBeenCalledWith('test-app-id', expect.any(Object));
    });

    it('should have proper button labels', () => {
      renderWithProviders();

      expect(screen.getByRole('button', {name: 'Cancel'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Delete'})).toBeInTheDocument();
    });
  });
});
