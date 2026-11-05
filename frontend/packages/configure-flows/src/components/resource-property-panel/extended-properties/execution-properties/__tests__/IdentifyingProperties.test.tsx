// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import IdentifyingProperties from '../IdentifyingProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.identifying.description': 'Configure the Identify User step behavior.',
        'flows:core.executions.identifying.mode.label': 'Mode',
        'flows:core.executions.identifying.mode.placeholder': 'Select an action mode',
        'flows:core.executions.identifying.mode.identify': 'Identify user',
        'flows:core.executions.identifying.mode.resolve': 'Resolve User',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('IdentifyingProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and mode label', () => {
    const resource = {data: {action: {executor: {mode: 'identify'}}}} as unknown as Resource;

    render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure the Identify User step behavior.')).toBeInTheDocument();
    expect(screen.getByText('Mode')).toBeInTheDocument();
  });

  it('should display placeholder when no mode is selected', () => {
    const resource = {data: {action: {executor: {}}}} as unknown as Resource;

    render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Select an action mode');
  });

  it('should call onChange with the existing display block preserved when a new mode is selected', async () => {
    const user = userEvent.setup();
    const resource = {
      data: {
        action: {executor: {mode: 'identify'}},
        display: {label: 'Identify User', extra: 'kept'},
      },
    } as unknown as Resource;

    render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByText('Resolve User'));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Resolve User', extra: 'kept'},
      }),
      resource,
    );
  });

  it('should default the display block when the node carries none', async () => {
    const user = userEvent.setup();
    const resource = {data: {action: {executor: {mode: 'identify'}}}} as unknown as Resource;

    render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByText('Resolve User'));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Resolve User'},
      }),
      resource,
    );
  });

  describe('identify options', () => {
    const boundLabel = 'flows:core.executions.identifying.boundToUserType.label';
    const noMatchLabel = 'flows:core.executions.identifying.continueOnNoMatch.label';
    const resourceWith = (mode: string, properties: Record<string, unknown> = {}): Resource =>
      ({data: {action: {executor: {mode}}, properties}}) as unknown as Resource;

    it('should offer the user type and no-match options in identify mode', () => {
      render(<IdentifyingProperties resource={resourceWith('identify')} onChange={mockOnChange} />);

      expect(screen.getByLabelText(boundLabel)).not.toBeChecked();
      expect(screen.getByLabelText(noMatchLabel)).not.toBeChecked();
    });

    it('should not offer them in resolve mode', () => {
      render(<IdentifyingProperties resource={resourceWith('resolve')} onChange={mockOnChange} />);

      expect(screen.queryByLabelText(boundLabel)).not.toBeInTheDocument();
      expect(screen.queryByLabelText(noMatchLabel)).not.toBeInTheDocument();
    });

    it('should set each property when checked', async () => {
      const user = userEvent.setup();
      const resource = resourceWith('identify');
      render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByLabelText(boundLabel));
      await user.click(screen.getByLabelText(noMatchLabel));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.boundToUserType', true, resource);
      expect(mockOnChange).toHaveBeenCalledWith('data.properties.continueOnNoMatch', true, resource);
    });

    it('should show the configured values', () => {
      render(
        <IdentifyingProperties
          resource={resourceWith('identify', {boundToUserType: true, continueOnNoMatch: true})}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByLabelText(boundLabel)).toBeChecked();
      expect(screen.getByLabelText(noMatchLabel)).toBeChecked();
    });
  });
});
