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

/* eslint-disable @typescript-eslint/no-unsafe-return */
import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {Controller, type Control} from 'react-hook-form';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import EditAgentAttributes from '../EditAgentAttributes';

const {mockUseGetAgentTypes, mockUseGetAgentType} = vi.hoisted(() => ({
  mockUseGetAgentTypes: vi.fn(),
  mockUseGetAgentType: vi.fn(),
}));

vi.mock('@thunderid/configure-agent-types', () => ({
  useGetAgentTypes: () => mockUseGetAgentTypes(),
  useGetAgentType: (id?: string) => mockUseGetAgentType(id),
}));

vi.mock('@thunderid/configure-users', () => ({
  renderSchemaField: (fieldName: string, _fieldDef: unknown, control: Control<Record<string, unknown>>) => (
    <div key={fieldName} data-testid={`field-${fieldName}`}>
      <Controller
        name={fieldName}
        control={control}
        render={({field}) => (
          <input
            aria-label={`${fieldName}-input`}
            value={typeof field.value === 'string' ? field.value : ''}
            onChange={(e) => field.onChange(e.target.value)}
          />
        )}
      />
    </div>
  ),
}));

vi.mock('@thunderid/hooks', () => ({
  useResolveDisplayName: () => ({resolveDisplayName: (v: string) => v}),
}));

describe('EditAgentAttributes', () => {
  const mockOnFieldChange = vi.fn();
  const baseAgent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    name: 'Test',
    attributes: {email: 'a@b.com', count: 5},
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
          // Credentials should be filtered out of the editable schema fields.
          password: {type: 'string', credential: true},
        },
      },
      isLoading: false,
    });
  });

  it('shows a loading spinner while the schema is loading', () => {
    mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: true});
    render(<EditAgentAttributes agent={baseAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows the edit form directly, with a field per editable schema entry', () => {
    render(<EditAgentAttributes agent={baseAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByTestId('field-email')).toBeInTheDocument();
    expect(screen.getByTestId('field-count')).toBeInTheDocument();
    expect(screen.queryByTestId('field-password')).not.toBeInTheDocument();
  });

  it('has no local Save/Cancel controls — the page-level Save bar is the only save path', () => {
    render(<EditAgentAttributes agent={baseAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.queryByRole('button', {name: /save/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /cancel/i})).not.toBeInTheDocument();
  });

  it('shows a message when the schema has no editable fields', () => {
    mockUseGetAgentType.mockReturnValue({
      data: {id: 'schema-1', name: 'default', ouId: 'ou-1', schema: {password: {type: 'string', credential: true}}},
      isLoading: false,
    });
    render(<EditAgentAttributes agent={baseAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('No schema available for editing')).toBeInTheDocument();
  });

  it('does not call onFieldChange on initial mount', () => {
    render(<EditAgentAttributes agent={baseAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(mockOnFieldChange).not.toHaveBeenCalled();
  });

  it('does not call onFieldChange once the schema finishes loading after mount', async () => {
    // Regression test: the schema (and therefore the editable fields/Controllers) can arrive
    // asynchronously after this component's own effects have already run once with no fields
    // registered yet. That later field-registration wave used to slip past a one-shot
    // "first render" guard and get mistaken for a real user edit.
    mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: true});
    const {rerender} = render(
      <EditAgentAttributes agent={baseAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
    );

    mockUseGetAgentType.mockReturnValue({
      data: {
        id: 'schema-1',
        name: 'default',
        ouId: 'ou-1',
        schema: {email: {type: 'string', required: true}, count: {type: 'number'}},
      },
      isLoading: false,
    });
    rerender(<EditAgentAttributes agent={baseAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    await screen.findByTestId('field-email');

    expect(mockOnFieldChange).not.toHaveBeenCalled();
  });

  it('stages the merged attributes into the shared editedAgent state as the user types', async () => {
    const user = userEvent.setup();
    render(<EditAgentAttributes agent={baseAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    const input = screen.getByLabelText('email-input');
    await user.clear(input);
    await user.type(input, 'new@b.com');

    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenLastCalledWith(
        'attributes',
        expect.objectContaining({email: 'new@b.com', count: 5}),
      );
    });
  });

  it('uses editedAgent.attributes over agent.attributes as the starting values', () => {
    render(
      <EditAgentAttributes
        agent={baseAgent}
        editedAgent={{attributes: {email: 'pending@b.com', count: 5}}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByLabelText('email-input')).toHaveValue('pending@b.com');
  });

  describe('read-only agents', () => {
    it('shows the read-only attribute summary instead of the edit form', () => {
      render(
        <EditAgentAttributes
          agent={{...baseAgent, isReadOnly: true}}
          editedAgent={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('email')).toBeInTheDocument();
      expect(screen.getByText('a@b.com')).toBeInTheDocument();
      expect(screen.queryByTestId('field-email')).not.toBeInTheDocument();
    });
  });
});
