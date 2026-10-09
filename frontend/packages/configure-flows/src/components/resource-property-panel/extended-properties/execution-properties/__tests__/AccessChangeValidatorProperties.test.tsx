// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import AccessChangeValidatorProperties from '../AccessChangeValidatorProperties';
import {ACCESS_CHANGE_VALIDATOR_MODES} from '../constants';

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
        executor: {name: 'AccessChangeValidator', ...(mode ? {mode} : {})},
        type: 'EXECUTOR',
      },
      display: {label: 'Existing Label'},
    },
    id: 'accesschangevalidator-node',
  } as unknown as Resource;
}

describe('AccessChangeValidatorProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the mode selector', () => {
    render(<AccessChangeValidatorProperties resource={makeResource('role_deletion')} onChange={mockOnChange} />);

    expect(screen.getByText('Mode')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('should show the mode configured on the node', () => {
    render(<AccessChangeValidatorProperties resource={makeResource('role_deletion')} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Validate and Plan Role Deletion');
  });

  it('should render a placeholder when the node carries no mode', () => {
    render(<AccessChangeValidatorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(screen.getByText('Select a mode')).toBeInTheDocument();
  });

  // A mode missing from the backend's SupportedModes would let the builder save a flow that fails validation.
  it('should offer exactly the modes the executor supports', async () => {
    const user = userEvent.setup();
    expect(ACCESS_CHANGE_VALIDATOR_MODES.map((mode) => mode.value)).toEqual([
      'role_assignment_removal',
      'role_deletion',
      'role_permission_removal',
      'group_deletion',
      'group_member_removal',
      'action_deletion',
    ]);

    render(<AccessChangeValidatorProperties resource={makeResource()} onChange={mockOnChange} />);
    await user.click(screen.getByRole('combobox'));

    const options = within(screen.getByRole('listbox'))
      .getAllByRole('option')
      .filter((option) => option.getAttribute('aria-disabled') !== 'true');
    expect(options.map((option) => option.getAttribute('data-value'))).toEqual([
      'role_assignment_removal',
      'role_deletion',
      'role_permission_removal',
      'group_deletion',
      'group_member_removal',
      'action_deletion',
    ]);
  });

  it('should set the mode and the matching label when a mode is selected', async () => {
    const user = userEvent.setup();
    const resource = makeResource();

    render(<AccessChangeValidatorProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Validate and Plan Membership Removal'}));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        action: expect.objectContaining({
          executor: {name: 'AccessChangeValidator', mode: 'group_member_removal'},
        }) as unknown,
        display: {label: 'Validate and Plan Membership Removal'},
      }),
      resource,
    );
  });
});
