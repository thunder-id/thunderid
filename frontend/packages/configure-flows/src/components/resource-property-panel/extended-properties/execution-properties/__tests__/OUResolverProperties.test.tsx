// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowTypeContext from '../../../../../context/FlowTypeContext';
import type {FlowType} from '../../../../../models/flows';
import type {Resource} from '../../../../../models/resources';
import OUResolverProperties from '../OUResolverProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.ouResolver.description': 'Configure how the organization unit is resolved.',
        'flows:core.executions.ouResolver.resolveFrom.label': 'Resolve from',
        'flows:core.executions.ouResolver.resolveFrom.caller': 'Caller',
        'flows:core.executions.ouResolver.resolveFrom.prompt': 'Prompt',
        'flows:core.executions.ouResolver.resolveFrom.promptAll': 'Prompt all',
        'flows:core.executions.ouResolver.directChildrenOnly.label': 'Limit to direct children',
        'flows:core.executions.ouResolver.parentOuId.label': 'Parent OU ID',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('OUResolverProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and resolve-from label', () => {
    const resource = {data: {properties: {resolveFrom: 'prompt'}}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure how the organization unit is resolved.')).toBeInTheDocument();
    expect(screen.getByText('Resolve from')).toBeInTheDocument();
  });

  it('should default resolveFrom to caller when the node has no properties', () => {
    const resource = {data: {}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Caller');
  });

  it('should show the configured resolveFrom value when present', () => {
    const resource = {data: {properties: {resolveFrom: 'promptAll'}}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Prompt all');
  });

  it('should call onChange when a new resolveFrom option is selected', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {resolveFrom: 'caller'}}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Prompt'}));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.resolveFrom', 'prompt', resource);
  });

  it('should offer the direct-children option only for the prompt strategy', () => {
    const {rerender} = render(
      <OUResolverProperties
        resource={{data: {properties: {resolveFrom: 'prompt'}}} as unknown as Resource}
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByRole('checkbox', {name: /Limit to direct children/})).not.toBeChecked();

    rerender(
      <OUResolverProperties
        resource={{data: {properties: {resolveFrom: 'promptAll'}}} as unknown as Resource}
        onChange={mockOnChange}
      />,
    );

    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('should show the option as checked and write it when toggled', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {resolveFrom: 'prompt', directChildrenOnly: true}}} as unknown as Resource;

    render(<OUResolverProperties resource={resource} onChange={mockOnChange} />);

    const checkbox = screen.getByRole('checkbox', {name: /Limit to direct children/});
    expect(checkbox).toBeChecked();

    await user.click(checkbox);

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.directChildrenOnly', false, resource);
  });

  const inFlow = (flowType: FlowType | undefined, resource: Resource) => (
    <FlowTypeContext.Provider value={flowType}>
      <OUResolverProperties resource={resource} onChange={mockOnChange} />
    </FlowTypeContext.Provider>
  );

  it('should offer the parent organization unit only for the prompt strategy in an authentication flow', () => {
    const {rerender} = render(
      inFlow('AUTHENTICATION', {
        data: {properties: {resolveFrom: 'prompt', parentOuId: 'ou-1'}},
      } as unknown as Resource),
    );

    expect(screen.getByLabelText(/Parent OU ID/)).toHaveValue('ou-1');

    rerender(inFlow('AUTHENTICATION', {data: {properties: {resolveFrom: 'caller'}}} as unknown as Resource));

    expect(screen.queryByLabelText(/Parent OU ID/)).not.toBeInTheDocument();
  });

  it('should mark the parent organization unit as required and invalid while it is empty', () => {
    const {rerender} = render(
      inFlow('AUTHENTICATION', {data: {properties: {resolveFrom: 'prompt'}}} as unknown as Resource),
    );

    expect(screen.getByLabelText(/Parent OU ID/)).toBeRequired();
    expect(screen.getByLabelText(/Parent OU ID/)).toBeInvalid();

    rerender(
      inFlow('AUTHENTICATION', {
        data: {properties: {resolveFrom: 'prompt', parentOuId: 'ou-1'}},
      } as unknown as Resource),
    );

    expect(screen.getByLabelText(/Parent OU ID/)).toBeValid();
  });

  it('should not offer the parent organization unit in other flow types, or outside the flow builder', () => {
    const resource = {data: {properties: {resolveFrom: 'prompt'}}} as unknown as Resource;
    const {rerender} = render(inFlow('REGISTRATION', resource));

    expect(screen.queryByLabelText(/Parent OU ID/)).not.toBeInTheDocument();

    rerender(inFlow('USER_ONBOARDING', resource));
    expect(screen.queryByLabelText(/Parent OU ID/)).not.toBeInTheDocument();

    rerender(inFlow(undefined, resource));
    expect(screen.queryByLabelText(/Parent OU ID/)).not.toBeInTheDocument();
  });

  it('should write the trimmed parent organization unit when the field is committed', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {resolveFrom: 'prompt'}}} as unknown as Resource;

    render(inFlow('AUTHENTICATION', resource));

    const field = screen.getByLabelText(/Parent OU ID/);
    await user.type(field, '  ou-9  ');
    await user.tab();

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.parentOuId', 'ou-9', resource);
  });
  describe('clearing properties the strategy no longer uses', () => {
    const chooseStrategy = async (user: ReturnType<typeof userEvent.setup>, name: string) => {
      await user.click(screen.getByRole('combobox'));
      await user.click(screen.getByRole('option', {name}));
    };

    it('should reset parentOuId and directChildrenOnly when resolveFrom moves away from prompt', async () => {
      const user = userEvent.setup();
      const resource = {
        data: {properties: {resolveFrom: 'prompt', parentOuId: 'ou-1', directChildrenOnly: true}},
      } as unknown as Resource;

      render(inFlow('AUTHENTICATION', resource));
      await chooseStrategy(user, 'Caller');

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.resolveFrom', 'caller', resource);
      expect(mockOnChange).toHaveBeenCalledWith('data.properties.parentOuId', '', resource);
      expect(mockOnChange).toHaveBeenCalledWith('data.properties.directChildrenOnly', false, resource);
    });

    it('should clear them when resolveFrom moves to promptAll', async () => {
      const user = userEvent.setup();
      const resource = {
        data: {properties: {resolveFrom: 'prompt', directChildrenOnly: true}},
      } as unknown as Resource;

      render(inFlow('REGISTRATION', resource));
      await chooseStrategy(user, 'Prompt all');

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.resolveFrom', 'promptAll', resource);
      expect(mockOnChange).toHaveBeenCalledWith('data.properties.directChildrenOnly', false, resource);
      expect(mockOnChange).not.toHaveBeenCalledWith('data.properties.parentOuId', '', resource);
    });

    it('should only write resolveFrom when there is nothing to clear', async () => {
      const user = userEvent.setup();
      const resource = {data: {properties: {resolveFrom: 'prompt'}}} as unknown as Resource;

      render(inFlow('REGISTRATION', resource));
      await chooseStrategy(user, 'Caller');

      expect(mockOnChange).toHaveBeenCalledTimes(1);
      expect(mockOnChange).toHaveBeenCalledWith('data.properties.resolveFrom', 'caller', resource);
    });

    it('should keep parentOuId and directChildrenOnly when resolveFrom stays on prompt', async () => {
      const user = userEvent.setup();
      const resource = {
        data: {properties: {resolveFrom: 'caller', parentOuId: 'ou-1', directChildrenOnly: true}},
      } as unknown as Resource;

      render(inFlow('AUTHENTICATION', resource));
      await chooseStrategy(user, 'Prompt');

      expect(mockOnChange).toHaveBeenCalledTimes(1);
      expect(mockOnChange).toHaveBeenCalledWith('data.properties.resolveFrom', 'prompt', resource);
    });
  });
});
