// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CredentialResetDialog from '../CredentialResetDialog';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string, values?: Record<string, string>) => {
      const template = typeof fallback === 'string' ? fallback : key;
      return template.replace(/{{(\w+)}}/g, (_match, name: string) => values?.[name] ?? _match);
    },
  }),
}));

const passwordField = {fieldName: 'password', label: 'Password'};

describe('CredentialResetDialog', () => {
  const onReset = vi.fn();
  const onClose = vi.fn();
  const getErrorMessage = vi.fn((error: Error) => `Could not reset: ${error.message}`);

  const renderDialog = (field: typeof passwordField | null = passwordField, open = true) =>
    render(
      <CredentialResetDialog
        open={open}
        field={field}
        onReset={onReset}
        getErrorMessage={getErrorMessage}
        onClose={onClose}
      />,
    );

  const fill = (placeholder: string, value: string) =>
    fireEvent.change(screen.getByPlaceholderText(placeholder), {target: {value}});

  const fillBoth = (newValue: string, confirmValue: string, name = 'password') => {
    fill(`Enter new ${name}`, newValue);
    fill(`Confirm new ${name}`, confirmValue);
  };

  beforeEach(() => {
    vi.clearAllMocks();
    onReset.mockResolvedValue(undefined);
  });

  it('shows the dialog for a field and nothing when it is closed', () => {
    const {unmount} = renderDialog();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Reset Password?')).toBeInTheDocument();
    unmount();

    renderDialog(passwordField, false);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('warns that the current value stops working and its tokens and sessions end', () => {
    renderDialog();

    expect(
      screen.getByText('A new password will be set. The current password will be invalidated immediately.'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'This action cannot be undone. The tokens and sessions issued with the current password will be ended as soon as you confirm.',
      ),
    ).toBeInTheDocument();
  });

  it('keeps Reset off until a value is entered', () => {
    renderDialog();

    expect(screen.getByRole('button', {name: 'Reset Password'})).toBeDisabled();
  });

  it('closes on Cancel', () => {
    renderDialog();

    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('refuses mismatched values without calling the update', () => {
    renderDialog();
    fillBoth('newpass123', 'differentpass');

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(screen.getByText('Values do not match.')).toBeInTheDocument();
    expect(onReset).not.toHaveBeenCalled();
  });

  it.each([
    ['new value', 'Enter new password'],
    ['confirm value', 'Confirm new password'],
  ])('clears the mismatch message when the %s changes', (_name, placeholder) => {
    renderDialog();
    fillBoth('newpass123', 'differentpass');
    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));
    expect(screen.getByText('Values do not match.')).toBeInTheDocument();

    fill(placeholder, 'newpass456');

    expect(screen.queryByText('Values do not match.')).not.toBeInTheDocument();
  });

  it('hands the field and the new value to the update', async () => {
    renderDialog();
    fillBoth('newpass123', 'newpass123');

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    await waitFor(() => {
      expect(onReset).toHaveBeenCalledWith(passwordField, 'newpass123');
    });
  });

  it('closes once the update succeeds', async () => {
    renderDialog();
    fillBoth('newpass123', 'newpass123');

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    await waitFor(() => {
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  it("shows the caller's message when the update fails, and stays open", async () => {
    onReset.mockRejectedValue(new Error('Network error'));
    renderDialog();
    fillBoth('newpass123', 'newpass123');

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(await screen.findByText('Could not reset: Network error')).toBeInTheDocument();
    expect(getErrorMessage).toHaveBeenCalledWith(expect.objectContaining({message: 'Network error'}));
    expect(onClose).not.toHaveBeenCalled();
  });

  it('drops a failed save once the form changes', async () => {
    onReset.mockRejectedValue(new Error('Network error'));
    renderDialog();
    fillBoth('newpass123', 'newpass123');
    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));
    await screen.findByText('Could not reset: Network error');

    fill('Enter new password', 'newpass456');

    expect(screen.queryByText('Could not reset: Network error')).not.toBeInTheDocument();
  });

  it('disables the buttons and says so while the update runs', async () => {
    let finish: () => void = () => undefined;
    onReset.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)));
    renderDialog();
    fillBoth('newpass123', 'newpass123');

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(await screen.findByRole('button', {name: 'Resetting…'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled();

    finish();
    await waitFor(() => {
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  it('does not update with a value that is only whitespace', () => {
    renderDialog();
    fillBoth('   ', '   ');

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(onReset).not.toHaveBeenCalled();
  });

  it('uses the label and name of whichever credential it is given', async () => {
    const pinField = {fieldName: 'pin', label: 'PIN'};
    renderDialog(pinField);

    expect(screen.getByText('Reset PIN?')).toBeInTheDocument();
    fillBoth('1234', '1234', 'pin');
    fireEvent.click(screen.getByRole('button', {name: 'Reset PIN'}));

    await waitFor(() => {
      expect(onReset).toHaveBeenCalledWith(pinField, '1234');
    });
  });

  describe('with a reset link', () => {
    const onSend = vi.fn();
    const getLinkErrorMessage = vi.fn((error: Error) => `Could not send: ${error.message}`);
    const SEND = {name: 'Send reset link'};

    const renderWithLink = (heldReason?: string) =>
      render(
        <CredentialResetDialog
          open
          field={passwordField}
          onReset={onReset}
          getErrorMessage={getErrorMessage}
          resetLink={{onSend, getErrorMessage: getLinkErrorMessage, heldReason}}
          onClose={onClose}
        />,
      );

    const chooseLink = () =>
      fireEvent.click(screen.getByRole('radio', {name: 'Email the user a link to set a new password'}));

    beforeEach(() => {
      onSend.mockResolvedValue(undefined);
    });

    it('offers both ways, setting the value here first', () => {
      renderWithLink();

      expect(screen.getByRole('radiogroup', {name: 'How should the password be reset?'})).toBeInTheDocument();
      expect(screen.getByRole('radio', {name: 'Set a new password now'})).toBeChecked();
      expect(screen.getByPlaceholderText('Enter new password')).toBeInTheDocument();
    });

    it('offers no choice without a reset link', () => {
      renderDialog();

      expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    });

    it('warns what the link does and sends it without asking for a value', async () => {
      renderWithLink();

      chooseLink();

      expect(screen.getByText('The user will receive an email with a link to set a new password.')).toBeInTheDocument();
      expect(screen.getByText(/^Password stops working once the email is sent/)).toBeInTheDocument();
      expect(screen.queryByPlaceholderText('Enter new password')).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', SEND));

      await waitFor(() => {
        expect(onClose).toHaveBeenCalledTimes(1);
      });
      expect(onSend).toHaveBeenCalledTimes(1);
      expect(onReset).not.toHaveBeenCalled();
    });

    it("shows the link's own message when the send fails, and stays open", async () => {
      onSend.mockRejectedValue(new Error('No email'));
      renderWithLink();
      chooseLink();

      fireEvent.click(screen.getByRole('button', SEND));

      expect(await screen.findByTestId('credential-reset-error')).toHaveTextContent('Could not send: No email');
      expect(screen.getByRole('button', SEND)).toBeEnabled();
      expect(onClose).not.toHaveBeenCalled();
    });

    it('drops a failed send when the other way is chosen', async () => {
      onSend.mockRejectedValue(new Error('No email'));
      renderWithLink();
      chooseLink();
      fireEvent.click(screen.getByRole('button', SEND));
      await screen.findByText('Could not send: No email');

      fireEvent.click(screen.getByRole('radio', {name: 'Set a new password now'}));

      expect(screen.queryByTestId('credential-reset-error')).not.toBeInTheDocument();
    });

    it('cannot be closed or switched while sending', async () => {
      let finish: () => void = () => undefined;
      onSend.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)));
      renderWithLink();
      chooseLink();

      fireEvent.click(screen.getByRole('button', SEND));

      expect(await screen.findByRole('button', {name: 'Sending…'})).toBeDisabled();
      expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled();
      expect(screen.getByRole('radio', {name: 'Set a new password now'})).toBeDisabled();
      fireEvent.keyDown(screen.getByRole('dialog'), {key: 'Escape'});
      expect(onClose).not.toHaveBeenCalled();

      finish();
      await waitFor(() => {
        expect(onClose).toHaveBeenCalledTimes(1);
      });
    });

    it('shows a held link disabled with the reason, leaving the value to be set here', () => {
      renderWithLink('Unsuspend the user to send a reset link.');

      expect(screen.getByRole('radio', {name: 'Email the user a link to set a new password'})).toBeDisabled();
      expect(screen.getByText('Unsuspend the user to send a reset link.')).toBeVisible();
      expect(screen.getByRole('radio', {name: 'Set a new password now'})).toBeChecked();
    });

    it('opens on setting the value again after a cancelled send', () => {
      const {rerender} = renderWithLink();
      chooseLink();
      fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));

      rerender(
        <CredentialResetDialog
          open
          field={passwordField}
          onReset={onReset}
          getErrorMessage={getErrorMessage}
          resetLink={{onSend, getErrorMessage: getLinkErrorMessage}}
          onClose={onClose}
        />,
      );

      expect(screen.getByRole('radio', {name: 'Set a new password now'})).toBeChecked();
    });
  });
});
