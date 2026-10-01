// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import PasskeyProperties from '../PasskeyProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.passkey.description': 'Configure the Passkey step behavior.',
        'flows:core.executions.passkey.mode.label': 'Mode',
        'flows:core.executions.passkey.mode.placeholder': 'Select an action mode',
        'flows:core.executions.passkey.mode.challenge': 'Request Passkey',
        'flows:core.executions.passkey.mode.verify': 'Verify Passkey',
        'flows:core.executions.passkey.mode.registerStart': 'Start Passkey Registration',
        'flows:core.executions.passkey.mode.registerFinish': 'Finish Passkey Registration',
        'flows:core.executions.passkey.relyingPartyId.label': 'Relying Party ID',
        'flows:core.executions.passkey.relyingPartyId.placeholder': 'example.com',
        'flows:core.executions.passkey.relyingPartyId.hint': 'The relying party identifier.',
        'flows:core.executions.passkey.relyingPartyName.label': 'Relying Party Name',
        'flows:core.executions.passkey.relyingPartyName.placeholder': 'My App',
        'flows:core.executions.passkey.relyingPartyName.hint': 'The relying party display name.',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('PasskeyProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and mode label', () => {
    const resource = {data: {action: {executor: {mode: 'verify'}}}} as unknown as Resource;

    render(<PasskeyProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure the Passkey step behavior.')).toBeInTheDocument();
    expect(screen.getByText('Mode')).toBeInTheDocument();
  });

  it('should hide relying party fields for a mode that does not need them', () => {
    const resource = {data: {action: {executor: {mode: 'verify'}}}} as unknown as Resource;

    render(<PasskeyProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.queryByText('Relying Party ID')).not.toBeInTheDocument();
  });

  it('should show relying party fields for a mode that needs them', () => {
    const resource = {
      data: {
        action: {executor: {mode: 'challenge'}},
        properties: {relyingPartyId: 'example.com', relyingPartyName: 'My App'},
      },
    } as unknown as Resource;

    render(<PasskeyProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Relying Party ID')).toBeInTheDocument();
    expect(screen.getByDisplayValue('example.com')).toBeInTheDocument();
    expect(screen.getByDisplayValue('My App')).toBeInTheDocument();
  });

  it('should default the display block when the node carries none', async () => {
    const user = userEvent.setup();
    const resource = {data: {action: {executor: {mode: 'verify'}}}} as unknown as Resource;

    render(<PasskeyProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByText('Start Passkey Registration'));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Start Passkey Registration'},
      }),
      resource,
    );
  });
});
