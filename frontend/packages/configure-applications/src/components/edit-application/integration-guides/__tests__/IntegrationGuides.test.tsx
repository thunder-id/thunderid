// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen, fireEvent, waitFor, within} from '@testing-library/react';
import {
  RuntimeEnvironmentsContext,
  type EnvironmentValue,
  type RuntimeEnvironment,
  type RuntimeEnvironmentsContextType,
} from '@thunderid/contexts';
import {LoggerProvider, LogLevel} from '@thunderid/logger';
import {MemoryRouter} from 'react-router';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import IntegrationGuides from '../IntegrationGuides';

const mockGetServerUrl = vi.fn(() => 'https://localhost:8090');
const mockGetRuntimeUrl = vi.fn(() => 'https://localhost:8090');
const mockGetDocumentationLink = vi.fn((key: string) => documentationLinks[key]);
const mockConfig: {brand: {product_name: string}; direct_api?: {enabled?: boolean}} = {
  brand: {product_name: 'ThunderID'},
};

const documentationLinks: Record<string, string> = {
  'applications.templates.react.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
  'applications.templates.react.playground':
    'https://stackblitz.com/fork/github/thunder-id/samples/tree/main/quickstarts/react?file=README.md',
  'applications.templates.react.llmPrompt.redirectBased': 'https://thunderid.dev/prompts/react/redirect-based.txt',
  'applications.templates.nextjs.llmPrompt.redirectBased': 'https://thunderid.dev/prompts/nextjs/redirect-based.txt',
  'applications.templates.ios.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/ios/',
  'applications.templates.android.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/android/',
  'applications.templates.android.llmPrompt.redirectBased': 'https://thunderid.dev/prompts/android/redirect-based.txt',
  'applications.templates.flutter.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/flutter/',
};

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      config: mockConfig,
      getServerUrl: mockGetServerUrl,
      getDocumentationLink: mockGetDocumentationLink,
    }),
    // With no gateway registered the runtime URL is the server URL, which is what most tests assert.
    useRuntimeUrl: () => mockGetRuntimeUrl(),
  };
});

vi.mock('@thunderid/design', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/design')>()),
  useGetTheme: (): unknown => mockUseGetTheme(),
  DefaultTheme: {colorSchemes: {light: {palette: {}}, dark: {palette: {}}}},
}));

vi.mock('@thunderid/configure-flows', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-flows')>()),
  useGetFlowById: () => ({data: undefined, isLoading: false}),
}));

vi.mock('@thunderid/configure-design', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-design')>()),
  GatePreview: () => <div data-testid="gate-preview" />,
}));

const mockUseGetOrganizationUnit = vi.fn(() => ({data: undefined}) as unknown);
const mockUseGetTheme = vi.fn(() => ({data: undefined, isLoading: false}) as unknown);

vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  useGetOrganizationUnit: (...args: unknown[]): unknown => mockUseGetOrganizationUnit(...(args as [])),
}));

const mockWriteText = vi.fn();
const mockFetch = vi.fn();

const promptFixtures: Record<string, string> = {
  'https://thunderid.dev/prompts/react/redirect-based.txt':
    'Integrate {{productName}} at {{baseUrl}} with clientId: {{clientId}}',
  'https://thunderid.dev/prompts/nextjs/redirect-based.txt': 'Integrate {{productName}} with Next.js',
};

const reactGuideUrl = 'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react.md';
const reactGuide = [
  '# React Quickstart',
  '',
  '## Create an Application',
  '',
  '1. Sign in to the Console.',
  '',
  '## Add the Provider',
  '',
  'Wrap your root component.',
  '',
  '```jsx title="src/main.jsx"',
  '<ThunderIDProvider clientId="<your-client-id>" baseUrl="https://localhost:8090" />',
  '```',
  '',
  '## Run Your App',
  '',
  'Start the development server.',
].join('\n');

const renderWithProviders = (component: React.ReactElement) =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <LoggerProvider logger={{level: LogLevel.DEBUG}}>{component}</LoggerProvider>
    </QueryClientProvider>,
  );

const reactApplication: Application = {
  id: 'app-123',
  name: 'Bifrost',
  template: 'react',
  type: 'browser',
  description: 'Test description',
  allowedUserTypes: ['admin', 'user'],
};

const oauth2Config: OAuth2Config = {
  clientId: 'client-123',
  clientSecret: 'secret-456',
  grantTypes: ['authorization_code'],
  responseTypes: ['code'],
  pkceRequired: true,
  publicClient: false,
  redirectUris: ['https://example.com/callback'],
};

describe('IntegrationGuides', () => {
  const originalClipboard = navigator.clipboard;

  beforeEach(() => {
    vi.useFakeTimers({shouldAdvanceTime: true});
    mockUseGetOrganizationUnit.mockReset().mockReturnValue({data: undefined});
    mockUseGetTheme.mockReset().mockReturnValue({data: undefined, isLoading: false});
    delete mockConfig.direct_api;
    mockGetRuntimeUrl.mockReset().mockReturnValue('https://localhost:8090');
    mockWriteText.mockReset().mockResolvedValue(undefined);
    mockGetDocumentationLink.mockImplementation((key: string) => documentationLinks[key]);
    mockFetch.mockReset().mockImplementation((url: string) =>
      Promise.resolve({
        ok: true,
        status: 200,
        text: () => Promise.resolve(promptFixtures[url] ?? ''),
      }),
    );
    vi.stubGlobal('fetch', mockFetch);
    Object.defineProperty(navigator, 'clipboard', {
      value: {writeText: mockWriteText},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    Object.defineProperty(navigator, 'clipboard', {value: originalClipboard, writable: true, configurable: true});
  });

  it('always shows application details, even for a template with no quickstart', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, template: 'unknown-template'}} />);

    expect(screen.getByText('Application details')).toBeInTheDocument();
    expect(screen.getByText('app-123')).toBeInTheDocument();
    expect(screen.queryByRole('link', {name: /Open on StackBlitz/i})).not.toBeInTheDocument();
  });

  it('shows the organization unit ID and handle in the application details card', () => {
    mockUseGetOrganizationUnit.mockReturnValue({data: {id: 'ou-1', handle: 'engineering'}});

    renderWithProviders(<IntegrationGuides application={{...reactApplication, ouId: 'ou-1'}} />);

    expect(screen.getByText('Organization unit ID')).toBeInTheDocument();
    expect(screen.getByText('ou-1')).toBeInTheDocument();
    expect(screen.getByText('Organization unit handle')).toBeInTheDocument();
    expect(screen.getByText('engineering')).toBeInTheDocument();
  });

  it('hides the organization unit rows when the application has no ouId', () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} />);

    expect(screen.queryByText('Organization unit ID')).not.toBeInTheDocument();
    expect(screen.queryByText('Organization unit handle')).not.toBeInTheDocument();
  });

  it('shows the organization unit ID alone when the handle has not resolved yet', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, ouId: 'ou-1'}} />);

    expect(screen.getByText('Organization unit ID')).toBeInTheDocument();
    expect(screen.queryByText('Organization unit handle')).not.toBeInTheDocument();
  });

  it('shows the OIDC endpoints and sign-in preview from the canonical type before the OAuth2 config resolves', () => {
    // oauth2Config isn't always resolved on first paint; the canonical application type (always
    // present) is enough to know a 'browser' app is OAuth2-based and user-facing.
    renderWithProviders(<IntegrationGuides application={reactApplication} />);

    expect(screen.getByText('app-123')).toBeInTheDocument();
    expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
    expect(screen.getByText('Preview')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: /Open on StackBlitz/i})).toBeInTheDocument();
  });

  it('hides the OIDC endpoints and sign-in preview for a custom application with no OAuth2 config', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, type: 'custom'}} />);

    expect(screen.queryByText('OIDC endpoints')).not.toBeInTheDocument();
    expect(screen.queryByText('Preview')).not.toBeInTheDocument();
  });

  it('hides only the sign-in preview for a machine-to-machine application with no OAuth2 config yet', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, type: 'm2m'}} />);

    expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
    expect(screen.queryByText('Preview')).not.toBeInTheDocument();
  });

  it('shows the OIDC endpoints (but not the Client ID row) when the OAuth2 config has no clientId yet', () => {
    renderWithProviders(
      <IntegrationGuides application={reactApplication} oauth2Config={{...oauth2Config, clientId: undefined}} />,
    );

    expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
    expect(screen.getByText('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
    expect(screen.queryByText('Client ID')).not.toBeInTheDocument();
  });

  it('renders the StackBlitz quickstart card for a template with a quickstart sample', () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    const stackblitzLink = screen.getByRole('link', {name: /Open on StackBlitz/i});
    expect(stackblitzLink).toHaveAttribute(
      'href',
      'https://stackblitz.com/fork/github/thunder-id/samples/tree/main/quickstarts/react?file=README.md',
    );
  });

  it('hides the integration guide when the docs link is not configured', () => {
    const documentationLinksWithoutDocs = {...documentationLinks};
    delete documentationLinksWithoutDocs['applications.templates.react.docs'];
    mockGetDocumentationLink.mockImplementation((key: string) => documentationLinksWithoutDocs[key]);

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(screen.queryByText('Integrate your application')).not.toBeInTheDocument();
    expect(screen.getByRole('link', {name: /Open on StackBlitz/i})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: /Copy prompt/i})).toBeInTheDocument();
  });

  it('shows the leaving-console confirmation before opening the docs guide', () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: 'Open integration guide'}));
    fireEvent.click(screen.getByRole('button', {name: /Open in docs/i}));
    expect(screen.getByText('You are leaving ThunderID')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'Continue'}));
    expect(openSpy).toHaveBeenCalledWith(
      'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
      '_blank',
      'noopener,noreferrer',
    );

    openSpy.mockRestore();
  });

  it('opens the docs guide in a dialog with the application values filled into the code', async () => {
    mockGetRuntimeUrl.mockReturnValue('https://gateway.example.com');
    promptFixtures[reactGuideUrl] = reactGuide;

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Open integration guide'}));

    expect(screen.getByRole('dialog', {name: 'Integration guide'})).toBeInTheDocument();
    expect(await screen.findByRole('button', {name: /Add the Provider/})).toBeInTheDocument();
    expect(screen.queryByText(/^Values for/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: /Run Your App/})).toBeInTheDocument();
    // Creating the application is already done by the time someone reads its Overview tab.
    expect(screen.queryByText('Sign in to the Console.')).not.toBeInTheDocument();
    expect(screen.getByLabelText('src/main.jsx')).toHaveTextContent(
      '<ThunderIDProvider clientId="client-123" baseUrl="https://gateway.example.com" />',
    );

    fireEvent.click(screen.getByRole('button', {name: 'Close'}));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: 'Open integration guide'}));
    fireEvent.keyDown(screen.getByRole('dialog', {name: 'Integration guide'}), {key: 'Escape'});
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    delete promptFixtures[reactGuideUrl];
  });

  it('keeps the link to the docs site when the page is not a quickstart it can show inline', async () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: 'Open integration guide'}));

    expect(await screen.findByText('Follow the step-by-step guide on the docs site.')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: /Open in docs/i})).toBeInTheDocument();
  });

  it('copies the coding agent prompt with placeholders replaced', async () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: /Copy prompt/i}));

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalledTimes(1);
    });
    const copiedText = mockWriteText.mock.calls[0][0] as string;
    expect(copiedText).toContain('client-123');
    expect(copiedText).toContain('ThunderID');
    expect(copiedText).not.toContain('{{clientId}}');
  });

  it('copies the coding agent prompt with the runtime URL as the base URL', async () => {
    mockGetRuntimeUrl.mockReturnValue('https://gateway.example.com');

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: /Copy prompt/i}));

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalledWith(
        'Integrate ThunderID at https://gateway.example.com with clientId: client-123',
      );
    });
  });

  it('renders application identifiers and OIDC endpoints using the configured server URL', () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(screen.getByText('app-123')).toBeInTheDocument();
    expect(screen.getByText('client-123')).toBeInTheDocument();
    expect(screen.getByText('https://localhost:8090/.well-known/openid-configuration')).toBeInTheDocument();
    expect(screen.getByText('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
    expect(screen.getByText('https://localhost:8090/oauth2/token')).toBeInTheDocument();
    expect(screen.getByText('https://localhost:8090/oauth2/userinfo')).toBeInTheDocument();
    expect(screen.getByText('https://localhost:8090/oauth2/jwks')).toBeInTheDocument();
  });

  // The endpoints are for someone to copy into their own application, so they name the gateway that
  // answers them, not the server the console talks to.
  it('renders the OIDC endpoints from the runtime gateway URL rather than the server URL', () => {
    mockGetRuntimeUrl.mockReturnValue('https://gateway.example.com');

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(screen.getByText('https://gateway.example.com/.well-known/openid-configuration')).toBeInTheDocument();
    expect(screen.getByText('https://gateway.example.com/oauth2/authorize')).toBeInTheDocument();
    expect(screen.getByText('https://gateway.example.com/oauth2/token')).toBeInTheDocument();
    expect(screen.queryByText('https://localhost:8090/oauth2/authorize')).not.toBeInTheDocument();
  });

  it('shows the assigned theme primary color in the sign-in preview swatch', () => {
    // A stored theme is plain JSON with no generated `vars`; the color lives under colorSchemes.
    mockUseGetTheme.mockReturnValue({
      data: {
        displayName: 'Acme Inc branding',
        theme: {
          colorSchemes: {light: {palette: {primary: {main: '#9333ea'}}}, dark: {palette: {primary: {main: '#9333ea'}}}},
        },
      },
      isLoading: false,
    });
    renderWithProviders(<IntegrationGuides application={{...reactApplication, themeId: 'theme-1'}} />);

    const swatch = screen.getByText('Acme Inc branding').previousElementSibling as HTMLElement;
    expect(getComputedStyle(swatch).backgroundColor).toBe('rgb(147, 51, 234)');
  });

  it('navigates to the Flows and Customization tabs via the sign-in preview links', () => {
    const onGoToFlows = vi.fn();
    const onGoToCustomization = vi.fn();
    renderWithProviders(
      <IntegrationGuides
        application={reactApplication}
        oauth2Config={oauth2Config}
        onGoToFlows={onGoToFlows}
        onGoToCustomization={onGoToCustomization}
      />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Edit in flows'}));
    expect(onGoToFlows).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', {name: 'Edit in customization'}));
    expect(onGoToCustomization).toHaveBeenCalledTimes(1);
  });

  it('does not show the sign-in preview for a machine-to-machine (client-credentials only) application', () => {
    renderWithProviders(
      <IntegrationGuides
        application={reactApplication}
        oauth2Config={{...oauth2Config, grantTypes: ['client_credentials']}}
      />,
    );

    expect(screen.queryByText('Preview')).not.toBeInTheDocument();
    expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
  });

  it('shows the coding agent prompt for a template whose default EMBEDDED variant has no guide content', () => {
    // Android defaults to the EMBEDDED sign-in approach (it's always app-native) but only has
    // REDIRECT_BASED guide content authored; every technology template should still get a
    // coding-agent prompt card.
    renderWithProviders(
      <IntegrationGuides
        application={{...reactApplication, template: 'android-embedded'}}
        oauth2Config={oauth2Config}
      />,
    );

    expect(screen.getByRole('button', {name: /Copy prompt/i})).toBeInTheDocument();
  });

  describe('native (mobile) applications', () => {
    const mobileApplication: Application = {
      ...reactApplication,
      template: 'mobile',
      type: 'mobile',
    };

    it('offers one quickstart guide per platform and no StackBlitz banner', () => {
      renderWithProviders(<IntegrationGuides application={mobileApplication} oauth2Config={oauth2Config} />);

      expect(screen.queryByRole('link', {name: /Open on StackBlitz/i})).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', {name: 'Open integration guide'}));

      const platforms = screen.getByRole('group', {name: 'Platform'});
      expect(within(platforms).getByRole('button', {name: 'iOS'})).toBeInTheDocument();
      expect(within(platforms).getByRole('button', {name: 'Android'})).toBeInTheDocument();
      expect(within(platforms).getByRole('button', {name: 'Flutter'})).toBeInTheDocument();
    });

    it('shows App Native flow endpoints instead of the OAuth2/OIDC endpoints', () => {
      renderWithProviders(<IntegrationGuides application={mobileApplication} oauth2Config={oauth2Config} />);

      expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
      expect(screen.queryByText('OIDC endpoints')).not.toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/execute')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/meta')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/register/passkey/start')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/register/passkey/finish')).toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/oauth2/authorize')).not.toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/oauth2/token')).not.toBeInTheDocument();
    });

    it('hides the passkey registration endpoints when the Direct API is disabled', () => {
      mockConfig.direct_api = {enabled: false};

      renderWithProviders(<IntegrationGuides application={mobileApplication} oauth2Config={oauth2Config} />);

      expect(screen.getByText('https://localhost:8090/flow/execute')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/meta')).toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/register/passkey/start')).not.toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/register/passkey/finish')).not.toBeInTheDocument();
    });

    it('shows the standard OAuth2/OIDC endpoints (not App Native ones) for a pure browser SPA', () => {
      renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

      expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/flow/execute')).not.toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/flow/meta')).not.toBeInTheDocument();
    });

    it('shows both OAuth2/OIDC and App Native endpoints for the Custom template', () => {
      renderWithProviders(
        <IntegrationGuides
          application={{...reactApplication, template: 'custom', type: 'm2m'}}
          oauth2Config={oauth2Config}
        />,
      );

      expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/execute')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/meta')).toBeInTheDocument();
    });

    it('also shows App Native flow endpoints for a fullstack application (e.g. Next.js)', () => {
      renderWithProviders(
        <IntegrationGuides
          application={{...reactApplication, template: 'nextjs', type: 'fullstack'}}
          oauth2Config={oauth2Config}
        />,
      );

      expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/execute')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/meta')).toBeInTheDocument();
    });

    it('renders the sign-in preview in a phone-style frame', () => {
      renderWithProviders(<IntegrationGuides application={mobileApplication} oauth2Config={oauth2Config} />);

      expect(screen.getByTestId('gate-preview')).toBeInTheDocument();
    });
  });

  describe('environments', () => {
    const development: RuntimeEnvironment = {
      id: 'env-dev',
      name: 'Development',
      runtimeUrl: 'https://dev.example.com',
      color: {light: '#0369A1', dark: '#5CD1FF'},
    };
    const production: RuntimeEnvironment = {id: 'env-prod', name: 'Production', runtimeUrl: 'https://prod.example.com'};
    const getValue = vi.fn<RuntimeEnvironmentsContextType['getValue']>();

    const renderWithEnvironments = (environments: RuntimeEnvironment[]) =>
      renderWithProviders(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
          <RuntimeEnvironmentsContext.Provider value={{environments, getValue}}>
            <IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />
          </RuntimeEnvironmentsContext.Provider>
        </QueryClientProvider>,
      );

    let clientIds: Record<string, EnvironmentValue>;

    beforeEach(() => {
      clientIds = {
        'env-dev': {status: 'set', value: 'client-dev'},
        'env-prod': {status: 'set', value: 'client-prod'},
      };
      getValue.mockReset().mockImplementation((environmentId: string) => Promise.resolve(clientIds[environmentId]));
    });

    it('shows no environment choice when the console deploys to a single environment', () => {
      renderWithEnvironments([development]);

      expect(screen.queryByText('Environment')).not.toBeInTheDocument();
      expect(screen.getByText('client-123')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
    });

    it('shows no client ID or endpoints where the only environment does not have the application', async () => {
      clientIds['env-dev'] = {status: 'absent'};
      renderWithEnvironments([development]);

      expect(await screen.findByText('Not deployed to Development yet')).toBeInTheDocument();
      expect(screen.queryByText('Environment')).not.toBeInTheDocument();
      expect(screen.queryByText('client-123')).not.toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/oauth2/authorize')).not.toBeInTheDocument();
      expect(screen.getByText('app-123')).toBeInTheDocument();
    });

    it("shows the first environment's client ID and endpoints when there are several", async () => {
      renderWithEnvironments([development, production]);

      expect(screen.getByText('Environment')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Development'})).toHaveAttribute('aria-pressed', 'true');
      expect(screen.getByRole('button', {name: 'Production'})).toHaveAttribute('aria-pressed', 'false');
      expect(screen.getByText('https://dev.example.com/oauth2/authorize')).toBeInTheDocument();
      expect(await screen.findByText('client-dev')).toBeInTheDocument();
      expect(screen.queryByText('client-123')).not.toBeInTheDocument();
      expect(getValue).toHaveBeenCalledWith('env-dev', {
        resourceType: 'application',
        resourceId: 'app-123',
        field: 'clientId',
      });
    });

    it('shows the client ID and endpoints of the environment chosen', async () => {
      renderWithEnvironments([development, production]);
      await screen.findByText('client-dev');

      fireEvent.click(screen.getByRole('button', {name: 'Production'}));

      expect(await screen.findByText('client-prod')).toBeInTheDocument();
      expect(screen.getByText('https://prod.example.com/oauth2/authorize')).toBeInTheDocument();
      expect(screen.queryByText('https://dev.example.com/oauth2/authorize')).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Production'})).toHaveAttribute('aria-pressed', 'true');
    });

    it("fills the integration guide and the coding agent prompt with the chosen environment's values", async () => {
      promptFixtures[reactGuideUrl] = reactGuide;
      renderWithEnvironments([development, production]);
      await waitFor(() => expect(screen.getByLabelText('Client ID')).toHaveTextContent('client-dev'));
      fireEvent.click(screen.getByRole('button', {name: 'Production'}));
      await waitFor(() => expect(screen.getByLabelText('Client ID')).toHaveTextContent('client-prod'));

      fireEvent.mouseOver(screen.getByRole('button', {name: /Copy prompt/i}));
      expect(
        await screen.findByText(
          'Copy a ready-made prompt for Claude, Cursor, or any agent, with the values for Production.',
        ),
      ).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', {name: /Copy prompt/i}));
      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith(
          'Integrate ThunderID at https://prod.example.com with clientId: client-prod',
        );
      });

      fireEvent.click(screen.getByRole('button', {name: 'Open integration guide'}));
      await screen.findByRole('button', {name: /Add the Provider/});
      expect(screen.getByLabelText('src/main.jsx')).toHaveTextContent(
        '<ThunderIDProvider clientId="client-prod" baseUrl="https://prod.example.com" />',
      );
      expect(
        within(screen.getByRole('dialog', {name: 'Integration guide'})).getByText('Values for Production'),
      ).toBeInTheDocument();

      delete promptFixtures[reactGuideUrl];
    });

    it("fills the chosen environment's URL into a prompt that names a local instance", async () => {
      const promptUrl = 'https://thunderid.dev/prompts/react/redirect-based.txt';
      const original = promptFixtures[promptUrl];
      promptFixtures[promptUrl] = 'Base URL: https://localhost:8090 (or your {{productName}} instance URL)';
      renderWithEnvironments([development, production]);
      await screen.findByText('client-dev');
      fireEvent.click(screen.getByRole('button', {name: 'Production'}));
      await screen.findByText('client-prod');

      fireEvent.click(screen.getByRole('button', {name: /Copy prompt/i}));
      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith(
          'Base URL: https://prod.example.com (or your ThunderID instance URL)',
        );
      });

      promptFixtures[promptUrl] = original;
    });

    it('leaves the client ID placeholder in the integration guide where the environment has none', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      promptFixtures[reactGuideUrl] = reactGuide;
      renderWithEnvironments([development, production]);
      await waitFor(() => expect(screen.getByLabelText('Client ID')).toHaveTextContent('client-dev'));
      fireEvent.click(screen.getByRole('button', {name: 'Production'}));
      await screen.findByText('Not promoted to Production yet');

      fireEvent.click(screen.getByRole('button', {name: /Copy prompt/i}));
      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith(
          'Integrate ThunderID at https://prod.example.com with clientId: <your-client-id>',
        );
      });

      fireEvent.click(screen.getByRole('button', {name: 'Open integration guide'}));
      await screen.findByRole('button', {name: /Add the Provider/});
      expect(screen.getByLabelText('src/main.jsx')).toHaveTextContent(
        '<ThunderIDProvider clientId="<your-client-id>" baseUrl="https://prod.example.com" />',
      );

      delete promptFixtures[reactGuideUrl];
    });

    it('says when the application is not deployed to an environment, or has no client ID there', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      clientIds['env-dev'] = {status: 'unset'};
      renderWithEnvironments([development, production]);

      expect(await screen.findByText('No client ID is set in Development yet.')).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', {name: 'Production'}));
      expect(await screen.findByText('Not promoted to Production yet')).toBeInTheDocument();
    });

    it('shows no client ID or endpoints for an environment the application is not in', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      renderWithEnvironments([development, production]);
      await screen.findByText('client-dev');

      fireEvent.click(screen.getByRole('button', {name: 'Production'}));

      expect(
        await screen.findByText(
          'There is no client ID or endpoints for Bifrost here. Promote it from Development to get them.',
          {exact: false},
        ),
      ).toBeInTheDocument();
      expect(screen.getByText('Application details')).toBeInTheDocument();
      expect(screen.queryByText('Client ID')).not.toBeInTheDocument();
      expect(screen.queryByText('Useful endpoints')).not.toBeInTheDocument();
      expect(screen.queryByText('https://prod.example.com/oauth2/authorize')).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', {name: 'Development'}));

      expect(await screen.findByText('client-dev')).toBeInTheDocument();
      expect(screen.getByText('https://dev.example.com/oauth2/authorize')).toBeInTheDocument();
    });

    it('marks the environments the application is not deployed to on the picker', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      renderWithEnvironments([development, production]);

      await waitFor(() =>
        expect(screen.getByRole('button', {name: 'Production'})).toHaveAccessibleDescription(
          'Not promoted to Production yet.',
        ),
      );
      expect(screen.getByRole('button', {name: 'Development'})).not.toHaveAccessibleDescription(
        'Not deployed to Development yet.',
      );
    });

    it('offers the way to deploy where the application is not deployed to an environment yet', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      renderWithProviders(
        <MemoryRouter>
          <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <RuntimeEnvironmentsContext.Provider
              value={{environments: [development, production], getValue, deployPath: '/projects/default/pipeline'}}
            >
              <IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />
            </RuntimeEnvironmentsContext.Provider>
          </QueryClientProvider>
        </MemoryRouter>,
      );
      await screen.findByText('client-dev');
      expect(screen.queryByRole('link', {name: 'Open deployment pipeline'})).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', {name: 'Production'}));

      expect(await screen.findByRole('link', {name: 'Open deployment pipeline'})).toHaveAttribute(
        'href',
        '/projects/default/pipeline',
      );
    });

    it('keeps environments that do not fit on one row that scrolls', async () => {
      const many: RuntimeEnvironment[] = Array.from({length: 8}, (_unused, index) => ({
        id: `env-${index}`,
        name: `Environment number ${index}`,
        runtimeUrl: `https://env-${index}.example.com`,
      }));
      many.forEach(({id}) => {
        clientIds[id] = {status: 'set', value: `client-${id}`};
      });
      renderWithEnvironments(many);
      await screen.findByText('client-env-0');

      const picker = screen.getByRole('group', {name: 'Environment to show'});
      const tops = within(picker)
        .getAllByRole('button')
        .map((button) => button.getBoundingClientRect().top);

      expect(new Set(tops).size).toBe(1);
      const scroller = picker.parentElement!;
      expect(scroller.scrollWidth).toBeGreaterThan(scroller.clientWidth);
      // More lies beyond the far edge, so that edge fades out under an arrow that scrolls that way.
      await waitFor(() =>
        expect(getComputedStyle(scroller).maskImage).toMatch(/(transparent|rgba\(0, 0, 0, 0\)) calc\(100% - 20px\)\)$/),
      );
      expect(screen.queryByRole('button', {name: 'Show earlier environments'})).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', {name: 'Show later environments'}));

      expect(await screen.findByRole('button', {name: 'Show earlier environments'})).toBeInTheDocument();
      expect(scroller.scrollLeft).toBeGreaterThan(0);
    });

    it('shows a failed read in place of a client ID read earlier', async () => {
      const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
      renderWithProviders(
        <QueryClientProvider client={client}>
          <RuntimeEnvironmentsContext.Provider value={{environments: [development, production], getValue}}>
            <IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />
          </RuntimeEnvironmentsContext.Provider>
        </QueryClientProvider>,
      );
      await screen.findByText('client-dev');

      getValue.mockRejectedValue(new Error('forbidden'));
      await client.refetchQueries();

      expect(await screen.findByRole('button', {name: 'Refresh'})).toBeInTheDocument();
      expect(screen.queryByText('client-dev')).not.toBeInTheDocument();
    });

    it('announces the environment whose values are shown', async () => {
      renderWithEnvironments([development, production]);
      await screen.findByText('client-dev');

      expect(screen.getByRole('status')).toHaveTextContent('Showing values for Development');
    });

    it('offers to read the client ID again when it cannot be read', async () => {
      getValue.mockRejectedValueOnce(new Error('forbidden'));
      renderWithEnvironments([development, production]);

      fireEvent.click(await screen.findByRole('button', {name: 'Refresh'}));

      expect(await screen.findByText('client-dev')).toBeInTheDocument();
    });
  });
});
