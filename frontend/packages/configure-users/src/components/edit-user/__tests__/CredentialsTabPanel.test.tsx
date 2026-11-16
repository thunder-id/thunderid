// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CredentialsTabPanel from '../CredentialsTabPanel';

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
      return key;
    },
  }),
}));

const mockMutateAsync = vi.fn();
vi.mock('../../../api/useUpdateUserCredentials', () => ({
  default: () => ({mutateAsync: mockMutateAsync}),
}));

const credentialFields = [{fieldName: 'password', label: 'Password'}];

function failWith(code: string): void {
  const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
  error.response = {data: {code}};
  mockMutateAsync.mockRejectedValue(error);
}

async function resetPassword(): Promise<void> {
  render(<CredentialsTabPanel userId="user-123" credentialFields={credentialFields} />);

  fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));
  fireEvent.change(await screen.findByPlaceholderText('Enter new password'), {target: {value: 'newpass123'}});
  fireEvent.change(screen.getByPlaceholderText('Confirm new password'), {target: {value: 'newpass123'}});
  fireEvent.click(screen.getAllByRole('button', {name: 'Reset Password'}).at(-1)!);
}

describe('CredentialsTabPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMutateAsync.mockResolvedValue(undefined);
  });

  it('offers a reset for each credential field of the user type', () => {
    render(<CredentialsTabPanel userId="user-123" credentialFields={credentialFields} />);

    expect(screen.getByRole('button', {name: 'Reset Password'})).toBeInTheDocument();
  });

  it('resets the credential of this user through the user update call', async () => {
    await resetPassword();

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        userId: 'user-123',
        data: {credentials: {password: 'newpass123'}},
      });
    });
  });

  it('shows the generic message when the update fails', async () => {
    mockMutateAsync.mockRejectedValue(new Error('Network error'));

    await resetPassword();

    expect(await screen.findByText('Failed to update credentials. Please try again.')).toBeInTheDocument();
  });

  it('shows the mapped message for a credential error code', async () => {
    failWith('USR-1024');

    await resetPassword();

    expect(
      await screen.findByText('One or more credential fields are not valid for this user type.'),
    ).toBeInTheDocument();
  });

  it('explains that the new value does not meet the user type requirements', async () => {
    failWith('USR-1041');

    await resetPassword();

    expect(
      await screen.findByText(
        'The new value does not meet the requirements set for this user type, such as the allowed length or characters.',
      ),
    ).toBeInTheDocument();
  });
});
