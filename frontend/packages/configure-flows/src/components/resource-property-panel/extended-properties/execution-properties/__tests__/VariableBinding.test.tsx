// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {FlowVariableDefinition} from '@thunderid/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowVariablesContext, {type VariablesGroup} from '../../../../../context/FlowVariablesContext';
import type {Resource} from '../../../../../models/resources';
import VariableBinding from '../VariableBinding';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue ?? key,
  }),
}));

vi.mock('../../../../variables-dialog/VariablesDialog', () => ({
  default: ({
    groups,
    initialIndex,
    onApply,
    open,
  }: {
    groups: {nodeId: string; variables: {identifier: string}[]}[];
    initialIndex?: number;
    onApply: (groups: unknown[]) => void;
    open: boolean;
  }) => (
    <div
      data-testid="variables-dialog"
      data-open={open}
      data-index={initialIndex}
      data-count={groups.reduce((total, group) => total + group.variables.length, 0)}
      data-first-count={groups[0]?.variables.length}
    >
      <button type="button" onClick={() => onApply(groups)}>
        apply-dialog
      </button>
    </div>
  ),
}));

const declared: FlowVariableDefinition[] = [
  {defaultValue: '', identifier: 'connection1', label: 'Connection', type: 'CONNECTION_SELECT'},
  {defaultValue: 'support@acme.io', identifier: 'supportEmail', type: 'EMAIL_INPUT'},
];

describe('VariableBinding', () => {
  const onChange = vi.fn();
  const setGroups = vi.fn();

  beforeEach(() => {
    onChange.mockClear();
    setGroups.mockClear();
  });

  const renderBinding = (
    properties: Record<string, unknown> = {},
    variables: FlowVariableDefinition[] | null = declared,
  ) => {
    const resource = {data: {properties}} as unknown as Resource;

    render(
      <FlowVariablesContext.Provider
        value={{
          groups: variables ? [{nodeId: 'variables-1', variables}] : [],
          setGroups,
          variables: variables ?? undefined,
        }}
      >
        <VariableBinding resource={resource} onChange={onChange} propertyKey="idpId" htmlFor="idp" label="Connection">
          <input id="idp" data-testid="control" />
        </VariableBinding>
      </FlowVariablesContext.Provider>,
    );

    return resource;
  };

  it('shows the label, the pen and the input while the property is not bound', () => {
    renderBinding();

    expect(screen.getByText('Connection')).toBeInTheDocument();
    expect(screen.getByTestId('variable-binding-idpId')).toBeInTheDocument();
    expect(screen.getByTestId('control')).toBeInTheDocument();
  });

  it('puts the pen beside an input that carries its own label, and names the variable once bound', () => {
    const resource = {data: {properties: {flag: '{{var(connection1)}}'}}} as unknown as Resource;

    render(
      <FlowVariablesContext.Provider
        value={{groups: [{nodeId: 'variables-1', variables: declared}], setGroups, variables: declared}}
      >
        <VariableBinding inline resource={resource} onChange={onChange} propertyKey="flag" label="Allow it">
          <input type="checkbox" data-testid="control" />
        </VariableBinding>
      </FlowVariablesContext.Provider>,
    );

    expect(screen.queryByTestId('control')).not.toBeInTheDocument();
    expect(screen.getByText('Allow it')).toBeInTheDocument();
    expect(screen.getByTestId('variable-binding-value-flag')).toHaveTextContent('Connection (connection1)');
    expect(screen.getByTestId('variable-binding-flag')).toBeInTheDocument();
  });

  it('offers the declared variables and an entry to add one', async () => {
    const user = userEvent.setup();

    renderBinding();
    await user.click(screen.getByTestId('variable-binding-idpId'));

    expect(screen.getByRole('menuitem', {name: /Connection/})).toBeInTheDocument();
    expect(screen.getByRole('menuitem', {name: /supportEmail/})).toBeInTheDocument();
    expect(screen.getByTestId('variable-binding-add')).toBeInTheDocument();
  });

  it('binds the property to the variable that is picked', async () => {
    const user = userEvent.setup();
    const resource = renderBinding();

    await user.click(screen.getByTestId('variable-binding-idpId'));
    await user.click(screen.getByRole('menuitem', {name: /Connection/}));

    expect(onChange).toHaveBeenCalledWith('data.properties.idpId', '{{var(connection1)}}', resource);
  });

  it('shows the variable instead of the input once the property is bound', () => {
    renderBinding({idpId: '{{var(connection1)}}'});

    expect(screen.queryByTestId('control')).not.toBeInTheDocument();
    expect(screen.getByTestId('variable-binding-value-idpId')).toHaveTextContent('Connection (connection1)');
  });

  it('goes back to a fixed value when the variable is cleared', async () => {
    const user = userEvent.setup();
    const resource = renderBinding({idpId: '{{var(connection1)}}'});

    await user.click(screen.getByRole('button', {name: 'Use a fixed value'}));

    expect(onChange).toHaveBeenCalledWith('data.properties.idpId', '', resource);
  });

  it('opens the variables dialog on a new variable and binds it once applied', async () => {
    const user = userEvent.setup();
    const resource = renderBinding();

    expect(screen.getByTestId('variables-dialog')).toHaveAttribute('data-open', 'false');

    await user.click(screen.getByTestId('variable-binding-idpId'));
    await user.click(screen.getByTestId('variable-binding-add'));

    const dialog = screen.getByTestId('variables-dialog');

    expect(dialog).toHaveAttribute('data-open', 'true');
    expect(dialog).toHaveAttribute('data-index', '2');
    expect(dialog).toHaveAttribute('data-count', '3');

    await user.click(screen.getByText('apply-dialog'));

    const applied = setGroups.mock.calls[0][0] as VariablesGroup[];

    expect(applied).toHaveLength(1);
    expect(applied[0].variables.map((variable) => variable.identifier)).toContain('variable3');
    expect(onChange).toHaveBeenCalledWith('data.properties.idpId', '{{var(variable3)}}', resource);
  });

  it('cannot add a variable while the flow has no variables node', async () => {
    const user = userEvent.setup();

    renderBinding({}, null);
    await user.click(screen.getByTestId('variable-binding-idpId'));

    expect(screen.getByTestId('variable-binding-add')).toHaveClass('Mui-disabled');
    expect(screen.queryByTestId('variables-dialog')).not.toBeInTheDocument();
  });

  it('lists the variables of every node, and adds a new one to the first node', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {}}} as unknown as Resource;
    const groups: VariablesGroup[] = [
      {nodeId: 'variables-1', variables: [declared[0]]},
      {nodeId: 'variables-2', variables: [declared[1]]},
    ];

    render(
      <FlowVariablesContext.Provider value={{groups, setGroups, variables: declared}}>
        <VariableBinding resource={resource} onChange={onChange} propertyKey="idpId" label="Connection">
          <input data-testid="control" />
        </VariableBinding>
      </FlowVariablesContext.Provider>,
    );
    await user.click(screen.getByTestId('variable-binding-idpId'));

    expect(screen.getByRole('menuitem', {name: /Connection/})).toBeInTheDocument();
    expect(screen.getByRole('menuitem', {name: /supportEmail/})).toBeInTheDocument();

    await user.click(screen.getByTestId('variable-binding-add'));

    const dialog = screen.getByTestId('variables-dialog');

    expect(dialog).toHaveAttribute('data-count', '3');
    expect(dialog).toHaveAttribute('data-first-count', '2');
    expect(dialog).toHaveAttribute('data-index', '1');

    await user.click(screen.getByText('apply-dialog'));

    expect(onChange).toHaveBeenCalledWith(
      'data.properties.idpId',
      expect.stringMatching(/^\{\{var\(variable3\)\}\}$/),
      resource,
    );
  });
});
