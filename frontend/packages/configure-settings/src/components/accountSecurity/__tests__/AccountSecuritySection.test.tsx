// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {afterEach, describe, it, expect, vi} from 'vitest';

const config = vi.hoisted(() => ({
  readOnly: {} as Record<string, unknown>,
  writable: {} as Record<string, unknown>,
  merged: undefined as Record<string, unknown> | undefined,
  isLoading: false,
  error: null as Error | null,
}));
const update = vi.hoisted(() => ({error: null as Error | null, mutate: vi.fn()}));

vi.mock('../../../api/useGetAccountAccessConfig', () => ({
  default: () => ({
    data: config.isLoading
      ? undefined
      : {readOnly: config.readOnly, writable: config.writable, merged: config.merged ?? config.readOnly},
    isLoading: config.isLoading,
    error: config.error,
    refetch: vi.fn(),
  }),
}));
vi.mock('../../../api/useUpdateAccountAccessConfig', () => ({
  default: () => ({
    mutate: update.mutate,
    isPending: false,
    isError: update.error !== null,
    error: update.error,
    reset: vi.fn(),
  }),
}));
vi.mock('../../../api/useGetAccountLifecycleFlows', () => ({
  default: () => ({data: {}, isLoading: false, error: null, refetch: vi.fn()}),
}));

const {default: AccountSecuritySection} = await import('../AccountSecuritySection');

/** The SettingsCard holding a title. */
const card = (title: string): HTMLElement => screen.getByText(title).closest<HTMLElement>('.MuiPaper-root')!;

describe('AccountSecuritySection', () => {
  afterEach(() => {
    config.readOnly = {};
    config.writable = {};
    config.merged = undefined;
    config.isLoading = false;
    config.error = null;
    update.error = null;
    update.mutate.mockReset();
  });

  // Lock, then where existing access is ended, then the unrelated recording switch.
  it('shows the lock, suspension and sign-in activity cards in that order', () => {
    renderWithProviders(<AccountSecuritySection />);

    const titles = ['Automatic account lock', 'Account suspension', 'Sign-in activity'].map(card);
    expect(titles[0].compareDocumentPosition(titles[1]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(titles[1].compareDocumentPosition(titles[2]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  // "Every sign-in method" sounds like it reaches live sessions, and it does not. The subtitle says
  // so and points at the card that does end access.
  it('says that a lock does not end existing access and points at suspension', () => {
    renderWithProviders(<AccountSecuritySection />);

    const description = within(card('Automatic account lock'));
    expect(description.getByText(/existing sessions and tokens stay active/)).toBeVisible();
    expect(description.getByText(/as described in Account suspension/)).toBeVisible();
  });

  it('holds one card per lockable category inside the lock card', () => {
    renderWithProviders(<AccountSecuritySection />);

    const section = within(card('Automatic account lock'));
    expect(section.getByTestId('account-lock-user')).toBeInTheDocument();
    expect(section.getByTestId('account-lock-agent')).toBeInTheDocument();
  });

  // One section, one save: either card's edit reaches the same bar and the same payload.
  it('saves a sign-in activity edit and a lock edit together', async () => {
    config.merged = {
      user: {default: {enabled: true, threshold: 5, failureWindowSeconds: 900, lockDurationsSeconds: [300]}},
    };
    renderWithProviders(<AccountSecuritySection />);

    await userEvent.click(screen.getByRole('switch', {name: 'Record sign-in activity for Agents'}));
    expect(screen.getByText('You have unsaved changes')).toBeVisible();
    await userEvent.click(screen.getByRole('switch', {name: 'Automatic lock for Users'}));
    await userEvent.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(update.mutate).toHaveBeenCalledWith(
      {data: {user: {default: {enabled: false}}, agent: {recordLastLogin: true}}},
      expect.anything(),
    );
  });

  // A declarative accountAccess locks the section on the server, so nothing here may look editable.
  it('disables both editable cards and explains where they live when managed declaratively', () => {
    config.readOnly = {user: {default: {enabled: true, threshold: 5}}};

    renderWithProviders(<AccountSecuritySection />);

    expect(screen.getByRole('switch', {name: 'Automatic lock for Users'})).toBeDisabled();
    expect(screen.getByRole('switch', {name: 'Automatic lock for Agents'})).toBeDisabled();
    expect(screen.getByRole('switch', {name: 'Record sign-in activity for Users'})).toBeDisabled();
    expect(within(card('Sign-in activity')).getByTestId('account-lock-read-only-hint')).toHaveTextContent(
      /managed declaratively/,
    );
    expect(within(card('Automatic account lock')).getByTestId('account-lock-read-only-hint')).toBeInTheDocument();
  });

  it('leaves the settings editable without a declarative layer', () => {
    renderWithProviders(<AccountSecuritySection />);

    expect(screen.getByRole('switch', {name: 'Automatic lock for Users'})).toBeEnabled();
    expect(screen.getByRole('switch', {name: 'Record sign-in activity for Users'})).toBeEnabled();
    expect(screen.queryByTestId('account-lock-read-only-hint')).not.toBeInTheDocument();
  });

  // Both editable cards read the same section, and each says so where its settings would be.
  it('reports a failed read in both editable cards but still lists the suspension flows', () => {
    config.error = new Error('boom');

    renderWithProviders(<AccountSecuritySection />);

    expect(
      within(card('Automatic account lock')).getByText('Failed to load the account security settings.'),
    ).toBeVisible();
    expect(within(card('Sign-in activity')).getByText('Failed to load the account security settings.')).toBeVisible();
    expect(within(card('Account suspension')).getByRole('region', {name: 'Users'})).toBeInTheDocument();
  });

  describe('when a save is refused', () => {
    it('reports an invalid value in account security terms', async () => {
      update.error = Object.assign(new Error('Request failed'), {response: {data: {code: 'SCF-1003'}}});

      renderWithProviders(<AccountSecuritySection />);
      await userEvent.click(screen.getByRole('switch', {name: 'Automatic lock for Users'}));

      expect(
        screen.getByText('One or more account security settings are not valid. Check the values and try again.'),
      ).toBeInTheDocument();
      expect(screen.queryByText(/origins are not valid/)).not.toBeInTheDocument();
    });
  });
});
