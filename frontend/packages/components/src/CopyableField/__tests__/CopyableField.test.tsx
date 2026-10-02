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

describe('CopyableField', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the label and value', () => {
    render(<CopyableField label="Client ID" value="abc123" />);

    expect(screen.getByText('Client ID')).toBeInTheDocument();
    expect(screen.getByDisplayValue('abc123')).toBeInTheDocument();
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

  it('does not throw when the clipboard write fails', async () => {
    const mockClipboard = {writeText: vi.fn().mockRejectedValue(new Error('denied'))};
    Object.defineProperty(navigator, 'clipboard', {value: mockClipboard, writable: true, configurable: true});

    render(<CopyableField label="Client ID" value="abc123" />);
    fireEvent.click(screen.getByRole('button'));

    await waitFor(() => {
      expect(mockClipboard.writeText).toHaveBeenCalled();
    });
  });

  it('uses a custom copy label for the button aria-label', () => {
    render(<CopyableField label="Client ID" value="abc123" copyLabel="Copy client ID" />);

    expect(screen.getByRole('button', {name: 'Copy client ID'})).toBeInTheDocument();
  });

  it('switches the button label to "Copied" after copying', async () => {
    const mockClipboard = {writeText: vi.fn().mockResolvedValue(undefined)};
    Object.defineProperty(navigator, 'clipboard', {value: mockClipboard, writable: true, configurable: true});

    render(<CopyableField label="Client ID" value="abc123" />);
    fireEvent.click(screen.getByRole('button', {name: 'common:actions.copy'}));

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'common:actions.copied'})).toBeInTheDocument();
    });
  });

  it('associates the label with the input', () => {
    render(<CopyableField label="Client ID" value="abc123" />);

    expect(screen.getByLabelText('Client ID')).toHaveValue('abc123');
  });

  it('uses the provided id on the input', () => {
    render(<CopyableField id="client-id-field" label="Client ID" value="abc123" />);

    expect(screen.getByLabelText('Client ID')).toHaveAttribute('id', 'client-id-field');
  });

  it('renders the hint only when provided', () => {
    const {rerender} = render(<CopyableField label="Client ID" value="abc123" />);
    expect(screen.queryByText('Used by your app')).not.toBeInTheDocument();

    rerender(<CopyableField label="Client ID" value="abc123" hint="Used by your app" />);
    expect(screen.getByText('Used by your app')).toBeInTheDocument();
  });
});
