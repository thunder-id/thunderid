// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, fireEvent, waitFor, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OrganizationUnit} from '../../models/organization-unit';
import type {OrganizationUnitListResponse} from '../../models/responses';
import OrganizationUnitPickerScreen from '../OrganizationUnitPickerScreen';

// Mock logger — stable reference to avoid useCallback churn
const stableLogger = {error: vi.fn(), info: vi.fn(), debug: vi.fn()};
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => stableLogger,
}));

const mockUseGetOrganizationUnits = vi.fn();
vi.mock('@/api/useGetOrganizationUnits', () => ({
  default: () =>
    mockUseGetOrganizationUnits() as {
      data: OrganizationUnitListResponse | undefined;
      isLoading: boolean;
      error: Error | null;
    },
}));

const mockUseGetOrganizationUnit = vi.fn();
vi.mock('@/api/useGetOrganizationUnit', () => ({
  default: () =>
    mockUseGetOrganizationUnit() as {
      data: OrganizationUnit | undefined;
      isLoading: boolean;
      error: Error | null;
    },
}));

const mockUseGetChildOrganizationUnits = vi.fn();
vi.mock('@/api/useGetChildOrganizationUnits', () => ({
  default: () =>
    mockUseGetChildOrganizationUnits() as {
      data: OrganizationUnitListResponse | undefined;
      isLoading: boolean;
      error: Error | null;
    },
}));

// Mock ThunderID — stable reference to avoid useCallback churn
const mockHttpRequest = vi.fn();
const stableHttp = {request: mockHttpRequest};
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...(actual as object),
    useThunderID: () => ({http: stableHttp}),
  };
});

// Mock config — stable reference to avoid useCallback churn
const stableConfig = {getServerUrl: () => 'http://localhost:8080'};
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => stableConfig,
  };
});

describe('OrganizationUnitPickerScreen', () => {
  const mockOUData: OrganizationUnitListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    organizationUnits: [
      {id: 'ou-1', handle: 'default', name: 'Default', parent: null},
      {id: 'ou-2', handle: 'wild-mails-smile', name: 'Wild Mails Smile', parent: null},
    ],
  };

  const defaultProps = {
    icon: <span>icon</span>,
    title: 'Where should this application belong?',
    subtitle: "Choose the organization unit that will own this application. You can't change this once created.",
    value: '',
    onChange: vi.fn(),
    onBack: vi.fn(),
    onContinue: vi.fn(),
    backLabel: 'Back',
    continueLabel: 'Continue',
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetOrganizationUnits.mockReturnValue({data: mockOUData, isLoading: false, error: null});
    mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false, error: null});
    mockUseGetChildOrganizationUnits.mockReturnValue({data: undefined, isLoading: false, error: null});
  });

  it('renders the title and subtitle', () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} />);

    expect(screen.getByText('Where should this application belong?')).toBeInTheDocument();
    expect(
      screen.getByText(
        "Choose the organization unit that will own this application. You can't change this once created.",
      ),
    ).toBeInTheDocument();
  });

  it('renders the organization unit tree, including nested organization units', async () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Default')).toBeInTheDocument();
      expect(screen.getByText('Wild Mails Smile')).toBeInTheDocument();
    });
  });

  it('calls onChange with the clicked organization unit id', async () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText('Default')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Default'));

    await waitFor(() => {
      expect(defaultProps.onChange).toHaveBeenCalledWith('ou-1');
    });
  });

  it('auto-selects the first organization unit when nothing is selected yet', async () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} />);

    await waitFor(() => {
      expect(defaultProps.onChange).toHaveBeenCalledWith('ou-1');
    });
  });

  it('should leave the selection empty when auto-selecting is turned off', async () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} autoSelectFirst={false} />);

    await waitFor(() => {
      expect(screen.getByText('Default')).toBeInTheDocument();
    });
    expect(defaultProps.onChange).not.toHaveBeenCalled();
    expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();
  });

  it('disables Continue until an organization unit is selected', () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} />);

    expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();
  });

  it('enables Continue and calls onContinue once a value is selected', () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} value="ou-1" />);

    const continueButton = screen.getByRole('button', {name: 'Continue'});
    expect(continueButton).toBeEnabled();

    fireEvent.click(continueButton);

    expect(defaultProps.onContinue).toHaveBeenCalled();
  });

  it('should show why the selected organization unit cannot be used and block Continue', () => {
    renderWithProviders(
      <OrganizationUnitPickerScreen {...defaultProps} value="ou-1" error="This organization unit is taken." />,
    );

    expect(screen.getByRole('alert')).toHaveTextContent('This organization unit is taken.');
    expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();
  });

  it('should show the action given with the error', () => {
    const onOpen = vi.fn();
    renderWithProviders(
      <OrganizationUnitPickerScreen
        {...defaultProps}
        value="ou-1"
        error="This organization unit is taken."
        errorAction={
          <button type="button" onClick={onOpen}>
            Open it
          </button>
        }
      />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Open it'}));

    expect(onOpen).toHaveBeenCalled();
  });

  it('should show no error by default', () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} value="ou-1" />);

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('calls onBack when the back link is clicked', () => {
    renderWithProviders(<OrganizationUnitPickerScreen {...defaultProps} />);

    fireEvent.click(screen.getByText('Back'));

    expect(defaultProps.onBack).toHaveBeenCalled();
  });
});
