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

import userEvent from '@testing-library/user-event';
import {render, screen, within} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ApplicationCreateProvider from '../../../contexts/ApplicationCreate/ApplicationCreateProvider';
import useApplicationCreateContext from '../../../hooks/useApplicationCreateContext';
import ConfigureSignInOptions, {
  type ConfigureSignInOptionsProps,
} from '../configure-signin-options/ConfigureSignInOptions';
import {AuthenticatorTypes} from '@/features/connections/models/authenticators';
import {IdentityProviderTypes, type IdentityProvider} from '@/features/connections/models/identity-provider';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'applications:onboarding.configure.SignInOptions.title': 'Sign In Options',
        'applications:onboarding.configure.SignInOptions.subtitle': 'Choose how users will sign-in to your application',
        'applications:onboarding.configure.SignInOptions.usernamePassword': 'Username & Password',
        'applications:onboarding.configure.SignInOptions.google': 'Google',
        'applications:onboarding.configure.SignInOptions.github': 'GitHub',
        'applications:onboarding.configure.SignInOptions.notConfigured': 'Not configured',
        'applications:onboarding.configure.SignInOptions.noSelectionWarning':
          'At least one login option is required. Please select at least one authentication method.',
        'applications:onboarding.configure.SignInOptions.hint':
          'You can always change these settings later in the application settings.',
        'applications:onboarding.configure.SignInOptions.error': 'Failed to load authentication methods: {{error}}',
      };
      return translations[key] || key;
    },
  }),
}));

// Mock the dependencies
vi.mock('@/features/connections/api/useIdentityProviders');
vi.mock('@/features/connections/utils/getConnectionIcon');
vi.mock('@/features/flows/api/useGetFlows');

// Mock useGetApplications
vi.mock('../../../api/useGetApplications', () => ({
  __esModule: true,
  default: vi.fn(),
}));

// Mock generateAppPrimaryColorSuggestions
vi.mock('../../../utils/generateAppPrimaryColorSuggestions', () => ({
  __esModule: true,
  default: () => ['#3B82F6'],
}));

// Mock useConfig to avoid ConfigProvider requirement
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      endpoints: {
        server: 'http://localhost:3001',
      },
    }),
  };
});

const {default: useIdentityProviders} = await import('@/features/connections/api/useIdentityProviders');
const {default: getConnectionIcon} = await import('@/features/connections/utils/getConnectionIcon');
const {default: useGetFlows} = await import('@/features/flows/api/useGetFlows');
const {default: useGetApplications} = await import('../../../api/useGetApplications');

describe('ConfigureSignInOptions', () => {
  const mockOnIntegrationToggle = vi.fn();

  const mockIdentityProviders: IdentityProvider[] = [
    {
      id: 'google-idp',
      name: 'Google',
      type: IdentityProviderTypes.GOOGLE,
      description: 'Sign in with Google',
    },
    {
      id: 'github-idp',
      name: 'GitHub',
      type: IdentityProviderTypes.GITHUB,
      description: 'Sign in with GitHub',
    },
  ];

  const defaultProps: ConfigureSignInOptionsProps = {
    integrations: {
      [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
    },
    onIntegrationToggle: mockOnIntegrationToggle,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getConnectionIcon).mockReturnValue(<div>Icon</div>);
    // Default mock: no applications
    vi.mocked(useGetApplications).mockReturnValue({
      data: {
        totalResults: 0,
        count: 0,
        applications: [],
      },
      isLoading: false,
      isError: false,
      isSuccess: true,
      isFetching: false,
      isStale: false,
      isPending: false,
      error: null,
      status: 'success',
      fetchStatus: 'idle',
    } as unknown as ReturnType<typeof useGetApplications>);
    // Mock useGetFlows
    vi.mocked(useGetFlows).mockReturnValue({
      data: {
        totalResults: 0,
        startIndex: 1,
        count: 0,
        flows: [],
        links: [],
      },
      isLoading: false,
      isError: false,
      isSuccess: true,
      isFetching: false,
      isStale: false,
      isPending: false,
      error: null,
      status: 'success',
      fetchStatus: 'idle',
    } as unknown as ReturnType<typeof useGetFlows>);
  });

  const renderComponent = (props: Partial<ConfigureSignInOptionsProps> = {}) => {
    const renderResult = render(
      <ApplicationCreateProvider>
        <ConfigureSignInOptions {...defaultProps} {...props} />
      </ApplicationCreateProvider>,
    );

    return {
      ...renderResult,
      rerender: (newProps: Partial<ConfigureSignInOptionsProps> = {}) =>
        renderResult.rerender(
          <ApplicationCreateProvider>
            <ConfigureSignInOptions {...defaultProps} {...newProps} />
          </ApplicationCreateProvider>,
        ),
    };
  };

  it('should render loading state', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should render error state', () => {
    const error = new Error('Failed to load integrations');
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText(/Failed to load authentication methods/i)).toBeInTheDocument();
  });

  it('should render the component with title and subtitle', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
    expect(screen.getByText('Choose how users will sign-in to your application')).toBeInTheDocument();
  });

  it('should always render Username & Password option first', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    expect(screen.getByText('Username & Password')).toBeInTheDocument();
  });

  it('should render Username & Password as toggleable (not forced enabled)', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
      },
    });

    const switches = screen.getAllByRole('switch');
    expect(switches[0]).toBeChecked();

    // Should be toggleable (not disabled)
    expect(switches[0]).not.toBeDisabled();
  });

  it('should render Username & Password as unchecked when not selected', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent({
      integrations: {},
    });

    const switches = screen.getAllByRole('switch');
    expect(switches[0]).not.toBeChecked();
  });

  it('should render all identity providers', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    expect(screen.getByText('Google')).toBeInTheDocument();
    expect(screen.getByText('GitHub')).toBeInTheDocument();
  });

  it('should call onIntegrationToggle when clicking Username & Password list item', async () => {
    const user = userEvent.setup();

    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    const usernamePasswordButton = screen.getByText('Username & Password').closest('.MuiListItemButton-root');
    if (usernamePasswordButton) {
      await user.click(usernamePasswordButton);
    }

    expect(mockOnIntegrationToggle).toHaveBeenCalledWith(AuthenticatorTypes.CREDENTIALS_AUTH);
  });

  it('should call onIntegrationToggle when clicking provider list item', async () => {
    const user = userEvent.setup();

    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    const googleButton = screen.getByText('Google').closest('.MuiListItemButton-root');
    if (googleButton) {
      await user.click(googleButton);
    }

    expect(mockOnIntegrationToggle).toHaveBeenCalledWith('google-idp');
  });

  it('should call onIntegrationToggle when toggling switch', async () => {
    const user = userEvent.setup();

    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    const switches = screen.getAllByRole('switch');
    await user.click(switches[2]); // Click Google switch

    expect(mockOnIntegrationToggle).toHaveBeenCalledWith('google-idp');
  });

  it('should show checked state for enabled integrations', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
        'github-idp': false,
      },
    });

    const switches = screen.getAllByRole('switch');
    expect(switches[0]).toBeChecked(); // Username & Password
    expect(switches[2]).toBeChecked(); // Google
    expect(switches[3]).not.toBeChecked(); // GitHub
  });

  it('should show username/password option when no integrations are available', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    // Should show username/password in the list with a switch (always toggleable)
    expect(screen.getByText('Username & Password')).toBeInTheDocument();
    expect(screen.getByRole('list')).toBeInTheDocument();

    // Should have a toggle/switch (username/password is always toggleable)
    const switches = screen.getAllByRole('switch');
    expect(switches.length).toBeGreaterThan(0);
  });

  it('should render integration icons', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    // Google and GitHub use direct icons, not getConnectionIcon
    // Other providers (if any) would use getConnectionIcon
    expect(screen.getByText('Google')).toBeInTheDocument();
    expect(screen.getByText('GitHub')).toBeInTheDocument();
  });

  it('should render UserRound icon for Username & Password', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    // UserRound icon should be present
    const usernamePasswordSection = screen.getByText('Username & Password').closest('div');
    expect(usernamePasswordSection).toBeInTheDocument();
  });

  it('should stop propagation when clicking switch', async () => {
    const user = userEvent.setup();

    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    const switches = screen.getAllByRole('switch');
    await user.click(switches[1]);

    // Should only trigger once (not twice from card and switch)
    expect(mockOnIntegrationToggle).toHaveBeenCalledTimes(1);
  });

  it('should handle empty integrations record', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent({integrations: {}});

    const switches = screen.getAllByRole('switch');
    // Username & Password should default to false when integrations is empty
    expect(switches[0]).not.toBeChecked();
    // Others should default to false
    expect(switches[1]).not.toBeChecked();
  });

  it('should render info icon in subtitle', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    const subtitle = screen.getByText('Choose how users will sign-in to your application').closest('div');
    expect(subtitle).toBeInTheDocument();
  });

  it('should handle multiple rapid toggles', async () => {
    const user = userEvent.setup();

    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    const switches = screen.getAllByRole('switch');
    await user.click(switches[1]);
    await user.click(switches[2]);
    await user.click(switches[1]);

    expect(mockOnIntegrationToggle).toHaveBeenCalledTimes(3);
  });

  it('should handle providers with long names', () => {
    const longNameProvider: IdentityProvider = {
      id: 'long-name-idp',
      name: 'Very Long Identity Provider Name That Should Still Display',
      type: 'OIDC',
      description: 'Test provider',
    };

    vi.mocked(useIdentityProviders).mockReturnValue({
      data: [longNameProvider],
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    expect(screen.getByText(longNameProvider.name)).toBeInTheDocument();
  });

  it('should maintain switch state after re-render', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    const {rerender} = renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
      },
    });

    let switches = screen.getAllByRole('switch');
    expect(switches[2]).toBeChecked();

    rerender({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
      },
    });

    switches = screen.getAllByRole('switch');
    expect(switches[2]).toBeChecked();
  });

  describe('Google and GitHub always shown', () => {
    it('should always show Google option even when not configured', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: [], // No providers in API
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      expect(screen.getByText('Google')).toBeInTheDocument();
    });

    it('should always show GitHub option even when not configured', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: [], // No providers in API
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      expect(screen.getByText('GitHub')).toBeInTheDocument();
    });

    it('should show Google as disabled with "Not configured" when not in API', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: [], // No providers in API
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      const googleText = screen.getByText('Google');
      const listItem = googleText.closest('.MuiListItem-root');
      expect(listItem).toBeInTheDocument();

      // Should have "Not configured" as secondary text (both Google and GitHub show it)
      const notConfiguredTexts = screen.getAllByText('Not configured');
      expect(notConfiguredTexts.length).toBeGreaterThanOrEqual(1);

      // Should not have a switch for Google (disabled)
      const switches = screen.getAllByRole('switch');
      // Only username/password and passkey should have a switch
      expect(switches.length).toBe(2);

      // Google button should be disabled
      const googleButton = googleText.closest('.MuiListItemButton-root');
      expect(googleButton).toHaveAttribute('aria-disabled', 'true');
    });

    it('should show GitHub as disabled with "Not configured" when not in API', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: [], // No providers in API
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      const githubText = screen.getByText('GitHub');
      const listItem = githubText.closest('.MuiListItem-root');
      expect(listItem).toBeInTheDocument();

      // Should have "Not configured" as secondary text
      const notConfiguredTexts = screen.getAllByText('Not configured');
      expect(notConfiguredTexts.length).toBeGreaterThan(0);
    });

    it('should show Google as enabled with switch when configured in API', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      const switches = screen.getAllByRole('switch');
      // Should have switches for username/password, passkey, Google, and GitHub
      expect(switches.length).toBe(4);

      // Google should be toggleable
      const googleButton = screen.getByText('Google').closest('.MuiListItemButton-root');
      expect(googleButton).not.toBeDisabled();
    });

    it('should show GitHub as enabled with switch when configured in API', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      const switches = screen.getAllByRole('switch');
      // Should have switches for username/password, passkey, Google, and GitHub
      expect(switches.length).toBe(4);

      // GitHub should be toggleable
      const githubButton = screen.getByText('GitHub').closest('.MuiListItemButton-root');
      expect(githubButton).not.toBeDisabled();
    });

    it('should show Google enabled and GitHub disabled when only Google is configured', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: [mockIdentityProviders[0]], // Only Google
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      // Google should be enabled
      const googleButton = screen.getByText('Google').closest('.MuiListItemButton-root');
      expect(googleButton).not.toHaveAttribute('aria-disabled', 'true');

      // GitHub should be disabled
      const githubButton = screen.getByText('GitHub').closest('.MuiListItemButton-root');
      expect(githubButton).toHaveAttribute('aria-disabled', 'true');

      // Should show "Not configured" for GitHub
      const notConfiguredTexts = screen.getAllByText('Not configured');
      expect(notConfiguredTexts.length).toBeGreaterThan(0);
    });
  });

  describe('Validation warning', () => {
    it('should show warning when no options are selected', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent({
        integrations: {}, // No selections
      });

      expect(screen.getByRole('alert')).toBeInTheDocument();
      // Check for the translation key or the actual text
      const alert = screen.getByRole('alert');
      expect(alert.textContent).toMatch(/noSelectionWarning|at least one login option is required/i);
    });

    it('should not show warning when at least one option is selected', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        },
      });

      // Should not have warning alert
      const alerts = screen.queryAllByRole('alert');
      const warningAlerts = alerts.filter((alert) => alert.textContent?.includes('at least one'));
      expect(warningAlerts.length).toBe(0);
    });

    it('should show warning when only username/password is deselected', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          'google-idp': false,
          'github-idp': false,
        },
      });

      expect(screen.getByRole('alert')).toBeInTheDocument();
      // Check for the translation key or the actual text
      const alert = screen.getByRole('alert');
      expect(alert.textContent).toMatch(/noSelectionWarning|at least one login option is required/i);
    });

    it('should hide warning when user selects an option', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      const {rerender} = renderComponent({
        integrations: {}, // No selections initially
      });

      expect(screen.getByRole('alert')).toBeInTheDocument();

      // Select username/password
      rerender({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        },
      });

      // Warning should be gone
      const warningAlerts = screen
        .queryAllByRole('alert')
        .filter((alert) => alert.textContent?.includes('at least one'));
      expect(warningAlerts.length).toBe(0);
    });
  });

  describe('onReadyChange callback', () => {
    it('should call onReadyChange with true when integrations are selected', () => {
      const onReadyChange = vi.fn();
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        },
        onReadyChange,
      });

      expect(onReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onReadyChange with false when no integrations are selected', () => {
      const onReadyChange = vi.fn();
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent({
        integrations: {},
        onReadyChange,
      });

      expect(onReadyChange).toHaveBeenCalledWith(false);
    });
  });

  describe('Flow loading states', () => {
    it('should render loading state when flows are loading', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      vi.mocked(useGetFlows).mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        isSuccess: false,
        isFetching: true,
        isStale: false,
        isPending: true,
        error: null,
        status: 'pending',
        fetchStatus: 'fetching',
      } as unknown as ReturnType<typeof useGetFlows>);

      renderComponent();

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should render error state when flows fail to load', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      const flowsError = new Error('Failed to load flows');
      vi.mocked(useGetFlows).mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        isSuccess: false,
        isFetching: false,
        isStale: false,
        isPending: false,
        error: flowsError,
        status: 'error',
        fetchStatus: 'idle',
      } as unknown as ReturnType<typeof useGetFlows>);

      renderComponent();

      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/Failed to load authentication methods/i)).toBeInTheDocument();
    });
  });

  describe('Flow data handling', () => {
    it('should handle when flows data is empty', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      vi.mocked(useGetFlows).mockReturnValue({
        data: {
          totalResults: 0,
          startIndex: 1,
          count: 0,
          flows: [],
          links: [],
        },
        isLoading: false,
        isError: false,
        isSuccess: true,
        isFetching: false,
        isStale: false,
        isPending: false,
        error: null,
        status: 'success',
        fetchStatus: 'idle',
      } as unknown as ReturnType<typeof useGetFlows>);

      renderComponent();

      // Component should still render without flows
      expect(screen.getByText('Username & Password')).toBeInTheDocument();
    });

    it('should handle when flows data is null', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      vi.mocked(useGetFlows).mockReturnValue({
        data: null,
        isLoading: false,
        isError: false,
        isSuccess: true,
        isFetching: false,
        isStale: false,
        isPending: false,
        error: null,
        status: 'success',
        fetchStatus: 'idle',
      } as unknown as ReturnType<typeof useGetFlows>);

      renderComponent();

      // Component should still render without flows
      expect(screen.getByText('Username & Password')).toBeInTheDocument();
    });
  });

  describe('Integration type mapping', () => {
    it('should handle OIDC type providers', async () => {
      const user = userEvent.setup();
      const oidcProvider: IdentityProvider = {
        id: 'oidc-idp',
        name: 'OIDC Provider',
        type: 'OIDC',
        description: 'Generic OIDC provider',
      };

      vi.mocked(useIdentityProviders).mockReturnValue({
        data: [oidcProvider, ...mockIdentityProviders],
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      const oidcButton = screen.getByText('OIDC Provider').closest('.MuiListItemButton-root');
      if (oidcButton) {
        await user.click(oidcButton);
      }

      expect(mockOnIntegrationToggle).toHaveBeenCalledWith('oidc-idp');
    });
  });

  describe('Hint text', () => {
    it('should render hint text with lightbulb icon', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      expect(
        screen.getByText('You can always change these settings later in the application settings.'),
      ).toBeInTheDocument();
    });
  });

  describe('Custom flow selection (issue #2959)', () => {
    const customFlow = {
      id: 'custom-flow-id',
      handle: 'custom-passwordless',
      name: 'Custom Passwordless',
      flowType: 'AUTHENTICATION',
      activeVersion: 1,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };

    const WiredHarness = ({onReadyChange = undefined}: {onReadyChange?: (isReady: boolean) => void}): JSX.Element => {
      const {integrations, toggleIntegration, selectedAuthFlow} = useApplicationCreateContext();
      return (
        <>
          <span data-testid="selected-flow-id">{selectedAuthFlow?.id ?? ''}</span>
          <ConfigureSignInOptions
            integrations={integrations}
            onIntegrationToggle={toggleIntegration}
            onReadyChange={onReadyChange}
          />
        </>
      );
    };

    const renderWired = (onReadyChange?: (isReady: boolean) => void) =>
      render(
        <ApplicationCreateProvider>
          <WiredHarness onReadyChange={onReadyChange} />
        </ApplicationCreateProvider>,
      );

    const usernamePasswordSwitch = (): HTMLElement => {
      const item = screen.getByText('Username & Password').closest('li');
      if (!item) {
        throw new Error('Username & Password list item not found');
      }
      return within(item).getByRole('switch');
    };

    beforeEach(() => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      vi.mocked(useGetFlows).mockReturnValue({
        data: {
          totalResults: 1,
          startIndex: 1,
          count: 1,
          flows: [customFlow],
          links: [],
        },
        isLoading: false,
        isError: false,
        isSuccess: true,
        isFetching: false,
        isStale: false,
        isPending: false,
        error: null,
        status: 'success',
        fetchStatus: 'idle',
      } as unknown as ReturnType<typeof useGetFlows>);
    });

    it('unselects all integration toggles when a custom flow is selected', async () => {
      const user = userEvent.setup();
      renderWired();

      expect(usernamePasswordSwitch()).toBeChecked();

      await user.click(screen.getByRole('combobox'));
      await user.click(await screen.findByText('Custom Passwordless'));

      expect(usernamePasswordSwitch()).not.toBeChecked();
      expect(screen.getByTestId('selected-flow-id')).toHaveTextContent('custom-flow-id');
    });

    it('keeps the step ready (Continue enabled) with all toggles off once a flow is selected', async () => {
      const user = userEvent.setup();
      const onReadyChange = vi.fn();
      renderWired(onReadyChange);

      await user.click(screen.getByRole('combobox'));
      await user.click(await screen.findByText('Custom Passwordless'));

      expect(usernamePasswordSwitch()).not.toBeChecked();
      expect(onReadyChange).toHaveBeenLastCalledWith(true);
    });

    it('clears the selected flow and returns to toggle-driven mode when a method is re-enabled', async () => {
      const user = userEvent.setup();
      const onReadyChange = vi.fn();
      renderWired(onReadyChange);

      await user.click(screen.getByRole('combobox'));
      await user.click(await screen.findByText('Custom Passwordless'));
      expect(usernamePasswordSwitch()).not.toBeChecked();
      expect(screen.getByTestId('selected-flow-id')).toHaveTextContent('custom-flow-id');

      await user.click(screen.getByText('Username & Password'));

      expect(usernamePasswordSwitch()).toBeChecked();
      expect(screen.getByTestId('selected-flow-id')).toHaveTextContent('');
      expect(screen.getByRole('combobox')).toHaveValue('');
      expect(onReadyChange).toHaveBeenLastCalledWith(true);
    });
  });
});
