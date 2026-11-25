// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CredentialResetSection from '../CredentialResetSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string, values?: Record<string, string>) => {
      const template = typeof fallback === 'string' ? fallback : key;
      return template.replace(/{{(\w+)}}/g, (_match, name: string) => values?.[name] ?? _match);
    },
  }),
}));

const fields = [{fieldName: 'password', label: 'Password'}];

describe('CredentialResetSection', () => {
  const onReset = vi.fn();
  const getErrorMessage = vi.fn((error: Error) => error.message);

  beforeEach(() => {
    vi.clearAllMocks();
    onReset.mockResolvedValue(undefined);
  });

  it('shows nothing when there is no credential to reset', () => {
    const {container} = render(
      <CredentialResetSection fields={[]} onReset={onReset} getErrorMessage={getErrorMessage} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it('offers a reset button for each credential', () => {
    render(
      <CredentialResetSection
        fields={[...fields, {fieldName: 'pin', label: 'PIN'}]}
        onReset={onReset}
        getErrorMessage={getErrorMessage}
      />,
    );

    expect(screen.getByRole('button', {name: 'Reset Password'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Reset PIN'})).toBeInTheDocument();
  });

  it('disables the buttons when told to', () => {
    render(<CredentialResetSection fields={fields} onReset={onReset} getErrorMessage={getErrorMessage} disabled />);

    expect(screen.getByRole('button', {name: 'Reset Password'})).toBeDisabled();
  });

  it('opens the dialog for a credential and hands its new value to the update', async () => {
    render(<CredentialResetSection fields={fields} onReset={onReset} getErrorMessage={getErrorMessage} />);

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));
    fireEvent.change(await screen.findByPlaceholderText('Enter new password'), {target: {value: 'N3w-pass!'}});
    fireEvent.change(screen.getByPlaceholderText('Confirm new password'), {target: {value: 'N3w-pass!'}});
    fireEvent.click(screen.getAllByRole('button', {name: 'Reset Password'}).at(-1)!);

    await waitFor(() => {
      expect(onReset).toHaveBeenCalledWith(fields[0], 'N3w-pass!');
    });
    await waitFor(() => {
      expect(screen.queryByText('Reset Password?')).not.toBeInTheDocument();
    });
  });

  it("offers a credential's reset link in its dialog only", async () => {
    const onSend = vi.fn();
    render(
      <CredentialResetSection
        fields={[...fields, {fieldName: 'pin', label: 'PIN'}]}
        onReset={onReset}
        getErrorMessage={getErrorMessage}
        getResetLink={(field) => (field.fieldName === 'password' ? {onSend, getErrorMessage} : undefined)}
      />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Reset PIN'}));
    expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: 'Reset Password'}));
    expect(screen.getByRole('radio', {name: 'Email the user a link to set a new password'})).toBeEnabled();
  });
});
