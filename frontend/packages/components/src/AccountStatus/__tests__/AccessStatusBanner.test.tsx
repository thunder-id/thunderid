// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import AccessStatusBanner from '../AccessStatusBanner';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string, values?: Record<string, string>) => {
      const template = typeof fallback === 'string' ? fallback : key;
      return template.replace(/{{(\w+)}}/g, (_match, name: string) => values?.[name] ?? _match);
    },
  }),
}));

const report = (value: string, details?: Record<string, unknown>): never => ({details, value}) as never;

describe('AccessStatusBanner', () => {
  it('renders nothing for an active, absent or unknown status', () => {
    // The page has nothing to report, and a banner saying so would be one an operator stops reading.
    for (const status of [report('ACTIVE'), undefined, report('DEACTIVATED')]) {
      const {container, unmount} = render(<AccessStatusBanner status={status} />);
      expect(container).toBeEmptyDOMElement();
      unmount();
    }
  });

  it('warns that an administrator suspended the account', () => {
    render(<AccessStatusBanner status={report('SUSPENDED')} />);

    const alert = screen.getByRole('status');
    expect(alert.className).toMatch(/Warning/);
    expect(alert).toHaveTextContent('Suspended');
    expect(alert).toHaveTextContent(
      'The account is suspended and cannot sign in until an administrator unsuspends it.',
    );
  });

  it("shows the operator's note only where one was recorded", () => {
    const {unmount} = render(
      <AccessStatusBanner status={report('SUSPENDED', {operatorNote: 'Credential exposure'})} />,
    );
    expect(screen.getByRole('status')).toHaveTextContent('Operator note: Credential exposure');
    unmount();

    render(<AccessStatusBanner status={report('SUSPENDED')} />);
    expect(screen.getByRole('status')).not.toHaveTextContent('Operator note:');
  });

  it('informs, rather than warns, about a lock', () => {
    render(<AccessStatusBanner status={report('LOCKED')} />);

    const alert = screen.getByRole('status');
    expect(alert.className).toMatch(/Info/);
    expect(alert).toHaveTextContent('Locked');
  });

  it('says a post-suspension lock will not lift on its own', () => {
    render(<AccessStatusBanner status={report('LOCKED', {reason: 'SUSPENSION_RELEASE_LOCK'})} />);

    expect(screen.getByRole('status')).toHaveTextContent(
      'remains locked until the user completes account recovery or an administrator unlocks it',
    );
  });

  it('promises a lock lifts on its own only when it has an expiry', () => {
    const timed = {lockedMethods: [{expiresAt: '2026-10-07T10:00:00Z', method: 'account'}], reason: 'FAILED_ATTEMPTS'};
    const {unmount} = render(<AccessStatusBanner status={report('LOCKED', timed)} />);
    expect(screen.getByRole('status')).toHaveTextContent('It unlocks automatically');
    unmount();

    const permanent = {lockedMethods: [{method: 'account'}], reason: 'FAILED_ATTEMPTS'};
    render(<AccessStatusBanner status={report('LOCKED', permanent)} />);
    expect(screen.getByRole('status')).not.toHaveTextContent('It unlocks automatically');
    // A held lockout ends on account recovery or an unlock, so the hint says so.
    expect(screen.getByRole('status')).toHaveTextContent(
      'It stays locked until the user completes account recovery or an administrator unlocks it.',
    );
    // That hint is the whole story; no lock line repeats it.
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
  });

  it('says when a lock on the whole account ends', () => {
    const expiresAt = '2026-10-07T10:00:00Z';
    render(
      <AccessStatusBanner
        status={report('LOCKED', {lockedMethods: [{expiresAt, method: 'account'}], reason: 'FAILED_ATTEMPTS'})}
      />,
    );

    const alert = screen.getByRole('status');
    expect(within(alert).getByText('Locked')).toBeInTheDocument();
    expect(
      within(alert)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual([`Locked until ${new Date(expiresAt).toLocaleString()}`]);
  });

  it('lists each locked sign-in method and its expiry, without saying the account is locked', () => {
    const expiresAt = '2026-10-07T10:00:00Z';
    const someMethods = {
      lockedMethods: [{expiresAt, method: 'credential'}, {method: 'otp'}],
      reason: 'FAILED_ATTEMPTS',
    };
    render(<AccessStatusBanner status={report('LOCKED', someMethods)} />);

    const alert = screen.getByRole('status');
    expect(within(alert).getByText('Sign-in locked for some methods')).toBeInTheDocument();
    expect(alert).toHaveTextContent('Other sign-in methods are not affected.');
    expect(alert).not.toHaveTextContent('The account is locked');
    expect(
      within(alert)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual([
      `Password or other stored credential: locked until ${new Date(expiresAt).toLocaleString()}`,
      'One-time password: locked until released',
    ]);
  });

  it('describes an agent as authenticating rather than signing in', () => {
    render(<AccessStatusBanner status={report('SUSPENDED')} category="agent" />);

    expect(screen.getByRole('status')).toHaveTextContent('cannot authenticate until an administrator unsuspends it.');
  });
});
