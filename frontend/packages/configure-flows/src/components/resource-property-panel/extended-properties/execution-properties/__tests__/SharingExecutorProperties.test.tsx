// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import SharingExecutorProperties from '../SharingExecutorProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({t: (key: string) => key}),
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

const VALIDATE = 'flows:core.executions.sharing.mode.validate';
const SHARE = 'flows:core.executions.sharing.mode.share';

describe('SharingExecutorProperties', () => {
  const mockOnChange = vi.fn();

  const createResource = (mode?: string, properties: Record<string, unknown> = {}): Resource =>
    ({
      data: {action: {executor: {name: 'SharingExecutor', ...(mode ? {mode} : {})}}, properties},
    }) as unknown as Resource;

  beforeEach(() => {
    mockOnChange.mockClear();
    inFlowOfType('INVITATION');
  });

  it('should read the type of the flow being edited', () => {
    render(<SharingExecutorProperties resource={createResource('share')} onChange={mockOnChange} />);

    expect(mockUseGetFlowById).toHaveBeenCalledWith('flow-1');
  });

  it('should offer validating and sharing in an invitation flow', () => {
    render(<SharingExecutorProperties resource={createResource()} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual([VALIDATE, SHARE]);
  });

  // Outside an invitation nobody started the flow to authorize a share as; a sign-up shares what it created.
  it('should offer only sharing in a sign-in or registration flow', () => {
    for (const flowType of ['AUTHENTICATION', 'REGISTRATION']) {
      inFlowOfType(flowType);
      const {unmount} = render(<SharingExecutorProperties resource={createResource()} onChange={mockOnChange} />);

      expect(offeredModes()).toEqual([SHARE]);
      unmount();
    }
  });

  it('should keep a mode already on the node listed in another flow type, so it can be changed', () => {
    inFlowOfType('AUTHENTICATION');
    render(<SharingExecutorProperties resource={createResource('validate')} onChange={mockOnChange} />);

    expect(offeredModes()).toEqual([VALIDATE, SHARE]);
  });

  it('should describe the sign-up share outside an invitation flow', () => {
    const {unmount} = render(<SharingExecutorProperties resource={createResource('share')} onChange={mockOnChange} />);
    expect(screen.getByText('flows:core.executions.sharing.description')).toBeInTheDocument();
    unmount();

    inFlowOfType('AUTHENTICATION');
    render(<SharingExecutorProperties resource={createResource('share')} onChange={mockOnChange} />);
    expect(screen.getByText('flows:core.executions.sharing.signUpDescription')).toBeInTheDocument();
  });

  it('should show the resource and target fields with their values', () => {
    render(
      <SharingExecutorProperties
        resource={createResource('share', {resourceType: 'user', resourceId: '{{ctx(userID)}}', targetOuId: 'ou-a'})}
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByLabelText('flows:core.executions.sharing.resourceType.label')).toHaveValue('user');
    expect(screen.getByLabelText('flows:core.executions.sharing.resourceId.label')).toHaveValue('{{ctx(userID)}}');
    expect(screen.getByLabelText('flows:core.executions.sharing.targetOuId.label')).toHaveValue('ou-a');
  });

  // Only validate authorizes a share for a resource created later, which needs its owning unit.
  it('should ask for the owning OU only in validate mode', () => {
    const {unmount} = render(<SharingExecutorProperties resource={createResource('share')} onChange={mockOnChange} />);

    expect(screen.queryByLabelText('flows:core.executions.sharing.owningOuId.label')).not.toBeInTheDocument();
    unmount();

    render(<SharingExecutorProperties resource={createResource('validate')} onChange={mockOnChange} />);

    expect(screen.getByLabelText('flows:core.executions.sharing.owningOuId.label')).toBeInTheDocument();
  });

  it('should store a committed field trimmed', async () => {
    render(<SharingExecutorProperties resource={createResource('validate')} onChange={mockOnChange} />);

    const field = screen.getByLabelText('flows:core.executions.sharing.targetOuId.label');
    await userEvent.type(field, ' ou-a ');
    await userEvent.tab();

    expect(mockOnChange).toHaveBeenCalledWith(
      'data.properties.targetOuId',
      'ou-a',
      expect.objectContaining({data: expect.anything() as unknown}),
    );
  });
});
