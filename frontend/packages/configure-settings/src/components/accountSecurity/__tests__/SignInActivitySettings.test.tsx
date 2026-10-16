// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi} from 'vitest';
import type {AccountLockCategory} from '../../../models/accountAccess';
import type {AccountLockForm} from '../../../utils/accountLockPayload';
import SignInActivitySettings from '../SignInActivitySettings';

const form = (overrides: Partial<AccountLockForm> = {}): AccountLockForm => ({
  enabled: true,
  granularity: 'authentication_method',
  threshold: 5,
  failureWindowSeconds: 900,
  lockDurationsSeconds: [300],
  lockDecaySeconds: 0,
  notifyEnabled: false,
  notifyRecipientAttribute: 'email',
  recordLastLogin: false,
  lastLoginResolutionSeconds: 3600,
  ...overrides,
});

const renderSettings = (
  user: Partial<AccountLockForm> = {},
  agent: Partial<AccountLockForm> = {},
  disabled = false,
): ReturnType<typeof vi.fn> => {
  const onChange = vi.fn<(category: AccountLockCategory, patch: Partial<AccountLockForm>) => void>();
  render(
    <SignInActivitySettings forms={{user: form(user), agent: form(agent)}} onChange={onChange} disabled={disabled} />,
  );
  return onChange;
};

const resolutionSelect = (category: AccountLockCategory): HTMLElement =>
  within(screen.getByTestId(`sign-in-activity-resolution-${category}`)).getByRole('combobox');

describe('SignInActivitySettings', () => {
  it('switches recording on for one category', async () => {
    const onChange = renderSettings();

    await userEvent.click(screen.getByRole('switch', {name: 'Record sign-in activity for Agents'}));

    expect(onChange).toHaveBeenCalledWith('agent', {recordLastLogin: true});
  });

  // The interval only matters while recording, and it belongs to each category.
  it('asks for the interval of each category only while it records', () => {
    renderSettings({recordLastLogin: true});

    expect(resolutionSelect('user')).toHaveTextContent('At most every 1 hour');
    expect(screen.queryByTestId('sign-in-activity-resolution-agent')).not.toBeInTheDocument();
  });

  it('offers every sign-in and the preset intervals', async () => {
    const onChange = renderSettings({}, {recordLastLogin: true});
    await userEvent.click(resolutionSelect('agent'));

    const options = (await screen.findAllByRole('option')).map((option) => option.textContent);
    expect(options).toEqual([
      'On every sign-in',
      'At most every 5 minutes',
      'At most every 15 minutes',
      'At most every 1 hour',
      'At most every 1 day',
    ]);

    await userEvent.click(screen.getByRole('option', {name: 'On every sign-in'}));
    expect(onChange).toHaveBeenCalledWith('agent', {lastLoginResolutionSeconds: 0});
  });

  // A save would otherwise rewrite an interval set through the API.
  it('lists a stored interval that is not a preset as it is', async () => {
    renderSettings({recordLastLogin: true, lastLoginResolutionSeconds: 7200});

    expect(resolutionSelect('user')).toHaveTextContent('At most every 2 hours');
    await userEvent.click(resolutionSelect('user'));
    expect(await screen.findByRole('option', {name: 'At most every 2 hours'})).toBeInTheDocument();
  });

  it('is read-only when the section is managed declaratively', () => {
    renderSettings({recordLastLogin: true}, {}, true);

    expect(screen.getByRole('switch', {name: 'Record sign-in activity for Users'})).toBeDisabled();
    expect(screen.getByRole('switch', {name: 'Record sign-in activity for Agents'})).toBeDisabled();
    expect(resolutionSelect('user')).toHaveAttribute('aria-disabled', 'true');
  });
});
