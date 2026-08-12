// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {TokenEndpointAuthMethods} from '@thunderid/configure-applications';
import {AuthenticatorTypes} from '@thunderid/configure-connections';
import {AllowedOriginTypes, createRow} from '@thunderid/configure-settings';
import {LoggerProvider, LogLevel} from '@thunderid/logger';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ApplicationCreateContext, {
  type ApplicationCreateContextType,
} from '../../../contexts/ApplicationCreate/ApplicationCreateContext';
import {ApplicationCreateFlowSignInApproach} from '../../../models/application-create-flow';
import {TechnologyApplicationTemplate, PlatformApplicationTemplate} from '../../../models/application-templates';
import type {ApplicationTemplate} from '../../../models/application-templates';
import ConfigureDetails from '../ConfigureDetails';

// Real @thunderid/configure-connections (imported for AuthenticatorTypes above) transitively
// resolves @thunderid/configure-organization-units' dist build, which fails to resolve its
// framer-motion import under vitest's transform. Stubbed here to avoid that (same workaround
// ApplicationCreatePage.test.tsx already applies); this component never renders anything from it.
vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: () => ({hasMultipleOUs: false, isLoading: false, ouList: []}),
  useGetOrganizationUnit: () => ({data: undefined}),
  OrganizationUnitPickerScreen: () => null,
}));

let translationLookup = (key: string): string => key;

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => translationLookup(key),
  }),
}));

const createTemplate = (name: string, redirectUris?: string[]): ApplicationTemplate => ({
  description: `${name} description`,
  defaults: {
    name,
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          redirectUris,
          grantTypes: ['authorization_code'],
          responseTypes: ['code'],
          tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
        },
      },
    ],
  },
});

// The Heidi wallet vendor's fixed client id (see constants/wallet-vendors.ts).
const HEIDI_CLIENT_ID = 'c3ce7a6c-2bbb-4abe-909c-41bc9463d3c5';

const createWalletTemplate = (): ApplicationTemplate => ({
  id: 'wallet',
  ...createTemplate('Digital Wallet', []),
});

// The sign-in approach picker itself now lives on the Design step (ConfigureDesign); this
// component only reacts to the currently selected approach (e.g. hiding URL fields for Embedded).
const defaultProps: Parameters<typeof ConfigureDetails>[0] = {
  technology: null,
  platform: null,
  onHostingUrlChange: vi.fn(),
  onCallbackUrlChange: vi.fn(),
  onReadyChange: vi.fn(),
  selectedApproach: ApplicationCreateFlowSignInApproach.REDIRECT_BASED,
};

const renderWithContext = (
  props: Partial<Parameters<typeof ConfigureDetails>[0]> = {},
  contextOverrides: Partial<ApplicationCreateContextType> = {},
) => {
  const baseContext: ApplicationCreateContextType = {
    currentStep: null as unknown as ApplicationCreateContextType['currentStep'],
    setCurrentStep: vi.fn(),
    appName: 'Test App',
    setAppName: vi.fn(),
    ouId: '',
    setOuId: vi.fn(),
    themeId: null,
    setThemeId: vi.fn(),
    selectedTheme: null,
    setSelectedTheme: vi.fn(),
    layoutId: null,
    setLayoutId: vi.fn(),
    selectedLayout: null,
    setSelectedLayout: vi.fn(),
    appLogo: null,
    setAppLogo: vi.fn(),
    selectedColor: '',
    setSelectedColor: vi.fn(),
    integrations: {},
    setIntegrations: vi.fn(),
    toggleIntegration: vi.fn(),
    isEmailOtpMfaEnabled: false,
    setIsEmailOtpMfaEnabled: vi.fn(),
    isSmsOtpMfaEnabled: false,
    setIsSmsOtpMfaEnabled: vi.fn(),
    smsOtpSenderId: '',
    setSmsOtpSenderId: vi.fn(),
    selectedAuthFlow: null,
    setSelectedAuthFlow: vi.fn(),
    signInApproach: null as unknown as ApplicationCreateContextType['signInApproach'],
    setSignInApproach: vi.fn(),
    registrationFlowId: null,
    setRegistrationFlowId: vi.fn(),
    isRegistrationFlowEnabled: false,
    setIsRegistrationFlowEnabled: vi.fn(),
    recoveryFlowId: null,
    setRecoveryFlowId: vi.fn(),
    isRecoveryFlowEnabled: false,
    setIsRecoveryFlowEnabled: vi.fn(),
    signOutFlowId: null,
    setSignOutFlowId: vi.fn(),
    isSignOutFlowEnabled: false,
    setIsSignOutFlowEnabled: vi.fn(),
    redirectUris: [],
    setRedirectUris: vi.fn(),
    postLogoutRedirectUris: [],
    setPostLogoutRedirectUris: vi.fn(),
    corsOrigins: [],
    setCorsOrigins: vi.fn(),
    ouDefaults: {signIn: false, signUp: false, recovery: false, signOut: false, theme: false, layout: false},
    setOuDefaults: vi.fn(),
    selectedTechnology: null,
    setSelectedTechnology: vi.fn(),
    selectedPlatform: null,
    setSelectedPlatform: vi.fn(),
    selectedTemplateConfig: null,
    setSelectedTemplateConfig: vi.fn(),
    mcpClientType: 'userDelegated',
    setMcpClientType: vi.fn(),
    mcpRedirectUris: [],
    setMcpRedirectUris: vi.fn(),
    hostingUrl: '',
    setHostingUrl: vi.fn(),
    callbackUrlFromConfig: '',
    setCallbackUrlFromConfig: vi.fn(),
    hasCompletedOnboarding: false,
    setHasCompletedOnboarding: vi.fn(),
    error: null,
    setError: vi.fn(),
    reset: vi.fn(),
    relyingPartyId: '',
    setRelyingPartyId: vi.fn(),
    relyingPartyName: '',
    setRelyingPartyName: vi.fn(),
    ...contextOverrides,
  };

  return render(
    <LoggerProvider
      logger={{
        level: LogLevel.ERROR,
        transports: [],
      }}
    >
      <ApplicationCreateContext.Provider value={baseContext}>
        <ConfigureDetails {...defaultProps} {...props} />
      </ApplicationCreateContext.Provider>
    </LoggerProvider>,
  );
};

describe('ConfigureDetails', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    translationLookup = (key: string): string => key;
  });

  it('renders the redirect URI editor for a redirect-capable template with a prefilled placeholder URI', () => {
    const template = createTemplate('Browser App', ['https://example.com/callback']);

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {selectedTemplateConfig: template},
    );

    expect(screen.getByTestId('application-configure-redirect-uris')).toBeInTheDocument();
  });

  it('renders no configuration UI for a non-redirect-capable template with a prefilled redirect URI', () => {
    const template: ApplicationTemplate = {
      description: 'Backend App description',
      defaults: {
        name: 'Backend App',
        inboundAuthConfig: [
          {
            type: 'oauth2',
            config: {
              redirectUris: ['https://example.com/callback'],
              grantTypes: ['client_credentials'],
              responseTypes: [],
              tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
            },
          },
        ],
      },
    };

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {selectedTemplateConfig: template},
    );

    // Nothing left to configure: no URLs section, no redirect URI editor, no passkey fields.
    expect(screen.queryByTestId('application-configure-redirect-uris')).not.toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText('applications:onboarding.configure.details.hostingUrl.placeholder'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('applications:onboarding.configure.details.passkey.title')).not.toBeInTheDocument();
  });

  it('renders passkey configuration even when no other configuration is required', () => {
    const template = createTemplate('Browser App', ['https://example.com/callback']);

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {
        selectedTemplateConfig: template,
        integrations: {[AuthenticatorTypes.PASSKEY]: true},
        selectedAuthFlow: null,
      },
    );

    expect(screen.getByText('applications:onboarding.configure.details.passkey.title')).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('applications:onboarding.configure.details.relyingPartyId.placeholder'),
    ).toBeInTheDocument();
  });

  it('shows URL configuration inputs and notifies callbacks when values change', async () => {
    const template = createTemplate('Browser App', []);
    const onHostingUrlChange = vi.fn();
    const onCallbackUrlChange = vi.fn();
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange,
        onCallbackUrlChange,
        onReadyChange,
      },
      {selectedTemplateConfig: template},
    );

    const hostingUrlInput = screen.getByPlaceholderText(
      'applications:onboarding.configure.details.hostingUrl.placeholder',
    );
    const user = userEvent.setup({delay: null}); // Remove typing delay for faster test

    await user.type(hostingUrlInput, 'https://example.com');

    await waitFor(() => expect(onHostingUrlChange).toHaveBeenLastCalledWith('https://example.com'));

    const customRadio = screen.getByRole('radio', {
      name: 'applications:onboarding.configure.details.callbackMode.custom',
    });
    await user.click(customRadio);

    const callbackUrlInput = document.getElementById('callback-url-input') as HTMLInputElement;
    await user.clear(callbackUrlInput);
    await user.type(callbackUrlInput, 'https://example.com/callback');

    await waitFor(() => expect(onCallbackUrlChange).toHaveBeenLastCalledWith('https://example.com/callback'), {
      timeout: 10000,
    });
    expect(onReadyChange).toHaveBeenCalled();
  }, 15000);

  it('displays deep link configuration and forwards values for mobile templates', async () => {
    const template = createTemplate('Mobile App', []);
    const onCallbackUrlChange = vi.fn();
    const onHostingUrlChange = vi.fn();
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.OTHER,
        platform: PlatformApplicationTemplate.MOBILE,
        onHostingUrlChange,
        onCallbackUrlChange,
        onReadyChange,
      },
      {selectedTemplateConfig: template},
    );

    expect(screen.getByText('applications:onboarding.configure.details.mobile.info')).toBeInTheDocument();

    const deeplinkInput = screen.getByPlaceholderText('applications:onboarding.configure.details.deeplink.placeholder');
    const user = userEvent.setup();
    await user.type(deeplinkInput, 'myapp://callback');

    await waitFor(() => expect(onCallbackUrlChange).toHaveBeenLastCalledWith('myapp://callback'));
    expect(onReadyChange).toHaveBeenCalled();
  });

  it('validates hosting URL input and shows validation errors', async () => {
    const template = createTemplate('Browser App', []);
    const onHostingUrlChange = vi.fn();
    const onCallbackUrlChange = vi.fn();
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange,
        onCallbackUrlChange,
        onReadyChange,
      },
      {selectedTemplateConfig: template},
    );

    const hostingUrlInput = screen.getByPlaceholderText(
      'applications:onboarding.configure.details.hostingUrl.placeholder',
    );
    const user = userEvent.setup();

    // Type invalid URL
    await user.type(hostingUrlInput, 'not-a-url');
    await user.tab(); // Trigger validation

    await waitFor(() => {
      expect(screen.getByText('Please enter a valid URL')).toBeInTheDocument();
    });

    // Clear and type valid URL
    await user.clear(hostingUrlInput);
    await user.type(hostingUrlInput, 'https://example.com');

    await waitFor(() => {
      expect(screen.queryByText('Please enter a valid URL')).not.toBeInTheDocument();
      expect(onHostingUrlChange).toHaveBeenLastCalledWith('https://example.com');
    });
  });

  it('validates callback URL when in custom mode', async () => {
    const template = createTemplate('Browser App', []);
    const onHostingUrlChange = vi.fn();
    const onCallbackUrlChange = vi.fn();
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange,
        onCallbackUrlChange,
        onReadyChange,
      },
      {selectedTemplateConfig: template},
    );

    const user = userEvent.setup();

    // Switch to custom callback mode
    const customRadio = screen.getByRole('radio', {
      name: 'applications:onboarding.configure.details.callbackMode.custom',
    });
    await user.click(customRadio);

    const callbackUrlInput = document.getElementById('callback-url-input') as HTMLInputElement;

    // Type invalid URL
    await user.type(callbackUrlInput, 'invalid-url');
    await user.tab(); // Trigger validation

    await waitFor(() => {
      expect(screen.getByText('Please enter a valid URL')).toBeInTheDocument();
    });
  });

  it('validates deep link input for mobile apps', async () => {
    const template = createTemplate('Mobile App', []);
    const onCallbackUrlChange = vi.fn();
    const onHostingUrlChange = vi.fn();
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.OTHER,
        platform: PlatformApplicationTemplate.MOBILE,
        onHostingUrlChange,
        onCallbackUrlChange,
        onReadyChange,
      },
      {selectedTemplateConfig: template},
    );

    const deeplinkInput = screen.getByPlaceholderText('applications:onboarding.configure.details.deeplink.placeholder');
    const user = userEvent.setup();

    // Type invalid deep link
    await user.type(deeplinkInput, 'invalid-deeplink');
    await user.tab(); // Trigger validation

    await waitFor(() => {
      expect(screen.getByText(/Please enter a valid deep link/)).toBeInTheDocument();
    });
  });

  it('handles same as hosting URL callback mode correctly', async () => {
    const template = createTemplate('Browser App', []);
    const onHostingUrlChange = vi.fn();
    const onCallbackUrlChange = vi.fn();
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange,
        onCallbackUrlChange,
        onReadyChange,
      },
      {selectedTemplateConfig: template},
    );

    const hostingUrlInput = screen.getByPlaceholderText(
      'applications:onboarding.configure.details.hostingUrl.placeholder',
    );
    const user = userEvent.setup();

    // Type hosting URL
    await user.type(hostingUrlInput, 'https://example.com');

    // By default, "Same as hosting" should be selected, so callback URL should sync
    await waitFor(() => {
      expect(onCallbackUrlChange).toHaveBeenLastCalledWith('https://example.com');
    });
  });

  it('renders user type selection when multiple user types are available', () => {
    // Create template with empty allowedUserTypes array to trigger user type selection
    const template: ApplicationTemplate = {
      ...createTemplate('Browser App', []),
      defaults: {
        ...createTemplate('Browser App', []).defaults,
        allowedUserTypes: [], // Empty array means user types selection is required
      },
    };
    const userTypes = [
      {id: 'user-type-1', name: 'Customer', ouId: 'ou-1', allowSelfRegistration: true},
      {id: 'user-type-2', name: 'Employee', ouId: 'ou-2', allowSelfRegistration: false},
    ];

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
        userTypes,
        selectedUserTypes: [],
        onUserTypesChange: vi.fn(),
      },
      {selectedTemplateConfig: template},
    );

    expect(screen.getByText('applications:onboarding.configure.details.userTypes.label')).toBeInTheDocument();
  });

  it('calls onUserTypesChange when user type selection changes', async () => {
    // Create template with empty allowedUserTypes array to trigger user type selection
    const template: ApplicationTemplate = {
      ...createTemplate('Browser App', []),
      defaults: {
        ...createTemplate('Browser App', []).defaults,
        allowedUserTypes: [], // Empty array means user types selection is required
      },
    };
    const userTypes = [
      {id: 'user-type-1', name: 'Customer', ouId: 'ou-1', allowSelfRegistration: true},
      {id: 'user-type-2', name: 'Employee', ouId: 'ou-2', allowSelfRegistration: false},
    ];
    const onUserTypesChange = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
        userTypes,
        selectedUserTypes: [],
        onUserTypesChange,
      },
      {selectedTemplateConfig: template},
    );

    const autocomplete = screen.getByRole('combobox');
    const user = userEvent.setup();
    await user.click(autocomplete);

    const customerOption = await screen.findByText('Customer');
    await user.click(customerOption);

    expect(onUserTypesChange).toHaveBeenCalledWith(['Customer']);
  });

  it('does not render user type selection when no user types are provided', () => {
    const template = createTemplate('Browser App', []);

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
        userTypes: [],
        selectedUserTypes: [],
      },
      {selectedTemplateConfig: template},
    );

    expect(screen.queryByText('applications:onboarding.configure.details.userTypes.label')).not.toBeInTheDocument();
  });

  it('notifies readiness based on form validity', async () => {
    const template = createTemplate('Browser App', []);
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange,
      },
      {selectedTemplateConfig: template},
    );

    // Initially should not be ready (no URLs entered)
    await waitFor(() => {
      expect(onReadyChange).toHaveBeenCalledWith(false);
    });

    const hostingUrlInput = screen.getByPlaceholderText(
      'applications:onboarding.configure.details.hostingUrl.placeholder',
    );
    const user = userEvent.setup();

    // Enter valid URL - should become ready
    await user.type(hostingUrlInput, 'https://example.com');

    await waitFor(() => {
      expect(onReadyChange).toHaveBeenCalledWith(true);
    });
  });

  describe('CORS readiness guard', () => {
    // A malformed origin used to be dropped on submit without a word, so the step has to block on it.
    const corsTemplate = (): ApplicationTemplate => ({
      ...createTemplate('Browser App', []),
      capabilities: {cors: true},
    });

    it('blocks the step while a CORS row is invalid, and releases it once the row is corrected', async () => {
      const onReadyChange = vi.fn();
      renderWithContext(
        {
          technology: TechnologyApplicationTemplate.REACT,
          platform: PlatformApplicationTemplate.BROWSER,
          onReadyChange,
        },
        {
          selectedTemplateConfig: corsTemplate(),
          corsOrigins: [createRow(AllowedOriginTypes.ORIGIN, 'https://example.com/path')],
        },
      );

      const user = userEvent.setup();
      await user.type(
        screen.getByPlaceholderText('applications:onboarding.configure.details.hostingUrl.placeholder'),
        'https://example.com',
      );

      // The URL config is valid, so only the malformed origin can be holding readiness back.
      await waitFor(() => {
        expect(onReadyChange).toHaveBeenLastCalledWith(false);
      });
    });

    it('does not block the step when every CORS row is valid', async () => {
      const onReadyChange = vi.fn();
      renderWithContext(
        {
          technology: TechnologyApplicationTemplate.REACT,
          platform: PlatformApplicationTemplate.BROWSER,
          onReadyChange,
        },
        {
          selectedTemplateConfig: corsTemplate(),
          corsOrigins: [createRow(AllowedOriginTypes.REGEX, '^https://[a-z]+\\.example\\.com$')],
        },
      );

      const user = userEvent.setup();
      await user.type(
        screen.getByPlaceholderText('applications:onboarding.configure.details.hostingUrl.placeholder'),
        'https://example.com',
      );

      await waitFor(() => {
        expect(onReadyChange).toHaveBeenLastCalledWith(true);
      });
    });

    it('ignores rows left over from a template that no longer shows the editor', async () => {
      const onReadyChange = vi.fn();
      renderWithContext(
        {
          technology: TechnologyApplicationTemplate.REACT,
          platform: PlatformApplicationTemplate.BROWSER,
          onReadyChange,
        },
        {
          // No cors capability, so the editor is hidden and its stale rows must not block the step.
          selectedTemplateConfig: createTemplate('Browser App', []),
          corsOrigins: [createRow(AllowedOriginTypes.ORIGIN, 'https://example.com/path')],
        },
      );

      const user = userEvent.setup();
      await user.type(
        screen.getByPlaceholderText('applications:onboarding.configure.details.hostingUrl.placeholder'),
        'https://example.com',
      );

      await waitFor(() => {
        expect(onReadyChange).toHaveBeenLastCalledWith(true);
      });
    });
  });

  it('handles server applications configuration correctly', () => {
    const template = createTemplate('Server Application', []);

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.NEXTJS,
        platform: PlatformApplicationTemplate.FULL_STACK,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {selectedTemplateConfig: template},
    );

    expect(screen.getByText('applications:onboarding.configure.details.urls.title')).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('applications:onboarding.configure.details.hostingUrl.placeholder'),
    ).toBeInTheDocument();
  });

  it('allows updating relying party ID for passkey configuration', async () => {
    const template = createTemplate('Browser App', ['https://example.com/callback']);
    const setRelyingPartyId = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {
        selectedTemplateConfig: template,
        integrations: {[AuthenticatorTypes.PASSKEY]: true},
        relyingPartyId: 'localhost',
        setRelyingPartyId,
      },
    );

    const relyingPartyIdInput = screen.getByPlaceholderText(
      'applications:onboarding.configure.details.relyingPartyId.placeholder',
    );
    const user = userEvent.setup();

    await user.clear(relyingPartyIdInput);
    await user.type(relyingPartyIdInput, 'example.com');

    expect(setRelyingPartyId).toHaveBeenCalled();
  });

  it('allows updating relying party name for passkey configuration', async () => {
    const template = createTemplate('Browser App', ['https://example.com/callback']);
    const setRelyingPartyName = vi.fn();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {
        selectedTemplateConfig: template,
        integrations: {[AuthenticatorTypes.PASSKEY]: true},
        relyingPartyName: 'Test App',
        setRelyingPartyName,
      },
    );

    const relyingPartyNameInput = screen.getByPlaceholderText(
      'applications:onboarding.configure.details.relyingPartyName.placeholder',
    );
    const user = userEvent.setup();

    await user.clear(relyingPartyNameInput);
    await user.type(relyingPartyNameInput, 'My Application');

    expect(setRelyingPartyName).toHaveBeenCalled();
  });

  it('renders both passkey and URL configuration when passkey is enabled', () => {
    const template = createTemplate('Browser App', []);

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {
        selectedTemplateConfig: template,
        integrations: {[AuthenticatorTypes.PASSKEY]: true},
      },
    );

    // Should show passkey configuration
    expect(screen.getByText('applications:onboarding.configure.details.passkey.title')).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('applications:onboarding.configure.details.relyingPartyId.placeholder'),
    ).toBeInTheDocument();

    // Should also show URL configuration
    expect(
      screen.getByPlaceholderText('applications:onboarding.configure.details.hostingUrl.placeholder'),
    ).toBeInTheDocument();
  });

  it('does not render passkey configuration when CREDENTIALS_AUTH is the only authenticator', () => {
    const template = createTemplate('Browser App', []);

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {
        selectedTemplateConfig: template,
        integrations: {[AuthenticatorTypes.CREDENTIALS_AUTH]: true},
      },
    );

    // Should not show passkey section
    expect(screen.queryByText('applications:onboarding.configure.details.passkey.title')).not.toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText('applications:onboarding.configure.details.relyingPartyId.placeholder'),
    ).not.toBeInTheDocument();
  });

  it('initializes passkey relying party defaults from hostname and app name', () => {
    const template = createTemplate('Browser App', ['https://example.com/callback']);

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {
        selectedTemplateConfig: template,
        integrations: {[AuthenticatorTypes.PASSKEY]: true},
        selectedAuthFlow: null,
        relyingPartyId: '',
        relyingPartyName: '',
      },
    );

    const relyingPartyIdInput = screen.getByDisplayValue(window.location.hostname);
    const relyingPartyNameInput = screen.getByDisplayValue('Test App');

    expect(relyingPartyIdInput).toHaveValue(window.location.hostname);
    expect(relyingPartyNameInput).toHaveValue('Test App');
  });

  it('falls back to default passkey labels and placeholders when translations are empty', () => {
    translationLookup = (): string => '';
    const template = createTemplate('Browser App', ['https://example.com/callback']);

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.REACT,
        platform: PlatformApplicationTemplate.BROWSER,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {
        selectedTemplateConfig: template,
        integrations: {[AuthenticatorTypes.PASSKEY]: true},
        selectedAuthFlow: null,
      },
    );

    expect(screen.getByText('Passkeys')).toBeInTheDocument();
    expect(screen.getByText('Relying Party ID')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('e.g., example.com')).toBeInTheDocument();
    expect(screen.getByText('Relying Party Name')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('e.g., My App')).toBeInTheDocument();
  });

  it('disables the card and does not select an already-connected wallet vendor', async () => {
    const onClientIdChange = vi.fn();
    const user = userEvent.setup();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.OTHER,
        platform: PlatformApplicationTemplate.WALLET,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
        onClientIdChange,
        existingClientIds: [HEIDI_CLIENT_ID],
      },
      {selectedTemplateConfig: createWalletTemplate()},
    );

    await user.click(screen.getByTestId('wallet-vendor-card-heidi'));

    expect(onClientIdChange).not.toHaveBeenCalledWith(HEIDI_CLIENT_ID);
    expect(screen.queryByTestId('wallet-duplicate-client-id-alert')).not.toBeInTheDocument();
  });

  it('warns and blocks the step when a custom client id is already in use', async () => {
    const onReadyChange = vi.fn();
    const user = userEvent.setup();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.OTHER,
        platform: PlatformApplicationTemplate.WALLET,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange,
        existingClientIds: ['taken-client-id'],
      },
      {selectedTemplateConfig: createWalletTemplate()},
    );

    const clientIdInput = screen.getByPlaceholderText(
      'applications:onboarding.configure.details.wallet.clientId.placeholder',
    );
    await user.type(clientIdInput, 'taken-client-id');

    expect(await screen.findByTestId('wallet-duplicate-client-id-alert')).toBeInTheDocument();
    await waitFor(() => expect(onReadyChange).toHaveBeenLastCalledWith(false));
  });

  it('does not warn when the selected wallet client id is not already in use', async () => {
    const onReadyChange = vi.fn();
    const user = userEvent.setup();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.OTHER,
        platform: PlatformApplicationTemplate.WALLET,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange,
        existingClientIds: ['a-different-client-id'],
      },
      {selectedTemplateConfig: createWalletTemplate()},
    );

    await user.click(screen.getByTestId('wallet-vendor-card-heidi'));

    expect(screen.queryByTestId('wallet-duplicate-client-id-alert')).not.toBeInTheDocument();
    await waitFor(() => expect(onReadyChange).toHaveBeenLastCalledWith(true));
  });

  it('shows the client id and deep link fields for Custom but hides them for a known wallet vendor', async () => {
    const user = userEvent.setup();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.OTHER,
        platform: PlatformApplicationTemplate.WALLET,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
      {selectedTemplateConfig: createWalletTemplate()},
    );

    expect(
      screen.getByPlaceholderText('applications:onboarding.configure.details.wallet.clientId.placeholder'),
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('applications:onboarding.configure.details.deeplink.placeholder'),
    ).toBeInTheDocument();

    await user.click(screen.getByTestId('wallet-vendor-card-heidi'));

    expect(
      screen.queryByPlaceholderText('applications:onboarding.configure.details.wallet.clientId.placeholder'),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText('applications:onboarding.configure.details.deeplink.placeholder'),
    ).not.toBeInTheDocument();
  });

  describe('Sign-in approach reactions', () => {
    it('hides the URL fields underneath and is ready when Embedded is selected', async () => {
      const onReadyChange = vi.fn();
      const template = createTemplate('Browser App', []);

      renderWithContext(
        {
          technology: TechnologyApplicationTemplate.REACT,
          platform: PlatformApplicationTemplate.BROWSER,
          selectedApproach: ApplicationCreateFlowSignInApproach.EMBEDDED,
          onReadyChange,
        },
        {selectedTemplateConfig: template},
      );

      expect(
        screen.queryByPlaceholderText('applications:onboarding.configure.details.hostingUrl.placeholder'),
      ).not.toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-redirect-uris')).not.toBeInTheDocument();
      await waitFor(() => expect(onReadyChange).toHaveBeenCalledWith(true));
    });

    it('shows the URL fields underneath when Inbuilt is selected', () => {
      const template = createTemplate('Browser App', []);

      renderWithContext(
        {
          technology: TechnologyApplicationTemplate.REACT,
          platform: PlatformApplicationTemplate.BROWSER,
          selectedApproach: ApplicationCreateFlowSignInApproach.REDIRECT_BASED,
        },
        {selectedTemplateConfig: template},
      );

      expect(
        screen.getByPlaceholderText('applications:onboarding.configure.details.hostingUrl.placeholder'),
      ).toBeInTheDocument();
    });
  });
});
