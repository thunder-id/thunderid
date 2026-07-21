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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import QuickCopySection from '../QuickCopySection';

// Mock the SettingsCard wrapper so DOM is easy to query.
vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, description, children}: {title: string; description: string; children: React.ReactNode}) => (
    <div data-testid="settings-card">
      <div data-testid="card-title">{title}</div>
      <div data-testid="card-description">{description}</div>
      {children}
    </div>
  ),
}));

describe('QuickCopySection (agent)', () => {
  const mockOnCopyToClipboard = vi.fn();

  const mockAgent: Agent = {
    id: 'agent-123',
    ouId: 'ou-1',
    type: 'default',
    name: 'Test Agent',
    owner: 'user-1',
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockOnCopyToClipboard.mockResolvedValue(undefined);
  });

  describe('Rendering', () => {
    it('renders the settings card', () => {
      render(<QuickCopySection agent={mockAgent} copiedField={null} onCopyToClipboard={mockOnCopyToClipboard} />);

      expect(screen.getByTestId('card-title')).toBeInTheDocument();
      expect(screen.getByTestId('card-description')).toBeInTheDocument();
    });

    it('renders the agent ID field', () => {
      render(<QuickCopySection agent={mockAgent} copiedField={null} onCopyToClipboard={mockOnCopyToClipboard} />);

      expect(screen.getByDisplayValue('agent-123')).toBeInTheDocument();
    });
  });

  describe('Copy Functionality', () => {
    it('copies the agent ID', async () => {
      const user = userEvent.setup();
      render(<QuickCopySection agent={mockAgent} copiedField={null} onCopyToClipboard={mockOnCopyToClipboard} />);

      const buttons = screen.getAllByRole('button');
      await user.click(buttons[0]);

      expect(mockOnCopyToClipboard).toHaveBeenCalledWith('agent-123', 'agent_id');
    });

    it('handles copy errors gracefully', async () => {
      const user = userEvent.setup();
      mockOnCopyToClipboard.mockRejectedValueOnce(new Error('Copy failed'));

      render(<QuickCopySection agent={mockAgent} copiedField={null} onCopyToClipboard={mockOnCopyToClipboard} />);

      const buttons = screen.getAllByRole('button');
      await user.click(buttons[0]);

      expect(mockOnCopyToClipboard).toHaveBeenCalled();
    });
  });

  describe('Visual Feedback', () => {
    it('shows the copied state for agent ID when copiedField === "agent_id"', () => {
      render(<QuickCopySection agent={mockAgent} copiedField="agent_id" onCopyToClipboard={mockOnCopyToClipboard} />);

      // Copy buttons render different icons; check via tooltip text
      expect(screen.getByLabelText(/copied/i)).toBeInTheDocument();
    });
  });

  describe('Read-only Behavior', () => {
    it('renders agent ID as read-only', () => {
      render(<QuickCopySection agent={mockAgent} copiedField={null} onCopyToClipboard={mockOnCopyToClipboard} />);

      expect(screen.getByDisplayValue('agent-123')).toHaveAttribute('readonly');
    });
  });
});
