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
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import AttributesSummarySection from '../AttributesSummarySection';

const {mockUseGetAgentTypes, mockUseGetAgentType} = vi.hoisted(() => ({
  mockUseGetAgentTypes: vi.fn(),
  mockUseGetAgentType: vi.fn(),
}));

vi.mock('@thunderid/configure-agent-types', () => ({
  useGetAgentTypes: () => mockUseGetAgentTypes(),
  useGetAgentType: (id?: string) => mockUseGetAgentType(id),
}));

vi.mock('@thunderid/hooks', () => ({
  useResolveDisplayName: () => ({resolveDisplayName: (v: string) => v}),
}));

describe('AttributesSummarySection', () => {
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
        schema: {email: {type: 'string', required: true}, count: {type: 'number'}},
      },
      isLoading: false,
    });
  });

  it('shows a loading spinner while the schema is loading', () => {
    mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: true});
    render(<AttributesSummarySection agent={baseAgent} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows the attribute key and formatted value for each attribute', () => {
    render(<AttributesSummarySection agent={baseAgent} />);

    expect(screen.getByText('email')).toBeInTheDocument();
    expect(screen.getByText('a@b.com')).toBeInTheDocument();
    expect(screen.getByText('count')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('isAdmin')).toBeInTheDocument();
    expect(screen.getByText('Yes')).toBeInTheDocument();
    expect(screen.getByText('tags')).toBeInTheDocument();
    expect(screen.getByText('a, b')).toBeInTheDocument();
  });

  it('shows an empty state when there are no attribute values', () => {
    render(<AttributesSummarySection agent={{...baseAgent, attributes: {}}} />);

    expect(screen.getByText('No attributes available.')).toBeInTheDocument();
  });

  it('never shows an edit action', () => {
    render(<AttributesSummarySection agent={baseAgent} />);

    expect(screen.queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();
  });

  it('shows the resolved display name for attributes when the schema defines one', () => {
    mockUseGetAgentType.mockReturnValue({
      data: {
        id: 'schema-1',
        name: 'default',
        ouId: 'ou-1',
        schema: {email: {type: 'string', displayName: 'Email Address'}},
      },
      isLoading: false,
    });
    render(<AttributesSummarySection agent={{...baseAgent, attributes: {email: 'a@b.com'}}} />);

    expect(screen.getByText('Email Address')).toBeInTheDocument();
  });
});
