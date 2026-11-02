// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import AccessChangeValidatorProperties from '../AccessChangeValidatorProperties';
import {ACCESS_CHANGE_VALIDATOR_MODES, GROUP_EXECUTOR_MODES, ROLE_EXECUTOR_MODES} from '../constants';
import CriteriaRevocationProperties from '../CriteriaRevocationProperties';
import GroupExecutorProperties from '../GroupExecutorProperties';
import ModeSelectorProperties from '../ModeSelectorProperties';
import RoleExecutorProperties from '../RoleExecutorProperties';
import type {CommonResourcePropertiesPropsInterface} from '../types';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

const MODES = [
  {value: 'first_mode', translationKey: 'flows:test.mode.first', displayLabel: 'First Mode'},
  {value: 'second_mode', translationKey: 'flows:test.mode.second', displayLabel: 'Second Mode'},
] as const;

function makeResource(name: string, mode?: string): Resource {
  return {
    data: {
      action: {
        executor: {name, ...(mode ? {mode} : {})},
        type: 'EXECUTOR',
      },
      display: {label: 'Existing Label'},
    },
    id: `${name}-node`,
  } as unknown as Resource;
}

async function offeredModes(): Promise<(string | null)[]> {
  await userEvent.setup().click(screen.getByRole('combobox'));

  return within(screen.getByRole('listbox'))
    .getAllByRole('option')
    .filter((option) => option.getAttribute('aria-disabled') !== 'true')
    .map((option) => option.getAttribute('data-value'));
}

describe('ModeSelectorProperties', () => {
  const mockOnChange = vi.fn();

  const renderSelector = (resource: Resource, hint?: string): void => {
    render(
      <ModeSelectorProperties
        resource={resource}
        onChange={mockOnChange}
        modes={MODES}
        selectId="test-mode-select"
        fallbackLabel="Test Executor"
        description="Test description"
        label="Mode"
        placeholder="Select a mode"
        hint={hint}
      />,
    );
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the description and the mode selector', () => {
    renderSelector(makeResource('TestExecutor', 'first_mode'));

    expect(screen.getByText('Test description')).toBeInTheDocument();
    expect(screen.getByText('Mode')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('should show the mode configured on the node', () => {
    renderSelector(makeResource('TestExecutor', 'second_mode'));

    expect(screen.getByRole('combobox')).toHaveTextContent('Second Mode');
  });

  it('should render a placeholder when the node carries no mode', () => {
    renderSelector(makeResource('TestExecutor'));

    expect(screen.getByText('Select a mode')).toBeInTheDocument();
  });

  it('should render the hint when one is given', () => {
    renderSelector(makeResource('TestExecutor'), 'Test hint');

    expect(screen.getByText('Test hint')).toBeInTheDocument();
  });

  it('should offer exactly the given modes', async () => {
    renderSelector(makeResource('TestExecutor'));

    expect(await offeredModes()).toEqual(['first_mode', 'second_mode']);
  });

  it('should set the mode and the matching label when a mode is selected', async () => {
    const user = userEvent.setup();
    const resource = makeResource('TestExecutor');
    renderSelector(resource);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Second Mode'}));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        action: expect.objectContaining({
          executor: {name: 'TestExecutor', mode: 'second_mode'},
        }) as unknown,
        display: {label: 'Second Mode'},
      }),
      resource,
    );
  });

  it('should set the node image only when the selected mode carries one', async () => {
    const user = userEvent.setup();
    const resource = makeResource('TestExecutor');
    render(
      <ModeSelectorProperties
        resource={resource}
        onChange={mockOnChange}
        modes={[
          {value: 'plain_mode', translationKey: 'flows:test.mode.plain', displayLabel: 'Plain Mode'},
          {
            value: 'image_mode',
            translationKey: 'flows:test.mode.image',
            displayLabel: 'Image Mode',
            image: 'assets/images/icons/image-mode.svg',
          },
        ]}
        selectId="test-mode-select"
        fallbackLabel="Test Executor"
        description="Test description"
        label="Mode"
        placeholder="Select a mode"
      />,
    );

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Image Mode'}));

    expect(mockOnChange).toHaveBeenLastCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Image Mode', image: 'assets/images/icons/image-mode.svg'},
      }),
      resource,
    );

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Plain Mode'}));

    expect(mockOnChange).toHaveBeenLastCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Plain Mode'},
      }),
      resource,
    );
  });
});

// A mode missing from the backend's SupportedModes would let the builder save a flow that fails validation.
describe.each<[string, (props: CommonResourcePropertiesPropsInterface) => ReactNode, readonly string[], boolean]>([
  [
    'AccessChangeValidator',
    AccessChangeValidatorProperties,
    [
      'role_assignment_removal',
      'role_deletion',
      'role_permission_removal',
      'group_deletion',
      'group_membership_removal',
      'action_deletion',
    ],
    false,
  ],
  [
    'RoleExecutor',
    RoleExecutorProperties,
    ['role_assignment_removal', 'role_deletion', 'role_permission_removal'],
    true,
  ],
  ['GroupExecutor', GroupExecutorProperties, ['group_deletion', 'group_membership_removal'], true],
  ['CriteriaRevocationExecutor', CriteriaRevocationProperties, ['revoke', 'advance_cutoff'], false],
])('%s properties', (name, Component, supportedModes, pairedWithValidator) => {
  it('should offer exactly the modes the executor supports', async () => {
    render(<Component resource={makeResource(name)} onChange={vi.fn()} />);

    expect(await offeredModes()).toEqual(supportedModes);
  });

  it('should say whether the mode must match the access change validator', () => {
    render(<Component resource={makeResource(name)} onChange={vi.fn()} />);

    expect(Boolean(screen.queryByText(/must match the mode of the access change validator/i))).toBe(
      pairedWithValidator,
    );
  });
});

describe('executor mode constants', () => {
  it('should use the same names for a validator mode and the executor mode it pairs with', () => {
    const validatorModes: string[] = ACCESS_CHANGE_VALIDATOR_MODES.map((mode) => mode.value);

    for (const mode of [...ROLE_EXECUTOR_MODES, ...GROUP_EXECUTOR_MODES]) {
      expect(validatorModes).toContain(mode.value);
    }
  });
});
