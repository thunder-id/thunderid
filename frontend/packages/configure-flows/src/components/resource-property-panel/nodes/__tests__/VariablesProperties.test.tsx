// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import FlowVariablesContext, {type VariablesGroup} from '../../../../context/FlowVariablesContext';
import type {Resource} from '../../../../models/resources';
import VariablesProperties from '../VariablesProperties';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue?: string, options?: Record<string, string>) =>
      Object.entries(options ?? {}).reduce(
        (text, [name, value]) => text.replace(`{{${name}}}`, value),
        defaultValue ?? _key,
      ),
  }),
}));

vi.mock('../../../variables-dialog/useFlowVariableConnections', () => ({default: () => []}));

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

    it('should save through the flow for this node only', () => {
      const setGroups = vi.fn();
      const onChange = vi.fn();

      render(
        <FlowVariablesContext.Provider value={{groups, setGroups, variables: []}}>
          <VariablesProperties resource={resource} onChange={onChange} />
        </FlowVariablesContext.Provider>,
      );
      fireEvent.click(screen.getByText('Alpha'));
      fireEvent.change(screen.getByLabelText('Label'), {target: {value: 'Beta'}});

      expect(setGroups).toHaveBeenCalledWith([
        {nodeId: 'vars', variables: [{defaultValue: '', identifier: 'a', label: 'Beta', type: 'TEXT_INPUT'}]},
      ]);
      expect(onChange).not.toHaveBeenCalled();
    });

    it('should preview only the variables of this node', () => {
      render(
        <FlowVariablesContext.Provider value={{groups, setGroups: vi.fn(), variables: []}}>
          <VariablesProperties resource={resource} onChange={vi.fn()} />
        </FlowVariablesContext.Provider>,
      );
      fireEvent.click(screen.getByTestId('variables-preview'));

      expect(screen.getByTestId('preview-dialog')).toHaveAttribute('data-count', '1');
    });
  });
});
