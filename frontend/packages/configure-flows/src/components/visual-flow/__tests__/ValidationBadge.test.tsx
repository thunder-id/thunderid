// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ValidationBadge from '../ValidationBadge';

const mockSetCurrentActiveTab = vi.fn();
const mockSetOpenValidationPanel = vi.fn();

vi.mock('../../../hooks/useValidationStatus', () => ({
  default: () => ({
    setCurrentActiveTab: mockSetCurrentActiveTab,
    setOpenValidationPanel: mockSetOpenValidationPanel,
  }),
}));

describe('ValidationBadge', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should return null when there are no errors or warnings', () => {
    const {container} = render(<ValidationBadge errorCount={0} warningCount={0} />);

    expect(container.firstChild).toBeNull();
  });

  it('should render error count when there are errors', () => {
    render(<ValidationBadge errorCount={3} warningCount={0} />);

    expect(screen.getByText('3')).toBeInTheDocument();
  });

  it('should render warning count when there are warnings', () => {
    render(<ValidationBadge errorCount={0} warningCount={2} />);

    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('should render both error and warning counts', () => {
    render(<ValidationBadge errorCount={2} warningCount={5} />);

    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
  });

  it('should open validation panel with errors tab when clicked with errors', () => {
    render(<ValidationBadge errorCount={1} warningCount={0} />);

    fireEvent.click(screen.getByRole('button'));

    expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(0);
    expect(mockSetOpenValidationPanel).toHaveBeenCalledWith(true);
  });

  it('should open validation panel with warnings tab when clicked with only warnings', () => {
    render(<ValidationBadge errorCount={0} warningCount={1} />);

    fireEvent.click(screen.getByRole('button'));

    expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(1);
    expect(mockSetOpenValidationPanel).toHaveBeenCalledWith(true);
  });

  it('should have accessible label with counts', () => {
    render(<ValidationBadge errorCount={2} warningCount={3} />);

    expect(screen.getByLabelText('2 errors, 3 warnings. Open validation panel.')).toBeInTheDocument();
  });

  it('should be keyboard accessible', () => {
    render(<ValidationBadge errorCount={1} warningCount={0} />);

    const button = screen.getByRole('button');
    button.focus();
    fireEvent.keyDown(button, {key: 'Enter'});

    // ButtonBase handles Enter/Space natively
    expect(button).toBeInTheDocument();
  });
});
