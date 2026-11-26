// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi} from 'vitest';
import SettingField from '../SettingField';

describe('SettingField', () => {
  it('shows the name, the control and what the setting does, and ties the hint to the control', () => {
    render(
      <SettingField label="Idle timeout" hint="Signs the user out after this long without activity." custom={false}>
        {(hintId) => <input aria-label="Idle timeout" aria-describedby={hintId} />}
      </SettingField>,
    );

    expect(screen.getByText('Idle timeout')).toBeVisible();
    expect(screen.getByRole('textbox', {name: 'Idle timeout'})).toHaveAccessibleDescription(
      'Signs the user out after this long without activity.',
    );
  });

  it('gives the control nothing to point at without a hint', () => {
    const control = vi.fn(() => <input aria-label="Idle timeout" />);
    render(
      <SettingField label="Idle timeout" custom={false}>
        {control}
      </SettingField>,
    );

    expect(control).toHaveBeenCalledWith(undefined);
  });

  it('marks a default value and offers no reset', () => {
    render(
      <SettingField label="Idle timeout" custom={false} onReset={vi.fn()}>
        {() => <input aria-label="Idle timeout" />}
      </SettingField>,
    );

    expect(screen.getByText('Default')).toBeVisible();
    expect(screen.queryByText('Custom')).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('marks a value set here and offers to return it to the default', async () => {
    const onReset = vi.fn();
    render(
      <SettingField label="Absolute timeout" custom onReset={onReset}>
        {() => <input aria-label="Absolute timeout" />}
      </SettingField>,
    );

    expect(screen.getByText('Custom')).toBeVisible();
    const reset = screen.getByRole('button', {name: 'Reset to default'});
    // Every custom field has a reset, so each says which field it belongs to.
    expect(reset).toHaveAccessibleDescription('Absolute timeout');

    await userEvent.click(reset);
    expect(onReset).toHaveBeenCalledTimes(1);
  });

  // A section set declaratively, for one, offers no way back to the default at all.
  it('offers no reset where the value cannot be changed here', () => {
    render(
      <SettingField label="Idle timeout" custom>
        {() => <input aria-label="Idle timeout" disabled />}
      </SettingField>,
    );

    expect(screen.getByText('Custom')).toBeVisible();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  // While a save is in flight, the reset stays in place but cannot be used.
  it('disables the reset while the field is disabled', () => {
    render(
      <SettingField label="Idle timeout" custom disabled onReset={vi.fn()}>
        {() => <input aria-label="Idle timeout" disabled />}
      </SettingField>,
    );

    expect(screen.getByRole('button', {name: 'Reset to default'})).toBeDisabled();
  });
});
