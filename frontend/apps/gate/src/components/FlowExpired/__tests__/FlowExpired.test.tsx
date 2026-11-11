// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowExpiryRedirect} from '@thunderid/react';
import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import FlowExpired from '../FlowExpired';

const redirect = (overrides: Partial<FlowExpiryRedirect> = {}): FlowExpiryRedirect => ({
  countdownSeconds: 10,
  isCountingDown: true,
  isRedirectFailed: false,
  secondsLeft: 7,
  url: 'https://acme.example.com',
  ...overrides,
});

describe('FlowExpired', () => {
  it('shows the countdown back to the application and focuses the heading', () => {
    render(<FlowExpired applicationName="Acme Portal" redirect={redirect()} />);

    expect(screen.getByRole('heading', {name: 'Session expired'})).toHaveFocus();
    expect(screen.getByText('Taking you back to Acme Portal in 7s.')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Back to Acme Portal now'})).toHaveAttribute(
      'href',
      'https://acme.example.com',
    );
    expect(screen.getByTestId('flow-expired-progress')).toHaveAttribute('aria-valuenow', '70');
  });

  it('does not show the countdown before it starts', () => {
    render(<FlowExpired applicationName="Acme Portal" redirect={redirect({isCountingDown: false, secondsLeft: 10})} />);

    expect(screen.getByText('Please go back to Acme Portal and try again.')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Back to Acme Portal'})).toBeInTheDocument();
    expect(screen.queryByTestId('flow-expired-progress')).not.toBeInTheDocument();
  });

  it('offers the link back once the redirect did not happen', () => {
    render(
      <FlowExpired
        applicationName="Acme Portal"
        redirect={redirect({isCountingDown: false, isRedirectFailed: true, secondsLeft: 0})}
      />,
    );

    expect(screen.getByText('Please go back to Acme Portal and try again.')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Back to Acme Portal'})).toHaveAttribute('href', 'https://acme.example.com');
    expect(screen.queryByTestId('flow-expired-progress')).not.toBeInTheDocument();
  });

  it('keeps a long application name whole in the text and truncates it only in the link', () => {
    const name = 'Acme Corporation Customer Self-Service Portal for Enterprise Partners';
    render(<FlowExpired applicationName={name} redirect={redirect({secondsLeft: 10})} />);

    expect(screen.getByText(`Taking you back to ${name} in 10s.`)).toBeInTheDocument();
    const link: HTMLElement = screen.getByRole('link', {name: `Back to ${name} now`});
    const nameInLink: HTMLElement = screen.getAllByText(name).find((element) => link.contains(element))!;
    expect(getComputedStyle(nameInLink).textOverflow).toBe('ellipsis');
    expect(getComputedStyle(nameInLink).whiteSpace).toBe('nowrap');
  });

  it('falls back to the application and offers no link or countdown without an application URL', () => {
    render(<FlowExpired applicationName="" redirect={null} />);

    expect(screen.getByText('Please go back to the application and try again.')).toBeInTheDocument();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(screen.queryByTestId('flow-expired-progress')).not.toBeInTheDocument();
  });
});
