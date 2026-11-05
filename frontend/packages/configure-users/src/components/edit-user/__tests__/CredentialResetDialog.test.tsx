// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {UpdateUserCredentialsVariables} from '../../../api/useUpdateUserCredentials';
import CredentialResetDialog from '../CredentialResetDialog';

// Mock react-i18next. Resolves inline defaults (string or {defaultValue}) like the real i18n.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, defaultValueOrOptions?: unknown, options?: unknown): string => {
      const translations: Record<string, string> = {
        'errors.USR-1024': 'One or more credential fields are not valid for this user type.',
        'errors.USR-1041':
          'The new value does not meet the requirements set for this user type, such as the allowed length or characters.',
      };
      if (translations[key]) return translations[key];

      // t(key, defaultString, { label }) — e.g. t('...', 'Reset {{label}}?', { label: 'Password' })
      if (typeof defaultValueOrOptions === 'string') {
        const vars = (options ?? {}) as Record<string, string>;
        return defaultValueOrOptions.replace(/\{\{(\w+)\}\}/g, (_, k: string) => vars[k] ?? '');
      }
      // t(key, { defaultValue, ...vars }) — the '' probe used to detect a missing translation
      if (
        defaultValueOrOptions &&
        typeof defaultValueOrOptions === 'object' &&
        'defaultValue' in defaultValueOrOptions
      ) {
        const obj = defaultValueOrOptions as Record<string, string>;
        return (obj['defaultValue'] ?? '').replace(/\{\{(\w+)\}\}/g, (_, k: string) => obj[k] ?? '');
      }
      return typeof defaultValueOrOptions === 'string' ? defaultValueOrOptions : key;
    },
  }),
}));

// Mock useUpdateUserCredentials hook.
const mockMutate = vi.fn();
const mockReset = vi.fn();
const mockUpdateCredentials: {
  mutate: ReturnType<typeof vi.fn>;
  reset: ReturnType<typeof vi.fn>;
  error: Error | null;
} = {
  mutate: mockMutate,
  reset: mockReset,
  error: null,
};
vi.mock('../../../api/useUpdateUserCredentials', () => ({
  default: () => mockUpdateCredentials,
}));

const passwordField = {fieldName: 'password', label: 'Password'};

describe('CredentialResetDialog', () => {
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdateCredentials.error = null;
  });

  it('should render dialog when open is true with a field', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Reset Password?')).toBeInTheDocument();
  });

  it('should not render dialog content when open is false', () => {
    render(<CredentialResetDialog open={false} field={passwordField} userId="user-123" onClose={mockOnClose} />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('should display warning alert and description', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    expect(
      screen.getByText(
        'A new password will be set for this user. The current password will be invalidated immediately.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText('This action cannot be undone. The current password will stop working as soon as you confirm.'),
    ).toBeInTheDocument();
  });

  it('should disable the reset button when new value is empty', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    expect(screen.getByRole('button', {name: 'Reset Password'})).toBeDisabled();
  });

  it('should call onClose when cancel button is clicked', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it('should reset mutation state when cancel is clicked', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(mockReset).toHaveBeenCalledTimes(1);
  });

  it('should show mismatch error when values do not match', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    const newInput = screen.getByPlaceholderText('Enter new password');
    const confirmInput = screen.getByPlaceholderText('Confirm new password');

    fireEvent.change(newInput, {target: {value: 'newpass123'}});
    fireEvent.change(confirmInput, {target: {value: 'differentpass'}});

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(screen.getByText('Values do not match.')).toBeInTheDocument();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('should clear mismatch error when new value input changes', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    const newInput = screen.getByPlaceholderText('Enter new password');
    const confirmInput = screen.getByPlaceholderText('Confirm new password');

    fireEvent.change(newInput, {target: {value: 'newpass123'}});
    fireEvent.change(confirmInput, {target: {value: 'differentpass'}});
    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(screen.getByText('Values do not match.')).toBeInTheDocument();

    // Changing the new value input should clear the error
    fireEvent.change(newInput, {target: {value: 'newpass456'}});

    expect(screen.queryByText('Values do not match.')).not.toBeInTheDocument();
  });

  it('should clear mismatch error when confirm value input changes', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    const newInput = screen.getByPlaceholderText('Enter new password');
    const confirmInput = screen.getByPlaceholderText('Confirm new password');

    fireEvent.change(newInput, {target: {value: 'newpass123'}});
    fireEvent.change(confirmInput, {target: {value: 'differentpass'}});
    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(screen.getByText('Values do not match.')).toBeInTheDocument();

    fireEvent.change(confirmInput, {target: {value: 'newpass123'}});

    expect(screen.queryByText('Values do not match.')).not.toBeInTheDocument();
  });

  it('should call mutate with correct data when values match', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    const newInput = screen.getByPlaceholderText('Enter new password');
    const confirmInput = screen.getByPlaceholderText('Confirm new password');

    fireEvent.change(newInput, {target: {value: 'newpass123'}});
    fireEvent.change(confirmInput, {target: {value: 'newpass123'}});

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {userId: 'user-123', data: {credentials: {password: 'newpass123'}}},
      expect.any(Object),
    );
  });

  it('should call onClose on successful mutation', async () => {
    mockMutate.mockImplementation((_vars: UpdateUserCredentialsVariables, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    const newInput = screen.getByPlaceholderText('Enter new password');
    const confirmInput = screen.getByPlaceholderText('Confirm new password');

    fireEvent.change(newInput, {target: {value: 'newpass123'}});
    fireEvent.change(confirmInput, {target: {value: 'newpass123'}});

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    await waitFor(() => {
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  it('should display the generic fallback message on mutation failure', () => {
    mockUpdateCredentials.error = new Error('Network error');

    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    expect(screen.getByText('Failed to update credentials. Please try again.')).toBeInTheDocument();
  });

  it('should display the mapped error message for a mapped credential error code', () => {
    const error = new Error('Conflict') as Error & {response?: {data?: {code: string}}};
    error.response = {data: {code: 'USR-1024'}};
    mockUpdateCredentials.error = error;

    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    expect(screen.getByText('One or more credential fields are not valid for this user type.')).toBeInTheDocument();
  });

  it('should explain that the new value does not meet the user type requirements', () => {
    const error = new Error('Bad Request') as Error & {response?: {data?: {code: string}}};
    error.response = {data: {code: 'USR-1041'}};
    mockUpdateCredentials.error = error;

    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    expect(
      screen.getByText(
        'The new value does not meet the requirements set for this user type, such as the allowed length or characters.',
      ),
    ).toBeInTheDocument();
  });

  it('should reset the mutation state when the new value input changes', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    const newInput = screen.getByPlaceholderText('Enter new password');
    fireEvent.change(newInput, {target: {value: 'newpass123'}});

    expect(mockReset).toHaveBeenCalled();
  });

  it('should reset the mutation state when the confirm value input changes', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    const confirmInput = screen.getByPlaceholderText('Confirm new password');
    fireEvent.change(confirmInput, {target: {value: 'newpass123'}});

    expect(mockReset).toHaveBeenCalled();
  });

  it('should not call mutate when new value is only whitespace', () => {
    render(<CredentialResetDialog open field={passwordField} userId="user-123" onClose={mockOnClose} />);

    const newInput = screen.getByPlaceholderText('Enter new password');
    const confirmInput = screen.getByPlaceholderText('Confirm new password');

    fireEvent.change(newInput, {target: {value: '   '}});
    fireEvent.change(confirmInput, {target: {value: '   '}});

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('should use the correct field name for different credential types', () => {
    const pinField = {fieldName: 'pin', label: 'PIN'};

    render(<CredentialResetDialog open field={pinField} userId="user-123" onClose={mockOnClose} />);

    expect(screen.getByText('Reset PIN?')).toBeInTheDocument();

    const newInput = screen.getByPlaceholderText('Enter new pin');
    const confirmInput = screen.getByPlaceholderText('Confirm new pin');

    fireEvent.change(newInput, {target: {value: '1234'}});
    fireEvent.change(confirmInput, {target: {value: '1234'}});

    fireEvent.click(screen.getByRole('button', {name: 'Reset PIN'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {userId: 'user-123', data: {credentials: {pin: '1234'}}},
      expect.any(Object),
    );
  });
});
