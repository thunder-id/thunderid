// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor, fireEvent} from '@testing-library/react';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {Group} from '../../models/group';
import EditGeneralSettings from '../edit-group/general-settings/EditGeneralSettings';

describe('EditGeneralSettings', () => {
  const mockGroup: Group = {
    id: 'g1',
    name: 'Test Group',
    description: 'Test desc',
    ouId: 'ou-123',
  };

  let mockWriteText: ReturnType<typeof vi.fn>;
  const originalClipboard = navigator.clipboard;

  const defaultProps = {
    group: mockGroup,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockWriteText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: {writeText: mockWriteText},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(navigator, 'clipboard', {
      value: originalClipboard,
      writable: true,
      configurable: true,
    });
  });

  it('should render organization unit section', () => {
    renderWithProviders(<EditGeneralSettings {...defaultProps} />);

    expect(screen.getAllByText('Organization unit').length).toBeGreaterThan(0);
    expect(screen.getByDisplayValue('ou-123')).toBeInTheDocument();
  });

  it('should have read-only organization unit field', () => {
    renderWithProviders(<EditGeneralSettings {...defaultProps} />);

    const ouInput = screen.getByDisplayValue('ou-123');
    expect(ouInput).toHaveAttribute('readonly');
  });

  it('should render copy button for organization unit ID', () => {
    renderWithProviders(<EditGeneralSettings {...defaultProps} />);

    expect(screen.getByLabelText('Copy organization unit ID')).toBeInTheDocument();
  });

  it('should copy organization unit ID to clipboard when copy button is clicked', async () => {
    renderWithProviders(<EditGeneralSettings {...defaultProps} />);

    const copyButton = screen.getByLabelText('Copy organization unit ID');
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalledWith('ou-123');
    });
  });
});
