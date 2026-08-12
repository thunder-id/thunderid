// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import type {Application, OAuth2Config} from '@thunderid/configure-applications';
import {LoggerProvider, LogLevel} from '@thunderid/logger';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import IntegrationGuides from '../IntegrationGuides';

const mockGetServerUrl = vi.fn(() => 'https://localhost:8090');
const mockGetDocumentationLink = vi.fn((key: string) => documentationLinks[key]);

const documentationLinks: Record<string, string> = {
  'applications.templates.react.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
  'applications.templates.react.playground':
    'https://stackblitz.com/fork/github/thunder-id/javascript-sdks/tree/main/samples/react/quickstart?file=README.md',
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
      config: {brand: {product_name: 'ThunderID'}},
      getServerUrl: mockGetServerUrl,
      getDocumentationLink: mockGetDocumentationLink,
    }),
  };
});

vi.mock('@thunderid/design', () => ({
  useGetTheme: () => ({data: undefined, isLoading: false}),
  DefaultTheme: {colorSchemes: {light: {palette: {}}, dark: {palette: {}}}},
}));

vi.mock('../../../../../flows/api/useGetFlowById', () => ({
  default: () => ({data: undefined, isLoading: false}),
}));

vi.mock('@thunderid/configure-design', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-design')>()),
  GatePreview: () => <div data-testid="gate-preview" />,
}));

const mockUseGetOrganizationUnit = vi.fn(() => ({data: undefined}) as unknown);

vi.mock('@thunderid/configure-organization-units', () => ({
  useGetOrganizationUnit: (...args: unknown[]): unknown => mockUseGetOrganizationUnit(...(args as [])),
}));

const mockWriteText = vi.fn();
const mockFetch = vi.fn();

const promptFixtures: Record<string, string> = {
  'https://thunderid.dev/prompts/react/redirect-based.txt': 'Integrate {{productName}} with clientId: {{clientId}}',
  'https://thunderid.dev/prompts/nextjs/redirect-based.txt': 'Integrate {{productName}} with Next.js',
};

const renderWithProviders = (component: React.ReactElement) =>
  render(<LoggerProvider logger={{level: LogLevel.DEBUG}}>{component}</LoggerProvider>);

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

    expect(screen.getByText('Organization Unit ID')).toBeInTheDocument();
    expect(screen.getByText('ou-1')).toBeInTheDocument();
    expect(screen.getByText('Organization Unit Handle')).toBeInTheDocument();
    expect(screen.getByText('engineering')).toBeInTheDocument();
  });

  it('hides the organization unit rows when the application has no ouId', () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} />);

    expect(screen.queryByText('Organization Unit ID')).not.toBeInTheDocument();
    expect(screen.queryByText('Organization Unit Handle')).not.toBeInTheDocument();
  });

  it('shows the organization unit ID alone when the handle has not resolved yet', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, ouId: 'ou-1'}} />);

    expect(screen.getByText('Organization Unit ID')).toBeInTheDocument();
    expect(screen.queryByText('Organization Unit Handle')).not.toBeInTheDocument();
  });

  it('shows the OIDC endpoints and sign-in preview from the canonical type before the OAuth2 config resolves', () => {
    // oauth2Config isn't always resolved on first paint; the canonical application type (always
    // present) is enough to know a 'browser' app is OAuth2-based and user-facing.
    renderWithProviders(<IntegrationGuides application={reactApplication} />);

    expect(screen.getByText('app-123')).toBeInTheDocument();
    expect(screen.getByText('Useful Endpoints')).toBeInTheDocument();
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

    expect(screen.getByText('Useful Endpoints')).toBeInTheDocument();
    expect(screen.queryByText('Preview')).not.toBeInTheDocument();
  });

  it('shows the OIDC endpoints (but not the Client ID row) when the OAuth2 config has no clientId yet', () => {
    renderWithProviders(
      <IntegrationGuides application={reactApplication} oauth2Config={{...oauth2Config, clientId: undefined}} />,
    );

    expect(screen.getByText('Useful Endpoints')).toBeInTheDocument();
    expect(screen.getByText('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
    expect(screen.queryByText('Client ID')).not.toBeInTheDocument();
  });

  it('renders the StackBlitz quickstart card for a template with a quickstart sample', () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    const stackblitzLink = screen.getByRole('link', {name: /Open on StackBlitz/i});
    expect(stackblitzLink).toHaveAttribute(
      'href',
      'https://stackblitz.com/fork/github/thunder-id/javascript-sdks/tree/main/samples/react/quickstart?file=README.md',
    );
  });

  it('hides the read-the-quickstart-guide card when the docs link is not configured', () => {
    const documentationLinksWithoutDocs = {...documentationLinks};
    delete documentationLinksWithoutDocs['applications.templates.react.docs'];
    mockGetDocumentationLink.mockImplementation((key: string) => documentationLinksWithoutDocs[key]);

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(screen.queryByText('Read the quickstart guide')).not.toBeInTheDocument();
    expect(screen.getByRole('link', {name: /Open on StackBlitz/i})).toBeInTheDocument();
  });

  it('shows the leaving-console confirmation before opening the docs guide', () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: /Open quickstart/i}));
    expect(screen.getByText('You are leaving ThunderID')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'Continue'}));
    expect(openSpy).toHaveBeenCalledWith(
      'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
      '_blank',
      'noopener,noreferrer',
    );

    openSpy.mockRestore();
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

    fireEvent.click(screen.getByRole('button', {name: 'Edit in Flows'}));
    expect(onGoToFlows).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', {name: 'Edit in Customization'}));
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
    expect(screen.getByText('Useful Endpoints')).toBeInTheDocument();
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

    it('shows one quickstart guide card per platform and no StackBlitz banner', () => {
      renderWithProviders(<IntegrationGuides application={mobileApplication} oauth2Config={oauth2Config} />);

      expect(screen.queryByRole('link', {name: /Open on StackBlitz/i})).not.toBeInTheDocument();
      expect(screen.getByText('iOS quickstart guide')).toBeInTheDocument();
      expect(screen.getByText('Android quickstart guide')).toBeInTheDocument();
      expect(screen.getByText('Flutter quickstart guide')).toBeInTheDocument();
    });

    it('shows App Native flow endpoints instead of the OAuth2/OIDC endpoints', () => {
      renderWithProviders(<IntegrationGuides application={mobileApplication} oauth2Config={oauth2Config} />);

      expect(screen.getByText('Useful Endpoints')).toBeInTheDocument();
      expect(screen.queryByText('OIDC endpoints')).not.toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/execute')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/meta')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/register/passkey/start')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/register/passkey/finish')).toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/oauth2/authorize')).not.toBeInTheDocument();
      expect(screen.queryByText('https://localhost:8090/oauth2/token')).not.toBeInTheDocument();
    });

    it('shows the standard OAuth2/OIDC endpoints (not App Native ones) for a pure browser SPA', () => {
      renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

      expect(screen.getByText('Useful Endpoints')).toBeInTheDocument();
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

      expect(screen.getByText('Useful Endpoints')).toBeInTheDocument();
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

      expect(screen.getByText('Useful Endpoints')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/execute')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/flow/meta')).toBeInTheDocument();
    });

    it('renders the sign-in preview in a phone-style frame', () => {
      renderWithProviders(<IntegrationGuides application={mobileApplication} oauth2Config={oauth2Config} />);

      expect(screen.getByTestId('gate-preview')).toBeInTheDocument();
    });
  });
});
