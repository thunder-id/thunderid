// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {FlowVariableDefinition} from '@thunderid/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import VariableSharingRules from '../VariableSharingRules';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

const mockConnections = vi.hoisted(() => ({error: null as Error | null}));

vi.mock('../../variables-dialog/useFlowVariableConnections', () => ({
  default: () => ({
    connections: [
      {categories: ['identity-provider'], id: 'c-1', name: 'Google login', type: 'google'},
      {categories: ['identity-provider'], id: 'c-2', name: 'GitHub login', type: 'github'},
    ],
    error: mockConnections.error,
    refetch: vi.fn(),
  }),
}));

vi.mock('../../variables-dialog/ConnectionsLoadError', () => ({
  default: () => <div data-testid="connections-load-error" />,
}));

const variables: FlowVariableDefinition[] = [
  {
    category: 'IDENTITY_PROVIDER',
    defaultValue: '',
    identifier: 'connection1',
    label: 'Connection',
    type: 'CONNECTION_SELECT',
  },
  {defaultValue: 'support@acme.io', identifier: 'supportEmail', type: 'EMAIL_INPUT'},
];

const openCard = async (identifier: string): Promise<void> => {
  await userEvent.click(
    within(screen.getByTestId(`share-variable-${identifier}`)).getByRole('button', {expanded: false}),
  );
};
const editableOf = (identifier: string): HTMLElement =>
  within(screen.getByTestId(`share-variable-${identifier}`)).getByRole('switch', {name: 'Editable'});

describe('VariableSharingRules', () => {
  it('says the connections failed to load rather than showing none', () => {
    mockConnections.error = new Error('down');
    render(<VariableSharingRules variables={variables} rules={undefined} onChange={vi.fn()} />);
    mockConnections.error = null;

    expect(screen.getByTestId('connections-load-error')).toBeInTheDocument();
  });

  const onChange = vi.fn();

  beforeEach(() => {
    onChange.mockClear();
  });

  it('shows why a saved unit default is not accepted', async () => {
    render(
      <VariableSharingRules
        variables={variables}
        rules={{'variables.supportEmail': {editable: true, value: ['not-an-email']}}}
        onChange={onChange}
      />,
    );
    await openCard('supportEmail');

    expect(screen.getByText('Enter a default that is valid for this input.')).toBeInTheDocument();
  });

  it('opens one card at a time, as a labelled region of its own', async () => {
    render(<VariableSharingRules variables={variables} rules={undefined} onChange={onChange} />);

    await openCard('connection1');

    const header = within(screen.getByTestId('share-variable-connection1')).getByRole('button', {expanded: true});

    expect(header).toHaveAttribute('aria-controls', 'share-connection1-content');
    expect(screen.getByRole('region', {name: /Connection/})).toBeInTheDocument();

    await openCard('supportEmail');

    // The closing card is taken out once its collapse animation ends.
    await waitFor(() =>
      expect(within(screen.getByTestId('share-variable-connection1')).queryByRole('switch')).not.toBeInTheDocument(),
    );
    expect(within(screen.getByTestId('share-variable-supportEmail')).getByRole('switch')).toBeInTheDocument();
  });

  it('shows a card for every variable, collapsed, none of them editable until a rule says so', async () => {
    render(<VariableSharingRules variables={variables} rules={undefined} onChange={onChange} />);

    expect(screen.getByTestId('share-variable-connection1')).toBeInTheDocument();
    expect(screen.getByTestId('share-variable-supportEmail')).toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();

    await openCard('connection1');

    expect(editableOf('connection1')).not.toBeChecked();
    await openCard('supportEmail');
    expect(editableOf('supportEmail')).not.toBeChecked();
    expect(screen.queryByText('Choices they may pick from')).not.toBeInTheDocument();
  });

  it('lets the units set a variable by writing an editable rule', async () => {
    const user = userEvent.setup();

    render(<VariableSharingRules variables={variables} rules={undefined} onChange={onChange} />);
    await openCard('connection1');
    await user.click(editableOf('connection1'));

    expect(onChange).toHaveBeenCalledWith({'variables.connection1': {editable: true}});
  });

  it('keeps the other variables rules when one changes', async () => {
    const user = userEvent.setup();

    render(
      <VariableSharingRules
        variables={variables}
        rules={{'variables.supportEmail': {editable: true}}}
        onChange={onChange}
      />,
    );
    await openCard('connection1');
    await user.click(editableOf('connection1'));

    expect(onChange).toHaveBeenCalledWith({
      'variables.connection1': {editable: true},
      'variables.supportEmail': {editable: true},
    });
  });

  it('offers as the default only the choices the units may pick from', async () => {
    const user = userEvent.setup();

    render(
      <VariableSharingRules
        variables={variables}
        rules={{'variables.connection1': {editable: true, allowedValues: ['c-1']}}}
        onChange={onChange}
      />,
    );
    await openCard('connection1');
    await user.click(
      within(screen.getByTestId('share-variable-connection1')).getByRole('combobox', {
        name: /Organization unit default/,
      }),
    );

    expect(screen.getByRole('option', {name: 'Google login'})).toBeInTheDocument();
    expect(screen.queryByRole('option', {name: 'GitHub login'})).not.toBeInTheDocument();
  });

  it('leaves out of the default the choices that were taken away', async () => {
    const user = userEvent.setup();

    render(
      <VariableSharingRules
        variables={variables}
        rules={{'variables.connection1': {editable: true, excludedValues: ['c-2']}}}
        onChange={onChange}
      />,
    );
    await openCard('connection1');
    await user.click(
      within(screen.getByTestId('share-variable-connection1')).getByRole('combobox', {
        name: /Organization unit default/,
      }),
    );

    expect(screen.getByRole('option', {name: 'Google login'})).toBeInTheDocument();
    expect(screen.queryByRole('option', {name: 'GitHub login'})).not.toBeInTheDocument();
  });

  it('clears a default that the narrowed choices no longer allow', async () => {
    const user = userEvent.setup();

    render(
      <VariableSharingRules
        variables={variables}
        rules={{'variables.connection1': {editable: true, value: ['c-2']}}}
        onChange={onChange}
      />,
    );
    await openCard('connection1');

    const card = within(screen.getByTestId('share-variable-connection1'));

    await user.click(card.getByRole('combobox', {name: /Choices they may pick from/}));
    await user.click(screen.getByRole('option', {name: 'Google login'}));

    expect(onChange).toHaveBeenLastCalledWith({'variables.connection1': {editable: true, allowedValues: ['c-1']}});
  });

  it('offers to narrow the choices of an editable choice variable only', async () => {
    render(
      <VariableSharingRules
        variables={variables}
        rules={{'variables.connection1': {editable: true}, 'variables.supportEmail': {editable: true}}}
        onChange={onChange}
      />,
    );

    await openCard('connection1');

    const connection = within(screen.getByTestId('share-variable-connection1'));

    expect(connection.getByText('Choices they may pick from')).toBeInTheDocument();
    expect(connection.getByText('Choices taken away')).toBeInTheDocument();
    expect(connection.getByText('Organization unit default')).toBeInTheDocument();

    await openCard('supportEmail');

    const email = within(screen.getByTestId('share-variable-supportEmail'));

    expect(email.queryByText('Choices they may pick from')).not.toBeInTheDocument();
    expect(email.getByText('Organization unit default')).toBeInTheDocument();
  });

  it('names the Organization unit default dropdown of a choice variable by its label', async () => {
    render(<VariableSharingRules variables={variables} rules={undefined} onChange={onChange} />);
    await openCard('connection1');

    expect(screen.getByRole('combobox', {name: /Organization unit default/})).toBeInTheDocument();
  });

  it('writes the value a variable starts with', async () => {
    const user = userEvent.setup();

    render(
      <VariableSharingRules
        variables={[variables[1]]}
        rules={{'variables.supportEmail': {editable: true}}}
        onChange={onChange}
      />,
    );
    await openCard('supportEmail');
    await user.type(screen.getByPlaceholderText("Use the flow's default"), 'a');

    expect(onChange).toHaveBeenLastCalledWith({'variables.supportEmail': {editable: true, value: ['a']}});
  });

  it('holds a variable the units may not set to a value', async () => {
    const user = userEvent.setup();

    render(
      <VariableSharingRules
        variables={[variables[1]]}
        rules={{'variables.supportEmail': {editable: true, value: ['help@acme.io']}}}
        onChange={onChange}
      />,
    );
    await openCard('supportEmail');
    await user.click(editableOf('supportEmail'));

    expect(onChange).toHaveBeenCalledWith({'variables.supportEmail': {editable: false, value: ['help@acme.io']}});
  });

  it('says a variable the units may not set is held to its value', async () => {
    render(
      <VariableSharingRules
        variables={[variables[1]]}
        rules={{'variables.supportEmail': {editable: false, value: ['help@acme.io']}}}
        onChange={onChange}
      />,
    );

    await openCard('supportEmail');

    expect(screen.getByText('Organization unit default')).toBeInTheDocument();
    expect(screen.getByDisplayValue('help@acme.io')).toBeInTheDocument();
  });

  it('changes nothing while disabled', async () => {
    render(<VariableSharingRules variables={variables} rules={undefined} onChange={onChange} disabled />);
    await openCard('connection1');

    expect(editableOf('connection1')).toBeDisabled();
  });
});
