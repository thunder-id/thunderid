// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SsoDisableConfirmDialog from '../SsoDisableConfirmDialog';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, options?: unknown) => {
      if (typeof options === 'string') {
        return options;
      }
      return key;
    },
  }),
}));

describe('SsoDisableConfirmDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnConfirm = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the dialog when open', () => {
    render(<SsoDisableConfirmDialog open checkpointCount={1} onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Remove single sign-on?')).toBeInTheDocument();
  });

  it('should not render dialog content when closed', () => {
    render(
      <SsoDisableConfirmDialog open={false} checkpointCount={1} onClose={mockOnClose} onConfirm={mockOnConfirm} />,
    );

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('should call onConfirm when the remove button is clicked', () => {
    render(<SsoDisableConfirmDialog open checkpointCount={2} onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    fireEvent.click(screen.getByTestId('sso-disable-confirm-button'));

    expect(mockOnConfirm).toHaveBeenCalledTimes(1);
    expect(mockOnClose).not.toHaveBeenCalled();
  });

  it('should call onClose when the cancel button is clicked', () => {
    render(<SsoDisableConfirmDialog open checkpointCount={1} onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    fireEvent.click(screen.getByText('Cancel'));

    expect(mockOnClose).toHaveBeenCalledTimes(1);
    expect(mockOnConfirm).not.toHaveBeenCalled();
  });
});
