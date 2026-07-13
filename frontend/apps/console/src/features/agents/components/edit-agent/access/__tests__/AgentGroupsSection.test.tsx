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
import AgentGroupsSection from '../AgentGroupsSection';

const {mockUseGetAgentGroups} = vi.hoisted(() => ({
  mockUseGetAgentGroups: vi.fn(),
}));

vi.mock('../../../../api/useGetAgentGroups', () => ({
  default: (...args: unknown[]): unknown => mockUseGetAgentGroups(...args) as unknown,
}));

describe('AgentGroupsSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows a loading indicator while groups are loading', () => {
    mockUseGetAgentGroups.mockReturnValue({data: undefined, isLoading: true});
    render(<AgentGroupsSection agentId="agent-1" />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows an error message instead of the empty-state placeholder when the request fails', () => {
    mockUseGetAgentGroups.mockReturnValue({data: undefined, isLoading: false, isError: true});
    render(<AgentGroupsSection agentId="agent-1" />);

    expect(screen.getByText('Failed to load groups for this agent.')).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('This agent does not belong to any groups.')).not.toBeInTheDocument();
  });

  it('renders group names once loaded', () => {
    mockUseGetAgentGroups.mockReturnValue({
      data: {
        totalResults: 2,
        startIndex: 1,
        count: 2,
        groups: [
          {id: 'g1', name: 'platform-agents', ouId: 'ou-1'},
          {id: 'g2', name: 'order-service-readers', ouId: 'ou-1'},
        ],
      },
      isLoading: false,
    });

    render(<AgentGroupsSection agentId="agent-1" />);

    expect(screen.getByText('platform-agents')).toBeInTheDocument();
    expect(screen.getByText('order-service-readers')).toBeInTheDocument();
  });

  it('renders groups as a read-only input, matching the Allowed User Types display', () => {
    mockUseGetAgentGroups.mockReturnValue({
      data: {
        totalResults: 1,
        startIndex: 1,
        count: 1,
        groups: [{id: 'g1', name: 'platform-agents', ouId: 'ou-1'}],
      },
      isLoading: false,
    });

    render(<AgentGroupsSection agentId="agent-1" />);

    const input = screen.getByRole('combobox');
    expect(input).toHaveAttribute('readonly');
    const chip = screen.getByText('platform-agents').closest('.MuiChip-root');
    expect(chip).not.toBeNull();
    expect(chip?.querySelector('svg')).not.toBeInTheDocument();
  });

  it('does not show a dropdown arrow, since this list is not expandable', () => {
    mockUseGetAgentGroups.mockReturnValue({
      data: {
        totalResults: 1,
        startIndex: 1,
        count: 1,
        groups: [{id: 'g1', name: 'platform-agents', ouId: 'ou-1'}],
      },
      isLoading: false,
    });

    render(<AgentGroupsSection agentId="agent-1" />);

    expect(document.querySelector('.MuiAutocomplete-popupIndicator')).not.toBeInTheDocument();
  });

  it('does not show the empty-state placeholder once the agent has groups', () => {
    mockUseGetAgentGroups.mockReturnValue({
      data: {
        totalResults: 1,
        startIndex: 1,
        count: 1,
        groups: [{id: 'g1', name: 'platform-agents', ouId: 'ou-1'}],
      },
      isLoading: false,
    });

    render(<AgentGroupsSection agentId="agent-1" />);

    expect(screen.queryByPlaceholderText('This agent does not belong to any groups.')).not.toBeInTheDocument();
  });

  it('shows a placeholder when the agent has no groups', () => {
    mockUseGetAgentGroups.mockReturnValue({
      data: {totalResults: 0, startIndex: 1, count: 0, groups: []},
      isLoading: false,
    });

    render(<AgentGroupsSection agentId="agent-1" />);

    expect(screen.getByPlaceholderText('This agent does not belong to any groups.')).toBeInTheDocument();
  });

  it('links to the Groups management page from within the description', () => {
    mockUseGetAgentGroups.mockReturnValue({
      data: {totalResults: 0, startIndex: 1, count: 0, groups: []},
      isLoading: false,
    });

    render(<AgentGroupsSection agentId="agent-1" />);

    expect(screen.getByRole('link', {name: 'Groups page'})).toHaveAttribute('href', '/groups');
  });
});
