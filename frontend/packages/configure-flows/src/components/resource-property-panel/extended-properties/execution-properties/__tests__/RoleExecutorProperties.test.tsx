// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import {ROLE_EXECUTOR_MODES} from '../constants';
import RoleExecutorProperties from '../RoleExecutorProperties';

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
        executor: {name: 'RoleExecutor', ...(mode ? {mode} : {})},
        type: 'EXECUTOR',
      },
      display: {label: 'Existing Label'},
    },
    id: 'roleexecutor-node',
  } as unknown as Resource;
}

describe('RoleExecutorProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the mode selector', () => {
    render(<RoleExecutorProperties resource={makeResource('delete')} onChange={mockOnChange} />);

    expect(screen.getByText('Mode')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('should show the mode configured on the node', () => {
    render(<RoleExecutorProperties resource={makeResource('delete')} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Delete Role');
  });

  it('should render a placeholder when the node carries no mode', () => {
    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(screen.getByText('Select a mode')).toBeInTheDocument();
  });

  it('should tell the user the mode must match the access change validator', () => {
    render(<RoleExecutorProperties resource={makeResource('delete')} onChange={mockOnChange} />);

    expect(screen.getByText(/must match the mode of the access change validator/i)).toBeInTheDocument();
  });

  // A mode missing from the backend's SupportedModes would let the builder save a flow that fails validation.
  it('should offer exactly the modes the executor supports', async () => {
    const user = userEvent.setup();
    expect(ROLE_EXECUTOR_MODES.map((mode) => mode.value)).toEqual([
      'remove_assignment',
      'delete',
      'remove_permissions',
    ]);

    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);
    await user.click(screen.getByRole('combobox'));

    const options = within(screen.getByRole('listbox'))
      .getAllByRole('option')
      .filter((option) => option.getAttribute('aria-disabled') !== 'true');
    expect(options.map((option) => option.getAttribute('data-value'))).toEqual([
      'remove_assignment',
      'delete',
      'remove_permissions',
    ]);
  });

  it('should set the mode and the matching label when a mode is selected', async () => {
    const user = userEvent.setup();
    const resource = makeResource();

    render(<RoleExecutorProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Change Role Permissions'}));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        action: expect.objectContaining({
          executor: {name: 'RoleExecutor', mode: 'remove_permissions'},
        }) as unknown,
        display: {label: 'Change Role Permissions'},
      }),
      resource,
    );
  });
});
