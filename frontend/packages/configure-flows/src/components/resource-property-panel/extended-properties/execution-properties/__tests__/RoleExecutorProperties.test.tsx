// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import RoleExecutorProperties from '../RoleExecutorProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useParams: () => ({flowId: 'flow-1'}),
}));

const mockUseGetFlowById = vi.fn<(flowId?: string) => unknown>();
vi.mock('../../../../../api/useGetFlowById', () => ({
  default: (flowId?: string): unknown => mockUseGetFlowById(flowId),
}));

function inFlowOfType(flowType?: string): void {
  mockUseGetFlowById.mockReturnValue({data: flowType ? {flowType} : undefined});
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

const REVOCATION_MODES = ['Change Role Permissions'];
const INVITATION_MODES = ['Select Roles', 'Assign Roles'];

describe('RoleExecutorProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    inFlowOfType('ADMINISTRATION');
  });

  it('should read the type of the flow being edited', () => {
    render(<RoleExecutorProperties resource={makeResource('role_permission_removal')} onChange={mockOnChange} />);

    expect(mockUseGetFlowById).toHaveBeenCalledWith('flow-1');
  });

  // Selecting and assigning roles runs only in an invitation flow; the server refuses them elsewhere.
  it('should offer only the revocation plan modes in an administration flow', () => {
    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual(REVOCATION_MODES);
  });

  it('should offer only selecting and assigning roles in an invitation flow', () => {
    inFlowOfType('INVITATION');
    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual(INVITATION_MODES);
  });

  it('should keep a mode already on the node listed in another flow type, so it can be changed', () => {
    inFlowOfType('INVITATION');
    render(<RoleExecutorProperties resource={makeResource('role_permission_removal')} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual(['Change Role Permissions', ...INVITATION_MODES]);
  });

  it('should offer every mode until the flow loads', () => {
    inFlowOfType(undefined);
    render(<RoleExecutorProperties resource={makeResource()} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual([...REVOCATION_MODES, ...INVITATION_MODES]);
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
