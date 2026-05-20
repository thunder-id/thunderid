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

/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-unsafe-assignment */
import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import EditAgentAttributes from '../EditAgentAttributes';

const {mockUseGetAgentTypes, mockUseGetAgentType, mockUseUpdateAgent, mockMutateAsync} = vi.hoisted(() => ({
  mockUseGetAgentTypes: vi.fn(),
  mockUseGetAgentType: vi.fn(),
  mockUseUpdateAgent: vi.fn(),
  mockMutateAsync: vi.fn(),
}));

vi.mock('@thunderid/configure-agent-types', () => ({
  useGetAgentTypes: () => mockUseGetAgentTypes(),
  useGetAgentType: (id?: string) => mockUseGetAgentType(id),
}));

vi.mock('../../../../api/useUpdateAgent', () => ({
  default: () => mockUseUpdateAgent(),
}));

vi.mock('@thunderid/configure-users', () => ({
  renderSchemaField: vi.fn((fieldName: string) => (
    <div key={fieldName} data-testid={`field-${fieldName}`}>
      {fieldName}
    </div>
  )),
}));

vi.mock('@thunderid/hooks', () => ({
  useResolveDisplayName: () => ({resolveDisplayName: (v: string) => v}),
}));

describe('EditAgentAttributes', () => {
  const baseAgent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    name: 'Test',
    attributes: {email: 'a@b.com', count: 5, isAdmin: true, tags: ['a', 'b']},
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetAgentTypes.mockReturnValue({
      data: {types: [{id: 'schema-1', name: 'default', ouId: 'ou-1'}]},
    });
    mockUseGetAgentType.mockReturnValue({
      data: {
        id: 'schema-1',
        name: 'default',
        ouId: 'ou-1',
        schema: {
          email: {type: 'string', required: true},
          count: {type: 'number'},
          // Credentials should be filtered out from edit mode
          password: {type: 'string', credential: true},
        },
      },
      isLoading: false,
    });
    mockUseUpdateAgent.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      error: null,
      reset: vi.fn(),
    });
  });

  it('shows a loading spinner while the schema is loading', () => {
    mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: true});
    render(<EditAgentAttributes agent={baseAgent} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('renders attribute values in view mode', () => {
    render(<EditAgentAttributes agent={baseAgent} />);

    expect(screen.getByText('a@b.com')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('Yes')).toBeInTheDocument();
    expect(screen.getByText('a, b')).toBeInTheDocument();
  });

  it('shows an empty placeholder when there are no attributes', () => {
    render(<EditAgentAttributes agent={{...baseAgent, attributes: {}}} />);

    expect(screen.getByText('No attributes available.')).toBeInTheDocument();
  });

  it('shows the Edit button when there are editable schema fields', () => {
    render(<EditAgentAttributes agent={baseAgent} />);

    expect(screen.getByRole('button', {name: /^Edit$/i})).toBeInTheDocument();
  });

  it('hides the Edit button when only credential schema fields exist', () => {
    mockUseGetAgentType.mockReturnValue({
      data: {
        id: 'schema-1',
        name: 'default',
        ouId: 'ou-1',
        schema: {password: {type: 'string', credential: true}},
      },
      isLoading: false,
    });

    render(<EditAgentAttributes agent={baseAgent} />);

    expect(screen.queryByRole('button', {name: /^Edit$/i})).not.toBeInTheDocument();
  });

  it('switches to edit mode when Edit is clicked', async () => {
    const user = userEvent.setup();
    render(<EditAgentAttributes agent={baseAgent} />);

    await user.click(screen.getByRole('button', {name: /^Edit$/i}));

    // Editable fields show, credential fields are filtered
    expect(screen.getByTestId('field-email')).toBeInTheDocument();
    expect(screen.getByTestId('field-count')).toBeInTheDocument();
    expect(screen.queryByTestId('field-password')).not.toBeInTheDocument();
  });

  it('cancels edit mode and resets form when Cancel is clicked', async () => {
    const user = userEvent.setup();
    render(<EditAgentAttributes agent={baseAgent} />);

    await user.click(screen.getByRole('button', {name: /^Edit$/i}));
    await user.click(screen.getByRole('button', {name: /Cancel/i}));

    // Back to view mode
    expect(screen.getByRole('button', {name: /^Edit$/i})).toBeInTheDocument();
  });

  it('calls updateAgent.mutateAsync on submit', async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValue(undefined);
    const onSaved = vi.fn();
    render(<EditAgentAttributes agent={baseAgent} onSaved={onSaved} />);

    await user.click(screen.getByRole('button', {name: /^Edit$/i}));
    await user.click(screen.getByRole('button', {name: /Save/i}));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          agentId: 'agent-1',
          data: expect.objectContaining({
            attributes: expect.any(Object) as Record<string, unknown>,
          }),
        }),
      );
    });

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled();
    });
  });

  it('shows the update error when the mutation has an error', async () => {
    mockUseUpdateAgent.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      error: new Error('Update failed'),
      reset: vi.fn(),
    });
    const user = userEvent.setup();
    render(<EditAgentAttributes agent={baseAgent} />);

    await user.click(screen.getByRole('button', {name: /^Edit$/i}));

    expect(screen.getByText('Update failed')).toBeInTheDocument();
  });

  it('disables save while pending', async () => {
    mockUseUpdateAgent.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: true,
      error: null,
      reset: vi.fn(),
    });
    const user = userEvent.setup();
    render(<EditAgentAttributes agent={baseAgent} />);

    await user.click(screen.getByRole('button', {name: /^Edit$/i}));

    expect(screen.getByRole('button', {name: /Saving\.\.\./i})).toBeDisabled();
  });

  it('formats null/undefined attribute values as "-"', () => {
    render(
      <EditAgentAttributes
        agent={{
          ...baseAgent,
          attributes: {nothing: null, missing: undefined, complex: {nested: true}},
        }}
      />,
    );

    expect(screen.getAllByText('-').length).toBeGreaterThan(0);
    expect(screen.getByText('{"nested":true}')).toBeInTheDocument();
  });
});
