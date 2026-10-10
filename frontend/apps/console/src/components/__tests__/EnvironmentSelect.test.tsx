// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {EnvironmentProvider, type Environment} from '@thunderid/contexts';
import {renderWithProviders, screen, userEvent, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import EnvironmentSelect from '../EnvironmentSelect';

const environments: Environment[] = [
  {id: 'gw-1', name: 'Dev', isDefault: true},
  {id: 'gw-2', name: 'Prod'},
];

const show = (selectedId: string | undefined, onSelect = vi.fn(), shown = environments) =>
  renderWithProviders(
    <EnvironmentProvider environments={shown} selectedId={selectedId} onSelect={onSelect}>
      <EnvironmentSelect />
    </EnvironmentProvider>,
  );

describe('EnvironmentSelect', () => {
  it('is not shown without environments', () => {
    show(undefined, vi.fn(), []);
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
  });

  it('shows the configuration until an environment is chosen', async () => {
    const onSelect = vi.fn();
    show(undefined, onSelect);

    const combobox = screen.getByRole('combobox', {name: 'Shown'});
    expect(combobox).toHaveTextContent('Configuration');
    await userEvent.click(combobox);
    const options = within(screen.getByRole('listbox'));
    expect(options.getByText('Environments (read only)')).toBeInTheDocument();
    expect(options.getByText('Dev (default)')).toBeInTheDocument();
    await userEvent.click(options.getByText('Prod'));
    expect(onSelect).toHaveBeenCalledWith('gw-2');
  });

  it('returns to the configuration from an environment', async () => {
    const onSelect = vi.fn();
    show('gw-2', onSelect);

    const combobox = screen.getByRole('combobox', {name: 'Shown'});
    expect(combobox).toHaveTextContent('Prod');
    await userEvent.click(combobox);
    await userEvent.click(within(screen.getByRole('listbox')).getByText('Configuration'));
    expect(onSelect).toHaveBeenCalledWith(undefined);
  });
});
