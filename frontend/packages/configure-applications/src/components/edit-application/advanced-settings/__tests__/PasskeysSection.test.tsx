// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import PasskeysSection from '../PasskeysSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

describe('PasskeysSection', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('renders the section title, description, and field label', () => {
      render(<PasskeysSection onPasskeysChange={mockOnChange} />);

      expect(screen.getByText('Passkeys')).toBeInTheDocument();
      expect(screen.getByText('Passkey settings for this application.')).toBeInTheDocument();
      expect(screen.getByText('Allowed Origins')).toBeInTheDocument();
    });

    it('renders an existing origin', () => {
      render(<PasskeysSection allowedOrigins={['https://app.example.com']} onPasskeysChange={mockOnChange} />);

      expect(screen.getByDisplayValue('https://app.example.com')).toBeInTheDocument();
    });

    it('renders multiple existing origins', () => {
      render(
        <PasskeysSection
          allowedOrigins={['https://app.example.com', 'https://mobile.example.com']}
          onPasskeysChange={mockOnChange}
        />,
      );

      expect(screen.getByDisplayValue('https://app.example.com')).toBeInTheDocument();
      expect(screen.getByDisplayValue('https://mobile.example.com')).toBeInTheDocument();
    });

    it('renders one empty input when allowedOrigins is empty', () => {
      render(<PasskeysSection allowedOrigins={[]} onPasskeysChange={mockOnChange} />);

      expect(screen.getAllByRole('textbox')).toHaveLength(1);
      expect(screen.getByRole('textbox')).toHaveValue('');
    });

    it('shows the Add Origin button when editable', () => {
      render(<PasskeysSection onPasskeysChange={mockOnChange} />);

      expect(screen.getByRole('button', {name: /Add Origin/i})).toBeInTheDocument();
    });

    it('hides the Add Origin and Delete buttons when read-only (no onChange)', () => {
      render(<PasskeysSection allowedOrigins={['https://app.example.com']} />);

      expect(screen.queryByRole('button', {name: /Add Origin/i})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: /Delete/i})).not.toBeInTheDocument();
    });

    it('hides the Add Origin and Delete buttons when disabled', () => {
      render(<PasskeysSection allowedOrigins={['https://app.example.com']} onPasskeysChange={mockOnChange} disabled />);

      expect(screen.queryByRole('button', {name: /Add Origin/i})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: /Delete/i})).not.toBeInTheDocument();
    });

    it('renders inputs as disabled when disabled prop is true', () => {
      render(<PasskeysSection allowedOrigins={['https://app.example.com']} onPasskeysChange={mockOnChange} disabled />);

      expect(screen.getByDisplayValue('https://app.example.com')).toBeDisabled();
    });
  });

  describe('Adding origins', () => {
    it('appends an empty string when "Add Origin" is clicked', async () => {
      const user = userEvent.setup();
      render(<PasskeysSection allowedOrigins={['https://app.example.com']} onPasskeysChange={mockOnChange} />);

      await user.click(screen.getByRole('button', {name: /Add Origin/i}));

      expect(mockOnChange).toHaveBeenCalledWith(['https://app.example.com', '']);
    });

    it('appends a second row to an empty list, rather than no-op-ing on the placeholder row', async () => {
      const user = userEvent.setup();
      render(<PasskeysSection allowedOrigins={[]} onPasskeysChange={mockOnChange} />);

      await user.click(screen.getByRole('button', {name: /Add Origin/i}));

      expect(mockOnChange).toHaveBeenCalledWith(['', '']);
    });
  });

  describe('Removing origins', () => {
    it('removes an origin when its Delete button is clicked', async () => {
      const user = userEvent.setup();
      render(
        <PasskeysSection
          allowedOrigins={['https://app.example.com', 'https://mobile.example.com']}
          onPasskeysChange={mockOnChange}
        />,
      );

      const deleteButtons = screen.getAllByRole('button', {name: /Delete/i});
      await user.click(deleteButtons[0]);

      expect(mockOnChange).toHaveBeenCalledWith(['https://mobile.example.com']);
    });

    it('removes the last origin, leaving an empty list', async () => {
      const user = userEvent.setup();
      render(<PasskeysSection allowedOrigins={['https://app.example.com']} onPasskeysChange={mockOnChange} />);

      await user.click(screen.getByRole('button', {name: /Delete/i}));

      expect(mockOnChange).toHaveBeenCalledWith([]);
    });

    it('shifts error indices down after removing an earlier entry', async () => {
      const user = userEvent.setup();
      render(<PasskeysSection allowedOrigins={['', '', '']} onPasskeysChange={mockOnChange} />);

      // Blur the first and third inputs to trigger empty-field errors on those two
      const inputs = screen.getAllByRole('textbox');
      await user.click(inputs[0]);
      await user.tab();
      await user.click(inputs[2]);
      await user.tab();

      expect(screen.getAllByText('Origin cannot be empty')).toHaveLength(2);

      // Remove the middle (index 1) entry — the remaining two error entries should shift
      const deleteButtons = screen.getAllByRole('button', {name: /Delete/i});
      await user.click(deleteButtons[1]);

      // onChange should be called with the two remaining empty strings
      expect(mockOnChange).toHaveBeenCalledWith(['', '']);
    });
  });

  describe('Editing origins', () => {
    it('calls onChange on each keystroke', async () => {
      const user = userEvent.setup({delay: null});
      render(<PasskeysSection allowedOrigins={['']} onPasskeysChange={mockOnChange} />);

      const input = screen.getByPlaceholderText('https://app.example.com');
      await user.type(input, 'h');

      expect(mockOnChange).toHaveBeenCalledWith(['h']);
    });

    it('clears a field error while the user is typing', async () => {
      const user = userEvent.setup();
      render(<PasskeysSection allowedOrigins={['']} onPasskeysChange={mockOnChange} />);

      const input = screen.getByPlaceholderText('https://app.example.com');

      // Blur to trigger the empty error
      await user.click(input);
      await user.tab();
      expect(screen.getByText('Origin cannot be empty')).toBeInTheDocument();

      // Type something — the error should disappear
      await user.type(input, 'h');
      expect(screen.queryByText('Origin cannot be empty')).not.toBeInTheDocument();
    });
  });

  describe('Validation', () => {
    it('shows an empty-field error when an empty input is blurred', async () => {
      const user = userEvent.setup();
      render(<PasskeysSection allowedOrigins={['']} onPasskeysChange={mockOnChange} />);

      const input = screen.getByPlaceholderText('https://app.example.com');
      await user.click(input);
      await user.tab();

      expect(screen.getByText('Origin cannot be empty')).toBeInTheDocument();
    });

    it('shows an invalid-URL error for a non-URL value on blur', async () => {
      const user = userEvent.setup();
      render(<PasskeysSection allowedOrigins={['not a url']} onPasskeysChange={mockOnChange} />);

      const input = screen.getByDisplayValue('not a url');
      await user.click(input);
      await user.tab();

      expect(screen.getByText('Enter a valid URL')).toBeInTheDocument();
    });

    it('shows no inline error for a valid URL after blur', async () => {
      const user = userEvent.setup();
      render(<PasskeysSection allowedOrigins={['https://app.example.com']} onPasskeysChange={mockOnChange} />);

      const input = screen.getByDisplayValue('https://app.example.com');
      await user.click(input);
      await user.tab();

      expect(screen.queryByText('Origin cannot be empty')).not.toBeInTheDocument();
      expect(screen.queryByText('Enter a valid URL')).not.toBeInTheDocument();
    });
  });
});
