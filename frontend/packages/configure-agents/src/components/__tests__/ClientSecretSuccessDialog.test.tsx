// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import ClientSecretSuccessDialog from '../ClientSecretSuccessDialog';

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => {
      const translations: Record<string, string> = {
        'agents:regenerateSecret.success.title': 'Client secret regenerated',
        'agents:regenerateSecret.success.subtitle':
          "Copy your new client secret and store it somewhere safe. It won't be shown again.",
        'agents:regenerateSecret.success.secretLabel': 'New Client Secret',
        'agents:regenerateSecret.success.copySecret': 'Copy client secret',
        'agents:regenerateSecret.success.copied': 'Copied',
        'agents:regenerateSecret.success.securityReminder.title': "You won't be able to see this secret again",
        'agents:regenerateSecret.success.securityReminder.description':
          "Store the new client secret somewhere safe. If you lose it, you'll need to regenerate it again.",
        'common:actions.done': 'Done',
      };
      return translations[key] ?? fallback ?? key;
    },
  }),
}));

describe('ClientSecretSuccessDialog', () => {
  const mockOnClose = vi.fn();
  const testClientSecret = 'test-client-secret-abc123xyz789';
  const mockWriteText = vi.fn().mockResolvedValue(undefined);

  const defaultProps = {
    open: true,
    clientSecret: testClientSecret,
    onClose: mockOnClose,
  };

  const renderDialog = (props = defaultProps) => render(<ClientSecretSuccessDialog {...props} />);

  beforeEach(() => {
    vi.clearAllMocks();
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
      expect(screen.getByText('Client secret regenerated')).toBeInTheDocument();
    });

    it('should display the subtitle message', () => {
      renderDialog();

      expect(
        screen.getByText("Copy your new client secret and store it somewhere safe. It won't be shown again."),
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

      expect(screen.getByText("You won't be able to see this secret again")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Store the new client secret somewhere safe. If you lose it, you'll need to regenerate it again.",
        ),
      ).toBeInTheDocument();
    });

    it('should not render dialog content when open is false', () => {
      renderDialog({...defaultProps, open: false});

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('should render Done and Copy client secret buttons', () => {
      renderDialog();

      expect(screen.getByRole('button', {name: 'Done'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Copy client secret'})).toBeInTheDocument();
    });
  });

  describe('User Interactions', () => {
    it('should call onClose when Done button is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();

      await user.click(screen.getByRole('button', {name: 'Done'}));

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should call onClose when Escape key is pressed', async () => {
      const user = userEvent.setup();
      renderDialog();

      await user.keyboard('{Escape}');

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('should copy client secret when Copy client secret button is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();
      const writeTextSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

      const copyButton = screen.getByRole('button', {name: 'Copy client secret'});
      await user.click(copyButton);

      expect(writeTextSpy).toHaveBeenCalledWith(testClientSecret);
    });

    it('should show "Copied" text after successful copy', async () => {
      vi.useFakeTimers({shouldAdvanceTime: true});
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});
      renderDialog();

      await user.click(screen.getByRole('button', {name: 'Copy client secret'}));

      await waitFor(() => {
        expect(screen.getByText('Copied')).toBeInTheDocument();
      });

      vi.useRealTimers();
    });

    it('should toggle secret visibility when visibility button is clicked', async () => {
      const user = userEvent.setup();
      renderDialog();

      // Initially password type
      expect(document.querySelector('input[type="password"]')).toBeInTheDocument();

      // Find toggle visibility button — the first IconButton in the input adornment
      const allButtons = screen.getAllByRole('button');
      const iconButtons = allButtons.filter((btn) => btn.querySelector('svg') && !btn.textContent);
      const visibilityButton = iconButtons[0];
      await user.click(visibilityButton);

      // Now visible
      expect(document.querySelector('input[type="text"]')).toBeInTheDocument();
    });

    it('should silently ignore clipboard failures', async () => {
      mockWriteText.mockRejectedValueOnce(new Error('Clipboard fail'));

      const user = userEvent.setup();
      renderDialog();
      const writeTextSpy = vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('Clipboard fail'));

      await user.click(screen.getByRole('button', {name: 'Copy client secret'}));

      expect(writeTextSpy).toHaveBeenCalled();
      // Should not transition to "Copied" state since copy failed
      expect(screen.queryByText('Copied')).not.toBeInTheDocument();
    });
  });

  describe('Text Field Properties', () => {
    it('should have a readonly text field', () => {
      renderDialog();

      const textField = document.querySelector('input');
      expect(textField).toHaveAttribute('readonly');
    });

    it('should reset visibility state when dialog is closed', async () => {
      const user = userEvent.setup();
      const {rerender} = renderDialog();

      const allButtons = screen.getAllByRole('button');
      const iconButtons = allButtons.filter((btn) => btn.querySelector('svg') && !btn.textContent);
      await user.click(iconButtons[0]);

      expect(document.querySelector('input[type="text"]')).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'Done'}));

      // Rerender with open: false then open: true
      rerender(<ClientSecretSuccessDialog {...defaultProps} open={false} />);
      rerender(<ClientSecretSuccessDialog {...defaultProps} open />);

      // Should be masked again after reopen
      expect(document.querySelector('input[type="password"]')).toBeInTheDocument();
    });
  });
});
