// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import parseQuickstartGuide, {fillQuickstartGuideCode} from '../parseQuickstartGuide';

const guide = `# React Quickstart

## Prerequisites

- Node.js installed on your system

## Run the Server

\`\`\`bash
npx example
\`\`\`

## Create an Application

1. Sign in to the Console.

## Install the SDK

Install the SDK in your project:

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
> Replace \`<your-client-id>\` with the Client ID.
>
> \`\`\`bash
> openssl rand -base64 32
> \`\`\`

\`\`\`jsx title="src/main.jsx" {3,9-12} showLineNumbers
import App from './App.jsx'

## Not a heading
<Provider clientId="<your-client-id>" />
\`\`\`

### Configuration Parameters

| Parameter | Description |
|-----------|-------------|
| \`clientId\` | The Client ID |

1. Open the app.

    > [!NOTE]
    > Nested note.

2. Sign in.

- First
- Second

## Run Your App

Start the
development server.

<ProductionOptions
  title="Leaked component"
>

    Text the export left behind.

## What's Next

- [Reference](https://example.com/docs/reference.md), the full reference.
`;

describe('parseQuickstartGuide', () => {
  it('returns the steps after the application is created, up to the closing links', () => {
    expect(parseQuickstartGuide(guide)?.map((step) => step.title)).toEqual([
      'Install the SDK',
      'Add the Provider',
      'Run Your App',
    ]);
  });

  it('merges labeled code samples into one code group', () => {
    expect(parseQuickstartGuide(guide)?.[0].blocks).toEqual([
      {type: 'paragraph', text: 'Install the SDK in your project:'},
      {
        type: 'codeGroup',
        options: [
          {label: 'npm', language: 'bash', code: 'npm install @example/react'},
          {label: 'pnpm', language: 'bash', code: 'pnpm add @example/react'},
        ],
      },
    ]);
  });

  it('parses alerts, titled code, headings, tables, and lists', () => {
    expect(parseQuickstartGuide(guide)?.[1].blocks).toEqual([
      {
        type: 'alert',
        kind: 'warning',
        title: 'Configuration',
        blocks: [
          {type: 'paragraph', text: 'Replace `<your-client-id>` with the Client ID.'},
          {type: 'code', language: 'bash', title: undefined, code: 'openssl rand -base64 32'},
        ],
      },
      {
        type: 'code',
        language: 'jsx',
        title: 'src/main.jsx',
        code: 'import App from \'./App.jsx\'\n\n## Not a heading\n<Provider clientId="<your-client-id>" />',
      },
      {type: 'heading', text: 'Configuration Parameters'},
      {type: 'table', header: ['Parameter', 'Description'], rows: [['`clientId`', 'The Client ID']]},
      {
        type: 'list',
        ordered: true,
        items: [
          [
            {type: 'paragraph', text: 'Open the app.'},
            {type: 'alert', kind: 'note', title: undefined, blocks: [{type: 'paragraph', text: 'Nested note.'}]},
          ],
          [{type: 'paragraph', text: 'Sign in.'}],
        ],
      },
      {
        type: 'list',
        ordered: false,
        items: [[{type: 'paragraph', text: 'First'}], [{type: 'paragraph', text: 'Second'}]],
      },
    ]);
  });

  it('joins wrapped paragraph lines and drops a component the export left as raw JSX', () => {
    expect(parseQuickstartGuide(guide)?.[2].blocks).toEqual([
      {type: 'paragraph', text: 'Start the development server.'},
    ]);
  });

  it('returns null for a page without the application creation section', () => {
    expect(parseQuickstartGuide('# Guide\n\n## Install\n\nInstall it.')).toBeNull();
    expect(parseQuickstartGuide('<!doctype html><html></html>')).toBeNull();
  });

  it('returns null when nothing follows the application creation section', () => {
    expect(parseQuickstartGuide("## Create an Application\n\nCreate it.\n\n## What's Next\n\n- Done")).toBeNull();
  });
});

describe('fillQuickstartGuideCode', () => {
  it('replaces placeholders that have a value and marks the replaced parts', () => {
    expect(
      fillQuickstartGuideCode('id="<your-client-id>" url="https://localhost:8090" secret="<your-secret>"', {
        '<your-client-id>': 'client-123',
        'https://localhost:8090': 'https://id.example.com',
        '<your-secret>': undefined,
      }),
    ).toEqual([
      {text: 'id="', filled: false},
      {text: 'client-123', filled: true},
      {text: '" url="', filled: false},
      {text: 'https://id.example.com', filled: true},
      {text: '" secret="<your-secret>"', filled: false},
    ]);
  });

  it('returns the code untouched when no placeholder has a value', () => {
    expect(fillQuickstartGuideCode('const a = 1', {'<your-client-id>': undefined})).toEqual([
      {text: 'const a = 1', filled: false},
    ]);
  });
});
