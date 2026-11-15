// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, afterEach} from 'vitest';
import CopyableField from '../CopyableField';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));

describe('CopyableField', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the label and value', () => {
    render(<CopyableField label="Client ID" value="abc123" />);

    expect(screen.getByText('Client ID')).toBeInTheDocument();
    expect(screen.getByText('abc123')).toBeInTheDocument();
  });

  it('copies the value to the clipboard on click', async () => {
    const mockClipboard = {writeText: vi.fn().mockResolvedValue(undefined)};
    Object.defineProperty(navigator, 'clipboard', {value: mockClipboard, writable: true, configurable: true});

    render(<CopyableField label="Client ID" value="abc123" />);
    fireEvent.click(screen.getByRole('button'));

    await waitFor(() => {
      expect(mockClipboard.writeText).toHaveBeenCalledWith('abc123');
    });
  });

  it('logs an error when the clipboard write fails', async () => {
    const mockClipboard = {writeText: vi.fn().mockRejectedValue(new Error('denied'))};
    Object.defineProperty(navigator, 'clipboard', {value: mockClipboard, writable: true, configurable: true});

    render(<CopyableField label="Client ID" value="abc123" />);
    fireEvent.click(screen.getByRole('button'));

    await waitFor(() => {
      expect(mockClipboard.writeText).toHaveBeenCalled();
    });
  });

  it('names the value, which takes focus to be scrolled, after its label', () => {
    render(<CopyableField label="Client ID" value="abc123" />);

    expect(screen.getByRole('group', {name: 'Client ID'})).toHaveTextContent('abc123');
  });

  it('uses a custom copy label for the button aria-label', () => {
    render(<CopyableField label="Client ID" value="abc123" copyLabel="Copy client ID" />);

    expect(screen.getByRole('button', {name: 'Copy client ID'})).toBeInTheDocument();
  });
});
