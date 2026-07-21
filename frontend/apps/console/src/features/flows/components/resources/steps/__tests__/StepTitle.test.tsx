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
import type {Edge, Node} from '@xyflow/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import StepTitle from '../StepTitle';

const mockSetNodes = vi.fn();
const mockSetEdges = vi.fn();
const mockGetNodes = vi.fn(
  (): Node[] =>
    [
      {id: 'credentials_auth', position: {x: 0, y: 0}, data: {}},
      {id: 'prompt_credentials', position: {x: 0, y: 0}, data: {}},
    ] as Node[],
);

const mockUseNodeId = vi.fn((): string | null => 'credentials_auth');

vi.mock('@xyflow/react', () => ({
  useNodeId: () => mockUseNodeId(),
  useReactFlow: () => ({getNodes: mockGetNodes, setNodes: mockSetNodes, setEdges: mockSetEdges}),
}));

const mockSetLastInteractedResource = vi.fn();
const mockSetLastInteractedStepId = vi.fn();
vi.mock('@/features/flows/hooks/useInteractionState', () => ({
  default: () => ({
    lastInteractedResource: undefined,
    lastInteractedStepId: '',
    setLastInteractedResource: mockSetLastInteractedResource,
    setLastInteractedStepId: mockSetLastInteractedStepId,
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback: string) => fallback,
  }),
}));

const startEditing = (): HTMLElement => {
  fireEvent.doubleClick(screen.getByText('credentials_auth'));
  return screen.getByRole('textbox', {name: 'Step ID'});
};

describe('StepTitle', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNodeId.mockReturnValue('credentials_auth');
  });

  it('should not enter edit mode on double-click without a node context', () => {
    mockUseNodeId.mockReturnValue(null);
    render(<StepTitle label="Identifier + Password" />);

    fireEvent.doubleClick(screen.getByText('Identifier + Password'));

    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('should render the step id as the title', () => {
    render(<StepTitle label="Identifier + Password" />);

    expect(screen.getByText('credentials_auth')).toBeInTheDocument();
    expect(screen.queryByText('Identifier + Password')).not.toBeInTheDocument();
  });

  it('should edit the step id on double-click', () => {
    render(<StepTitle label="Identifier + Password" />);

    expect(startEditing()).toHaveValue('credentials_auth');
  });

  it('should rename the node and rewire its edges on commit', () => {
    render(<StepTitle label="Identifier + Password" />);

    const input = startEditing();
    fireEvent.change(input, {target: {value: 'verify_credentials'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    const nodesUpdater = mockSetNodes.mock.calls[0][0] as (nodes: Node[]) => Node[];
    expect(nodesUpdater([{id: 'credentials_auth', position: {x: 0, y: 0}, data: {}}] as Node[])[0].id).toBe(
      'verify_credentials',
    );

    const edgesUpdater = mockSetEdges.mock.calls[0][0] as (edges: Edge[]) => Edge[];
    const rewired = edgesUpdater([
      {id: 'e1', source: 'prompt_credentials', target: 'credentials_auth', sourceHandle: 'action_001_NEXT'},
      {id: 'e2', source: 'credentials_auth', target: 'next_step', sourceHandle: 'credentials_auth_NEXT'},
      {id: 'e3', source: 'credentials_auth', target: 'prompt_credentials', sourceHandle: 'credentials_auth_INCOMPLETE'},
    ] as Edge[]);
    expect(rewired[0]).toMatchObject({target: 'verify_credentials', sourceHandle: 'action_001_NEXT'});
    expect(rewired[1]).toMatchObject({source: 'verify_credentials', sourceHandle: 'verify_credentials_NEXT'});
    expect(rewired[2]).toMatchObject({source: 'verify_credentials', sourceHandle: 'verify_credentials_INCOMPLETE'});
  });

  it('should reject an id that is already used by another node', () => {
    render(<StepTitle label="Identifier + Password" />);

    const input = startEditing();
    fireEvent.change(input, {target: {value: 'prompt_credentials'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(mockSetNodes).not.toHaveBeenCalled();
    expect(input).toHaveAttribute('aria-invalid', 'true');
  });

  it('should reject an id with invalid characters', () => {
    render(<StepTitle label="Identifier + Password" />);

    const input = startEditing();
    fireEvent.change(input, {target: {value: 'has spaces!'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(mockSetNodes).not.toHaveBeenCalled();
    expect(input).toHaveAttribute('aria-invalid', 'true');
  });

  it('should cancel on Escape without renaming', () => {
    render(<StepTitle label="Identifier + Password" />);

    const input = startEditing();
    fireEvent.change(input, {target: {value: 'discarded'}});
    fireEvent.keyDown(input, {key: 'Escape'});

    expect(mockSetNodes).not.toHaveBeenCalled();
    expect(screen.getByText('credentials_auth')).toBeInTheDocument();
  });

  it('should discard an invalid draft on blur', () => {
    render(<StepTitle label="Identifier + Password" />);

    const input = startEditing();
    fireEvent.change(input, {target: {value: 'has spaces!'}});
    fireEvent.blur(input);

    expect(mockSetNodes).not.toHaveBeenCalled();
    expect(screen.getByText('credentials_auth')).toBeInTheDocument();
  });

  it('should not commit an unchanged id', () => {
    render(<StepTitle label="Identifier + Password" />);

    const input = startEditing();
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(mockSetNodes).not.toHaveBeenCalled();
  });
});
