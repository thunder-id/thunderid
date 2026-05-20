/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {screen, fireEvent, waitFor, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import OrganizationUnitDeleteDialog from '../OrganizationUnitDeleteDialog';

// Mock the delete hook — controllable per test
const mockMutate = vi.fn();
const mockDeleteHook = {mutate: mockMutate, isPending: false};
vi.mock('@/api/useDeleteOrganizationUnit', () => ({
  default: () => mockDeleteHook,
}));

// Mock translations
vi.mock('react-i18next', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useTranslation: () => ({
      t: (key: string) => {
        const translations: Record<string, string> = {
          'organizationUnits:delete.dialog.title': 'Delete Organization Unit',
          'organizationUnits:delete.dialog.message':
            'Are you sure you want to delete this organization unit? This action cannot be undone.',
          'organizationUnits:delete.dialog.disclaimer':
            'Warning: All associated data, configurations, and user assignments will be permanently removed.',
          'organizationUnits:delete.dialog.error': 'Failed to delete organization unit. Please try again.',
          'common:actions.cancel': 'Cancel',
          'common:actions.delete': 'Delete',
          'common:status.deleting': 'Deleting...',
        };
        return translations[key] ?? key;
      },
    }),
  };
});

describe('OrganizationUnitDeleteDialog', () => {
  const defaultProps = {
    open: true,
    organizationUnitId: 'ou-123',
    onClose: vi.fn(),
    onSuccess: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockReset();
  });

  it('should render dialog when open is true', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    expect(screen.getByText('Delete Organization Unit')).toBeInTheDocument();
    expect(
      screen.getByText('Are you sure you want to delete this organization unit? This action cannot be undone.'),
    ).toBeInTheDocument();
  });

  it('should not render dialog content when open is false', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} open={false} />);

    expect(screen.queryByText('Delete Organization Unit')).not.toBeInTheDocument();
  });

  it('should call onClose when cancel button is clicked', () => {
    const onClose = vi.fn();
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} />);

    fireEvent.click(screen.getByText('Cancel'));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('should call mutate with correct id when delete button is clicked', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    fireEvent.click(screen.getByText('Delete'));

    expect(mockMutate).toHaveBeenCalledWith('ou-123', expect.any(Object));
  });

  it('should not call mutate when organizationUnitId is null', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} organizationUnitId={null} />);

    fireEvent.click(screen.getByText('Delete'));

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('should call onClose and onSuccess on successful deletion', async () => {
    const onClose = vi.fn();
    const onSuccess = vi.fn();
    mockMutate.mockImplementation((_id, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} onSuccess={onSuccess} />);

    fireEvent.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
      expect(onSuccess).toHaveBeenCalled();
    });
  });

  it('should call onClose and onError on deletion failure', async () => {
    const onClose = vi.fn();
    const onError = vi.fn();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(
        Object.assign(new Error('Network error'), {
          response: {data: {code: 'ERR', message: 'fail', description: 'Network error'}},
        }),
      );
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} onError={onError} />);

    fireEvent.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
      expect(onError).toHaveBeenCalledWith('Network error');
    });
  });

  it('should use fallback error message when error has no response data', async () => {
    const onError = vi.fn();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(new Error());
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onError={onError} />);

    fireEvent.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith('Failed to delete organization unit. Please try again.');
    });
  });

  it('should work without onSuccess callback', async () => {
    const onClose = vi.fn();
    mockMutate.mockImplementation((_id, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} onSuccess={undefined} />);

    fireEvent.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('should work without onError callback', async () => {
    const onClose = vi.fn();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(new Error('Network error'));
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onClose={onClose} onError={undefined} />);

    fireEvent.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('should display cancel and delete buttons', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    expect(screen.getByText('Cancel')).toBeInTheDocument();
    expect(screen.getByText('Delete')).toBeInTheDocument();
  });

  it('should use error message when response has no description', async () => {
    const onError = vi.fn();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(
        Object.assign(new Error('Something went wrong'), {
          response: {data: {code: 'ERR', message: 'fail'}},
        }),
      );
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onError={onError} />);

    fireEvent.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith('Something went wrong');
    });
  });

  it('should use fallback when error message is only whitespace', async () => {
    const onError = vi.fn();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(new Error('   '));
    });

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} onError={onError} />);

    fireEvent.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith('Failed to delete organization unit. Please try again.');
    });
  });

  it('should render warning disclaimer alert', () => {
    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    expect(
      screen.getByText(
        'Warning: All associated data, configurations, and user assignments will be permanently removed.',
      ),
    ).toBeInTheDocument();
  });
});

describe('OrganizationUnitDeleteDialog - pending state', () => {
  const defaultProps = {
    open: true,
    organizationUnitId: 'ou-123',
    onClose: vi.fn(),
    onSuccess: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockReset();
    mockDeleteHook.isPending = false;
  });

  it('should show deleting text and disable buttons when pending', () => {
    mockDeleteHook.isPending = true;

    renderWithProviders(<OrganizationUnitDeleteDialog {...defaultProps} />);

    expect(screen.getByText('Deleting...')).toBeInTheDocument();

    // Both buttons should be disabled
    const cancelButton = screen.getByText('Cancel').closest('button');
    const deleteButton = screen.getByText('Deleting...').closest('button');
    expect(cancelButton).toBeDisabled();
    expect(deleteButton).toBeDisabled();
  });
});
