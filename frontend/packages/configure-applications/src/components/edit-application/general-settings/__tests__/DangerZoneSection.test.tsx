// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, fireEvent, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import DangerZoneSection from '../DangerZoneSection';

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'applications:edit.general.sections.dangerZone.title': 'Danger Zone',
        'applications:edit.general.sections.dangerZone.description':
          'Actions in this section are irreversible. Proceed with caution.',
        'applications:edit.general.sections.dangerZone.deleteApplication.title': 'Delete Application',
        'applications:edit.general.sections.dangerZone.deleteApplication.description':
          'Permanently delete this application and all associated data. This action cannot be undone.',
        'applications:edit.general.sections.dangerZone.deleteApplication.button': 'Delete Application',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('DangerZoneSection', () => {
  const mockOnDeleteClick = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render the danger zone section', () => {
    renderWithProviders(<DangerZoneSection onDeleteClick={mockOnDeleteClick} />);

    expect(screen.getByText('Danger Zone')).toBeInTheDocument();
    expect(screen.getByText('Actions in this section are irreversible. Proceed with caution.')).toBeInTheDocument();
  });

  it('should always render delete application section', () => {
    renderWithProviders(<DangerZoneSection onDeleteClick={mockOnDeleteClick} />);

    expect(screen.getByRole('heading', {name: 'Delete Application', level: 6})).toBeInTheDocument();
    expect(
      screen.getByText('Permanently delete this application and all associated data. This action cannot be undone.'),
    ).toBeInTheDocument();
  });

  it('should render delete button', () => {
    renderWithProviders(<DangerZoneSection onDeleteClick={mockOnDeleteClick} />);

    const deleteButton = screen.getByRole('button', {name: 'Delete Application'});
    expect(deleteButton).toBeInTheDocument();
  });

  it('should call onDeleteClick when delete button is clicked', () => {
    renderWithProviders(<DangerZoneSection onDeleteClick={mockOnDeleteClick} />);

    const deleteButton = screen.getByRole('button', {name: 'Delete Application'});
    fireEvent.click(deleteButton);

    expect(mockOnDeleteClick).toHaveBeenCalledTimes(1);
  });

  it('should render delete button with error color', () => {
    renderWithProviders(<DangerZoneSection onDeleteClick={mockOnDeleteClick} />);

    const deleteButton = screen.getByRole('button', {name: 'Delete Application'});
    expect(deleteButton).toHaveClass('MuiButton-colorError');
  });
});
