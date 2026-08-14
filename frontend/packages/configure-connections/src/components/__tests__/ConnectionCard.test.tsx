// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {ConnectionCardModel} from '../../models/connection';
import ConnectionCard from '../ConnectionCard';

const baseCard: ConnectionCardModel = {
  id: 'google',
  vendorKey: 'google',
  backendType: 'google',
  displayName: 'Google Login',
  descriptionKey: 'connections:vendor.google.description',
  logo: 'logo' as unknown as JSX.Element,
  categories: ['social-login'],
  status: 'not-configured',
  comingSoon: false,
  navTarget: '/connections/google/configure',
};

describe('ConnectionCard', () => {
  const onAction = vi.fn();
  beforeEach(() => vi.clearAllMocks());

  it('renders the vendor name, status, and hashtag category tags', () => {
    render(<ConnectionCard card={baseCard} onAction={onAction} />);
    expect(screen.getByText('Google Login')).toBeInTheDocument();
    expect(screen.getByText('Not configured')).toBeInTheDocument();
    expect(screen.getByText('#social login')).toBeInTheDocument();
  });

  it('invokes onAction when the whole card is clicked', () => {
    render(<ConnectionCard card={baseCard} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('connection-card-action-google'));
    expect(onAction).toHaveBeenCalledWith(baseCard);
  });

  it('invokes onAction when Enter is pressed on the card', () => {
    render(<ConnectionCard card={baseCard} onAction={onAction} />);
    fireEvent.keyDown(screen.getByTestId('connection-card-action-google'), {key: 'Enter'});
    expect(onAction).toHaveBeenCalledWith(baseCard);
  });

  it('invokes onAction when Space is pressed on the card', () => {
    render(<ConnectionCard card={baseCard} onAction={onAction} />);
    fireEvent.keyDown(screen.getByTestId('connection-card-action-google'), {key: ' '});
    expect(onAction).toHaveBeenCalledWith(baseCard);
  });

  it('ignores unrelated key presses on the card', () => {
    render(<ConnectionCard card={baseCard} onAction={onAction} />);
    fireEvent.keyDown(screen.getByTestId('connection-card-action-google'), {key: 'a'});
    expect(onAction).not.toHaveBeenCalled();
  });

  it('shows the configured status for a configured connection', () => {
    const configured: ConnectionCardModel = {...baseCard, status: 'configured'};
    render(<ConnectionCard card={configured} onAction={onAction} />);
    expect(screen.getByText('Configured')).toBeInTheDocument();
  });

  it('disables interaction for coming-soon cards (no clickable action area)', () => {
    const soon: ConnectionCardModel = {
      ...baseCard,
      id: 'twilio',
      vendorKey: 'twilio',
      comingSoon: true,
      navTarget: null,
    };
    render(<ConnectionCard card={soon} onAction={onAction} />);
    expect(screen.queryByTestId('connection-card-action-twilio')).not.toBeInTheDocument();
    expect(screen.getByText('Coming soon')).toBeInTheDocument();
  });
});
