// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {UserUsagesResponse} from '../../models/users';
import UserDeleteDialog from '../UserDeleteDialog';

// Mock react-i18next. Resolves inline defaults (string or {defaultValue}) like the real i18n.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, arg?: unknown): string => {
      const translations: Record<string, string> = {
        'delete.title': 'Delete User',
        'delete.message': 'Are you sure you want to delete this user?',
        'delete.disclaimer': 'All associated data will be permanently removed.',
        'delete.usages.loading': 'Checking affected resources…',
        'delete.usages.title': 'The following agents list this user as their owner:',
        'common:actions.cancel': 'Cancel',
        'common:actions.delete': 'Delete',
        'common:status.deleting': 'Deleting...',
      };
      if (translations[key]) return translations[key];
      if (typeof arg === 'string') return arg;
      if (arg && typeof arg === 'object') {
        const obj = arg as {defaultValue?: string; count?: number; dependencies?: string};
        if (obj.defaultValue) {
          return obj.defaultValue
            .replace('{{count}}', String(obj.count ?? ''))
            .replace('{{dependencies}}', obj.dependencies ?? '');
        }
      }
      return key;
    },
  }),
}));

// Mock useDeleteUser hook.
const mockMutate = vi.fn();
const mockReset = vi.fn();
const mockDeleteUser = {
  mutate: mockMutate,
  reset: mockReset,
  isPending: false,
};
vi.mock('../../api/useDeleteUser', () => ({
  default: () => mockDeleteUser,
}));

// Mock useGetUserUsages hook.
const {getUsagesMock} = vi.hoisted(() => ({
  getUsagesMock: vi.fn<() => {data: UserUsagesResponse | undefined; isLoading: boolean}>(),
}));
vi.mock('../../api/useGetUserUsages', () => ({
  default: () => getUsagesMock(),
}));

describe('UserDeleteDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnSuccess = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockDeleteUser.isPending = false;
    getUsagesMock.mockReturnValue({data: undefined, isLoading: false});
  });

  it('should render dialog when open is true', () => {
    render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Delete User')).toBeInTheDocument();
  });

  it('should not render dialog when open is false', () => {
    render(<UserDeleteDialog open={false} userId="user-123" onClose={mockOnClose} />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('should call mutate with userId when delete button is clicked', () => {
    render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

    fireEvent.click(screen.getByRole('button', {name: 'Delete'}));

    expect(mockMutate).toHaveBeenCalledWith('user-123', expect.any(Object));
  });

  it('should call onClose when cancel button is clicked', () => {
    render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  describe('Usages', () => {
    it('should show a loading alert while usages are being fetched', () => {
      getUsagesMock.mockReturnValue({data: undefined, isLoading: true});

      render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

      expect(screen.getByText('Checking affected resources…')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Delete'})).toBeDisabled();
    });

    it('should list blocking agents and disable delete when the user owns agents', () => {
      const usages: UserUsagesResponse = {
        totalResults: 2,
        count: 2,
        summary: {agent: 2},
        usages: [
          {resourceType: 'agent', id: 'agent-1', displayName: 'Support Agent', behaviorOnDelete: 'restrict'},
          {resourceType: 'agent', id: 'agent-2', displayName: 'Billing Agent', behaviorOnDelete: 'restrict'},
        ],
      };
      getUsagesMock.mockReturnValue({data: usages, isLoading: false});

      render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

      expect(
        screen.getByText('This user cannot be deleted until the following agents are reassigned or removed:'),
      ).toBeInTheDocument();
      expect(screen.getByText('Support Agent')).toBeInTheDocument();
      expect(screen.getByText('Billing Agent')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Delete'})).toBeDisabled();
    });

    it('should show a "+N more" row when blocking usages exceed the visible limit', () => {
      const usages: UserUsagesResponse = {
        totalResults: 7,
        count: 7,
        summary: {agent: 7},
        usages: Array.from({length: 7}, (_, i) => ({
          resourceType: 'agent',
          id: `agent-${i}`,
          displayName: `Agent ${i}`,
          behaviorOnDelete: 'restrict' as const,
        })),
      };
      getUsagesMock.mockReturnValue({data: usages, isLoading: false});

      render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

      expect(screen.getByText('+2 more')).toBeInTheDocument();
    });

    it('should omit the usages alert and allow deletion when there are no associated agents', () => {
      getUsagesMock.mockReturnValue({
        data: {totalResults: 0, count: 0, summary: {}, usages: []},
        isLoading: false,
      });

      render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Delete'})).toBeEnabled();
    });

    it('should fall back to the disclaimer when usage data is unknown', () => {
      getUsagesMock.mockReturnValue({
        data: {totalResults: null, count: 0, summary: null, usages: []},
        isLoading: false,
      });

      render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

      expect(screen.getByText('All associated data will be permanently removed.')).toBeInTheDocument();
    });
  });

  describe('Callbacks', () => {
    it('should call onClose and onSuccess on successful deletion', async () => {
      mockMutate.mockImplementation((_userId: string, options: {onSuccess: () => void}) => {
        options.onSuccess();
      });

      render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} onSuccess={mockOnSuccess} />);

      fireEvent.click(screen.getByRole('button', {name: 'Delete'}));

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalled();
        expect(mockOnSuccess).toHaveBeenCalled();
      });
    });

    it('should display the generic fallback message on deletion failure', async () => {
      mockMutate.mockImplementation((_userId: string, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Network error'));
      });

      render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

      fireEvent.click(screen.getByRole('button', {name: 'Delete'}));

      await waitFor(() => {
        expect(screen.getByText('Failed to delete user. Please try again.')).toBeInTheDocument();
      });
    });

    it('should display the interpolated dependency summary for a mapped USR-1027 failure', async () => {
      const error = new Error('Conflict') as Error & {response?: {data?: {code: string; description?: unknown}}};
      error.response = {data: {code: 'USR-1027', description: {params: {dependencies: '2 agent(s)'}}}};
      mockMutate.mockImplementation((_userId: string, options: {onError: (err: Error) => void}) => {
        options.onError(error);
      });

      render(<UserDeleteDialog open userId="user-123" onClose={mockOnClose} />);

      fireEvent.click(screen.getByRole('button', {name: 'Delete'}));

      await waitFor(() => {
        expect(
          screen.getByText(
            'This user cannot be deleted because 2 agent(s) depend on it. Remove or reassign them first.',
          ),
        ).toBeInTheDocument();
      });
    });
  });
});
