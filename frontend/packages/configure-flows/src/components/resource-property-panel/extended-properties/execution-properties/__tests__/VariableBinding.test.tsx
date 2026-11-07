// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {FlowVariableDefinition, FlowVariableType} from '@thunderid/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowVariablesContext, {type FlowVariablesContextValue} from '../../../../../context/FlowVariablesContext';
import type {Resource} from '../../../../../models/resources';
import VariableBinding from '../VariableBinding';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string, options?: Record<string, string>) =>
      Object.entries(options ?? {}).reduce(
        (text, [name, value]) => text.replace(`{{${name}}}`, value),
        defaultValue ?? key,
      ),
  }),
}));

const declared: FlowVariableDefinition[] = [
  {defaultValue: '', identifier: 'connection1', label: 'Connection', type: 'CONNECTION_SELECT'},
  {defaultValue: 'support@acme.io', identifier: 'supportEmail', type: 'EMAIL_INPUT'},
];

describe('VariableBinding', () => {
  const onChange = vi.fn();
  const openVariablesNode = vi.fn();

  beforeEach(() => {
    onChange.mockClear();
    openVariablesNode.mockClear();
  });

  // The flow context, with a variables node before the step unless it is told there is none.
  const flowContext = (
    variables: FlowVariableDefinition[] | undefined,
    nodeBefore: string | null = 'variables-1',
  ): FlowVariablesContextValue => ({
    countBindings: () => 0,
    groups: variables ? [{nodeId: 'variables-1', variables}] : [],
    isShared: () => false,
    openVariablesNode,
    removeBindings: vi.fn(),
    renameBindings: vi.fn(),
    variables,
    variablesBefore: (stepId: string) => (stepId === 'executor-1' && nodeBefore ? (variables ?? []) : []),
    variablesNodeBefore: (stepId: string) => (stepId === 'executor-1' ? (nodeBefore ?? undefined) : undefined),
  });

  const renderBinding = (
    properties: Record<string, unknown> = {},
    variables: FlowVariableDefinition[] | null = declared,
    extra: {showLabel?: boolean} = {},
  ) => {
    const resource = {data: {properties}, id: 'executor-1'} as unknown as Resource;

    render(
      <FlowVariablesContext.Provider value={flowContext(variables ?? undefined, variables ? 'variables-1' : null)}>
        <VariableBinding
          resource={resource}
          onChange={onChange}
          propertyKey="idpId"
          htmlFor="idp"
          label="Connection"
          {...extra}
        >
          {() => <input id="idp" data-testid="control" />}
        </VariableBinding>
      </FlowVariablesContext.Provider>,
    );

    return resource;
  };

  it('shows the pen and the input, without a label of its own, while the property is not bound', () => {
    renderBinding();

    expect(screen.queryByText('Connection')).not.toBeInTheDocument();
    expect(screen.getByTestId('variable-binding-idpId')).toBeInTheDocument();
    expect(screen.getByTestId('control')).toBeInTheDocument();
  });

  it('names the pen after the property it binds, so each one is told apart', () => {
    renderBinding();

    expect(screen.getByRole('button', {name: 'Use a variable for Connection'})).toBe(
      screen.getByTestId('variable-binding-idpId'),
    );
  });

  it('draws the label above the input when asked to', () => {
    renderBinding({}, declared, {showLabel: true});

    expect(screen.getByText('Connection')).toBeInTheDocument();
    expect(screen.getByTestId('control')).toBeInTheDocument();
  });

  it('puts the pen beside an input that carries its own label, and names the variable once bound', () => {
    const resource = {data: {properties: {flag: '{{var(connection1)}}'}}} as unknown as Resource;

    render(
      <FlowVariablesContext.Provider value={flowContext(declared)}>
        <VariableBinding inline resource={resource} onChange={onChange} propertyKey="flag" label="Allow it">
          {() => <input type="checkbox" data-testid="control" />}
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

  it('hands the input the label, the value and a way to set it at the path of the key', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {idpId: 'idp-1'}}, id: 'executor-1'} as unknown as Resource;

    render(
      <FlowVariablesContext.Provider value={flowContext(declared)}>
        <VariableBinding resource={resource} onChange={onChange} propertyKey="idpId" label="Connection">
          {({commit, label, value}) => (
            <button type="button" data-testid="control" onClick={() => commit('idp-2')}>
              {label} {String(value)}
            </button>
          )}
        </VariableBinding>
      </FlowVariablesContext.Provider>,
    );

    expect(screen.getByTestId('control')).toHaveTextContent('Connection idp-1');

    await user.click(screen.getByTestId('control'));

    expect(onChange).toHaveBeenCalledWith('data.properties.idpId', 'idp-2', resource);
  });

  it('offers only the variables declared before the step, and says so when none are', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {}}, id: 'executor-1'} as unknown as Resource;
    const context = {...flowContext(declared), variablesBefore: () => [declared[1]]};

    render(
      <FlowVariablesContext.Provider value={context}>
        <VariableBinding resource={resource} onChange={onChange} propertyKey="idpId" label="Connection">
          {() => <input data-testid="control" />}
        </VariableBinding>
      </FlowVariablesContext.Provider>,
    );
    await user.click(screen.getByTestId('variable-binding-idpId'));

    expect(screen.getByRole('menuitem', {name: /supportEmail/})).toBeInTheDocument();
    expect(screen.queryByRole('menuitem', {name: /connection1/})).not.toBeInTheDocument();
  });

  it('says no variables are declared before the step when the flow has some elsewhere', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {}}, id: 'executor-1'} as unknown as Resource;
    const context = {...flowContext(declared), variablesBefore: () => []};

    render(
      <FlowVariablesContext.Provider value={context}>
        <VariableBinding resource={resource} onChange={onChange} propertyKey="idpId" label="Connection">
          {() => <input data-testid="control" />}
        </VariableBinding>
      </FlowVariablesContext.Provider>,
    );
    await user.click(screen.getByTestId('variable-binding-idpId'));

    expect(screen.getByTestId('variable-binding-none')).toHaveTextContent('No variables declared before this step');
  });

  describe('limiting the menu to what the property takes', () => {
    const mixed: FlowVariableDefinition[] = [
      {defaultValue: '', identifier: 'region', label: 'Region', type: 'TEXT_INPUT'},
      {defaultValue: 0, identifier: 'limit', label: 'Limit', type: 'NUMBER_INPUT'},
      {category: 'IDENTITY_PROVIDER', identifier: 'anyIdp', label: 'Any IdP', type: 'CONNECTION_SELECT'},
      {
        category: 'IDENTITY_PROVIDER',
        connectionTypes: ['google'],
        identifier: 'googleOnly',
        label: 'Google only',
        type: 'CONNECTION_SELECT',
      },
      {
        category: 'IDENTITY_PROVIDER',
        connectionTypes: ['github'],
        identifier: 'githubOnly',
        label: 'GitHub only',
        type: 'CONNECTION_SELECT',
      },
    ];

    const open = async (props: {accepts?: FlowVariableType[]; connectionType?: string}) => {
      const user = userEvent.setup();
      const resource = {data: {properties: {}}, id: 'executor-1'} as unknown as Resource;

      render(
        <FlowVariablesContext.Provider value={flowContext(mixed)}>
          <VariableBinding resource={resource} onChange={onChange} propertyKey="idpId" label="Connection" {...props}>
            {() => <input data-testid="control" />}
          </VariableBinding>
        </FlowVariablesContext.Provider>,
      );
      await user.click(screen.getByTestId('variable-binding-idpId'));
    };

    const offered = () => screen.getAllByRole('menuitem').map((item) => item.textContent);

    it('offers every variable when the property names no types', async () => {
      await open({});

      expect(offered()).toEqual(expect.arrayContaining(['Regionregion', 'Limitlimit', 'Any IdPanyIdp']));
    });

    it('offers only the variables of the types the property takes', async () => {
      await open({accepts: ['NUMBER_INPUT']});

      expect(screen.getByRole('menuitem', {name: /Limit/})).toBeInTheDocument();
      expect(screen.queryByRole('menuitem', {name: /Region/})).not.toBeInTheDocument();
      expect(screen.queryByRole('menuitem', {name: /Any IdP/})).not.toBeInTheDocument();
    });

    it('offers a connection property only the connection variables that allow its type', async () => {
      await open({accepts: ['CONNECTION_SELECT'], connectionType: 'GOOGLE'});

      expect(screen.getByRole('menuitem', {name: /Any IdP/})).toBeInTheDocument();
      expect(screen.getByRole('menuitem', {name: /Google only/})).toBeInTheDocument();
      expect(screen.queryByRole('menuitem', {name: /GitHub only/})).not.toBeInTheDocument();
      expect(screen.queryByRole('menuitem', {name: /Region/})).not.toBeInTheDocument();
    });

    it('says no variable matches when none of the declared ones fit', async () => {
      await open({accepts: ['BOOLEAN_INPUT']});

      expect(screen.getByTestId('variable-binding-none')).toHaveTextContent('No variable of a matching type');
    });

    it('still shows a property bound to a variable the menu would not offer', () => {
      const resource = {data: {properties: {idpId: '{{var(region)}}'}}, id: 'executor-1'} as unknown as Resource;

      render(
        <FlowVariablesContext.Provider value={flowContext(mixed)}>
          <VariableBinding
            resource={resource}
            onChange={onChange}
            propertyKey="idpId"
            label="Connection"
            accepts={['CONNECTION_SELECT']}
          >
            {() => <input data-testid="control" />}
          </VariableBinding>
        </FlowVariablesContext.Provider>,
      );

      expect(screen.getByTestId('variable-binding-value-idpId')).toHaveTextContent('Region (region)');
    });
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

  it('removes the property, and only that one, when the variable is cleared', async () => {
    const user = userEvent.setup();
    const resource = renderBinding({idpId: '{{var(connection1)}}', timeout: 5});

    await user.click(screen.getByRole('button', {name: 'Use a fixed value'}));

    expect(onChange).toHaveBeenCalledWith('data.properties', {timeout: 5}, resource);
  });

  it('goes to the variables node before the step when Add variable is chosen', async () => {
    const user = userEvent.setup();

    renderBinding();
    await user.click(screen.getByTestId('variable-binding-idpId'));
    await user.click(screen.getByTestId('variable-binding-add'));

    expect(openVariablesNode).toHaveBeenCalledWith('variables-1');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('cannot add a variable while no variables node comes before the step', async () => {
    const user = userEvent.setup();
    const resource = {data: {properties: {}}, id: 'executor-1'} as unknown as Resource;

    render(
      <FlowVariablesContext.Provider value={flowContext(declared, null)}>
        <VariableBinding resource={resource} onChange={onChange} propertyKey="idpId" label="Connection">
          {() => <input data-testid="control" />}
        </VariableBinding>
      </FlowVariablesContext.Provider>,
    );
    await user.click(screen.getByTestId('variable-binding-idpId'));
    await user.click(screen.getByTestId('variable-binding-add'));

    expect(screen.getByTestId('variable-binding-add')).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByTestId('variable-binding-add')).toHaveTextContent(
      'Add a Variables step before this step in the flow first.',
    );
    expect(openVariablesNode).not.toHaveBeenCalled();
    // The menu stays open, since nothing was chosen.
    expect(screen.getByTestId('variable-binding-add')).toBeInTheDocument();
  });

  it('cannot add a variable while the flow has no variables node', async () => {
    const user = userEvent.setup();

    renderBinding({}, null);
    await user.click(screen.getByTestId('variable-binding-idpId'));

    expect(screen.getByTestId('variable-binding-add')).toHaveAttribute('aria-disabled', 'true');
  });

  it('keeps the add entry reachable from the keyboard when the flow has no variables', async () => {
    const user = userEvent.setup();

    renderBinding({}, null);
    await user.click(screen.getByTestId('variable-binding-idpId'));

    expect(screen.getByTestId('variable-binding-add')).toHaveFocus();
  });
});
