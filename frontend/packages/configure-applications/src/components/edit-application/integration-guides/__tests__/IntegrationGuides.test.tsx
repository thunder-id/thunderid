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
const mockConfig: {
  brand: {product_name: string};
  direct_api?: {enabled?: boolean};
  documentation?: {ecosystemReleasesUrl?: string};
} = {
  brand: {product_name: 'ThunderID'},
  documentation: {ecosystemReleasesUrl: 'https://thunderid.dev/data/ecosystem-releases.json'},
};

const documentationLinks: Record<string, string> = {
  'applications.templates.react.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
  'applications.templates.react.playground':
    'https://stackblitz.com/fork/github/thunder-id/samples/tree/main/quickstarts/react?file=README.md',
  'applications.templates.react.llmPrompt.redirectBased': 'https://thunderid.dev/prompts/react/redirect-based.txt',
  'applications.templates.nextjs.llmPrompt.redirectBased': 'https://thunderid.dev/prompts/nextjs/redirect-based.txt',
  'applications.templates.browser.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/browser/',
  'applications.templates.mcpClient.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-mcp/python/',
  'applications.templates.vue.docs':
    'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/vue/',
  'applications.templates.vue.llmPrompt.redirectBased': 'https://thunderid.dev/prompts/vue/redirect-based.txt',
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
  useGetFlowById: (id?: string): unknown => ({data: id ? flowNames[id] : undefined, isLoading: false}),
}));

vi.mock('@thunderid/configure-design', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-design')>()),
  GatePreview: () => <div data-testid="gate-preview" />,
}));

const flowNames: Record<string, {name: string}> = {};
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
  'https://thunderid.dev/prompts/vue/redirect-based.txt': 'Integrate {{productName}} with Vue',
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

const reactGuideWithSample = [
  reactGuide,
  '',
  '## Run the Sample App',
  '',
  'The [React sample](https://github.com/thunder-id/samples/tree/main/quickstarts/react) is a working version of the app this guide builds.',
  '',
  "1. Create `.env` in the sample's folder with your application's values:",
  '',
  '   ```dotenv title=".env"',
  '   VITE_THUNDERID_BASE_URL=https://localhost:8090',
  '   VITE_THUNDERID_CLIENT_ID=<your-client-id>',
  '   ```',
  '',
  "## What's Next",
  '',
  '- [Docs](https://thunderid.dev/docs)',
].join('\n');

const ecosystemReleases = JSON.stringify({
  releases: [
    {
      type: 'sample',
      packageId: 'quickstarts/react',
      latestVersion: '1.0.0',
      assets: [
        {
          downloadUrl: 'https://github.com/thunder-id/samples/releases/download/react.zip',
          sizeLabel: '48 KB',
        },
      ],
    },
  ],
});

const renderWithProviders = (component: React.ReactElement) =>
  render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient()}>
        <LoggerProvider logger={{level: LogLevel.DEBUG}}>{component}</LoggerProvider>
      </QueryClientProvider>
    </MemoryRouter>,
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
        json: () => Promise.resolve(JSON.parse(promptFixtures[url] ?? '{}') as unknown),
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

  it('keeps the application details at a readable width when there is no guide beside them', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, template: 'custom', type: 'custom'}} />);

    const details = screen.getByText('Application details').closest('.MuiPaper-root')!;
    expect(details.getBoundingClientRect().width).toBeLessThanOrEqual(520);
    expect(details.getBoundingClientRect().height).toBeLessThan(520);
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
    expect(screen.getByText('Sign-in experience')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: /Open on StackBlitz/i})).toBeInTheDocument();
  });

  it('hides the OIDC endpoints and sign-in preview for a custom application with no OAuth2 config', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, type: 'custom'}} />);

    expect(screen.queryByText('OIDC endpoints')).not.toBeInTheDocument();
    expect(screen.queryByText('Sign-in experience')).not.toBeInTheDocument();
  });

  it('hides only the sign-in preview for a machine-to-machine application with no OAuth2 config yet', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, type: 'm2m'}} />);

    expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
    expect(screen.queryByText('Sign-in experience')).not.toBeInTheDocument();
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

  it('offers only the coding agent prompt when the docs link is not configured', async () => {
    const documentationLinksWithoutDocs = {...documentationLinks};
    delete documentationLinksWithoutDocs['applications.templates.react.docs'];
    mockGetDocumentationLink.mockImplementation((key: string) => documentationLinksWithoutDocs[key]);

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(screen.getByRole('link', {name: /Open on StackBlitz/i})).toBeInTheDocument();
    expect(await screen.findByRole('button', {name: /Copy prompt/i})).toBeInTheDocument();
    // With no guide there is nothing to follow by hand, so there is no choice of setup.
    expect(screen.queryByRole('button', {name: 'Manual setup'})).not.toBeInTheDocument();
    expect(screen.getByRole('heading', {name: 'Connect your app'})).toBeInTheDocument();
  });

  it('opens the docs guide in a new tab without the leaving-console confirmation', () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    const docsLink = screen.getByRole('link', {name: /Read the full guide/i});
    expect(docsLink).toHaveAttribute(
      'href',
      'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
    );
    expect(docsLink).toHaveAttribute('target', '_blank');
    expect(docsLink).toHaveAttribute('rel', 'noopener noreferrer');
    expect(screen.queryByText('You are leaving ThunderID')).not.toBeInTheDocument();
  });

  it('shows the docs guide inline under manual setup with the application values filled into the code', async () => {
    mockGetRuntimeUrl.mockReturnValue('https://gateway.example.com');
    promptFixtures[reactGuideUrl] = reactGuide;

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(screen.getByRole('heading', {name: 'Connect your React app'})).toBeInTheDocument();
    // Agent setup leads, since handing the prompt to a coding agent is the quickest way in.
    expect(screen.getByRole('button', {name: 'Agent setup'})).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', {name: 'Manual setup'}));

    expect(await screen.findByRole('heading', {name: 'Add the Provider'})).toBeInTheDocument();
    expect(screen.getByRole('heading', {name: 'Run Your App'})).toBeInTheDocument();
    // Creating the application is already done by the time someone reads its Overview tab.
    expect(screen.queryByText('Sign in to the Console.')).not.toBeInTheDocument();
    expect(screen.getByLabelText('src/main.jsx')).toHaveTextContent(
      '<ThunderIDProvider clientId="client-123" baseUrl="https://gateway.example.com" />',
    );
    // A guide with no sample section offers no sample.
    expect(screen.queryByRole('radio', {name: /Use our sample/})).not.toBeInTheDocument();

    delete promptFixtures[reactGuideUrl];
  });

  it('offers the sample app from the docs guide, with its archive and the application values', async () => {
    promptFixtures[reactGuideUrl] = reactGuideWithSample;
    promptFixtures['https://thunderid.dev/data/ecosystem-releases.json'] = ecosystemReleases;

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: 'Manual setup'}));
    fireEvent.click(await screen.findByRole('radio', {name: /Use our sample/}));

    expect(await screen.findByText('React sample app')).toBeInTheDocument();
    expect(await screen.findByText('Version 1.0.0, 48 KB')).toBeInTheDocument();
    expect(screen.getByLabelText('.env')).toHaveTextContent('VITE_THUNDERID_CLIENT_ID=client-123');
    // The sample is an alternative to the steps, not one of them.
    expect(screen.queryByRole('heading', {name: 'Add the Provider'})).not.toBeInTheDocument();

    // The download is the product's own sample, so it skips the leaving-console confirmation.
    expect(screen.getByRole('link', {name: 'Download sample'})).toHaveAttribute(
      'href',
      'https://github.com/thunder-id/samples/releases/download/react.zip',
    );
    delete promptFixtures[reactGuideUrl];
    delete promptFixtures['https://thunderid.dev/data/ecosystem-releases.json'];
  });

  it('keeps the link to the docs site when the page is not a quickstart it can show inline', async () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: 'Manual setup'}));

    expect(await screen.findByText('Follow the step-by-step guide on the docs site.')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: /Read the full guide/i})).toBeInTheDocument();
  });

  it('shows the coding agent prompt filled in, and expands it', async () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    const prompt = await screen.findByLabelText('React prompt');
    expect(prompt).toHaveTextContent('Integrate ThunderID at https://localhost:8090 with clientId: client-123');

    fireEvent.click(screen.getByRole('button', {name: 'Expand prompt'}));
    expect(screen.getByRole('button', {name: 'Collapse prompt'})).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Collapse prompt'}));
    expect(screen.getByRole('button', {name: 'Expand prompt'})).toBeInTheDocument();
  });

  it('says when the coding agent prompt cannot be loaded', async () => {
    mockFetch.mockImplementation(() => Promise.resolve({ok: false, status: 500, text: () => Promise.resolve('')}));

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(await screen.findByText('The prompt could not be loaded.')).toBeInTheDocument();

    mockFetch.mockImplementation((url: string) =>
      Promise.resolve({ok: true, status: 200, text: () => Promise.resolve(promptFixtures[url] ?? '')}),
    );
    fireEvent.click(screen.getByRole('button', {name: /Refresh/i}));

    expect(await screen.findByLabelText('React prompt')).toHaveTextContent('with clientId: client-123');
  });

  it('leaves a prompt slot as written when the application has no value for it', async () => {
    renderWithProviders(
      <IntegrationGuides application={reactApplication} oauth2Config={{...oauth2Config, clientId: ''}} />,
    );

    expect(await screen.findByLabelText('React prompt')).toHaveTextContent('with clientId: {{clientId}}');
  });

  it('keeps the setup chosen when it is chosen again, and goes back from the sample to the steps', async () => {
    promptFixtures[reactGuideUrl] = reactGuideWithSample;

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: 'Agent setup'}));
    expect(screen.getByRole('button', {name: 'Agent setup'})).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(screen.getByRole('button', {name: 'Manual setup'}));
    fireEvent.click(await screen.findByRole('radio', {name: /Use our sample/}));
    expect(await screen.findByLabelText('.env')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('radio', {name: /Follow the steps/}));
    expect(await screen.findByRole('heading', {name: 'Add the Provider'})).toBeInTheDocument();
    delete promptFixtures[reactGuideUrl];
  });

  it('points to a test user or self sign-up to sign in with', () => {
    const onGoToFlows = vi.fn();
    renderWithProviders(
      <IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} onGoToFlows={onGoToFlows} />,
    );

    expect(
      screen.getByText('When the agent finishes, start your app, open http://localhost:5173, and sign in.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', {name: /Add a test user/})).toHaveAttribute('href', '/users/add');
    fireEvent.click(screen.getByRole('button', {name: /Turn on self sign-up/}));
    expect(onGoToFlows).toHaveBeenCalledTimes(1);
    // The development server is not an allowed redirect URI of this application.
    expect(screen.queryByText('http://localhost:5173 is an allowed redirect URI')).not.toBeInTheDocument();
  });

  it('says self sign-up is on, and that the development server is an allowed redirect URI', () => {
    renderWithProviders(
      <IntegrationGuides
        application={{...reactApplication, isRegistrationFlowEnabled: true}}
        oauth2Config={{...oauth2Config, redirectUris: ['http://localhost:5173']}}
        onGoToFlows={vi.fn()}
      />,
    );

    expect(screen.getByText('Self sign-up is on')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Turn on self sign-up/})).not.toBeInTheDocument();
    expect(screen.getByRole('link', {name: /Add a test user/})).toBeInTheDocument();
    expect(screen.getByText('http://localhost:5173 is an allowed redirect URI')).toBeInTheDocument();
  });

  it('folds the endpoints of a sign-in-facing application until they are asked for', () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    const toggle = screen.getByRole('button', {name: /Useful endpoints/});
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(toggle).toHaveTextContent('OpenID configuration and 4 more');

    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('https://localhost:8090/oauth2/token')).toBeVisible();
  });

  it('fills the code placeholders before the prompt slots, so a filled value is not filled again', async () => {
    mockGetRuntimeUrl.mockReturnValue('https://localhost:8090/runtime');
    const promptUrl = 'https://thunderid.dev/prompts/react/redirect-based.txt';
    const original = promptFixtures[promptUrl];
    promptFixtures[promptUrl] = 'Base URL: {{baseUrl}}';

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(await screen.findByLabelText('React prompt')).toHaveTextContent(
      /^Base URL: https:\/\/localhost:8090\/runtime$/,
    );
    promptFixtures[promptUrl] = original;
  });

  it('fills prompt values as written, special replacement characters included', async () => {
    mockConfig.brand.product_name = 'ACME $$ $&';
    const promptUrl = 'https://thunderid.dev/prompts/react/redirect-based.txt';
    const original = promptFixtures[promptUrl];
    promptFixtures[promptUrl] = 'Integrate {{productName}}';

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    expect(await screen.findByLabelText('React prompt')).toHaveTextContent('Integrate ACME $$ $&');
    promptFixtures[promptUrl] = original;
    mockConfig.brand.product_name = 'ThunderID';
  });

  it('says when the sample download cannot be found, and keeps the sample instructions', async () => {
    promptFixtures[reactGuideUrl] = reactGuideWithSample;
    const releasesUrl = 'https://thunderid.dev/data/ecosystem-releases.json';
    mockFetch.mockImplementation((url: string) =>
      Promise.resolve(
        url === releasesUrl
          ? {ok: false, status: 500, text: () => Promise.resolve(''), json: () => Promise.resolve({})}
          : {ok: true, status: 200, text: () => Promise.resolve(promptFixtures[url] ?? '')},
      ),
    );

    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(screen.getByRole('button', {name: 'Manual setup'}));
    fireEvent.click(await screen.findByRole('radio', {name: /Use our sample/}));

    expect(
      await screen.findByText("The sample's download could not be found. You can still download it from its releases."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('.env')).toHaveTextContent('VITE_THUNDERID_CLIENT_ID=client-123');
    expect(screen.queryByRole('button', {name: 'Download sample'})).not.toBeInTheDocument();
    delete promptFixtures[reactGuideUrl];
  });

  it('shows an enabled flow as set up while its name is unavailable', () => {
    renderWithProviders(
      <IntegrationGuides
        application={{...reactApplication, isRegistrationFlowEnabled: true, registrationFlowId: 'flow-1'}}
        oauth2Config={oauth2Config}
        onGoToFlows={vi.fn()}
      />,
    );

    expect(screen.queryByRole('button', {name: 'Set up sign-up'})).not.toBeInTheDocument();
    expect(screen.getByText('Configured')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Set up recovery'})).toBeInTheDocument();
  });

  it('shows a recovery flow as set up, by name once it loads, and sign-up turned on with no flow as not', () => {
    flowNames['flow-recovery'] = {name: 'Basic recovery'};
    const application: Application = {
      ...reactApplication,
      isRegistrationFlowEnabled: true,
      isRecoveryFlowEnabled: true,
      recoveryFlowId: 'flow-recovery',
    };

    const {unmount} = renderWithProviders(
      <IntegrationGuides application={application} oauth2Config={oauth2Config} onGoToFlows={vi.fn()} />,
    );

    // Sign-up is turned on but has no flow, so it is not set up yet.
    expect(screen.getByRole('button', {name: 'Set up sign-up'})).toBeInTheDocument();
    expect(screen.getByText('Basic recovery')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Set up recovery'})).not.toBeInTheDocument();
    unmount();

    // A set-up recovery flow whose name has not loaded is still set up.
    delete flowNames['flow-recovery'];
    renderWithProviders(
      <IntegrationGuides application={application} oauth2Config={oauth2Config} onGoToFlows={vi.fn()} />,
    );

    expect(screen.getByText('Configured')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Set up recovery'})).not.toBeInTheDocument();
  });

  it('says the assigned theme is loading rather than showing the default theme', () => {
    renderWithProviders(<IntegrationGuides application={{...reactApplication, themeId: 'theme-1'}} />);

    expect(screen.getByText('Loading the theme')).toBeInTheDocument();
    expect(screen.queryByText('Default')).not.toBeInTheDocument();
  });

  it('leads with agent setup when the prompt arrives after the guide first shows', () => {
    const {rerender} = renderWithProviders(
      <IntegrationGuides application={{...reactApplication, template: undefined}} oauth2Config={oauth2Config} />,
    );

    rerender(
      <MemoryRouter>
        <QueryClientProvider client={new QueryClient()}>
          <LoggerProvider logger={{level: LogLevel.DEBUG}}>
            <IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />
          </LoggerProvider>
        </QueryClientProvider>
      </MemoryRouter>,
    );

    expect(screen.getByRole('button', {name: 'Agent setup'})).toHaveAttribute('aria-pressed', 'true');
  });

  it('offers no sample from outside the section a guide is limited to', async () => {
    const mcpGuideUrl = 'https://thunderid.dev/docs/next/guides/getting-started/connect-your-mcp/python.md';
    promptFixtures[mcpGuideUrl] = [
      '## Create an Application',
      '',
      'Create it.',
      '',
      '## Build the Server',
      '',
      'Build it.',
      '',
      '## Connect an MCP Client',
      '',
      '### Register the Client',
      '',
      'Register it.',
      '',
      '## Run the Sample App',
      '',
      'The [sample](https://github.com/thunder-id/samples/tree/main/quickstarts/react) runs instead.',
    ].join('\n');

    renderWithProviders(
      <IntegrationGuides
        application={{...reactApplication, template: 'mcp-client', type: 'mcp'}}
        oauth2Config={oauth2Config}
      />,
    );

    expect(await screen.findByRole('heading', {name: 'Register the Client'})).toBeInTheDocument();
    expect(screen.queryByRole('radio', {name: /Use our sample/})).not.toBeInTheDocument();
    delete promptFixtures[mcpGuideUrl];
  });

  it('says when the assigned theme cannot be loaded, rather than showing the default theme', () => {
    const refetch = vi.fn();
    mockUseGetTheme.mockReturnValue({data: undefined, isLoading: false, error: new Error('forbidden'), refetch});

    renderWithProviders(<IntegrationGuides application={{...reactApplication, themeId: 'theme-1'}} />);

    expect(screen.getByText('The theme could not be loaded.')).toBeInTheDocument();
    expect(screen.queryByText('Default')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: /Refresh/i}));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('sends a flow that is not set up to the Flows tab', () => {
    const onGoToFlows = vi.fn();
    renderWithProviders(
      <IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} onGoToFlows={onGoToFlows} />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Set up sign-up'}));
    fireEvent.click(screen.getByRole('button', {name: 'Set up recovery'}));
    expect(onGoToFlows).toHaveBeenCalledTimes(2);
  });

  it('copies the coding agent prompt with placeholders replaced', async () => {
    renderWithProviders(<IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />);

    fireEvent.click(await screen.findByRole('button', {name: /Copy prompt/i}));

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

    fireEvent.click(await screen.findByRole('button', {name: /Copy prompt/i}));

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

    expect(screen.queryByText('Sign-in experience')).not.toBeInTheDocument();
    expect(screen.getByText('Useful endpoints')).toBeInTheDocument();
  });

  it('shows the coding agent prompt for a template whose default EMBEDDED variant has no guide content', async () => {
    // Android defaults to the EMBEDDED sign-in approach (it's always app-native) but only has
    // REDIRECT_BASED guide content authored; every technology template should still get a
    // coding-agent prompt card.
    renderWithProviders(
      <IntegrationGuides
        application={{...reactApplication, template: 'android-embedded'}}
        oauth2Config={oauth2Config}
      />,
    );

    expect(await screen.findByRole('button', {name: /Copy prompt/i})).toBeInTheDocument();
  });

  it('offers each technology a generic template covers, the first chosen, with its own prompt', async () => {
    renderWithProviders(
      <IntegrationGuides application={{...reactApplication, template: 'browser'}} oauth2Config={oauth2Config} />,
    );

    expect(screen.getByRole('heading', {name: 'Connect your React app'})).toBeInTheDocument();
    expect(await screen.findByLabelText('React prompt')).toHaveTextContent('with clientId: client-123');

    // The technology in the heading opens a menu of the others the template covers.
    fireEvent.click(screen.getByRole('button', {name: 'React'}));
    const technologies = screen.getByRole('menu', {name: 'Technology'});
    expect(
      within(technologies)
        .getAllByRole('menuitemradio')
        .map((item) => item.textContent),
    ).toEqual(['React', 'Vue', 'JavaScript']);
    expect(within(technologies).getByRole('menuitemradio', {name: 'React'})).toHaveAttribute('aria-checked', 'true');

    // Closing the menu keeps the technology chosen.
    fireEvent.keyDown(technologies, {key: 'Escape'});
    await waitFor(() => expect(screen.queryByRole('menu', {name: 'Technology'})).not.toBeInTheDocument());
    expect(screen.getByRole('heading', {name: 'Connect your React app'})).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'React'}));

    fireEvent.click(within(screen.getByRole('menu', {name: 'Technology'})).getByRole('menuitemradio', {name: 'Vue'}));

    expect(screen.getByRole('heading', {name: 'Connect your Vue app'})).toBeInTheDocument();
    expect(await screen.findByLabelText('Vue prompt')).toHaveTextContent('Integrate ThunderID with Vue');
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
      expect(screen.getByRole('heading', {name: 'Connect your iOS app'})).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', {name: 'iOS'}));
      const technologies = screen.getByRole('menu', {name: 'Technology'});
      expect(within(technologies).getByRole('menuitemradio', {name: 'iOS'})).toHaveAttribute('aria-checked', 'true');
      expect(within(technologies).getByRole('menuitemradio', {name: 'Android'})).toBeInTheDocument();
      expect(within(technologies).getByRole('menuitemradio', {name: 'Flutter'})).toBeInTheDocument();

      fireEvent.click(within(technologies).getByRole('menuitemradio', {name: 'Android'}));
      expect(screen.getByRole('heading', {name: 'Connect your Android app'})).toBeInTheDocument();
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

    const chooseEnvironment = (name: string) => {
      fireEvent.mouseDown(screen.getByRole('combobox', {name: 'Environment'}));
      fireEvent.click(within(screen.getByRole('listbox')).getByRole('option', {name: new RegExp(`^${name}`)}));
    };

    beforeEach(() => {
      clientIds = {
        'env-dev': {status: 'set', value: 'client-dev'},
        'env-prod': {status: 'set', value: 'client-prod'},
      };
      getValue.mockReset().mockImplementation((environmentId: string) => Promise.resolve(clientIds[environmentId]));
    });

    it("shows a single environment's client ID and endpoints, with no environment choice", async () => {
      renderWithEnvironments([development]);

      expect(screen.queryByText('Environment')).not.toBeInTheDocument();
      expect(await screen.findByText('client-dev')).toBeInTheDocument();
      expect(screen.queryByText('client-123')).not.toBeInTheDocument();
      expect(screen.getByText('https://dev.example.com/oauth2/authorize')).toBeInTheDocument();
    });

    it('shows no client ID or endpoints where the only environment does not have the application', async () => {
      clientIds['env-dev'] = {status: 'absent'};
      renderWithEnvironments([development]);

      expect(await screen.findByRole('status')).toHaveTextContent('Not deployed to Development yet');
      expect(screen.queryByText('Environment')).not.toBeInTheDocument();
      expect(screen.queryByText('client-123')).not.toBeInTheDocument();
      expect(screen.queryByText('https://dev.example.com/oauth2/authorize')).not.toBeInTheDocument();
      expect(screen.getByText('app-123')).toBeInTheDocument();
    });

    it("shows the first environment's client ID and endpoints when there are several", async () => {
      renderWithEnvironments([development, production]);

      expect(screen.getByText('Environment')).toBeInTheDocument();
      expect(screen.getByRole('combobox', {name: 'Environment'})).toHaveTextContent('Development');
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

      chooseEnvironment('Production');

      expect(await screen.findByText('client-prod')).toBeInTheDocument();
      expect(screen.getByText('https://prod.example.com/oauth2/authorize')).toBeInTheDocument();
      expect(screen.queryByText('https://dev.example.com/oauth2/authorize')).not.toBeInTheDocument();
      expect(screen.getByRole('combobox', {name: 'Environment'})).toHaveTextContent('Production');
    });

    it("fills the integration guide and the coding agent prompt with the chosen environment's values", async () => {
      promptFixtures[reactGuideUrl] = reactGuide;
      renderWithEnvironments([development, production]);
      await waitFor(() => expect(screen.getByLabelText('Client ID')).toHaveTextContent('client-dev'));
      chooseEnvironment('Production');
      await waitFor(() => expect(screen.getByLabelText('Client ID')).toHaveTextContent('client-prod'));

      expect(
        screen.getByText('The guide, client ID, and endpoints on this tab use Production values.'),
      ).toBeInTheDocument();
      fireEvent.click(await screen.findByRole('button', {name: /Copy prompt/i}));
      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith(
          'Integrate ThunderID at https://prod.example.com with clientId: client-prod',
        );
      });

      fireEvent.click(screen.getByRole('button', {name: 'Manual setup'}));
      await screen.findByRole('heading', {name: 'Add the Provider'});
      expect(screen.getByLabelText('src/main.jsx')).toHaveTextContent(
        '<ThunderIDProvider clientId="client-prod" baseUrl="https://prod.example.com" />',
      );

      delete promptFixtures[reactGuideUrl];
    });

    it("fills the chosen environment's URL into a prompt that names a local instance", async () => {
      const promptUrl = 'https://thunderid.dev/prompts/react/redirect-based.txt';
      const original = promptFixtures[promptUrl];
      promptFixtures[promptUrl] = 'Base URL: https://localhost:8090 (or your {{productName}} instance URL)';
      renderWithEnvironments([development, production]);
      await screen.findByText('client-dev');
      chooseEnvironment('Production');
      await screen.findByText('client-prod');

      fireEvent.click(await screen.findByRole('button', {name: /Copy prompt/i}));
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
      chooseEnvironment('Production');
      await screen.findByText('Not promoted to Production yet');

      fireEvent.click(await screen.findByRole('button', {name: /Copy prompt/i}));
      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith(
          'Integrate ThunderID at https://prod.example.com with clientId: <your-client-id>',
        );
      });

      fireEvent.click(screen.getByRole('button', {name: 'Manual setup'}));
      await screen.findByRole('heading', {name: 'Add the Provider'});
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
      chooseEnvironment('Production');
      expect(await screen.findByText('Not promoted to Production yet')).toBeInTheDocument();
    });

    it('shows no client ID or endpoints for an environment the application is not in', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      renderWithEnvironments([development, production]);
      await screen.findByText('client-dev');

      chooseEnvironment('Production');

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

      chooseEnvironment('Development');

      expect(await screen.findByText('client-dev')).toBeInTheDocument();
      expect(screen.getByText('https://dev.example.com/oauth2/authorize')).toBeInTheDocument();
    });

    it('marks the environments the application is not deployed to on the picker', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      renderWithEnvironments([development, production]);
      await screen.findByText('client-dev');

      fireEvent.mouseDown(screen.getByRole('combobox', {name: 'Environment'}));
      const options = within(screen.getByRole('listbox'));

      await waitFor(() =>
        expect(options.getByRole('option', {name: /^Production/})).toHaveTextContent('Not promoted to Production yet.'),
      );
      expect(options.getByRole('option', {name: /^Development/})).not.toHaveTextContent('Not deployed');
    });

    it('offers the way to deploy where the application is not deployed to an environment yet', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      renderWithProviders(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
          <RuntimeEnvironmentsContext.Provider
            value={{environments: [development, production], getValue, deployPath: '/projects/default/pipeline'}}
          >
            <IntegrationGuides application={reactApplication} oauth2Config={oauth2Config} />
          </RuntimeEnvironmentsContext.Provider>
        </QueryClientProvider>,
      );
      await screen.findByText('client-dev');
      expect(screen.queryByRole('link', {name: 'Open deployment pipeline'})).not.toBeInTheDocument();

      chooseEnvironment('Production');

      expect(await screen.findByRole('link', {name: 'Open deployment pipeline'})).toHaveAttribute(
        'href',
        '/projects/default/pipeline',
      );
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

      expect(screen.getByRole('status')).toHaveTextContent(
        'The guide, client ID, and endpoints on this tab use Development values.',
      );
    });

    it('offers to read the client ID again when it cannot be read', async () => {
      getValue.mockRejectedValueOnce(new Error('forbidden'));
      renderWithEnvironments([development, production]);

      fireEvent.click(await screen.findByRole('button', {name: 'Refresh'}));

      expect(await screen.findByText('client-dev')).toBeInTheDocument();
    });
  });
});
