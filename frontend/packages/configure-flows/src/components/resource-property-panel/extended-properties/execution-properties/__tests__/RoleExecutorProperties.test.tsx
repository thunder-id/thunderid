// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render as renderInBuilder, screen, within} from '@testing-library/react';
import type {ReactElement, ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import FlowTypeContext from '../../../../../context/FlowTypeContext';
import type {FlowType} from '../../../../../models/flows';
import type {Resource} from '../../../../../models/resources';
import RoleExecutorProperties from '../RoleExecutorProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

let builderFlowType: FlowType | undefined;

/**
 * Renders inside the flow builder's flow type, which it knows for an unsaved draft too.
 */
function render(ui: ReactElement): ReturnType<typeof renderInBuilder> {
  return renderInBuilder(ui, {
    wrapper: ({children}: {children: ReactNode}) => (
      <FlowTypeContext.Provider value={builderFlowType}>{children}</FlowTypeContext.Provider>
    ),
  });
}

function inFlowOfType(flowType?: string): void {
  builderFlowType = flowType as FlowType | undefined;
}

/**
 * Opens the mode selector and returns the modes it offers.
 */
function offeredModes(): string[] {
  fireEvent.mouseDown(screen.getByRole('combobox'));

  return within(screen.getByRole('listbox'))
    .getAllByRole('option')
    .filter((option) => option.getAttribute('aria-disabled') !== 'true')
    .map((option) => option.textContent ?? '');
}

function makeResource(mode?: string, properties?: Record<string, unknown>): Resource {
  return {
    data: {
      action: {
        executor: {name: 'RoleExecutor', ...(mode ? {mode} : {})},
        type: 'EXECUTOR',
      },
      display: {label: 'Role Executor'},
      ...(properties ? {properties} : {}),
    },
    id: 'role_executor',
  } as unknown as Resource;
}

const INVITATION_MODES = ['Select Roles', 'Assign Roles'];

describe('RoleExecutorProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    inFlowOfType('ADMINISTRATION');
  });

  it('should offer only selecting and assigning roles in an invitation flow', () => {
    inFlowOfType('INVITATION');
    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual(INVITATION_MODES);
  });

  // Selecting and assigning roles runs only in an invitation flow; the server refuses them elsewhere.
  it('should keep a mode already on the node listed in another flow type, so it can be changed', () => {
    render(<RoleExecutorProperties resource={makeResource('role_selection')} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual(['Select Roles']);
  });

  it('should offer every mode until the flow loads', () => {
    inFlowOfType(undefined);
    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual(INVITATION_MODES);
  });

  it('should say the mode must match the access change validator only outside an invitation flow', () => {
    const {unmount} = render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);
    expect(screen.getByText(/must match the mode of the access change validator/i)).toBeInTheDocument();
    unmount();

    inFlowOfType('INVITATION');
    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);
    expect(screen.queryByText(/must match the mode of the access change validator/i)).not.toBeInTheDocument();
  });

  it('should take the roles to assign only in Select Roles mode', () => {
    inFlowOfType('INVITATION');
    const {unmount} = render(
      <RoleExecutorProperties resource={makeResource('role_assignment')} onChange={mockOnChange} />,
    );
    expect(screen.queryByLabelText('Roles')).not.toBeInTheDocument();
    unmount();

    render(
      <RoleExecutorProperties
        resource={makeResource('role_selection', {roles: 'role-1 role-2'})}
        onChange={mockOnChange}
      />,
    );
    expect(screen.getByLabelText('Roles')).toHaveValue('role-1 role-2');
  });

  // A sign-up flow has no admin to pick roles, so it can only assign the ones the node names.
  it('should offer only assigning roles in a sign-up flow', () => {
    inFlowOfType('REGISTRATION');
    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual(['Assign Roles']);
  });

  it('should take the roles to assign in Assign Roles mode in a sign-up flow', () => {
    inFlowOfType('REGISTRATION');
    render(
      <RoleExecutorProperties resource={makeResource('role_assignment', {roles: 'role-1'})} onChange={mockOnChange} />,
    );

    expect(screen.getByLabelText('Roles')).toHaveValue('role-1');
    expect(screen.getByText(/assigned to the new user/i)).toBeInTheDocument();
    expect(screen.queryByText(/must match the mode of the access change validator/i)).not.toBeInTheDocument();
  });

  it('should store the roles it is given, trimmed, as the node property', () => {
    inFlowOfType('INVITATION');
    const resource = makeResource('role_selection');
    render(<RoleExecutorProperties resource={resource} onChange={mockOnChange} />);

    const field = screen.getByLabelText('Roles');
    fireEvent.change(field, {target: {value: '  role-1 role-2 '}});
    fireEvent.blur(field);

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.roles', 'role-1 role-2', resource);
  });
});
