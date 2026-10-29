// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, fireEvent, waitFor, renderWithProviders, renderHook} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, it, expect, vi, beforeEach, beforeAll} from 'vitest';
import OrganizationUnitDeleteDialog from '../OrganizationUnitDeleteDialog';

// Mock the delete hook — controllable per test
const mockMutate = vi.fn();
const mockReset = vi.fn();
const mockDeleteHook: {mutate: typeof mockMutate; isPending: boolean; error: Error | null; reset: typeof mockReset} = {
  mutate: mockMutate,
  isPending: false,
  error: null,
  reset: mockReset,
};
vi.mock('@/api/useDeleteOrganizationUnit', () => ({
  default: () => mockDeleteHook,
}));

describe('OrganizationUnitDeleteDialog', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  const defaultProps = {
    open: true,
    organizationUnitId: 'ou-123',
    onClose: vi.fn(),
    onSuccess: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockReset();
    mockReset.mockReset();
    mockDeleteHook.error = null;
    mockDeleteHook.isPending = false;
  });

  it('should render dialog when open is true', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    expect(screen.getByText(t('organizationUnits:delete.dialog.title'))).toBeInTheDocument();
    expect(screen.getByText(t('organizationUnits:delete.dialog.message'))).toBeInTheDocument();
  });

  it('should not render dialog content when open is false', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} open={false} />);

    expect(screen.queryByText(t('organizationUnits:delete.dialog.title'))).not.toBeInTheDocument();
  });

  it('should call onClose and reset the mutation when cancel button is clicked', () => {
    const onClose = vi.fn();
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} />);

    fireEvent.click(screen.getByText(t('common:actions.cancel')));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mockReset).toHaveBeenCalledTimes(1);
  });

  it('should call mutate with correct id when delete button is clicked', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    fireEvent.click(screen.getByText(t('common:actions.delete')));

    expect(mockMutate).toHaveBeenCalledWith('ou-123', expect.any(Object));
  });

  it('should not call mutate when organizationUnitId is null', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} organizationUnitId={null} />);

    fireEvent.click(screen.getByText(t('common:actions.delete')));

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('should call onClose and onSuccess on successful deletion', async () => {
    const onClose = vi.fn();
    const onSuccess = vi.fn();
    mockMutate.mockImplementation((_id, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} onSuccess={onSuccess} />);

    fireEvent.click(screen.getByText(t('common:actions.delete')));

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
      expect(onSuccess).toHaveBeenCalled();
    });
  });

  it('should not close the dialog on deletion failure, and should render the error inline', async () => {
    const onClose = vi.fn();
    mockMutate.mockImplementation(() => {
      // The mutation's own error state (not an onError callback) is how the dialog learns of a failure.
      mockDeleteHook.error = Object.assign(new Error('Network error'), {
        response: {data: {code: 'ERR'}},
      });
    });

    const {rerender} = renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} />);

    fireEvent.click(screen.getByText(t('common:actions.delete')));
    rerender(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} />);

    await waitFor(() => {
      expect(onClose).not.toHaveBeenCalled();
      expect(screen.getByText('Failed to delete organization unit. Please try again.')).toBeInTheDocument();
    });
  });

  it('should resolve a known error code through the catalog', async () => {
    mockMutate.mockImplementation(() => {
      mockDeleteHook.error = Object.assign(new Error('Organization unit has children'), {
        response: {data: {code: 'OU-1006'}},
      });
    });

    const {rerender} = renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    fireEvent.click(screen.getByText(t('common:actions.delete')));
    rerender(<OrganizationUnitDeleteDialog {...defaultProps} />);

    await waitFor(() => {
      expect(
        screen.getByText(
          'This organization unit has child units, users, groups, connections, or flows and cannot be deleted while they exist.',
        ),
      ).toBeInTheDocument();
    });
  });

  it('should use the fallback message when the error has no known code', async () => {
    mockMutate.mockImplementation(() => {
      mockDeleteHook.error = new Error('boom');
    });

    const {rerender} = renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    fireEvent.click(screen.getByText(t('common:actions.delete')));
    rerender(<OrganizationUnitDeleteDialog {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText(t('organizationUnits:delete.dialog.error'))).toBeInTheDocument();
    });
  });

  it('should work without onSuccess callback', async () => {
    const onClose = vi.fn();
    mockMutate.mockImplementation((_id, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} onSuccess={undefined} />);

    fireEvent.click(screen.getByText(t('common:actions.delete')));

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('should display cancel and delete buttons', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    expect(screen.getByText(t('common:actions.cancel'))).toBeInTheDocument();
    expect(screen.getByText(t('common:actions.delete'))).toBeInTheDocument();
  });

  it('should render warning disclaimer alert', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    expect(screen.getByText(t('organizationUnits:delete.dialog.disclaimer'))).toBeInTheDocument();
  });
});

describe('OrganizationUnitDeleteDialog - pending state', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation()).result.current);
  });

  const defaultProps = {
    open: true,
    organizationUnitId: 'ou-123',
    onClose: vi.fn(),
    onSuccess: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockReset();
    mockReset.mockReset();
    mockDeleteHook.error = null;
    mockDeleteHook.isPending = false;
  });

  it('should show deleting text and disable buttons when pending', () => {
    mockDeleteHook.isPending = true;

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    expect(screen.getByText(t('common:status.deleting'))).toBeInTheDocument();

    // Both buttons should be disabled
    const cancelButton = screen.getByText(t('common:actions.cancel')).closest('button');
    const deleteButton = screen.getByText(t('common:status.deleting')).closest('button');
    expect(cancelButton).toBeDisabled();
    expect(deleteButton).toBeDisabled();
  });
});
