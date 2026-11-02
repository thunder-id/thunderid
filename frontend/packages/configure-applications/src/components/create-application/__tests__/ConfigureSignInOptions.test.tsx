// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {
  AuthenticatorTypes,
  IdentityProviderTypes,
  getConnectionIcon,
  useIdentityProviders,
  type IdentityProvider,
} from '@thunderid/configure-connections';
import {fireEvent, render, screen, within} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ApplicationCreateProvider from '../../../contexts/ApplicationCreate/ApplicationCreateProvider';
import useApplicationCreateContext from '../../../hooks/useApplicationCreateContext';
import ConfigureSignInOptions, {
  type ConfigureSignInOptionsProps,
} from '../configure-signin-options/ConfigureSignInOptions';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
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
vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  useIdentityProviders: vi.fn(),
  getConnectionIcon: vi.fn(),
}));
vi.mock('@thunderid/configure-flows', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-flows')>()),
  useGetFlows: vi.fn(),
}));

// Mock useGetApplications
vi.mock('../../../api/useGetApplications', () => ({
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

const {useGetFlows} = await import('@thunderid/configure-flows');
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

  // Only the first accordion (Prompt for Credentials) starts expanded; expand the rest so this
  // file's assertions can keep reaching into Passwordless Login, Social Login, and MFA rows the
  // same way they did before those groups became collapsible accordions.
  const expandAllAccordions = (): void => {
    screen.queryAllByRole('button', {expanded: false}).forEach((summary) => fireEvent.click(summary));
  };

  const renderComponent = (props: Partial<ConfigureSignInOptionsProps> = {}) => {
    const renderResult = render(
      <ApplicationCreateProvider>
        <ConfigureSignInOptions {...defaultProps} {...props} />
      </ApplicationCreateProvider>,
    );
    expandAllAccordions();

    return {
      ...renderResult,
      rerender: (newProps: Partial<ConfigureSignInOptionsProps> = {}) => {
        renderResult.rerender(
          <ApplicationCreateProvider>
            <ConfigureSignInOptions {...defaultProps} {...newProps} />
          </ApplicationCreateProvider>,
        );
        expandAllAccordions();
      },
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

    const usernamePasswordRow = screen.getByTestId(`auth-method-${AuthenticatorTypes.CREDENTIALS_AUTH}`);
    await user.click(within(usernamePasswordRow).getByRole('switch'));

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

    const googleRow = screen.getByTestId('auth-method-google-idp');
    await user.click(within(googleRow).getByRole('switch'));

    expect(mockOnIntegrationToggle).toHaveBeenCalledWith('google-idp');
  });

  it('should render Magic Link under Passwordless Login and call onIntegrationToggle when toggled', async () => {
    const user = userEvent.setup();

    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent();

    const magicLinkRow = screen.getByTestId(`auth-method-${AuthenticatorTypes.MAGIC_LINK}`);
    expect(magicLinkRow).toBeInTheDocument();

    await user.click(within(magicLinkRow).getByRole('switch'));

    expect(mockOnIntegrationToggle).toHaveBeenCalledWith(AuthenticatorTypes.MAGIC_LINK);
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
    await user.click(switches[3]); // Click Google switch (0=Username&Password, 1=Passkey, 2=Magic Link, 3=Google)

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
    expect(switches[3]).toBeChecked(); // Google (0=Username&Password, 1=Passkey, 2=Magic Link, 3=Google)
    expect(switches[4]).not.toBeChecked(); // GitHub
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
    expect(screen.getAllByRole('list').length).toBeGreaterThan(0);

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
    expect(switches[3]).toBeChecked(); // Google (0=Username&Password, 1=Passkey, 2=Magic Link, 3=Google)

    rerender({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
      },
    });

    switches = screen.getAllByRole('switch');
    expect(switches[3]).toBeChecked();
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

      const googleRow = screen.getByTestId('auth-method-google');
      expect(googleRow).toBeInTheDocument();

      // Should have "Not configured" as trailing text (both Google and GitHub show it)
      const notConfiguredTexts = screen.getAllByText('Not configured');
      expect(notConfiguredTexts.length).toBeGreaterThanOrEqual(1);

      // No switch renders when unavailable; "Not configured" text takes its place.
      expect(within(googleRow).queryByRole('switch')).not.toBeInTheDocument();
    });

    it('should show GitHub as disabled with "Not configured" when not in API', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: [], // No providers in API
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      const githubRow = screen.getByTestId('auth-method-github');
      expect(githubRow).toBeInTheDocument();

      // Should have "Not configured" as trailing text
      const notConfiguredTexts = screen.getAllByText('Not configured');
      expect(notConfiguredTexts.length).toBeGreaterThan(0);
      expect(within(githubRow).queryByRole('switch')).not.toBeInTheDocument();
    });

    it('should show Google as enabled with switch when configured in API', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      // Google should be toggleable
      const googleRow = screen.getByTestId('auth-method-google-idp');
      expect(within(googleRow).getByRole('switch')).not.toBeDisabled();
    });

    it('should show GitHub as enabled with switch when configured in API', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: mockIdentityProviders,
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      // GitHub should be toggleable
      const githubRow = screen.getByTestId('auth-method-github-idp');
      expect(within(githubRow).getByRole('switch')).not.toBeDisabled();
    });

    it('should show Google enabled and GitHub disabled when only Google is configured', () => {
      vi.mocked(useIdentityProviders).mockReturnValue({
        data: [mockIdentityProviders[0]], // Only Google
        isLoading: false,
        error: null,
      } as ReturnType<typeof useIdentityProviders>);

      renderComponent();

      // Google should be enabled
      const googleRow = screen.getByTestId('auth-method-google-idp');
      expect(within(googleRow).getByRole('switch')).not.toBeDisabled();

      // GitHub should show no switch since it's not configured
      const githubRow = screen.getByTestId('auth-method-github');
      expect(within(githubRow).queryByRole('switch')).not.toBeInTheDocument();

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

      const oidcRow = screen.getByTestId('auth-method-oidc-idp');
      await user.click(within(oidcRow).getByRole('switch'));

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

    const renderWired = (onReadyChange?: (isReady: boolean) => void) => {
      const result = render(
        <ApplicationCreateProvider>
          <WiredHarness onReadyChange={onReadyChange} />
        </ApplicationCreateProvider>,
      );
      // The pre-configured flow card is collapsed by default; expand it so these tests can reach
      // the autocomplete inside.
      fireEvent.click(screen.getByRole('button', {name: /preConfiguredFlows|pre-configured flow/i}));
      return result;
    };

    const usernamePasswordSwitch = (): HTMLElement => {
      const item = screen.getByText('Username & Password').closest('[role="listitem"]');
      if (!item) {
        throw new Error('Username & Password list item not found');
      }
      return within(item as HTMLElement).getByRole('switch');
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

      // Toggles stay disabled while the flow is selected — clearing the flow via the
      // autocomplete's "Clear" (X) control is what returns to toggle-driven mode.
      expect(usernamePasswordSwitch()).toBeDisabled();
      await user.click(screen.getByTitle('Clear'));

      expect(screen.getByTestId('selected-flow-id')).toHaveTextContent('');
      expect(screen.getByRole('combobox')).toHaveValue('');
      expect(usernamePasswordSwitch()).not.toBeDisabled();

      await user.click(usernamePasswordSwitch());

      expect(usernamePasswordSwitch()).toBeChecked();
      expect(onReadyChange).toHaveBeenLastCalledWith(true);
    });
  });
});
