// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowDeleteDialog from '../FlowDeleteDialog';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, options?: string | Record<string, unknown>) => {
      const translations: Record<string, string> = {
        'flows:delete.title': 'Delete Flow',
        'flows:delete.message': 'Are you sure you want to delete this flow?',
        'flows:delete.disclaimer': 'This action cannot be undone.',
        'flows:delete.error': 'Failed to delete flow',
        'common:actions.cancel': 'Cancel',
        'common:actions.delete': 'Delete',
        'common:status.deleting': 'Deleting...',
      };
      if (translations[key] !== undefined) return translations[key];
      if (typeof options === 'string') return options || key;
      if (options && typeof options === 'object' && 'defaultValue' in options) {
        return (options['defaultValue'] as string) || '';
      }
      return key;
    },
  }),
}));

// Mock useDeleteFlow hook
const mockMutate = vi.fn();
const mockReset = vi.fn();
const mockDeleteFlow = {
  mutate: mockMutate,
  reset: mockReset,
  isPending: false,
  isError: false,
};

vi.mock('../../api/useDeleteFlow', () => ({
  default: () => mockDeleteFlow,
}));

// Mock useGetFlowUsages hook
vi.mock('../../api/useGetFlowUsages', () => ({
  default: () => ({
    data: undefined,
    isLoading: false,
  }),
}));

describe('FlowDeleteDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnSuccess = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockDeleteFlow.isPending = false;
    mockDeleteFlow.isError = false;
  });

  describe('Dialog Visibility', () => {
    it('should render dialog when open is true', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should not render dialog when open is false', () => {
      render(<FlowDeleteDialog open={false} flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  describe('Dialog Content', () => {
    it('should display title', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.getByText('Delete Flow')).toBeInTheDocument();
    });

    it('should display confirmation message', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.getByText('Are you sure you want to delete this flow?')).toBeInTheDocument();
    });

    it('should display warning disclaimer', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.getByText('This action cannot be undone.')).toBeInTheDocument();
    });

    it('should display warning alert', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      const alert = screen.getByRole('alert');
      expect(alert).toBeInTheDocument();
    });
  });

  describe('Cancel Button', () => {
    it('should render cancel button', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.getByRole('button', {name: 'Cancel'})).toBeInTheDocument();
    });

    it('should call onClose when cancel button is clicked', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      fireEvent.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('Delete Button', () => {
    it('should render delete button', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.getByRole('button', {name: 'Delete'})).toBeInTheDocument();
    });

    it('should call mutate with flowId when delete button is clicked', () => {
      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      fireEvent.click(deleteButton);

      expect(mockMutate).toHaveBeenCalledWith('flow-123', expect.any(Object));
    });

    it('should not call mutate when flowId is null', () => {
      render(<FlowDeleteDialog open flowId={null} onClose={mockOnClose} />);

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      fireEvent.click(deleteButton);

      expect(mockMutate).not.toHaveBeenCalled();
    });

    it('should show deleting text when isPending is true', () => {
      mockDeleteFlow.isPending = true;

      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.getByRole('button', {name: 'Deleting...'})).toBeInTheDocument();
    });

    it('should disable buttons when isPending is true', () => {
      mockDeleteFlow.isPending = true;

      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Deleting...'})).toBeDisabled();
    });
  });

  describe('Success Callback', () => {
    it('should call onClose and onSuccess on successful deletion', async () => {
      mockMutate.mockImplementation((_flowId: string, options: {onSuccess: () => void}) => {
        options.onSuccess();
      });

      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} onSuccess={mockOnSuccess} />);

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalled();
        expect(mockOnSuccess).toHaveBeenCalled();
      });
    });

    it('should work without onSuccess callback', async () => {
      mockMutate.mockImplementation((_flowId: string, options: {onSuccess: () => void}) => {
        options.onSuccess();
      });

      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalled();
      });
    });
  });

  describe('Error Handling', () => {
    it('should display a resolved error message on deletion failure, never the raw server text', async () => {
      mockMutate.mockImplementation((_flowId: string, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Network error'));
      });

      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to delete flow')).toBeInTheDocument();
        expect(screen.queryByText('Network error')).not.toBeInTheDocument();
      });
    });

    it('should display default error message when error has no message', async () => {
      mockMutate.mockImplementation((_flowId: string, options: {onError: (err: Record<string, unknown>) => void}) => {
        options.onError({});
      });

      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to delete flow')).toBeInTheDocument();
      });
    });

    it('should clear error and reset the mutation when dialog is cancelled', async () => {
      mockMutate.mockImplementation((_flowId: string, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Network error'));
      });

      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      // Trigger error
      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText('Failed to delete flow')).toBeInTheDocument();
      });

      // Simulate the mutation now being in an errored state, as it would be after onError fires.
      mockDeleteFlow.isError = true;

      // Cancel should clear the error and reset the now-errored mutation
      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      fireEvent.click(cancelButton);

      expect(mockReset).toHaveBeenCalledTimes(1);
      expect(mockOnClose).toHaveBeenCalled();
    });

    it('should not reset the mutation on cancel while a delete is still pending', () => {
      mockDeleteFlow.isPending = true;
      mockDeleteFlow.isError = true;

      render(<FlowDeleteDialog open flowId="flow-123" onClose={mockOnClose} />);

      // The cancel button is disabled while pending, but the underlying handler also guards on
      // isPending directly: resetting a still-pending mutation would flip isPending back to false
      // before the in-flight request settles.
      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      fireEvent.click(cancelButton);

      expect(mockReset).not.toHaveBeenCalled();
      expect(mockOnClose).not.toHaveBeenCalled();
    });
  });
});
