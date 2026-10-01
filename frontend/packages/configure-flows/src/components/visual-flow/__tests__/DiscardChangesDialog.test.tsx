// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import DiscardChangesDialog from '../DiscardChangesDialog';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({t: (key: string, fallback?: string) => fallback ?? key}),
}));

describe('DiscardChangesDialog', () => {
  const onClose = vi.fn();
  const onConfirm = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders when open', () => {
    render(<DiscardChangesDialog open onClose={onClose} onConfirm={onConfirm} />);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Discard unsaved changes?')).toBeInTheDocument();
  });

  it('does not render content when closed', () => {
    render(<DiscardChangesDialog open={false} onClose={onClose} onConfirm={onConfirm} />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('calls onConfirm when the discard button is clicked', () => {
    render(<DiscardChangesDialog open onClose={onClose} onConfirm={onConfirm} />);

    fireEvent.click(screen.getByTestId('discard-changes-confirm-button'));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('calls onClose when the keep-editing button is clicked', () => {
    render(<DiscardChangesDialog open onClose={onClose} onConfirm={onConfirm} />);

    fireEvent.click(screen.getByText('Keep editing'));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
