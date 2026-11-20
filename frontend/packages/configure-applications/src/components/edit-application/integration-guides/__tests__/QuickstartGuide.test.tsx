// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import QuickstartGuide from '../QuickstartGuide';

// No releases feed by default, so the sample view shows the docs instructions without a download.
const mockDocumentation: {ecosystemReleasesUrl?: string} = {};

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({config: {documentation: mockDocumentation}}),
}));

const reactGuide = `## Create an Application

Create it.

## Install the SDK

Install the \`@example/react\` package. See the [reference](https://example.com/docs/reference.md).

**npm**

\`\`\`bash
npm install @example/react
\`\`\`

**pnpm**

\`\`\`bash
pnpm add @example/react
\`\`\`

## Add the Provider

> [!WARNING]
> **Configuration**
>
> Use your **Client ID**.

\`\`\`jsx title="src/main.jsx"
<Provider clientId="<your-client-id>" />
\`\`\`

| Parameter | Description |
|-----------|-------------|
| \`clientId\` | The Client ID |

## Run Your App

**npm**

\`\`\`bash
npm run dev
\`\`\`

**pnpm**

\`\`\`bash
pnpm run dev
\`\`\`
`;

const vueGuide = `## Create an Application

Create it.

## Register the Plugin

### Before you start

- Open \`main.js\`.
- Register the plugin.

1. Save the file.

\`\`\`
app.use(plugin)
\`\`\`
`;

const reactGuideWithSample = `${reactGuide}
## Run the Sample App

The [React sample](https://github.com/example/samples/tree/main/quickstarts/react) runs instead.

\`\`\`dotenv title=".env"
CLIENT_ID=<your-client-id>
\`\`\`
`;

const serviceGuide = `## Create an Application

Create it.

## Authenticate as the Service

> [!NOTE]
> **Client ID**
>
> Replace \`<your-client-id>\` with your Client ID.

\`\`\`js title="index.mjs"
init({clientId: '<your-client-id>', clientSecret: '<your-client-secret>'})
\`\`\`

> [!WARNING]
> **Configuration**
>
> Replace \`<your-client-id>\` and \`<your-client-secret>\` with your credentials.

> [!TIP]
> **Scopes**
>
> Ask only for the scopes you use.
`;

const alertsGuide = `## Create an Application

Create it.

## Authenticate as the Service

> [!NOTE]
> **Lifecycle**
>
> Call the \`constructor\` only once.

> [!WARNING]
> **Credentials**
>
> Replace \`<your-client-id>\` and the values below.
>
> | Placeholder | Value |
> | --- | --- |
> | \`<your-client-secret>\` | Your client secret |
`;

const reactSource = {label: 'React', docsUrl: 'https://example.com/docs/react/'};
const serviceSource = {label: 'Node.js', docsUrl: 'https://example.com/docs/service/'};
const alertsSource = {label: 'Express', docsUrl: 'https://example.com/docs/alerts/'};
const vueSource = {label: 'Vue', docsUrl: 'https://example.com/docs/vue/'};

const pages: Record<string, string> = {
  'https://example.com/docs/react.md': reactGuide,
  'https://example.com/docs/vue.md': vueGuide,
  'https://example.com/docs/react-sample.md': reactGuideWithSample,
  'https://example.com/docs/service.md': serviceGuide,
  'https://example.com/docs/alerts.md': alertsGuide,
};

const mockFetch = vi.fn();
const mockWriteText = vi.fn();
const onOpenLink = vi.fn();

const renderGuide = (source = reactSource, view: 'steps' | 'sample' = 'steps') =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <QuickstartGuide
        guide={source}
        codeValues={{'<your-client-id>': 'client-123'}}
        onOpenLink={onOpenLink}
        view={view}
      />
    </QueryClientProvider>,
  );

describe('QuickstartGuide', () => {
  const originalClipboard = navigator.clipboard;

  beforeEach(() => {
    onOpenLink.mockReset();
    mockWriteText.mockReset().mockResolvedValue(undefined);
    mockFetch
      .mockReset()
      .mockImplementation((url: string) =>
        Promise.resolve({ok: url in pages, status: url in pages ? 200 : 404, text: () => Promise.resolve(pages[url])}),
      );
    vi.stubGlobal('fetch', mockFetch);
    Object.defineProperty(navigator, 'clipboard', {
      value: {writeText: mockWriteText},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    delete mockDocumentation.ecosystemReleasesUrl;
    vi.unstubAllGlobals();
    Object.defineProperty(navigator, 'clipboard', {value: originalClipboard, writable: true, configurable: true});
  });

  it('shows every step at once, numbered, with the application values filled in', async () => {
    renderGuide();

    // Highlighted code is split into tokens, so each sample is read whole, by its label.
    await waitFor(() => expect(screen.getAllByLabelText('npm')[0]).toHaveTextContent('npm install @example/react'));
    const steps = within(screen.getByRole('list')).getAllByRole('heading', {level: 3});
    expect(steps.map((step) => step.textContent)).toEqual(['Install the SDK', 'Add the Provider', 'Run Your App']);
    expect(screen.getByLabelText('src/main.jsx')).toHaveTextContent('<Provider clientId="client-123" />');
    expect(screen.getByRole('alert')).toHaveTextContent('Configuration');
    expect(screen.getByRole('columnheader', {name: 'Parameter'})).toBeInTheDocument();
    expect(screen.getAllByLabelText('npm')[1]).toHaveTextContent('npm run dev');
  });

  it('keeps the picked package manager across code groups', async () => {
    renderGuide();

    fireEvent.click((await screen.findAllByRole('tab', {name: 'pnpm'}))[0]);

    expect(screen.getAllByLabelText('pnpm')[0]).toHaveTextContent('pnpm add @example/react');
    expect(screen.getAllByLabelText('pnpm')[1]).toHaveTextContent('pnpm run dev');
  });

  it('copies the code with the application values filled in', async () => {
    renderGuide();

    const provider = (await screen.findByLabelText('src/main.jsx')).parentElement!;
    fireEvent.click(within(provider).getByRole('button', {name: 'Copy'}));

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalledWith('<Provider clientId="client-123" />');
    });
  });

  it('leaves out an alert whose placeholders are all filled in', async () => {
    renderGuide(serviceSource);

    expect(await screen.findByLabelText('index.mjs')).toHaveTextContent("clientId: 'client-123'");
    const alertTitles = screen
      .getAllByRole('alert')
      .map((alert) => alert.querySelector('.MuiAlertTitle-root')?.textContent);
    expect(alertTitles).toEqual(['Configuration', 'Scopes']);
  });

  it('marks the filled-in placeholders of an alert that still asks for others', async () => {
    renderGuide(serviceSource);

    await screen.findByLabelText('index.mjs');
    const configuration = screen.getAllByRole('alert')[0];
    expect(within(configuration).getAllByLabelText('Filled in')).toHaveLength(1);
    expect(within(configuration).getByText('<your-client-id>')).toHaveStyle({textDecoration: 'line-through'});
    expect(within(configuration).getByText('<your-client-secret>')).not.toHaveStyle({textDecoration: 'line-through'});
  });

  it('keeps every alert when no placeholder is filled in', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <QuickstartGuide guide={serviceSource} codeValues={{'<your-client-id>': undefined}} onOpenLink={onOpenLink} />
      </QueryClientProvider>,
    );

    await screen.findByLabelText('index.mjs');
    expect(screen.getAllByRole('alert')).toHaveLength(3);
    expect(screen.queryByLabelText('Filled in')).not.toBeInTheDocument();
  });

  it('keeps an alert whose code spans only match names inherited from Object', async () => {
    renderGuide(alertsSource);

    const lifecycle = await screen.findByText('Lifecycle');
    expect(within(lifecycle.closest('[role="alert"]')!).queryByLabelText('Filled in')).not.toBeInTheDocument();
  });

  it('keeps an alert whose table names a placeholder that is not filled in', async () => {
    renderGuide(alertsSource);

    const credentials = (await screen.findByText('Credentials')).closest('[role="alert"]')!;
    expect(within(credentials).getByRole('table')).toBeInTheDocument();
    expect(within(credentials).getAllByLabelText('Filled in')).toHaveLength(1);
  });

  it('opens docs links as pages rather than their Markdown export', async () => {
    renderGuide();

    fireEvent.click(await screen.findByRole('button', {name: 'reference'}));
    expect(onOpenLink).toHaveBeenCalledWith('https://example.com/docs/reference');
  });

  it('renders nested headings, lists, and untitled code', async () => {
    renderGuide(vueSource);

    expect(await screen.findByRole('heading', {name: 'Register the Plugin'})).toBeInTheDocument();
    expect(screen.getByRole('heading', {name: 'Before you start'})).toBeInTheDocument();
    expect(screen.getByLabelText('Code sample')).toHaveTextContent('app.use(plugin)');
  });

  it('keeps the sample section out of the steps and shows it as the sample view', async () => {
    const source = {label: 'React', docsUrl: 'https://example.com/docs/react-sample/'};
    const {unmount} = renderGuide(source);

    expect(await screen.findByRole('heading', {name: 'Run Your App'})).toBeInTheDocument();
    expect(screen.queryByText('Run the Sample App')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('.env')).not.toBeInTheDocument();
    unmount();

    renderGuide(source, 'sample');

    expect(await screen.findByLabelText('.env')).toHaveTextContent('CLIENT_ID=client-123');
    expect(screen.queryByRole('heading', {name: 'Install the SDK'})).not.toBeInTheDocument();
  });

  it('shows the steps when no view is asked for', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <QuickstartGuide guide={reactSource} codeValues={{}} onOpenLink={onOpenLink} />
      </QueryClientProvider>,
    );

    expect(await screen.findByRole('heading', {name: 'Install the SDK'})).toBeInTheDocument();
  });

  it("shows the sample's version alone when the releases feed gives no size", async () => {
    const releasesUrl = 'https://example.com/data/ecosystem-releases.json';
    mockDocumentation.ecosystemReleasesUrl = releasesUrl;
    pages[releasesUrl] = JSON.stringify({
      releases: [
        {
          type: 'sample',
          packageId: 'quickstarts/react',
          latestVersion: '1.2.0',
          assets: [{downloadUrl: 'https://example.com/react.zip'}],
        },
      ],
    });
    mockFetch.mockImplementation((url: string) =>
      Promise.resolve({
        ok: url in pages,
        status: url in pages ? 200 : 404,
        text: () => Promise.resolve(pages[url]),
        json: () => Promise.resolve(JSON.parse(pages[url]) as unknown),
      }),
    );

    renderGuide({label: 'React', docsUrl: 'https://example.com/docs/react-sample/'}, 'sample');

    expect(await screen.findByText('Version 1.2.0')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Download sample'})).toHaveAttribute(
      'href',
      'https://example.com/react.zip',
    );
    delete pages[releasesUrl];
  });

  it('looks for the sample download again when it could not be found', async () => {
    const releasesUrl = 'https://example.com/data/missing-releases.json';
    mockDocumentation.ecosystemReleasesUrl = releasesUrl;

    renderGuide({label: 'React', docsUrl: 'https://example.com/docs/react-sample/'}, 'sample');

    expect(
      await screen.findByText("The sample's download could not be found. You can still download it from its releases."),
    ).toBeInTheDocument();
    const fetchesBefore = mockFetch.mock.calls.filter(([url]) => url === releasesUrl).length;

    fireEvent.click(screen.getByRole('button', {name: /Refresh/i}));

    await waitFor(() =>
      expect(mockFetch.mock.calls.filter(([url]) => url === releasesUrl).length).toBe(fetchesBefore + 1),
    );
  });

  it('shows an error with a retry when the guide cannot be fetched', async () => {
    renderGuide({label: 'Nuxt', docsUrl: 'https://example.com/docs/nuxt/'});

    expect(await screen.findByRole('alert')).toBeInTheDocument();

    pages['https://example.com/docs/nuxt.md'] = vueGuide;
    fireEvent.click(screen.getByRole('button', {name: /Refresh/i}));

    expect(await screen.findByRole('heading', {name: 'Register the Plugin'})).toBeInTheDocument();
    delete pages['https://example.com/docs/nuxt.md'];
  });
});
