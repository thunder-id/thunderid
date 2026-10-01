// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import InviteProperties from '../InviteProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.invite.description': 'Configure the Invite step behavior.',
        'flows:core.executions.invite.mode.label': 'Mode',
        'flows:core.executions.invite.mode.placeholder': 'Select an action mode',
        'flows:core.executions.invite.mode.generate': 'Generate Invite',
        'flows:core.executions.invite.mode.verify': 'Verify Invite',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('InviteProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and mode label', () => {
    const resource = {data: {action: {executor: {mode: 'generate'}}}} as unknown as Resource;

    render(<InviteProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure the Invite step behavior.')).toBeInTheDocument();
    expect(screen.getByText('Mode')).toBeInTheDocument();
  });

  it('should display placeholder when no mode is selected', () => {
    const resource = {data: {action: {executor: {}}}} as unknown as Resource;

    render(<InviteProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Select an action mode');
  });

  it('should call onChange with the existing display block preserved when a new mode is selected', async () => {
    const user = userEvent.setup();
    const resource = {
      data: {
        action: {executor: {mode: 'generate'}},
        display: {label: 'Generate Invite', extra: 'kept'},
      },
    } as unknown as Resource;

    render(<InviteProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByText('Verify Invite'));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Verify Invite', extra: 'kept'},
      }),
      resource,
    );
  });

  it('should default the display block when the node carries none', async () => {
    const user = userEvent.setup();
    const resource = {data: {action: {executor: {mode: 'generate'}}}} as unknown as Resource;

    render(<InviteProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByText('Verify Invite'));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Verify Invite'},
      }),
      resource,
    );
  });
});
