// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, renderWithProviders, screen, waitFor} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import ManageChildOrganizationUnitSection from '../ManageChildOrganizationUnitSection';

vi.mock('@/components/OrganizationUnitTreePicker', () => ({
  default: ({
    rootOuId,
    hideRoot,
    onChange,
    onItemActivate,
  }: {
    rootOuId?: string;
    hideRoot?: boolean;
    onChange?: (organizationUnitId: string) => void;
    onItemActivate?: (organizationUnitId: string) => void;
  }) => (
    <div data-root-ou-id={rootOuId} data-hide-root={String(hideRoot)}>
      <button type="button" data-testid="organization-unit-subtree" onClick={() => onItemActivate?.('nested-ou')}>
        Organization unit subtree
      </button>
      <button type="button" data-testid="organization-unit-selection" onClick={() => onChange?.('nested-ou')}>
        Select organization unit
      </button>
    </div>
  ),
}));

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockLoggerError = vi.fn();
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: mockLoggerError}),
}));

describe('ManageChildOrganizationUnitSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
  });

  it('should render the child organization unit hierarchy', () => {
    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="parent-ou" organizationUnitName="Engineering" />,
    );

    expect(screen.getByText('Child Organization Units')).toBeInTheDocument();
    expect(screen.getByTestId('organization-unit-subtree').parentElement).toHaveAttribute(
      'data-root-ou-id',
      'parent-ou',
    );
    expect(screen.getByTestId('organization-unit-subtree').parentElement).toHaveAttribute('data-hide-root', 'true');
  });

  it('should ignore tree selection events', () => {
    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="parent-ou" organizationUnitName="Engineering" />,
    );

    fireEvent.click(screen.getByTestId('organization-unit-selection'));

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('should navigate to a selected organization unit with back-navigation state', async () => {
    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="parent-ou" organizationUnitName="Engineering" />,
    );

    fireEvent.click(screen.getByTestId('organization-unit-subtree'));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/organization-units/nested-ou', {
        state: {fromOU: {id: 'parent-ou', name: 'Engineering'}},
      });
    });
  });

  it('should log navigation failures', async () => {
    mockNavigate.mockRejectedValue(new Error('Navigation failed'));

    renderWithProviders(
      <ManageChildOrganizationUnitSection organizationUnitId="parent-ou" organizationUnitName="Engineering" />,
    );

    fireEvent.click(screen.getByTestId('organization-unit-subtree'));

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledTimes(1);
      const [message, details] = mockLoggerError.mock.calls[0] as [string, {error: unknown; ouId: string}];
      expect(message).toBe('Failed to navigate to child organization unit');
      expect(details.error).toBeInstanceOf(Error);
      expect(details.ouId).toBe('nested-ou');
    });
  });
});
