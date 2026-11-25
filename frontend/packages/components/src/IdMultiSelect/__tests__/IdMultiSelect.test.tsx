// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi} from 'vitest';
import IdMultiSelect, {type IdMultiSelectProps} from '../IdMultiSelect';

const options = [
  {id: 'id-1', name: 'Engineering'},
  {id: 'id-2', name: 'Sales'},
];

const renderSelect = (props: Partial<IdMultiSelectProps> = {}) => {
  const onChange = vi.fn();
  const onRetry = vi.fn();
  render(
    <IdMultiSelect
      id="picker"
      value={[]}
      options={options}
      loading={false}
      onChange={onChange}
      onRetry={onRetry}
      placeholder="Select groups"
      noOptionsText="No groups found"
      unavailableLabel={(id) => `Unavailable: ${id}`}
      errorFallback={{key: 'test:loadError', defaultValue: 'Failed to load groups.'}}
      {...props}
    />,
  );
  return {onChange, onRetry};
};

describe('IdMultiSelect', () => {
  it('shows the name of a selected ID', () => {
    renderSelect({value: ['id-1']});

    expect(screen.getByText('Engineering')).toBeInTheDocument();
  });

  it('shows the placeholder only while nothing is selected', () => {
    const {unmount} = render(
      <IdMultiSelect
        id="empty"
        value={[]}
        options={options}
        loading={false}
        onChange={vi.fn()}
        onRetry={vi.fn()}
        placeholder="Select groups"
        noOptionsText="No groups found"
        unavailableLabel={(id) => id}
        errorFallback={{key: 'test:loadError', defaultValue: 'Failed'}}
      />,
    );
    expect(screen.getByPlaceholderText('Select groups')).toBeInTheDocument();
    unmount();

    renderSelect({value: ['id-1']});
    expect(screen.queryByPlaceholderText('Select groups')).not.toBeInTheDocument();
  });

  it('reports the selected IDs when an option is picked', async () => {
    const user = userEvent.setup();
    const {onChange} = renderSelect({value: ['id-1']});

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Sales'}));

    expect(onChange).toHaveBeenCalledWith(['id-1', 'id-2']);
  });

  it('marks a selected ID that matches no option as unavailable and keeps it on the next change', async () => {
    const user = userEvent.setup();
    const {onChange} = renderSelect({value: ['gone']});

    expect(screen.getByText('Unavailable: gone')).toBeInTheDocument();

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Sales'}));

    expect(onChange).toHaveBeenCalledWith(['gone', 'id-2']);
  });

  it('does not mark a selected ID as unavailable while the options are loading', () => {
    renderSelect({value: ['id-1'], options: [], loading: true});

    expect(screen.queryByText(/Unavailable/)).not.toBeInTheDocument();
  });

  it('offers a retry and does not mark IDs unavailable when the options fail to load', async () => {
    const user = userEvent.setup();
    const {onRetry} = renderSelect({value: ['id-1'], options: [], error: new Error('boom')});

    expect(screen.getByText('Failed to load groups.')).toBeInTheDocument();
    expect(screen.queryByText(/Unavailable/)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Refresh'}));

    expect(onRetry).toHaveBeenCalled();
  });
});
