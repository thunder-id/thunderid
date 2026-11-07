// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
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

vi.mock('../../variables-dialog/useFlowVariableConnections', () => ({
  default: () => [
    {categories: ['identity-provider'], id: 'c-1', name: 'Google login', type: 'google'},
    {categories: ['identity-provider'], id: 'c-2', name: 'GitHub login', type: 'github'},
  ],
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
  const onChange = vi.fn();

  beforeEach(() => {
    onChange.mockClear();
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
    expect(connection.getByText('Unit default')).toBeInTheDocument();

    await openCard('supportEmail');

    const email = within(screen.getByTestId('share-variable-supportEmail'));

    expect(email.queryByText('Choices they may pick from')).not.toBeInTheDocument();
    expect(email.getByText('Unit default')).toBeInTheDocument();
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

    expect(screen.getByText('Unit default')).toBeInTheDocument();
    expect(screen.getByDisplayValue('help@acme.io')).toBeInTheDocument();
  });

  it('changes nothing while disabled', async () => {
    render(<VariableSharingRules variables={variables} rules={undefined} onChange={onChange} disabled />);
    await openCard('connection1');

    expect(editableOf('connection1')).toBeDisabled();
  });
});
