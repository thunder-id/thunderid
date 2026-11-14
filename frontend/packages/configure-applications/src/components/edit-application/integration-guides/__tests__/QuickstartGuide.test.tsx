// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import QuickstartGuide from '../QuickstartGuide';

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

1. Open \`main.js\`.
2. Register the plugin.
`;

const guides = [
  {label: 'React', docsUrl: 'https://example.com/docs/react/'},
  {label: 'Vue', docsUrl: 'https://example.com/docs/vue/'},
];

const pages: Record<string, string> = {
  'https://example.com/docs/react.md': reactGuide,
  'https://example.com/docs/vue.md': vueGuide,
};

const mockFetch = vi.fn();
const mockWriteText = vi.fn();
const onOpenLink = vi.fn();

const renderGuide = (sources = guides) =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <QuickstartGuide guides={sources} codeValues={{'<your-client-id>': 'client-123'}} onOpenLink={onOpenLink} />
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
    vi.unstubAllGlobals();
    Object.defineProperty(navigator, 'clipboard', {value: originalClipboard, writable: true, configurable: true});
  });

  it('opens the first step and moves through the steps', async () => {
    renderGuide([guides[0]]);

    expect(await screen.findByText('npm install @example/react')).toBeInTheDocument();
    expect(screen.getByText('React quickstart')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Back'})).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'Next step'}));

    expect(await screen.findByLabelText('src/main.jsx')).toHaveTextContent('<Provider clientId="client-123" />');
    expect(screen.getByRole('alert')).toHaveTextContent('Configuration');
    expect(screen.getByRole('columnheader', {name: 'Parameter'})).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: /Run Your App/}));

    expect(await screen.findByText('npm run dev')).toBeInTheDocument();
    // The step that was open collapses out before it unmounts.
    await waitFor(() => {
      expect(screen.queryByRole('button', {name: 'Next step'})).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', {name: 'Back'}));

    expect(await screen.findByLabelText('src/main.jsx')).toBeInTheDocument();
  });

  it('keeps the picked package manager across code groups', async () => {
    renderGuide([guides[0]]);

    fireEvent.click(await screen.findByRole('tab', {name: 'pnpm'}));
    expect(screen.getByText('pnpm add @example/react')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: /Run Your App/}));

    expect(await screen.findByText('pnpm run dev')).toBeInTheDocument();
  });

  it('copies the code with the application values filled in', async () => {
    renderGuide([guides[0]]);

    fireEvent.click(await screen.findByRole('button', {name: /Add the Provider/}));
    const provider = (await screen.findByLabelText('src/main.jsx')).parentElement!;
    fireEvent.click(within(provider).getByRole('button', {name: 'Copy'}));

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalledWith('<Provider clientId="client-123" />');
    });
  });

  it('opens docs links as pages rather than their Markdown export', async () => {
    renderGuide([guides[0]]);

    fireEvent.click(await screen.findByRole('button', {name: 'reference'}));
    expect(onOpenLink).toHaveBeenCalledWith('https://example.com/docs/reference');

    fireEvent.click(screen.getByRole('button', {name: 'Open in docs'}));
    expect(onOpenLink).toHaveBeenCalledWith('https://example.com/docs/react/');
  });

  it('switches between platform guides', async () => {
    renderGuide();

    expect(await screen.findByRole('button', {name: /Install the SDK/})).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'Vue'}));

    expect(await screen.findByRole('button', {name: /Register the Plugin/})).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')).toHaveLength(2);
    expect(screen.queryByRole('button', {name: /Install the SDK/})).not.toBeInTheDocument();
  });

  it('shows an error with a retry when the guide cannot be fetched', async () => {
    renderGuide([{label: 'Nuxt', docsUrl: 'https://example.com/docs/nuxt/'}]);

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Open in docs'})).toBeInTheDocument();

    pages['https://example.com/docs/nuxt.md'] = vueGuide;
    fireEvent.click(screen.getByRole('button', {name: /Refresh/i}));

    expect(await screen.findByRole('button', {name: /Register the Plugin/})).toBeInTheDocument();
    delete pages['https://example.com/docs/nuxt.md'];
  });
});
