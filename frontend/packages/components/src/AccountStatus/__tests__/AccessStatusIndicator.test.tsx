// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import AccessStatusIndicator from '../AccessStatusIndicator';

// The package initializes no i18next instance, so a real `t` would hand back the fallback with its
// placeholders still in it.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string, values?: Record<string, string>) => {
      const template = typeof fallback === 'string' ? fallback : key;
      return template.replace(/{{(\w+)}}/g, (_match, name: string) => values?.[name] ?? _match);
    },
  }),
}));

const report = (value: string, details?: Record<string, unknown>): never => ({details, value}) as never;

describe('AccessStatusIndicator', () => {
  it('labels an active account rather than leaving the cell blank', () => {
    // An empty status cell is indistinguishable from a value that failed to load.
    render(<AccessStatusIndicator status={report('ACTIVE')} />);

    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('reads an absent status as active', () => {
    // The server omits the field for the ordinary case rather than sending ACTIVE.
    render(<AccessStatusIndicator />);

    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('renders nothing for a status this build does not know', () => {
    const {container} = render(<AccessStatusIndicator status={report('DEACTIVATED')} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('labels a suspended account as suspended, never as locked', () => {
    // An operator who reads a suspension as a lock will wait for a timer that is never coming.
    render(<AccessStatusIndicator status={report('SUSPENDED')} />);

    expect(screen.getByText('Suspended')).toBeInTheDocument();
    expect(screen.queryByText('Locked')).not.toBeInTheDocument();
    expect(
      screen.getByLabelText('The account is suspended and cannot sign in until an administrator unsuspends it.'),
    ).toBeInTheDocument();
  });

  it("carries the operator's note in the hover text", () => {
    render(<AccessStatusIndicator status={report('SUSPENDED', {operatorNote: 'Credential exposure'})} />);

    expect(screen.getByLabelText(/Operator note: Credential exposure/)).toBeInTheDocument();
  });

  it('labels a locked account as locked', () => {
    render(<AccessStatusIndicator status={report('LOCKED')} />);

    expect(screen.getByText('Locked')).toBeInTheDocument();
  });

  it('describes an agent as authenticating rather than signing in', () => {
    render(<AccessStatusIndicator status={report('SUSPENDED')} category="agent" />);

    expect(
      screen.getByLabelText('The agent is suspended and cannot authenticate until an administrator unsuspends it.'),
    ).toBeInTheDocument();
  });

  it('describes an application as authenticating rather than signing in', () => {
    render(<AccessStatusIndicator status={report('ACTIVE')} category="application" />);

    expect(screen.getByLabelText('This application can authenticate normally.')).toBeInTheDocument();
  });
});
