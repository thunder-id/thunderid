// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi} from 'vitest';
import AccessStatusIndicator from '../AccessStatusIndicator';

// The package initializes no i18next instance, so the fallback is the rendered copy.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string, values?: Record<string, string>) => {
      const template = typeof fallback === 'string' ? fallback : key;
      return template.replace(/{{(\w+)}}/g, (_match, name: string) => values?.[name] ?? _match);
    },
  }),
}));

const report = (value: string, details?: Record<string, unknown>): never => ({details, value}) as never;

/**
 * Tabs to the indicator and returns the hint its tooltip shows.
 */
async function hintOnFocus(): Promise<string> {
  await userEvent.setup().tab();

  return (await screen.findByRole('tooltip')).textContent ?? '';
}

describe('AccessStatusIndicator', () => {
  it('labels an active account rather than leaving the cell blank', () => {
    render(<AccessStatusIndicator status={report('ACTIVE')} />);

    expect(screen.getByRole('img', {name: 'Active'})).toBeInTheDocument();
  });

  it('reads an absent status as active', () => {
    render(<AccessStatusIndicator />);

    expect(screen.getByRole('img', {name: 'Active'})).toBeInTheDocument();
  });

  it('renders nothing for a status this build does not know', () => {
    const {container} = render(<AccessStatusIndicator status={report('DEACTIVATED')} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('labels a suspended account as suspended, never as locked', async () => {
    render(<AccessStatusIndicator status={report('SUSPENDED')} />);

    expect(screen.getByRole('img', {name: 'Suspended'})).toBeInTheDocument();
    expect(screen.queryByText('Locked')).not.toBeInTheDocument();
    expect(await hintOnFocus()).toBe(
      'The account is suspended and cannot sign in until an administrator unsuspends it.',
    );
  });

  it('shows its hint to keyboard users, as the description of the focused indicator', async () => {
    render(<AccessStatusIndicator status={report('LOCKED')} />);

    await hintOnFocus();

    const indicator = screen.getByRole('img', {name: 'Locked'});
    expect(indicator).toHaveFocus();
    expect(indicator).toHaveAccessibleDescription(/The account is locked and cannot sign in/);
  });

  it("carries the operator's note in the hint", async () => {
    render(<AccessStatusIndicator status={report('SUSPENDED', {operatorNote: 'Credential exposure'})} />);

    expect(await hintOnFocus()).toContain('Operator note: Credential exposure');
  });

  it('describes an agent as authenticating rather than signing in', async () => {
    render(<AccessStatusIndicator status={report('SUSPENDED')} category="agent" />);

    expect(await hintOnFocus()).toBe(
      'The agent is suspended and cannot authenticate until an administrator unsuspends it.',
    );
  });

  it('describes an application as authenticating rather than signing in', async () => {
    render(<AccessStatusIndicator status={report('ACTIVE')} category="application" />);

    expect(await hintOnFocus()).toBe('This application can authenticate normally.');
  });

  it('takes the tab order it is given, so a grid cell keeps its roving focus', () => {
    const {unmount} = render(<AccessStatusIndicator status={report('LOCKED')} />);
    expect(screen.getByRole('img', {name: 'Locked'})).toHaveAttribute('tabindex', '0');
    unmount();

    render(<AccessStatusIndicator status={report('LOCKED')} tabIndex={-1} />);
    expect(screen.getByRole('img', {name: 'Locked'})).toHaveAttribute('tabindex', '-1');
  });
});
