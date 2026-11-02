// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import GroupDeleteDialog from '../GroupDeleteDialog';

const mockMutate = vi.fn();
const mockReset = vi.fn();
vi.mock('../../api/useDeleteGroup', () => ({
  default: () => ({
    mutate: mockMutate,
    reset: mockReset,
    isPending: false,
  }),
}));

describe('GroupDeleteDialog', () => {
  const defaultProps = {
    open: true,
    groupId: 'g1',
    onClose: vi.fn(),
    onSuccess: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render dialog when open', () => {
    renderWithProviders(<GroupDeleteDialog {...defaultProps} />);

    expect(screen.getByText('Delete group')).toBeInTheDocument();
    expect(screen.getByText('Are you sure you want to delete this group?')).toBeInTheDocument();
    expect(
      screen.getByText('This action cannot be undone. All group associations will be permanently removed.'),
    ).toBeInTheDocument();
  });

  it('should not render content when closed', () => {
    renderWithProviders(<GroupDeleteDialog {...defaultProps} open={false} />);

    expect(screen.queryByText('Delete group')).not.toBeInTheDocument();
  });

  it('should call onClose when cancel is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupDeleteDialog {...defaultProps} />);

    await user.click(screen.getByText('Cancel'));

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('should call mutate when confirm is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GroupDeleteDialog {...defaultProps} />);

    await user.click(screen.getByText('Delete'));

    expect(mockMutate).toHaveBeenCalledWith(
      'g1',
      expect.objectContaining({onSuccess: expect.any(Function) as unknown, onError: expect.any(Function) as unknown}),
    );
  });

  it('should call onSuccess and onClose on successful delete', async () => {
    mockMutate.mockImplementation((_id: string, opts: {onSuccess: () => void}) => {
      opts.onSuccess();
    });

    const user = userEvent.setup();
    renderWithProviders(<GroupDeleteDialog {...defaultProps} />);

    await user.click(screen.getByText('Delete'));

    expect(defaultProps.onClose).toHaveBeenCalled();
    expect(defaultProps.onSuccess).toHaveBeenCalled();
  });

  it('should display error on failed delete', async () => {
    mockMutate.mockImplementation((_id: string, opts: {onError: (err: Error) => void}) => {
      opts.onError(new Error('Delete failed'));
    });

    const user = userEvent.setup();
    renderWithProviders(<GroupDeleteDialog {...defaultProps} />);

    await user.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(screen.getByText('Failed to delete group. Please try again.')).toBeInTheDocument();
    });
  });

  it('should display mapped error message for a declarative group', async () => {
    mockMutate.mockImplementation((_id: string, opts: {onError: (err: Error) => void}) => {
      const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
      error.response = {data: {code: 'GRP-1015'}};
      opts.onError(error);
    });

    const user = userEvent.setup();
    renderWithProviders(<GroupDeleteDialog {...defaultProps} />);

    await user.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(
        screen.getByText('This group is managed declaratively and cannot be edited or deleted.'),
      ).toBeInTheDocument();
    });
  });

  it('should not call mutate when groupId is null', () => {
    renderWithProviders(<GroupDeleteDialog {...defaultProps} groupId={null} />);

    // Button should be disabled when groupId is null, preventing mutation
    const deleteButton = screen.getByText('Delete').closest('button');
    expect(deleteButton).toBeDisabled();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('should disable delete button when groupId is null', () => {
    renderWithProviders(<GroupDeleteDialog {...defaultProps} groupId={null} />);

    const deleteButton = screen.getByText('Delete').closest('button');
    expect(deleteButton).toBeDisabled();
  });
});
