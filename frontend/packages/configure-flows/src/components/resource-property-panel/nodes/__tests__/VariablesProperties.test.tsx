// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {ContextType} from 'react';
import {describe, it, expect, vi} from 'vitest';
import FlowVariablesContext, {type VariablesGroup} from '../../../../context/FlowVariablesContext';
import type {Resource} from '../../../../models/resources';
import VariablesProperties from '../VariablesProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, second?: string | Record<string, unknown>, third?: Record<string, string>) => {
      if (typeof second === 'object') {
        const count = Number(second['count']);
        const text = (second[count === 1 ? 'defaultValue_one' : 'defaultValue_other'] as string | undefined) ?? key;

        return text.replace('{{count}}', String(count));
      }

      return Object.entries(third ?? {}).reduce(
        (text, [name, value]) => text.replace(`{{${name}}}`, value),
        second ?? key,
      );
    },
  }),
}));

const mockConnections = vi.hoisted(() => ({error: null as Error | null}));

vi.mock('../../../variables-dialog/useFlowVariableConnections', () => ({
  default: () => ({connections: [], error: mockConnections.error, refetch: vi.fn()}),
}));

vi.mock('../../../variables-dialog/ConnectionsLoadError', () => ({
  default: () => <div data-testid="connections-load-error" />,
}));

vi.mock('../../../variables-dialog/VariablesPreviewDialog', () => ({
  default: ({open, variables}: {open: boolean; variables: unknown[]}) =>
    open ? <div data-testid="preview-dialog" data-count={variables.length} /> : null,
}));

const resource = {
  data: {inputs: [{identifier: 'a', label: 'Alpha', type: 'TEXT_INPUT'}]},
  id: 'vars',
  type: 'VARIABLES',
} as unknown as Resource;

describe('VariablesProperties', () => {
  it('should say the connections failed to load rather than showing none', () => {
    mockConnections.error = new Error('down');
    render(<VariablesProperties resource={resource} onChange={vi.fn()} />);
    mockConnections.error = null;

    expect(screen.getByTestId('connections-load-error')).toBeInTheDocument();
  });

  it('should not offer a Show variables button', () => {
    render(<VariablesProperties resource={resource} onChange={vi.fn()} />);

    expect(screen.queryByText('Show variables')).not.toBeInTheDocument();
    expect(screen.queryByTestId('variables-show')).not.toBeInTheDocument();
  });

  it('should configure the selected variable directly, with Type and Allowed connection types labels', () => {
    const connection = {
      data: {inputs: [{identifier: 'connection1', type: 'CONNECTION_SELECT', category: 'IDENTITY_PROVIDER'}]},
      id: 'vars',
      type: 'VARIABLES',
    } as unknown as Resource;

    render(<VariablesProperties resource={connection} onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', {name: /connection1/, expanded: false}));

    expect(screen.getByTestId('variable-editor')).toBeInTheDocument();
    expect(screen.getByText('Type')).toBeInTheDocument();
    expect(screen.queryByText('Input type')).not.toBeInTheDocument();
    expect(screen.getByText('Allowed connection types')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('All')).toBeInTheDocument();
  });

  it('should write an edit to the data of the node outside a flow', () => {
    const onChange = vi.fn();

    render(<VariablesProperties resource={resource} onChange={onChange} />);
    fireEvent.click(screen.getByText('Alpha'));
    fireEvent.change(screen.getByLabelText('Label'), {target: {value: 'Beta'}});

    expect(onChange).toHaveBeenCalledWith(
      'data.inputs',
      [{identifier: 'a', label: 'Beta', type: 'TEXT_INPUT'}],
      resource,
    );
  });

  it('should keep every variable collapsed until it is clicked, and collapse it when clicked again', () => {
    render(<VariablesProperties resource={resource} onChange={vi.fn()} />);

    expect(screen.queryByTestId('variable-editor')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('Alpha'));

    expect(screen.getByTestId('variable-editor')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Alpha'));

    expect(screen.queryByTestId('variable-editor')).not.toBeInTheDocument();
  });

  it('should add and remove a variable', () => {
    const onChange = vi.fn();

    render(<VariablesProperties resource={resource} onChange={onChange} />);
    fireEvent.click(screen.getByTestId('variables-add'));

    expect(onChange.mock.calls[0][1]).toHaveLength(2);

    fireEvent.click(screen.getByRole('button', {name: 'Remove'}));

    expect(onChange).toHaveBeenLastCalledWith('data.inputs', [], resource);
  });

  it('should remove a variable from the keyboard without opening its row', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(<VariablesProperties resource={resource} onChange={onChange} />);
    screen.getByRole('button', {name: 'Remove'}).focus();
    await user.keyboard('{Enter}');

    expect(onChange).toHaveBeenLastCalledWith('data.inputs', [], resource);
    expect(screen.queryByTestId('variable-editor')).not.toBeInTheDocument();
  });

  it('should show an empty state when the node declares no variables', () => {
    render(
      <VariablesProperties resource={{id: 'vars', type: 'VARIABLES'} as unknown as Resource} onChange={vi.fn()} />,
    );

    expect(screen.getByTestId('variables-empty')).toBeInTheDocument();
    expect(screen.queryByTestId('variable-editor')).not.toBeInTheDocument();
  });

  it('should open the preview with the variables of the node when Preview is clicked', () => {
    render(<VariablesProperties resource={resource} onChange={vi.fn()} />);

    expect(screen.queryByTestId('preview-dialog')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('variables-preview'));

    expect(screen.getByTestId('preview-dialog')).toHaveAttribute('data-count', '1');
  });

  describe('in a flow with several variables nodes', () => {
    const groups: VariablesGroup[] = [
      {nodeId: 'other', variables: [{defaultValue: '', identifier: 'b', type: 'TEXT_INPUT'}]},
      {nodeId: 'vars', variables: [{defaultValue: '', identifier: 'a', label: 'Alpha', type: 'TEXT_INPUT'}]},
    ];

    it('should write an edit to this node through the panel, whatever other variables nodes the flow has', () => {
      const onChange = vi.fn();

      render(
        <FlowVariablesContext.Provider
          value={{
            countBindings: () => 0,
            groups,
            isShared: () => false,
            openVariablesNode: vi.fn(),
            removeBindings: vi.fn(),
            renameBindings: vi.fn(),
            variables: [],
            variablesNodeBefore: () => undefined,
          }}
        >
          <VariablesProperties resource={resource} onChange={onChange} />
        </FlowVariablesContext.Provider>,
      );
      fireEvent.click(screen.getByText('Alpha'));
      fireEvent.change(screen.getByLabelText('Label'), {target: {value: 'Beta'}});

      expect(onChange).toHaveBeenCalledWith(
        'data.inputs',
        [{defaultValue: '', identifier: 'a', label: 'Beta', type: 'TEXT_INPUT'}],
        resource,
      );
    });

    const renderWith = (overrides: Partial<ContextType<typeof FlowVariablesContext>>) => {
      const value = {
        countBindings: () => 0,
        groups,
        isShared: () => false,
        openVariablesNode: vi.fn(),
        removeBindings: vi.fn(),
        renameBindings: vi.fn(),
        variables: [],
        variablesNodeBefore: () => undefined,
        ...overrides,
      };
      const onChange = vi.fn();

      render(
        <FlowVariablesContext.Provider value={value}>
          <VariablesProperties resource={resource} onChange={onChange} />
        </FlowVariablesContext.Provider>,
      );

      return {...value, onChange};
    };

    const rename = (identifier: string): void => {
      fireEvent.click(screen.getByText('Alpha'));
      fireEvent.click(screen.getByTestId('variable-identifier-edit'));
      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: identifier}});
      fireEvent.click(screen.getByTestId('variable-identifier-save'));
    };

    it('should rename a variable and its bindings once the edit is confirmed', () => {
      const value = renderWith({});

      fireEvent.click(screen.getByText('Alpha'));
      fireEvent.click(screen.getByTestId('variable-identifier-edit'));
      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'fresh'}});
      expect(value.onChange).not.toHaveBeenCalled();
      expect(value.renameBindings).not.toHaveBeenCalled();

      fireEvent.click(screen.getByTestId('variable-identifier-save'));

      expect(value.renameBindings).toHaveBeenCalledWith('a', 'fresh');
      expect(value.onChange).toHaveBeenCalledWith(
        'data.inputs',
        [{defaultValue: '', identifier: 'fresh', label: 'Alpha', type: 'TEXT_INPUT'}],
        resource,
      );
    });

    it('should keep the identifier when the edit is cancelled', () => {
      const value = renderWith({});

      fireEvent.click(screen.getByText('Alpha'));
      fireEvent.click(screen.getByTestId('variable-identifier-edit'));
      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'fresh'}});
      fireEvent.click(screen.getByTestId('variable-identifier-cancel'));

      expect(value.onChange).not.toHaveBeenCalled();
      expect(screen.getByLabelText('Identifier').value).toBe('a');
    });

    it('should not accept an identifier another variable uses', () => {
      renderWith({});

      fireEvent.click(screen.getByText('Alpha'));
      fireEvent.click(screen.getByTestId('variable-identifier-edit'));
      fireEvent.change(screen.getByLabelText('Identifier'), {target: {value: 'b'}});

      expect(screen.getByTestId('variable-identifier-save')).toBeDisabled();
    });

    it('should ask before renaming a variable the sharing policies name', () => {
      const value = renderWith({isShared: (identifier: string) => identifier === 'a'});

      rename('fresh');

      expect(screen.getByTestId('variable-change-dialog')).toHaveTextContent('deleted when you save');
      expect(value.renameBindings).not.toHaveBeenCalled();

      fireEvent.click(screen.getByTestId('variable-change-confirm'));

      expect(value.renameBindings).toHaveBeenCalledWith('a', 'fresh');
    });

    it('should ask before removing a variable that steps use, and unbind them', () => {
      const value = renderWith({countBindings: () => 2});

      fireEvent.click(screen.getByRole('button', {name: 'Remove'}));

      expect(screen.getByTestId('variable-change-dialog')).toHaveTextContent('2 steps use this variable');
      expect(value.onChange).not.toHaveBeenCalled();

      fireEvent.click(screen.getByTestId('variable-change-confirm'));

      expect(value.removeBindings).toHaveBeenCalledWith('a');
      expect(value.onChange).toHaveBeenCalledWith('data.inputs', [], resource);
    });

    it('should preview only the variables of this node', () => {
      render(
        <FlowVariablesContext.Provider
          value={{
            countBindings: () => 0,
            groups,
            isShared: () => false,
            openVariablesNode: vi.fn(),
            removeBindings: vi.fn(),
            renameBindings: vi.fn(),
            variables: [],
            variablesNodeBefore: () => undefined,
          }}
        >
          <VariablesProperties resource={resource} onChange={vi.fn()} />
        </FlowVariablesContext.Provider>,
      );
      fireEvent.click(screen.getByTestId('variables-preview'));

      expect(screen.getByTestId('preview-dialog')).toHaveAttribute('data-count', '1');
    });
  });
});
