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
import type {Agent} from '../../../../models/agent';
import OwnerSummarySection from '../OwnerSummarySection';

const {mockUseGetUsers} = vi.hoisted(() => ({
  mockUseGetUsers: vi.fn(),
}));

vi.mock('@thunderid/configure-users', () => ({
  useGetUsers: (...args: unknown[]): unknown => mockUseGetUsers(...args) as unknown,
}));

describe('OwnerSummarySection', () => {
  const mockAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent', owner: 'user-1'};

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', display: 'Alice'}]},
      isLoading: false,
    });
  });

  it('shows the resolved owner label', () => {
    render(<OwnerSummarySection agent={mockAgent} />);

    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('shows a placeholder when no owner is assigned', () => {
    render(<OwnerSummarySection agent={{...mockAgent, owner: undefined}} />);

    expect(screen.getByText('No owner assigned')).toBeInTheDocument();
  });

  it('falls back to the raw owner id when the user list has not resolved it', () => {
    mockUseGetUsers.mockReturnValue({data: {users: []}, isLoading: false});
    render(<OwnerSummarySection agent={mockAgent} />);

    expect(screen.getByText('user-1')).toBeInTheDocument();
  });

  it('never shows an editable control', () => {
    render(<OwnerSummarySection agent={mockAgent} />);

    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
