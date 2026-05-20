/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {screen, fireEvent, waitFor, renderWithProviders, act} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OrganizationUnit} from '../../../../models/organization-unit';
import EditGeneralSettings from '../EditGeneralSettings';

// Mock child components
vi.mock('@/components/edit-organization-unit/general-settings/QuickCopySection', () => ({
  default: ({
    organizationUnit,
    copiedField,
    onCopyToClipboard,
  }: {
    organizationUnit: OrganizationUnit;
    copiedField: string | null;
    onCopyToClipboard: (text: string, field: string) => void;
  }) => (
    <div data-testid="quick-copy-section">
      QuickCopySection - {organizationUnit.handle}
      <button type="button" onClick={() => onCopyToClipboard('test', 'handle')}>
        Copy Handle
      </button>
      {copiedField && <span>Copied: {copiedField}</span>}
    </div>
  ),
}));

vi.mock('@/components/edit-organization-unit/general-settings/ParentSettingsSection', () => ({
  default: ({organizationUnit}: {organizationUnit: OrganizationUnit}) => (
    <div data-testid="parent-settings-section">ParentSettingsSection - {organizationUnit.name}</div>
  ),
}));

vi.mock('@/components/edit-organization-unit/general-settings/DangerZoneSection', () => ({
  default: ({onDeleteClick}: {onDeleteClick: () => void}) => (
    <div data-testid="danger-zone-section">
      DangerZoneSection
      <button type="button" onClick={onDeleteClick}>
        Delete
      </button>
    </div>
  ),
}));

describe('EditGeneralSettings', () => {
  const mockOrganizationUnit: OrganizationUnit = {
    id: 'ou-123',
    handle: 'engineering',
    name: 'Engineering',
    description: 'Engineering department',
    parent: null,
  };

  const mockOnDeleteClick = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render all three sections', () => {
    renderWithProviders(
      <EditGeneralSettings organizationUnit={mockOrganizationUnit} onDeleteClick={mockOnDeleteClick} />,
    );

    expect(screen.getByTestId('quick-copy-section')).toBeInTheDocument();
    expect(screen.getByTestId('parent-settings-section')).toBeInTheDocument();
    expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
  });

  it('should pass organizationUnit to QuickCopySection', () => {
    renderWithProviders(
      <EditGeneralSettings organizationUnit={mockOrganizationUnit} onDeleteClick={mockOnDeleteClick} />,
    );

    expect(screen.getByText(/QuickCopySection - engineering/)).toBeInTheDocument();
  });

  it('should pass organizationUnit to ParentSettingsSection', () => {
    renderWithProviders(
      <EditGeneralSettings organizationUnit={mockOrganizationUnit} onDeleteClick={mockOnDeleteClick} />,
    );

    expect(screen.getByText(/ParentSettingsSection - Engineering/)).toBeInTheDocument();
  });

  it('should pass onDeleteClick to DangerZoneSection', () => {
    renderWithProviders(
      <EditGeneralSettings organizationUnit={mockOrganizationUnit} onDeleteClick={mockOnDeleteClick} />,
    );

    const deleteButton = screen.getByText('Delete');
    fireEvent.click(deleteButton);

    expect(mockOnDeleteClick).toHaveBeenCalledTimes(1);
  });

  it('should handle clipboard copy and show copied state', async () => {
    // Mock clipboard API
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: {
        writeText: writeTextMock,
      },
    });

    renderWithProviders(
      <EditGeneralSettings organizationUnit={mockOrganizationUnit} onDeleteClick={mockOnDeleteClick} />,
    );

    const copyButton = screen.getByText('Copy Handle');
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(writeTextMock).toHaveBeenCalledWith('test');
    });

    expect(screen.getByText('Copied: handle')).toBeInTheDocument();
  });

  it('should clear copied state after 2 seconds', async () => {
    vi.useRealTimers();
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout');
    // Mock clipboard API
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    });

    renderWithProviders(
      <EditGeneralSettings organizationUnit={mockOrganizationUnit} onDeleteClick={mockOnDeleteClick} />,
    );

    const copyButton = screen.getByText('Copy Handle');
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(screen.getByText('Copied: handle')).toBeInTheDocument();
    });

    expect(setTimeoutSpy).toHaveBeenCalledWith(expect.any(Function), 2000);

    // Manually trigger the timeout callback
    const timeoutCallback = setTimeoutSpy.mock.calls.find((call) => call[1] === 2000)?.[0] as (this: void) => void;
    if (typeof timeoutCallback === 'function') {
      act(() => {
        timeoutCallback();
      });
    }

    await waitFor(() => {
      expect(screen.queryByText('Copied: handle')).not.toBeInTheDocument();
    });
    setTimeoutSpy.mockRestore();
  });

  it('should clear previous timeout when copying again', async () => {
    vi.useRealTimers();
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout');
    const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout');

    // Mock clipboard API
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    });

    renderWithProviders(
      <EditGeneralSettings organizationUnit={mockOrganizationUnit} onDeleteClick={mockOnDeleteClick} />,
    );

    const copyButton = screen.getByText('Copy Handle');

    // First copy
    fireEvent.click(copyButton);
    await waitFor(() => {
      expect(screen.getByText('Copied: handle')).toBeInTheDocument();
    });

    // Capture the first timeout ID (mocked returns are usually numbers in jsdom)
    // But we just need to verify clearTimeout was called

    // Second copy (should reset the timer)
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(clearTimeoutSpy).toHaveBeenCalled();
    });

    // Should still set a new timeout
    expect(setTimeoutSpy).toHaveBeenCalledWith(expect.any(Function), 2000);

    // Ensure the state is still copied
    expect(screen.getByText('Copied: handle')).toBeInTheDocument();

    setTimeoutSpy.mockRestore();
    clearTimeoutSpy.mockRestore();
  });

  it('should cleanup timeout on unmount', () => {
    const {unmount} = renderWithProviders(
      <EditGeneralSettings organizationUnit={mockOrganizationUnit} onDeleteClick={mockOnDeleteClick} />,
    );

    // Should not throw on unmount
    expect(() => unmount()).not.toThrow();
  });
});
