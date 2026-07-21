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

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {AuthenticatorTypes} from '@thunderid/configure-connections';
import {LoggerProvider, LogLevel} from '@thunderid/logger';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ApplicationCreateContext, {
  type ApplicationCreateContextType,
} from '../../../contexts/ApplicationCreate/ApplicationCreateContext';
import {TechnologyApplicationTemplate, PlatformApplicationTemplate} from '../../../models/application-templates';
import type {ApplicationTemplate} from '../../../models/application-templates';
import {TokenEndpointAuthMethods} from '../../../models/oauth';
import ConfigureDetails from '../ConfigureDetails';

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

const renderWithContext = (
  props: Parameters<typeof ConfigureDetails>[0],
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
    appLogo: null,
    setAppLogo: vi.fn(),
    selectedColor: '',
    setSelectedColor: vi.fn(),
    integrations: {},
    setIntegrations: vi.fn(),
    toggleIntegration: vi.fn(),
    selectedAuthFlow: null,
    setSelectedAuthFlow: vi.fn(),
    signInApproach: null as unknown as ApplicationCreateContextType['signInApproach'],
    setSignInApproach: vi.fn(),
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
        <ConfigureDetails {...props} />
      </ApplicationCreateContext.Provider>
    </LoggerProvider>,
  );
};

describe('ConfigureDetails', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    translationLookup = (key: string): string => key;
  });

  it('renders the no-configuration message when redirect URIs are already populated', () => {
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

    expect(
      screen.getByText('applications:onboarding.configure.details.noConfigRequired.description'),
    ).toBeInTheDocument();
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

    expect(
      screen.queryByText('applications:onboarding.configure.details.noConfigRequired.title'),
    ).not.toBeInTheDocument();
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

    expect(screen.getByText('applications:onboarding.configure.details.title')).toBeInTheDocument();
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

    expect(screen.getByText('Passkey Settings')).toBeInTheDocument();
    expect(screen.getByText('Relying Party ID')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('e.g., example.com')).toBeInTheDocument();
    expect(screen.getByText('Relying Party Name')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('e.g., My App')).toBeInTheDocument();
  });

  it('warns and blocks the step when a known wallet vendor is already connected', async () => {
    const onReadyChange = vi.fn();
    const user = userEvent.setup();

    renderWithContext(
      {
        technology: TechnologyApplicationTemplate.OTHER,
        platform: PlatformApplicationTemplate.WALLET,
        onHostingUrlChange: vi.fn(),
        onCallbackUrlChange: vi.fn(),
        onReadyChange,
        existingClientIds: [HEIDI_CLIENT_ID],
      },
      {selectedTemplateConfig: createWalletTemplate()},
    );

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Heidi'}));

    expect(await screen.findByTestId('wallet-duplicate-client-id-alert')).toBeInTheDocument();
    await waitFor(() => expect(onReadyChange).toHaveBeenLastCalledWith(false));
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

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Heidi'}));

    expect(screen.queryByTestId('wallet-duplicate-client-id-alert')).not.toBeInTheDocument();
    await waitFor(() => expect(onReadyChange).toHaveBeenLastCalledWith(true));
  });
});
