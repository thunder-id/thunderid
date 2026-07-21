/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CallProperties from '../CallProperties';
import type {Resource} from '@/features/flows/models/resources';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}));

vi.mock('react-router', () => ({
  useParams: () => ({flowId: 'current-flow-id'}),
}));

const mockUseGetFlows = vi.fn<() => unknown>();
vi.mock('@/features/flows/api/useGetFlows', () => ({
  default: (): unknown => mockUseGetFlows(),
}));

vi.mock('@/features/flows/hooks/useValidationStatus', () => ({
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

  it('renders the description and the flow dropdown label', () => {
    mockUseGetFlows.mockReturnValue({data: {flows: []}, isLoading: false, error: null});
    render(<CallProperties resource={makeResource()} onChange={onChange} />);
    expect(screen.getByText('Pick the flow to invoke when this node executes.')).toBeInTheDocument();
    expect(screen.getByText('Referenced flow')).toBeInTheDocument();
  });

  it('shows an error message when the flow list fails to load', () => {
    mockUseGetFlows.mockReturnValue({data: undefined, isLoading: false, error: new Error('boom')});
    render(<CallProperties resource={makeResource()} onChange={onChange} />);
    expect(screen.getByTestId('call-properties-error')).toBeInTheDocument();
  });

  it('disables the dropdown while flows are loading', () => {
    mockUseGetFlows.mockReturnValue({data: undefined, isLoading: true, error: null});
    render(<CallProperties resource={makeResource()} onChange={onChange} />);
    const select = screen.getByTestId('call-flow-ref-select');
    const combobox = select.querySelector('[role="combobox"]');
    expect(combobox).toHaveAttribute('aria-disabled', 'true');
  });

  it('lists flows from the API and excludes the flow being edited and sign-out flows', () => {
    mockUseGetFlows.mockReturnValue({
      data: {
        flows: [
          {id: 'flow-a', name: 'Flow A', flowType: 'AUTHENTICATION'},
          {id: 'flow-b', name: 'Flow B', flowType: 'REGISTRATION'},
          {id: 'flow-so', name: 'Sign Out', flowType: 'SIGNOUT'},
          {id: 'current-flow-id', name: 'Self', flowType: 'AUTHENTICATION'},
        ],
      },
      isLoading: false,
      error: null,
    });
    render(<CallProperties resource={makeResource()} onChange={onChange} />);
    fireEvent.mouseDown(screen.getByTestId('call-flow-ref-select').querySelector('[role="combobox"]')!);
    expect(screen.getByText('Flow A (AUTHENTICATION)')).toBeInTheDocument();
    expect(screen.getByText('Flow B (REGISTRATION)')).toBeInTheDocument();
    expect(screen.queryByText('Self (AUTHENTICATION)')).not.toBeInTheDocument();
    // Sign-out flows are not valid call targets and must not appear in the picker.
    expect(screen.queryByText('Sign Out (SIGNOUT)')).not.toBeInTheDocument();
  });

  it('writes the chosen flow id back to data.flow', () => {
    mockUseGetFlows.mockReturnValue({
      data: {flows: [{id: 'flow-a', name: 'Flow A', flowType: 'AUTHENTICATION'}]},
      isLoading: false,
      error: null,
    });
    const resource = makeResource();
    render(<CallProperties resource={resource} onChange={onChange} />);
    fireEvent.mouseDown(screen.getByTestId('call-flow-ref-select').querySelector('[role="combobox"]')!);
    fireEvent.click(screen.getByText('Flow A (AUTHENTICATION)'));
    expect(onChange).toHaveBeenCalledWith('data.flow', {ref: 'flow-a'}, resource);
  });
});
