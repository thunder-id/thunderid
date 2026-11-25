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

let resetFlowHandle = '';
let heldReason = '';
vi.mock('../../../hooks/useCredentialResetLink', () => ({
  default: () => ({handle: resetFlowHandle, heldReason}),
}));

const mockSendMutateAsync = vi.fn();
vi.mock('../../../api/useSendUserCredentialReset', () => ({
  default: () => ({mutateAsync: mockSendMutateAsync}),
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
    mockSendMutateAsync.mockResolvedValue(undefined);
    resetFlowHandle = '';
    heldReason = '';
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

  describe('reset link', () => {
    const fields = [
      {fieldName: 'password', label: 'Password'},
      {fieldName: 'secret', label: 'Secret'},
      {fieldName: 'pin', label: 'PIN'},
    ];
    const textCredentials = ['password', 'secret'];
    const LINK_SECRET = {name: 'Email the user a link to set a new secret'};

    beforeEach(() => {
      resetFlowHandle = 'default-admin-user-credential-reset-flow';
    });

    function openReset(label: string, accessStatus?: 'SUSPENDED'): void {
      render(
        <CredentialsTabPanel
          userId="user-123"
          credentialFields={fields}
          resetLinkFieldNames={textCredentials}
          accessStatus={accessStatus}
        />,
      );
      fireEvent.click(screen.getByRole('button', {name: `Reset ${label}`}));
    }

    it('keeps one reset button per credential, with no separate link button', () => {
      render(<CredentialsTabPanel userId="user-123" credentialFields={fields} resetLinkFieldNames={textCredentials} />);

      expect(screen.getAllByRole('button')).toHaveLength(3);
      expect(screen.queryByRole('button', {name: /link/i})).not.toBeInTheDocument();
    });

    it('offers the link in the reset dialog of a text credential', () => {
      openReset('Secret');

      expect(screen.getByRole('radio', LINK_SECRET)).toBeEnabled();
    });

    it('does not offer the link for a credential that is not text', () => {
      openReset('PIN');

      expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    });

    it('is not offered when no flow is configured', () => {
      resetFlowHandle = '';
      openReset('Secret');

      expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    });

    it('starts the flow for this user and the chosen credential', async () => {
      openReset('Secret');

      fireEvent.click(screen.getByRole('radio', LINK_SECRET));
      fireEvent.click(screen.getByRole('button', {name: 'Send reset link'}));

      await waitFor(() => {
        expect(mockSendMutateAsync).toHaveBeenCalledWith({
          userId: 'user-123',
          credential: 'secret',
          label: 'Secret',
          handle: 'default-admin-user-credential-reset-flow',
        });
      });
      expect(mockMutateAsync).not.toHaveBeenCalled();
    });

    it('shows the users message for a refused send', async () => {
      const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
      error.response = {data: {code: 'UNKNOWN'}};
      mockSendMutateAsync.mockRejectedValue(error);
      openReset('Secret');

      fireEvent.click(screen.getByRole('radio', LINK_SECRET));
      fireEvent.click(screen.getByRole('button', {name: 'Send reset link'}));

      expect(await screen.findByText('Could not send the reset link. Please try again.')).toBeInTheDocument();
    });

    it('is disabled with the reason for a held user, leaving the direct reset available', async () => {
      heldReason = 'Unsuspend the user to send a reset link.';
      openReset('Secret', 'SUSPENDED');

      expect(screen.getByRole('radio', LINK_SECRET)).toBeDisabled();
      // The dialog fades in, so its text is visible a moment after it opens.
      await waitFor(() => {
        expect(screen.getByText('Unsuspend the user to send a reset link.')).toBeVisible();
      });
      expect(screen.getByRole('radio', {name: 'Set a new secret now'})).toBeChecked();
    });
  });
});
