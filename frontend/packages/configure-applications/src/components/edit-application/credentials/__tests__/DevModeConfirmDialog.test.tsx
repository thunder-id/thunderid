// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import DevModeConfirmDialog from '../DevModeConfirmDialog';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

describe('DevModeConfirmDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnConfirm = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the dialog when open', () => {
    render(<DevModeConfirmDialog open onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Enable Dev Mode?')).toBeInTheDocument();
  });

  it('should not render dialog content when closed', () => {
    render(<DevModeConfirmDialog open={false} onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('should call onConfirm when the confirm button is clicked', () => {
    render(<DevModeConfirmDialog open onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    fireEvent.click(screen.getByTestId('dev-mode-confirm-button'));

    expect(mockOnConfirm).toHaveBeenCalledTimes(1);
    expect(mockOnClose).not.toHaveBeenCalled();
  });

  it('should call onClose when the cancel button is clicked', () => {
    render(<DevModeConfirmDialog open onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    fireEvent.click(screen.getByText('Cancel'));

    expect(mockOnClose).toHaveBeenCalledTimes(1);
    expect(mockOnConfirm).not.toHaveBeenCalled();
  });
});
