// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi} from 'vitest';
import type {AccountLockForm} from '../../../utils/accountLockPayload';
import AccountLockCard from '../AccountLockCard';

const form = (overrides: Partial<AccountLockForm> = {}): AccountLockForm => ({
  enabled: true,
  granularity: 'authentication_method',
  threshold: 5,
  failureWindowSeconds: 900,
  lockDurationsSeconds: [300, 900, 3600, 86400],
  lockDecaySeconds: 86400,
  maxConsecutiveFailures: 50,
  notifyEnabled: false,
  ...overrides,
});

const renderCard = (overrides: Partial<AccountLockForm> = {}, errors = {}) =>
  render(<AccountLockCard category="user" form={form(overrides)} errors={errors} onChange={vi.fn()} />);

/** Renders the card and opens *Advanced settings*, where duration and its reset live. */
const renderAdvanced = async (overrides: Partial<AccountLockForm> = {}, errors = {}) => {
  const result = renderCard(overrides, errors);
  await userEvent.click(screen.getByTestId('advanced-user'));
  return result;
};

describe('AccountLockCard', () => {
  describe('the policy summary', () => {
    // Four number fields and a list of rungs describe a behaviour nobody can assemble from the
    // fields alone, and reading the outcome back is how a typo becomes visible.
    it('states what the settings add up to', () => {
      renderCard();

      expect(screen.getByTestId('policy-summary-user')).toHaveTextContent(
        'After 5 failed attempts with no more than 15 minutes between them, the affected sign-in method is locked. ' +
          'The first lockout lasts 5 minutes, increasing to 1 day for subsequent lockouts.',
      );
    });

    // The field validation already says what is missing.
    it('says nothing while the policy is incomplete', () => {
      renderCard({lockDurationsSeconds: []}, {lockDurationsSeconds: 'Add at least one lock duration.'});

      expect(screen.queryByTestId('policy-summary-user')).not.toBeInTheDocument();
      expect(screen.getByText('Add at least one lock duration.')).toBeVisible();
    });
  });

  describe('the card body', () => {
    it('aligns the threshold, duration count and unit control heights', () => {
      render(
        <div style={{width: 900}}>
          <AccountLockCard category="user" form={form()} errors={{}} onChange={vi.fn()} />
        </div>,
      );
      const field = (testId: string): Element => screen.getByTestId(testId).closest('.MuiInputBase-root')!;
      const threshold = field('threshold-user').getBoundingClientRect();
      const duration = field('window-user').getBoundingClientRect();
      const unit = screen.getByTestId('window-user-unit').querySelector('.MuiInputBase-root')!.getBoundingClientRect();
      expect(threshold.height).toBe(40);
      expect(duration.height).toBe(threshold.height);
      expect(unit.height).toBe(threshold.height);
      expect(unit.top).toBe(threshold.top);
    });

    // The two decisions an operator actually makes: when a lock forms, and what it holds.
    it('asks when to lock and what to hold, on the page', () => {
      renderCard();

      expect(screen.getByText('Lock after')).toBeVisible();
      expect(screen.getByText('failed attempts within')).toBeVisible();
      expect(screen.getByText('Lockout applies to')).toBeVisible();
      expect(screen.getByText('Only the affected sign-in method')).toBeVisible();
      // The window is a gap between failures, not a fixed period, and the card says so.
      expect(screen.getByText('A longer gap between two failed attempts starts the count again.')).toBeVisible();
      expect(screen.getByText('All sign-in methods')).toBeVisible();
    });

    // The section renders no settings while the toggle is off, so it is a title, a description and
    // a switch. That is why the agent client-secret caveat is in the description.
    it('hides every setting when locking is off', () => {
      renderCard({enabled: false});

      expect(screen.queryByText('Lockout applies to')).not.toBeInTheDocument();
      expect(screen.queryByText('Lock after')).not.toBeInTheDocument();
      expect(screen.getByText('Temporarily block sign-in after repeated failed attempts.')).toBeVisible();
    });

    // With every setting hidden, the summary is what says a switched-off card locks nothing.
    it('says that failed attempts lock nobody while locking is off', () => {
      renderCard({enabled: false});
      expect(screen.getByTestId('policy-summary-user')).toHaveTextContent(
        'Automatic locking is off. Failed sign-in attempts do not lock users.',
      );
    });

    it('says the same for agents in their own words', () => {
      render(<AccountLockCard category="agent" form={form({enabled: false})} errors={{}} onChange={vi.fn()} />);
      expect(screen.getByTestId('policy-summary-agent')).toHaveTextContent(
        'Automatic locking is off. Failed authentication attempts do not lock agents.',
      );
    });

    it('names the lockout scope choice after its visible label', async () => {
      const onChange = vi.fn();
      render(<AccountLockCard category="user" form={form()} errors={{}} onChange={onChange} />);

      const scope = screen.getByRole('radiogroup', {name: 'Lockout applies to'});
      await userEvent.click(within(scope).getByRole('radio', {name: /^All sign-in methods/}));
      expect(onChange).toHaveBeenLastCalledWith({granularity: 'account'});
    });
  });

  describe('the advanced disclosure', () => {
    // Duration and its reset are tuning. Collapsed is the default because most operators only decide
    // when a lock forms and what it holds.
    it('keeps the duration and its reset collapsed', () => {
      renderCard();

      expect(screen.getByTestId('advanced-user')).toBeVisible();
      expect(screen.queryByText('Lockout duration')).not.toBeInTheDocument();
      expect(screen.queryByText('Reset lockout progression')).not.toBeInTheDocument();
    });

    // One disclosure holding both, rather than a section each: they are the same decision.
    it('holds both of them together once opened', async () => {
      await renderAdvanced();

      expect(screen.getByText('Lockout duration')).toBeVisible();
      expect(screen.getByText('Reset lockout progression')).toBeVisible();
    });

    // The reason collapsing is safe at all: the reader still learns how long a lock lasts without
    // opening anything, so the disclosure hides the controls and not the outcome.
    it('leaves the duration stated in the summary while collapsed', () => {
      renderCard();

      expect(screen.getByTestId('policy-summary-user')).toHaveTextContent(
        'The first lockout lasts 5 minutes, increasing to 1 day for subsequent lockouts.',
      );
    });

    // A disclosure that can swallow a blocking message is worse than no disclosure.
    it('is forced open by a validation error inside it', () => {
      renderCard({lockDurationsSeconds: [0]}, {lockDurationsSeconds: 'A lock duration cannot be negative.'});

      expect(screen.getByText('A lock duration cannot be negative.')).toBeVisible();
    });

    // And held open: clicking the summary must not put the error back out of sight.
    it('cannot be collapsed while that error stands', async () => {
      renderCard({lockDecaySeconds: -1}, {lockDecaySeconds: 'A reset period cannot be negative.'});
      await userEvent.click(screen.getByTestId('advanced-user'));

      expect(screen.getByText('A reset period cannot be negative.')).toBeVisible();
    });
  });

  describe('the lock notification', () => {
    // Users only. An application has no account holder, and an agent's audience is its controller
    // rather than an address on the agent's own record.
    it('is offered on the user card', async () => {
      await renderAdvanced();

      expect(screen.getByText('Notify user by email')).toBeVisible();
    });

    // The email goes only to the primary email, so there is nothing else to configure.
    it('is a checkbox that says who gets the email', async () => {
      const onChange = vi.fn();
      render(<AccountLockCard category="user" form={form()} errors={{}} onChange={onChange} />);
      await userEvent.click(screen.getByTestId('advanced-user'));

      const checkbox = screen.getByRole('checkbox', {name: 'Notify user by email'});
      expect(checkbox).not.toBeChecked();
      expect(checkbox).toHaveAccessibleDescription(
        "Send an email to the user's primary email address when a sign-in method is locked. Users without a primary email address won't receive notifications.",
      );
      await userEvent.click(checkbox);
      expect(onChange).toHaveBeenCalledWith({notifyEnabled: true});
    });

    it('shows a saved choice and asks for nothing more', async () => {
      await renderAdvanced({notifyEnabled: true});

      expect(screen.getByRole('checkbox', {name: 'Notify user by email'})).toBeChecked();
      expect(screen.queryByText(/recipient/i)).not.toBeInTheDocument();
    });

    it('is read-only when the card is', async () => {
      render(<AccountLockCard category="user" form={form()} errors={{}} onChange={vi.fn()} disabled />);
      await userEvent.click(screen.getByTestId('advanced-user'));

      expect(screen.getByRole('checkbox', {name: 'Notify user by email'})).toBeDisabled();
    });

    it('is absent on the agent card', async () => {
      render(<AccountLockCard category="agent" form={form()} errors={{}} onChange={vi.fn()} />);
      await userEvent.click(screen.getByTestId('advanced-agent'));

      expect(screen.queryByText('Notify user by email')).not.toBeInTheDocument();
    });
  });

  describe('the escalation reset', () => {
    // A single rung repeats, so a period that returns the escalation to the first duration is a
    // control with no effect.
    it('is absent while there is nothing to reset', async () => {
      await renderAdvanced({lockDurationsSeconds: [900], lockDecaySeconds: 0});

      expect(screen.queryByText('Reset lockout progression')).not.toBeInTheDocument();
    });

    it('appears once the ladder escalates', async () => {
      await renderAdvanced();

      expect(screen.getByText('Reset lockout progression')).toBeVisible();
    });

    // A deployment that already set one must not have it dropped by a save from a screen that never
    // showed it.
    it('stays visible for a value already configured against a single rung', async () => {
      await renderAdvanced({lockDurationsSeconds: [900], lockDecaySeconds: 3600});

      expect(screen.getByText('Reset lockout progression')).toBeVisible();
    });

    // Choosing it on a single rung would write the zero that takes this very section off the screen.
    // With no escalation the two settings mean the same thing, so nothing is lost by withholding it.
    it('offers "never reset" only while the ladder escalates', async () => {
      await renderAdvanced({lockDurationsSeconds: [900], lockDecaySeconds: 3600});
      await userEvent.click(within(screen.getByTestId('decay-user-unit')).getByRole('combobox'));

      expect(screen.queryByRole('option', {name: 'Never reset'})).not.toBeInTheDocument();
    });

    it('offers it once there is an escalation to reset', async () => {
      await renderAdvanced();
      await userEvent.click(within(screen.getByTestId('decay-user-unit')).getByRole('combobox'));

      // Presence rather than visibility, matching the negative case above: the menu opens through a
      // transition, and asserting the painted state races it.
      expect(await screen.findByRole('option', {name: 'Never reset'})).toBeInTheDocument();
    });
  });

  describe('the failure window', () => {
    it('offers a window that never resets on time', async () => {
      const onChange = vi.fn();
      render(<AccountLockCard category="user" form={form()} errors={{}} onChange={onChange} />);
      await userEvent.click(within(screen.getByTestId('window-user-unit')).getByRole('combobox'));
      await userEvent.click(await screen.findByRole('option', {name: 'No time limit'}));

      expect(onChange).toHaveBeenCalledWith({failureWindowSeconds: 0});
    });

    it('shows a zero window as no time limit, with no number to enter', () => {
      renderCard({failureWindowSeconds: 0});

      expect(screen.queryByTestId('window-user')).not.toBeInTheDocument();
      expect(screen.getByTestId('policy-summary-user')).toHaveTextContent(
        'After 5 failed attempts without a successful sign-in, the affected sign-in method is locked.',
      );
    });
  });

  describe('the escalation ladder', () => {
    it('is a single unlabelled duration until there is something to escalate', async () => {
      await renderAdvanced({lockDurationsSeconds: [900]});

      expect(screen.queryByText('1st lockout')).not.toBeInTheDocument();
      expect(screen.getByText('Add a longer duration for subsequent lockouts')).toBeVisible();
    });

    it('numbers the rows and names the repeating one once it escalates', async () => {
      await renderAdvanced();

      expect(screen.getByText('1st lockout')).toBeVisible();
      expect(screen.getByText('3rd lockout')).toBeVisible();
      // The last rung repeats, and this label is the only place that is legible.
      expect(screen.getByText('4th and later')).toBeVisible();
      expect(screen.getByText('Add duration')).toBeVisible();
    });

    // A lock that never lifts on its own is a supported policy, chosen once after the timed
    // durations rather than reachable only through the API.
    it('offers holding later locks until an administrator unlocks', async () => {
      const onChange = vi.fn();
      render(
        <AccountLockCard category="user" form={form({lockDurationsSeconds: [900]})} errors={{}} onChange={onChange} />,
      );
      await userEvent.click(screen.getByTestId('advanced-user'));

      await userEvent.click(
        screen.getByRole('radio', {name: /^Require account recovery or administrator intervention/}),
      );

      expect(onChange).toHaveBeenCalledWith({lockDurationsSeconds: [900, 0]});
    });

    // Rungs before a held end are still reached, so the list can always grow.
    it('keeps inviting another rung when later locks are held', async () => {
      await renderAdvanced({lockDurationsSeconds: [300, 0]});

      expect(screen.getByText('Add duration')).toBeVisible();
      expect(screen.getByTestId('policy-summary-user')).toHaveTextContent(
        'The first lockout lasts 5 minutes. Subsequent lockouts last until account recovery or administrator intervention.',
      );
    });
  });

  describe('the consecutive-failure limit', () => {
    it('edits the limit in the advanced settings', async () => {
      const onChange = vi.fn();
      render(<AccountLockCard category="user" form={form()} errors={{}} onChange={onChange} />);
      await userEvent.click(screen.getByTestId('advanced-user'));

      const field = screen.getByTestId('ceiling-user');
      expect(field).toHaveValue(50);
      // The card is controlled, so the change event carries the whole new value.
      fireEvent.change(field, {target: {value: '7'}});

      expect(onChange).toHaveBeenLastCalledWith({maxConsecutiveFailures: 7});
    });

    // An agent has no self-service recovery, so its copy names only the administrator.
    it('tells an agent that only an administrator releases the lock', async () => {
      render(<AccountLockCard category="agent" form={form()} errors={{}} onChange={vi.fn()} />);
      await userEvent.click(screen.getByTestId('advanced-agent'));

      expect(screen.getByText(/Require administrator intervention when/)).toBeVisible();
      expect(screen.queryByText(/account recovery/)).not.toBeInTheDocument();
    });

    // An agent authenticates rather than signs in, so its card names authentication methods.
    it('names authentication methods on the agent card', async () => {
      render(<AccountLockCard category="agent" form={form()} errors={{}} onChange={vi.fn()} />);

      expect(screen.getByText('Only the affected authentication method')).toBeVisible();
      expect(screen.getByText('All authentication methods')).toBeVisible();
      expect(screen.queryByText(/sign-in method/)).not.toBeInTheDocument();

      await userEvent.click(screen.getByTestId('advanced-agent'));
      expect(screen.getByRole('radio', {name: /^Require administrator intervention/})).toBeInTheDocument();
      expect(screen.getByText('Keep the lockout in place until an administrator unlocks the agent.')).toBeVisible();
    });

    it('holds the advanced settings open while the limit is invalid', () => {
      renderCard({maxConsecutiveFailures: 3}, {maxConsecutiveFailures: 'Enter 0 for no limit.'});

      expect(screen.getByText('Enter 0 for no limit.')).toBeVisible();
    });
  });
});
