// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import ClientSecretSuccessDialog from '../ClientSecretSuccessDialog';

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'applications:regenerateSecret.success.title': 'Save Your New Client Secret',
        'applications:regenerateSecret.success.subtitle':
          "This is the only time you'll see this secret. Store it somewhere safe.",
        'applications:regenerateSecret.success.secretLabel': 'New Client Secret',
        'applications:regenerateSecret.success.copyButton': 'Copy to clipboard',
        'applications:regenerateSecret.success.toggleVisibility': 'Toggle secret visibility',
        'applications:regenerateSecret.success.copySecret': 'Copy Secret',
        'applications:regenerateSecret.success.copied': 'Copied to clipboard',
        'applications:regenerateSecret.success.securityReminder.title': 'Security Reminder',
        'applications:regenerateSecret.success.securityReminder.description':
          'Never share your client secret publicly or store it in version control.',
        'common:actions.done': 'Done',
      };
      return translations[key] || key;
    },
  }),
}));

describe('ClientSecretSuccessDialog', () => {
  const mockOnClose = vi.fn();
  const testClientSecret = 'test-client-secret-abc123xyz789';
  // Stored at describe scope so assertions can reference it even after userEvent.setup()
  // replaces navigator.clipboard with its own stub.
  const mockWriteText = vi.fn().mockResolvedValue(undefined);

  const defaultProps = {
    open: true,
    clientSecret: testClientSecret,
    onClose: mockOnClose,
  };

  const renderDialog = (props = defaultProps) => render(<ClientSecretSuccessDialog {...props} />);

  beforeEach(() => {
    vi.clearAllMocks();
    // Mock clipboard API using defineProperty since navigator.clipboard is a getter-only property.
    // Re-apply each time because userEvent.setup() may replace navigator.clipboard with its own stub.
    mockWriteText.mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: mockWriteText,
      },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the dialog when open is true', () => {
      renderDialog();

      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByText('Save Your New Client Secret')).toBeInTheDocument();
    });

    it('should display the subtitle message', () => {
      renderDialog();

      expect(
        screen.getByText("This is the only time you'll see this secret. Store it somewhere safe."),
      ).toBeInTheDocument();
    });

    it('should display the client secret label', () => {
      renderDialog();

      expect(screen.getByText('New Client Secret')).toBeInTheDocument();
    });

    it('should have the client secret as a masked password field by default', () => {
      renderDialog();

      const textField = document.querySelector('input[type="password"]');
      expect(textField).toBeInTheDocument();
      expect(textField).toHaveValue(testClientSecret);
    });

    it('should display the security reminder', () => {
      renderDialog();

      expect(screen.getByText('Security Reminder')).toBeInTheDocument();
      expect(
        screen.getByText('Never share your client secret publicly or store it in version control.'),
      ).toBeInTheDocument();
    });

    it('should not render dialog content when open is false', () => {
      renderDialog({...defaultProps, open: false});

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('should render Done and Copy Secret buttons', () => {
      renderDialog();

      expect(screen.getByRole('button', {name: 'Done'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Copy Secret'})).toBeInTheDocument();
    });

    it('should render the warning icon', () => {
      renderDialog();

      // AlertTriangle icon should be present
      const svgIcons = document.querySelectorAll('svg');
      expect(svgIcons.length).toBeGreaterThan(0);
    });
  });

  describe('User Interactions', () => {
    it('should call onClose when Done button is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();

      const doneButton = screen.getByRole('button', {name: 'Done'});
      await user.click(doneButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should call onClose when Escape key is pressed', async () => {
      const user = userEvent.setup();
      renderDialog();

      await user.keyboard('{Escape}');

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should copy client secret to clipboard when Copy Secret button is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();
      // Spy after userEvent.setup() so we intercept the clipboard stub it installs
      const writeTextSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

      const copyButton = screen.getByRole('button', {name: 'Copy Secret'});
      await user.click(copyButton);

      expect(writeTextSpy).toHaveBeenCalledWith(testClientSecret);
    });

    it('should copy client secret when inline copy icon is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();
      // Spy after userEvent.setup() so we intercept the clipboard stub it installs
      const writeTextSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

      const copyButton = screen.getByRole('button', {name: 'Copy to clipboard'});
      await user.click(copyButton);

      expect(writeTextSpy).toHaveBeenCalledWith(testClientSecret);
    });

    it('should show "Copied to clipboard" text after successful copy', async () => {
      vi.useFakeTimers({shouldAdvanceTime: true});
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});
      renderDialog();

      const copyButton = screen.getByRole('button', {name: 'Copy Secret'});
      await user.click(copyButton);

      await waitFor(() => {
        expect(screen.getByText('Copied to clipboard')).toBeInTheDocument();
      });

      vi.useRealTimers();
    });

    it('should toggle secret visibility when visibility button is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();

      // Initially should be password type (masked)
      const passwordField = document.querySelector('input[type="password"]');
      expect(passwordField).toBeInTheDocument();

      // Click toggle visibility button
      const toggleButton = screen.getByRole('button', {name: 'Toggle secret visibility'});
      await user.click(toggleButton);

      // Should now be text type (visible)
      const textField = document.querySelector('input[type="text"]');
      expect(textField).toBeInTheDocument();
      expect(textField).toHaveValue(testClientSecret);
    });
  });

  describe('Text Field Properties', () => {
    it('should have a readonly text field', () => {
      renderDialog();

      const textField = document.querySelector('input');
      expect(textField).toHaveAttribute('readonly');
    });

    it('should display the full client secret when visible', async () => {
      const user = userEvent.setup();
      const longSecret = 'a'.repeat(64);
      renderDialog({...defaultProps, clientSecret: longSecret});

      // Toggle visibility to see the full secret
      const toggleButton = screen.getByRole('button', {name: 'Toggle secret visibility'});
      await user.click(toggleButton);

      const textField = document.querySelector('input[type="text"]');
      expect(textField).toHaveValue(longSecret);
    });

    it('should reset visibility state when dialog is closed and reopened', async () => {
      const user = userEvent.setup();
      const {rerender} = renderDialog();

      // Toggle visibility
      const toggleButton = screen.getByRole('button', {name: 'Toggle secret visibility'});
      await user.click(toggleButton);

      // Should be visible
      expect(document.querySelector('input[type="text"]')).toBeInTheDocument();

      // Click Done to close
      const doneButton = screen.getByRole('button', {name: 'Done'});
      await user.click(doneButton);

      // Rerender with open: false then open: true
      rerender(<ClientSecretSuccessDialog {...defaultProps} open={false} />);
      rerender(<ClientSecretSuccessDialog {...defaultProps} open />);

      // Should be masked again
      expect(document.querySelector('input[type="password"]')).toBeInTheDocument();
    });
  });
});
