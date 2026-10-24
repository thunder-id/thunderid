// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import EditAdvancedSettings from '../edit-group/advanced-settings/EditAdvancedSettings';

describe('EditAdvancedSettings', () => {
  const defaultProps = {
    onDeleteClick: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render danger zone section', () => {
    renderWithProviders(<EditAdvancedSettings {...defaultProps} />);

    expect(screen.getByText('Danger zone')).toBeInTheDocument();
    expect(screen.getByText('Delete this group')).toBeInTheDocument();
  });

  it('should call onDeleteClick when delete button is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<EditAdvancedSettings {...defaultProps} />);

    await user.click(screen.getByText('Delete'));

    expect(defaultProps.onDeleteClick).toHaveBeenCalled();
  });

  it('should not render danger zone section when onDeleteClick is not provided', () => {
    renderWithProviders(<EditAdvancedSettings />);

    expect(screen.queryByText('Danger zone')).not.toBeInTheDocument();
  });
});
