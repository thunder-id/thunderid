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

vi.mock('@thunderid/configure-user-types', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-user-types')>()),
  useGetUserTypeAttributes: () => ({
    attributes: [
      {attribute: 'email', credential: false, unique: true, userTypes: ['Person', 'Employee']},
      {attribute: 'given_name', credential: false, unique: false, userTypes: ['Person']},
      {attribute: 'password', credential: true, unique: false, userTypes: ['Person']},
      {attribute: 'username', credential: false, unique: true, userTypes: ['Person']},
    ],
    isLoading: false,
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

  describe('CIBA login hint', () => {
    const cibaLabel = 'flows:core.executions.identifying.cibaLoginHint.label';
    const attributeLabel = 'flows:core.executions.identifying.loginHintAttribute.label';
    const resourceWith = (mode: string, properties: Record<string, unknown> = {}): Resource =>
      ({data: {action: {executor: {mode}}, properties}}) as unknown as Resource;

    it('should offer the option unchecked in identify mode, without the attribute field', () => {
      render(<IdentifyingProperties resource={resourceWith('identify')} onChange={mockOnChange} />);

      expect(screen.getByLabelText(cibaLabel)).not.toBeChecked();
      expect(screen.queryByLabelText(attributeLabel)).not.toBeInTheDocument();
    });

    it('should not offer the option in resolve mode when it is off', () => {
      render(<IdentifyingProperties resource={resourceWith('resolve')} onChange={mockOnChange} />);

      expect(screen.queryByLabelText(cibaLabel)).not.toBeInTheDocument();
    });

    it('should keep the option in resolve mode when it is on, so it can be turned off', () => {
      render(
        <IdentifyingProperties
          resource={resourceWith('resolve', {loginHintAttribute: 'username'})}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByLabelText(cibaLabel)).toBeChecked();
    });

    it('should show the option checked with the configured attribute', () => {
      render(
        <IdentifyingProperties
          resource={resourceWith('identify', {loginHintAttribute: 'email'})}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByLabelText(cibaLabel)).toBeChecked();
      expect(screen.getByLabelText(attributeLabel)).toHaveValue('email');
    });

    it('should show the new attribute when the configured value changes', () => {
      const {rerender} = render(
        <IdentifyingProperties
          resource={resourceWith('identify', {loginHintAttribute: 'username'})}
          onChange={mockOnChange}
        />,
      );

      rerender(
        <IdentifyingProperties
          resource={resourceWith('identify', {loginHintAttribute: 'email'})}
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByLabelText(attributeLabel)).toHaveValue('email');
    });

    it('should set the default attribute when checked', async () => {
      const user = userEvent.setup();
      const resource = resourceWith('identify');
      render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByLabelText(cibaLabel));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.loginHintAttribute', 'username', resource);
    });

    it('should clear the attribute when unchecked', async () => {
      const user = userEvent.setup();
      const resource = resourceWith('identify', {loginHintAttribute: 'username'});
      render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

      await user.click(screen.getByLabelText(cibaLabel));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.loginHintAttribute', '', resource);
    });

    it('should suggest only the unique user type attributes', async () => {
      const user = userEvent.setup();
      render(
        <IdentifyingProperties
          resource={resourceWith('identify', {loginHintAttribute: 'username'})}
          onChange={mockOnChange}
        />,
      );

      await user.clear(screen.getByLabelText(attributeLabel));
      await user.keyboard('{ArrowDown}');

      expect(screen.getByRole('option', {name: /email/})).toHaveTextContent('Person, Employee');
      expect(screen.getByRole('option', {name: /username/})).toBeInTheDocument();
      expect(screen.queryByRole('option', {name: /given_name/})).not.toBeInTheDocument();
      expect(screen.queryByRole('option', {name: /password/})).not.toBeInTheDocument();
    });

    it('should commit a suggested attribute when selected', async () => {
      const user = userEvent.setup();
      const resource = resourceWith('identify', {loginHintAttribute: 'username'});
      render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

      await user.clear(screen.getByLabelText(attributeLabel));
      await user.type(screen.getByLabelText(attributeLabel), 'em');
      await user.click(screen.getByRole('option', {name: /email/}));

      expect(mockOnChange).toHaveBeenCalledTimes(1);
      expect(mockOnChange).toHaveBeenCalledWith('data.properties.loginHintAttribute', 'email', resource);
      expect(screen.getByLabelText(attributeLabel)).toHaveValue('email');
    });

    it('should commit a typed attribute trimmed on blur, not while typing', async () => {
      const user = userEvent.setup();
      const resource = resourceWith('identify', {loginHintAttribute: 'username'});
      render(<IdentifyingProperties resource={resource} onChange={mockOnChange} />);

      await user.clear(screen.getByLabelText(attributeLabel));
      await user.type(screen.getByLabelText(attributeLabel), '  mobileNumber  ');
      expect(mockOnChange).not.toHaveBeenCalled();

      await user.tab();

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.loginHintAttribute', 'mobileNumber', resource);
    });

    it('should restore the attribute instead of clearing it when the field is emptied', async () => {
      const user = userEvent.setup();
      render(
        <IdentifyingProperties
          resource={resourceWith('identify', {loginHintAttribute: 'username'})}
          onChange={mockOnChange}
        />,
      );

      await user.clear(screen.getByLabelText(attributeLabel));
      await user.tab();

      expect(mockOnChange).not.toHaveBeenCalled();
      expect(screen.getByLabelText(attributeLabel)).toHaveValue('username');
    });

    it('should not commit on blur when the attribute is unchanged', async () => {
      const user = userEvent.setup();
      render(
        <IdentifyingProperties
          resource={resourceWith('identify', {loginHintAttribute: 'username'})}
          onChange={mockOnChange}
        />,
      );

      await user.click(screen.getByLabelText(attributeLabel));
      await user.tab();

      expect(mockOnChange).not.toHaveBeenCalled();
    });
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
