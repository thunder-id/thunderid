// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Resource} from '../../../../models/resources';
import CallProperties from '../CallProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}));

const mockUseGetFlows = vi.fn<() => unknown>();
vi.mock('../../../../api/useGetFlows', () => ({
  default: (): unknown => mockUseGetFlows(),
}));

vi.mock('../../../../hooks/useValidationStatus', () => ({
  default: () => ({selectedNotification: null}),
}));

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'call-step-1',
    type: 'CALL',
    category: 'WORKFLOW',
    resourceType: 'STEP',
    data: {flow: {ref: ''}},
    ...overrides,
  }) as Resource;

describe('CallProperties', () => {
  const onChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the mode and fixed-flow selector by default', () => {
    mockUseGetFlows.mockReturnValue({data: {flows: []}, isLoading: false, error: null});
    render(<CallProperties resource={makeResource()} onChange={onChange} />);
    expect(screen.getByText('Mode')).toBeInTheDocument();
    expect(screen.getByText('Referenced flow')).toBeInTheDocument();
  });

  it('shows an error message when the flow list fails to load', () => {
    mockUseGetFlows.mockReturnValue({data: undefined, isLoading: false, error: new Error('boom')});
    render(<CallProperties resource={makeResource()} onChange={onChange} />);
    expect(screen.getByTestId('call-properties-error')).toBeInTheDocument();
  });

  it('disables the fixed-flow selector while flows are loading', () => {
    mockUseGetFlows.mockReturnValue({data: undefined, isLoading: true, error: null});
    render(<CallProperties resource={makeResource()} onChange={onChange} />);
    const select = screen.getByTestId('call-flow-ref-select');
    expect(select.querySelector('[role="combobox"]')).toHaveAttribute('aria-disabled', 'true');
  });

  it('lists available flows except sign-out flows', () => {
    mockUseGetFlows.mockReturnValue({
      data: {
        flows: [
          {id: 'flow-a', name: 'Flow A', flowType: 'AUTHENTICATION'},
          {id: 'flow-b', name: 'Flow B', flowType: 'REGISTRATION'},
          {id: 'flow-so', name: 'Sign Out', flowType: 'SIGNOUT'},
          {id: 'current-flow-id', name: 'Current Flow', flowType: 'AUTHENTICATION'},
        ],
      },
      isLoading: false,
      error: null,
    });
    render(<CallProperties resource={makeResource()} onChange={onChange} />);
    fireEvent.mouseDown(screen.getByTestId('call-flow-ref-select').querySelector('[role="combobox"]')!);
    expect(screen.getByText('Flow A (AUTHENTICATION)')).toBeInTheDocument();
    expect(screen.getByText('Flow B (REGISTRATION)')).toBeInTheDocument();
    expect(screen.getByText('Current Flow (AUTHENTICATION)')).toBeInTheDocument();
    expect(screen.queryByText('Sign Out (SIGNOUT)')).not.toBeInTheDocument();
  });

  it('writes a selected fixed flow id to the resource', () => {
    mockUseGetFlows.mockReturnValue({
      data: {flows: [{id: 'flow-a', name: 'Flow A', flowType: 'AUTHENTICATION'}]},
      isLoading: false,
      error: null,
    });
    const resource = makeResource();
    render(<CallProperties resource={resource} onChange={onChange} />);
    fireEvent.mouseDown(screen.getByTestId('call-flow-ref-select').querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByText('Flow A (AUTHENTICATION)'));
    expect(onChange).toHaveBeenCalledWith('data.flow', {mode: 'fixed', flowId: 'flow-a'}, resource);
  });

  it('allows selecting an application flow type outside system flows and excludes sign-out', () => {
    mockUseGetFlows.mockReturnValue({data: {flows: []}, isLoading: false, error: null});
    const resource = makeResource({
      data: {flow: {mode: 'flow_type', flowType: 'AUTHENTICATION'}},
    } as Partial<Resource>);
    render(<CallProperties resource={resource} onChange={onChange} />);
    const flowType = screen.getByTestId('call-flow-type-select');
    fireEvent.mouseDown(flowType.querySelector('[role="combobox"]')!);
    expect(screen.getByRole('option', {name: 'AUTHENTICATION'})).toBeInTheDocument();
    expect(screen.getByRole('option', {name: 'REGISTRATION'})).toBeInTheDocument();
    expect(screen.getByRole('option', {name: 'RECOVERY'})).toBeInTheDocument();
    expect(screen.queryByRole('option', {name: 'SIGNOUT'})).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('option', {name: 'REGISTRATION'}));
    expect(onChange).toHaveBeenCalledWith('data.flow', {mode: 'flow_type', flowType: 'REGISTRATION'}, resource);
    expect(screen.queryByTestId('call-flow-ref-select')).not.toBeInTheDocument();
  });

  it('switches between fixed flow and flow type modes', () => {
    mockUseGetFlows.mockReturnValue({data: {flows: []}, isLoading: false, error: null});
    const resource = makeResource({
      data: {flow: {mode: 'flow_type', flowType: 'AUTHENTICATION'}},
    } as Partial<Resource>);
    const {rerender} = render(<CallProperties resource={resource} onChange={onChange} />);
    const select = screen.getByTestId('call-flow-source-select');
    fireEvent.mouseDown(select.querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByRole('option', {name: 'Select a flow'}));
    expect(onChange).toHaveBeenCalledWith('data.flow', {mode: 'fixed'}, resource);

    const fixedResource = makeResource({data: {flow: {mode: 'fixed'}}} as Partial<Resource>);
    rerender(<CallProperties resource={fixedResource} onChange={onChange} />);
    fireEvent.mouseDown(screen.getByTestId('call-flow-source-select').querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByRole('option', {name: 'Select a flow type'}));
    expect(onChange).toHaveBeenLastCalledWith(
      'data.flow',
      {mode: 'flow_type', flowType: 'AUTHENTICATION'},
      fixedResource,
    );
  });
});
