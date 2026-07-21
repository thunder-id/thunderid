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
import {describe, it, expect, vi} from 'vitest';
import type {Agent} from '../../../../models/agent';
import EditAccessSettings from '../EditAccessSettings';

vi.mock('../AgentGroupsSection', () => ({default: () => <div data-testid="agent-groups" />}));
vi.mock('../AgentRolesSection', () => ({default: () => <div data-testid="agent-roles" />}));

describe('EditAccessSettings', () => {
  const mockAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent'};

  it('renders groups and roles directly, with no sub-tab experience', () => {
    render(<EditAccessSettings agent={mockAgent} />);

    expect(screen.getByTestId('agent-groups')).toBeInTheDocument();
    expect(screen.getByTestId('agent-roles')).toBeInTheDocument();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
  });
});
