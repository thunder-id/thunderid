// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import SettingsLockNotice from '../SettingsLockNotice';

describe('SettingsLockNotice', () => {
  it('renders only the children when unlocked', () => {
    render(
      <SettingsLockNotice isUnlocked message="Turn on Delegated mode to unlock these settings.">
        <div data-testid="content">content</div>
      </SettingsLockNotice>,
    );

    expect(screen.getByTestId('content')).toBeInTheDocument();
    expect(screen.queryByText('Turn on Delegated mode to unlock these settings.')).not.toBeInTheDocument();
  });

  it('shows the given message and gives the (still visible) children a frozen look when locked', () => {
    render(
      <SettingsLockNotice isUnlocked={false} message="Turn on Delegated mode to unlock these settings.">
        <div data-testid="content">content</div>
      </SettingsLockNotice>,
    );

    expect(screen.getByTestId('content')).toBeInTheDocument();
    expect(screen.getByText('Turn on Delegated mode to unlock these settings.')).toBeInTheDocument();
    expect(screen.getByTestId('content').parentElement).toHaveStyle({pointerEvents: 'none', opacity: '0.6'});
  });
});
