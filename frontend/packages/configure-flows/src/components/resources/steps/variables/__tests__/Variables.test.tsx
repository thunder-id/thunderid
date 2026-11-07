// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, it, expect, vi} from 'vitest';
import type {Step} from '../../../../../models/steps';
import Variables from '../Variables';

const setLastInteractedResource = vi.fn();
const setLastInteractedStepId = vi.fn();
const setIsOpenResourcePropertiesPanel = vi.fn();

const deleteElements = vi.fn();

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({deleteElements}),
  Handle: ({type, position, id}: {type: string; position: string; id: string | undefined}) => (
    <div data-testid={`handle-${type}`} data-position={position} data-handle-id={id} />
  ),
  Position: {
    Left: 'left',
    Right: 'right',
  },
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue?: string, options?: {count?: number}) =>
      (defaultValue ?? _key).replace('{{count}}', String(options?.count ?? '')),
  }),
}));

vi.mock('../../StepTitle', () => ({
  default: ({label}: {label: string}) => <span data-testid="step-title">{label}</span>,
}));

vi.mock('../../../../../hooks/useInteractionState', () => ({
  default: () => ({setLastInteractedResource, setLastInteractedStepId}),
}));

vi.mock('../../../../../hooks/useUIPanelState', () => ({
  default: () => ({setIsOpenResourcePropertiesPanel}),
}));

describe('Variables', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should be labelled Resolve Variables', () => {
    render(<Variables id="vars" />);

    expect(screen.getByTestId('variables-node-content')).toHaveTextContent('Resolve Variables');
  });

  it('should count the variables the node declares', () => {
    render(<Variables id="vars" data={{inputs: [{identifier: 'a'}, {identifier: 'b'}]}} />);

    expect(screen.getByTestId('variables-node-count')).toHaveTextContent('2 variables');
  });

  it('should count none when the node declares nothing', () => {
    render(<Variables id="vars" />);

    expect(screen.getByTestId('variables-node-count')).toHaveTextContent('0 variables');
  });

  it('should open the configuration panel for the node from its cog', () => {
    render(<Variables id="vars" data={{inputs: [{identifier: 'a'}]}} />);

    fireEvent.click(screen.getByTestId('variables-node-configure'));

    expect(setLastInteractedStepId).toHaveBeenCalledWith('vars');

    const resource = setLastInteractedResource.mock.calls[0][0] as Step;

    expect(resource.id).toBe('vars');
    expect(resource.type).toBe('VARIABLES');
    expect(resource.category).toBe('WORKFLOW');
    expect(resource.deletable).toBe(true);
    expect(resource.data?.['inputs']).toEqual([{identifier: 'a'}]);
    expect(setIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(true);
  });

  it('should select the node without opening the panel when its body is clicked', () => {
    render(<Variables id="vars" />);

    fireEvent.click(screen.getByTestId('variables-node-content'));

    expect(setLastInteractedStepId).toHaveBeenCalledWith('vars');
    expect(setIsOpenResourcePropertiesPanel).not.toHaveBeenCalled();
  });

  it('should take paths in on the left', () => {
    render(<Variables id="vars" />);

    expect(screen.getByTestId('handle-target')).toHaveAttribute('data-position', 'left');
  });

  it('should send the path on from the right, with a handle named after the node', () => {
    render(<Variables id="vars" />);

    const handle = screen.getByTestId('handle-source');
    expect(handle).toHaveAttribute('data-position', 'right');
    expect(handle).toHaveAttribute('data-handle-id', 'vars_NEXT');
  });

  it('should remove its node from the graph when deleted', () => {
    render(<Variables id="vars" />);

    fireEvent.click(screen.getByTestId('variables-node-delete'));

    expect(deleteElements).toHaveBeenCalledWith({nodes: [{id: 'vars'}]});
  });
});
