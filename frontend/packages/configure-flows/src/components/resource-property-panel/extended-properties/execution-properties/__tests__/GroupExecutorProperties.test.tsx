// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import {GROUP_EXECUTOR_MODES} from '../constants';
import GroupExecutorProperties from '../GroupExecutorProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

function makeResource(mode?: string): Resource {
  return {
    data: {
      action: {
        executor: {name: 'GroupExecutor', ...(mode ? {mode} : {})},
        type: 'EXECUTOR',
      },
      display: {label: 'Existing Label'},
    },
    id: 'groupexecutor-node',
  } as unknown as Resource;
}

describe('GroupExecutorProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the mode selector', () => {
    render(<GroupExecutorProperties resource={makeResource('delete')} onChange={mockOnChange} />);

    expect(screen.getByText('Mode')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('should show the mode configured on the node', () => {
    render(<GroupExecutorProperties resource={makeResource('delete')} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Delete Group');
  });

  it('should render a placeholder when the node carries no mode', () => {
    render(<GroupExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(screen.getByText('Select a mode')).toBeInTheDocument();
  });

  it('should tell the user the mode must match the access change validator', () => {
    render(<GroupExecutorProperties resource={makeResource('delete')} onChange={mockOnChange} />);

    expect(screen.getByText(/must match the mode of the access change validator/i)).toBeInTheDocument();
  });

  // A mode missing from the backend's SupportedModes would let the builder save a flow that fails validation.
  it('should offer exactly the modes the executor supports', async () => {
    const user = userEvent.setup();
    expect(GROUP_EXECUTOR_MODES.map((mode) => mode.value)).toEqual(['delete', 'remove_member']);

    render(<GroupExecutorProperties resource={makeResource()} onChange={mockOnChange} />);
    await user.click(screen.getByRole('combobox'));

    const options = within(screen.getByRole('listbox'))
      .getAllByRole('option')
      .filter((option) => option.getAttribute('aria-disabled') !== 'true');
    expect(options.map((option) => option.getAttribute('data-value'))).toEqual(['delete', 'remove_member']);
  });

  it('should set the mode and the matching label when a mode is selected', async () => {
    const user = userEvent.setup();
    const resource = makeResource();

    render(<GroupExecutorProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Remove Group Member'}));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        action: expect.objectContaining({
          executor: {name: 'GroupExecutor', mode: 'remove_member'},
        }) as unknown,
        display: {label: 'Remove Group Member'},
      }),
      resource,
    );
  });
});
