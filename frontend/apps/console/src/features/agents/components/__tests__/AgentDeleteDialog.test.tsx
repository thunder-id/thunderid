/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import * as useDeleteAgentModule from '../../api/useDeleteAgent';
import AgentDeleteDialog, {type AgentDeleteDialogProps} from '../AgentDeleteDialog';

// Mock the useDeleteAgent hook
vi.mock('../../api/useDeleteAgent');

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | {defaultValue?: string}) => {
      const translations: Record<string, string> = {
        'agents:delete.title': 'Delete agent',
        'agents:delete.message': 'Are you sure you want to delete this agent? This action cannot be undone.',
        'agents:delete.disclaimer': 'Deleting this agent will revoke all its credentials and access tokens.',
        'common:actions.cancel': 'Cancel',
        'common:actions.delete': 'Delete',
        'common:status.deleting': 'Deleting...',
      };
      if (translations[key]) return translations[key];
      if (typeof fallback === 'string') return fallback || key;
      if (fallback && typeof fallback === 'object') return fallback.defaultValue ?? key;
      return key;
    },
  }),
}));

describe('AgentDeleteDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnSuccess = vi.fn();
  const mockMutate = vi.fn();

  const defaultProps: AgentDeleteDialogProps = {
    open: true,
    agentId: 'test-agent-id',
    onClose: mockOnClose,
    onSuccess: mockOnSuccess,
  };

  const renderWithProviders = (props: AgentDeleteDialogProps = defaultProps) =>
    render(<AgentDeleteDialog {...props} />);

  beforeEach(() => {
    vi.mocked(useDeleteAgentModule.default).mockReturnValue({
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
      expect(screen.getByText('Delete agent')).toBeInTheDocument();
      expect(
        screen.getByText('Are you sure you want to delete this agent? This action cannot be undone.'),
      ).toBeInTheDocument();
      expect(
        screen.getByText('Deleting this agent will revoke all its credentials and access tokens.'),
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

      await user.keyboard('{Escape}');

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should trigger delete mutation when Delete button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockMutate).toHaveBeenCalledTimes(1);
      expect(mockMutate).toHaveBeenCalledWith('test-agent-id', expect.any(Object));
    });

    it('should not trigger delete mutation when agentId is null', async () => {
      const user = userEvent.setup();
      renderWithProviders({...defaultProps, agentId: null});

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockMutate).not.toHaveBeenCalled();
    });
  });

  describe('Delete Success Flow', () => {
    it('should call onClose and onSuccess callbacks on successful delete', async () => {
      const user = userEvent.setup();

      mockMutate.mockImplementation((_, options: {onSuccess?: () => void}) => {
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

      renderWithProviders({open: true, agentId: 'test-agent-id', onClose: mockOnClose});

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe('Delete Error Flow', () => {
    it('should display error message when delete fails', async () => {
      const user = userEvent.setup();
      const errorMessage = 'Failed to delete agent';

      mockMutate.mockImplementation((_, options: {onError?: (error: Error) => void}) => {
        options?.onError?.(new Error(errorMessage));
      });

      renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText(errorMessage)).toBeInTheDocument();
      });

      expect(mockOnClose).not.toHaveBeenCalled();
      expect(mockOnSuccess).not.toHaveBeenCalled();
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
        expect(screen.getByText('Delete failed')).toBeInTheDocument();
      });

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      await user.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('Loading State', () => {
    it('should disable buttons when delete is pending', () => {
      vi.mocked(useDeleteAgentModule.default).mockReturnValue({
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
        variables: 'test-agent-id',
      });

      renderWithProviders();

      expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Deleting...'})).toBeDisabled();
    });

    it('should show "Deleting..." text on Delete button when pending', () => {
      vi.mocked(useDeleteAgentModule.default).mockReturnValue({
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
        variables: 'test-agent-id',
      });

      renderWithProviders();

      expect(screen.getByRole('button', {name: 'Deleting...'})).toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    });

    it('should not call onClose via Cancel when pending', async () => {
      const user = userEvent.setup();
      vi.mocked(useDeleteAgentModule.default).mockReturnValue({
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
        variables: 'test-agent-id',
      });

      renderWithProviders();

      const cancelButton = screen.getByRole('button', {name: 'Cancel'});
      // The cancel button is disabled — clicking it should not invoke onClose
      await user.click(cancelButton).catch(() => null);

      expect(mockOnClose).not.toHaveBeenCalled();
    });
  });

  describe('Edge Cases', () => {
    it('should handle changing agentId while dialog is open', async () => {
      const user = userEvent.setup();
      const {rerender} = renderWithProviders();

      const deleteButton = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButton);

      expect(mockMutate).toHaveBeenCalledWith('test-agent-id', expect.any(Object));

      rerender(<AgentDeleteDialog {...defaultProps} agentId="new-agent-id" />);

      mockMutate.mockClear();

      const deleteButtonAfterChange = screen.getByRole('button', {name: 'Delete'});
      await user.click(deleteButtonAfterChange);

      expect(mockMutate).toHaveBeenCalledWith('new-agent-id', expect.any(Object));
    });
  });
});
