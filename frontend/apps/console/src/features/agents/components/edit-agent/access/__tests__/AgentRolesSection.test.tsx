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

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentRolesSection from '../AgentRolesSection';

const {mockUseGetAgentRoles} = vi.hoisted(() => ({
  mockUseGetAgentRoles: vi.fn(),
}));

vi.mock('../../../../api/useGetAgentRoles', () => ({
  default: (...args: unknown[]): unknown => mockUseGetAgentRoles(...args) as unknown,
}));

describe('AgentRolesSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows a loading indicator while roles are loading', () => {
    mockUseGetAgentRoles.mockReturnValue({data: undefined, isLoading: true});
    render(<AgentRolesSection agentId="agent-1" />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows an error message instead of the empty-state placeholder when the request fails', () => {
    mockUseGetAgentRoles.mockReturnValue({data: undefined, isLoading: false, isError: true});
    render(<AgentRolesSection agentId="agent-1" />);

    expect(screen.getByText('Failed to load roles for this agent.')).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('This agent does not have any roles assigned.')).not.toBeInTheDocument();
  });

  it('renders role names once loaded', () => {
    mockUseGetAgentRoles.mockReturnValue({
      data: {totalResults: 1, startIndex: 1, count: 1, roles: ['order-service-reader']},
      isLoading: false,
    });

    render(<AgentRolesSection agentId="agent-1" />);

    expect(screen.getByText('order-service-reader')).toBeInTheDocument();
  });

  it('renders roles as a read-only input, matching the Allowed User Types display', () => {
    mockUseGetAgentRoles.mockReturnValue({
      data: {totalResults: 1, startIndex: 1, count: 1, roles: ['order-service-reader']},
      isLoading: false,
    });

    render(<AgentRolesSection agentId="agent-1" />);

    const input = screen.getByRole('combobox');
    expect(input).toHaveAttribute('readonly');
    const chip = screen.getByText('order-service-reader').closest('.MuiChip-root');
    expect(chip).not.toBeNull();
    expect(chip?.querySelector('svg')).not.toBeInTheDocument();
  });

  it('does not show a dropdown arrow, since this list is not expandable', () => {
    mockUseGetAgentRoles.mockReturnValue({
      data: {totalResults: 1, startIndex: 1, count: 1, roles: ['order-service-reader']},
      isLoading: false,
    });

    render(<AgentRolesSection agentId="agent-1" />);

    expect(document.querySelector('.MuiAutocomplete-popupIndicator')).not.toBeInTheDocument();
  });

  it('does not show the empty-state placeholder once the agent has roles', () => {
    mockUseGetAgentRoles.mockReturnValue({
      data: {totalResults: 1, startIndex: 1, count: 1, roles: ['order-service-reader']},
      isLoading: false,
    });

    render(<AgentRolesSection agentId="agent-1" />);

    expect(screen.queryByPlaceholderText('This agent does not have any roles assigned.')).not.toBeInTheDocument();
  });

  it('shows a placeholder when the agent has no roles', () => {
    mockUseGetAgentRoles.mockReturnValue({
      data: {totalResults: 0, startIndex: 1, count: 0, roles: []},
      isLoading: false,
    });

    render(<AgentRolesSection agentId="agent-1" />);

    expect(screen.getByPlaceholderText('This agent does not have any roles assigned.')).toBeInTheDocument();
  });

  it('links to the Roles management page from within the description', () => {
    mockUseGetAgentRoles.mockReturnValue({
      data: {totalResults: 0, startIndex: 1, count: 0, roles: []},
      isLoading: false,
    });

    render(<AgentRolesSection agentId="agent-1" />);

    expect(screen.getByRole('link', {name: 'Roles page'})).toHaveAttribute('href', '/roles');
  });
});
