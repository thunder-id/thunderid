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
    expect(parseQuickstartGuide(guide)?.steps.map((step) => step.title)).toEqual([
      'Install the SDK',
      'Add the Provider',
      'Run Your App',
    ]);
  });

  it('merges labeled code samples into one code group', () => {
    expect(parseQuickstartGuide(guide)?.steps[0].blocks).toEqual([
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
    expect(parseQuickstartGuide(guide)?.steps[1].blocks).toEqual([
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
    expect(parseQuickstartGuide(guide)?.steps[2].blocks).toEqual([
      {type: 'paragraph', text: 'Start the development server.'},
    ]);
  });

  it('treats a plain quote and an empty alert as untitled notes', () => {
    const steps = parseQuickstartGuide(
      ['## Create an Application', '', 'Create it.', '', '## Notes', '', '> A plain quote.', '', '> [!TIP]'].join('\n'),
    );

    expect(steps?.steps[0].blocks).toEqual([
      {type: 'alert', kind: 'note', title: undefined, blocks: [{type: 'paragraph', text: 'A plain quote.'}]},
      {type: 'alert', kind: 'tip', title: undefined, blocks: []},
    ]);
  });

  it('keeps a table row with no divider under it as text', () => {
    const steps = parseQuickstartGuide('## Create an Application\n\nCreate it.\n\n## Notes\n\n| not a table |');

    expect(steps?.steps[0].blocks).toEqual([{type: 'paragraph', text: '| not a table |'}]);
  });

  it('returns null for a page without the application creation section', () => {
    expect(parseQuickstartGuide('# Guide\n\n## Install\n\nInstall it.')).toBeNull();
    expect(parseQuickstartGuide('<!doctype html><html></html>')).toBeNull();
  });

  it('returns null when nothing follows the application creation section', () => {
    expect(parseQuickstartGuide("## Create an Application\n\nCreate it.\n\n## What's Next\n\n- Done")).toBeNull();
  });

  it('takes the steps from a named section, by its third-level headings, when one is given', () => {
    const page = [
      '## Create the Server',
      '',
      'Build it.',
      '',
      '## Connect an MCP Client',
      '',
      'Two ways to connect.',
      '',
      '**MCP Inspector**',
      '',
      '### Register an MCP Client',
      '',
      'Add the application.',
      '',
      '```bash',
      '### not a heading',
      'npx inspector',
      '```',
      '',
      '**Claude Code**',
      '',
      '### Connect Claude Code',
      '',
      'Run `/mcp`.',
      '',
      "## What's Next",
      '',
      '- Done',
    ].join('\n');

    const content = parseQuickstartGuide(page, 'connect-an-mcp-client');

    expect(content?.steps.map((step) => step.title)).toEqual(['Register an MCP Client', 'Connect Claude Code']);
    expect(content?.steps[0].blocks).toEqual([
      {type: 'paragraph', text: 'Add the application.'},
      {type: 'code', language: 'bash', title: undefined, code: '### not a heading\nnpx inspector'},
    ]);
    expect(content?.sample).toBeNull();
    expect(parseQuickstartGuide(page, 'no-such-section')).toBeNull();
  });

  it('has no sample when the page has no sample section', () => {
    expect(parseQuickstartGuide(guide)?.sample).toBeNull();
  });

  it('keeps the sample section apart from the steps, identified by its source in the samples repository', () => {
    const content = parseQuickstartGuide(
      [
        '## Create an Application',
        '',
        'Create it.',
        '',
        '## Install',
        '',
        'Install it.',
        '',
        '## Run the Sample App',
        '',
        'The [React sample](https://github.com/thunder-id/samples/tree/main/quickstarts/react) runs instead.',
        '',
        '```dotenv title=".env"',
        'CLIENT_ID=<your-client-id>',
        '```',
        '',
        "## What's Next",
        '',
        '- Done',
      ].join('\n'),
    );

    expect(content?.steps.map((step) => step.title)).toEqual(['Install']);
    expect(content?.sample?.id).toBe('quickstarts/react');
    expect(content?.sample?.blocks).toEqual([
      {
        type: 'paragraph',
        text: 'The [React sample](https://github.com/thunder-id/samples/tree/main/quickstarts/react) runs instead.',
      },
      {type: 'code', language: 'dotenv', title: '.env', code: 'CLIENT_ID=<your-client-id>'},
    ]);
  });

  it('has no sample when the sample section does not link its source', () => {
    const content = parseQuickstartGuide(
      [
        '## Create an Application',
        '',
        'Create it.',
        '',
        '## Install',
        '',
        'Install it.',
        '',
        '## Run the Sample App',
        '',
        'Download it.',
      ].join('\n'),
    );

    expect(content?.steps.map((step) => step.title)).toEqual(['Install']);
    expect(content?.sample).toBeNull();
  });
});

describe('parseQuickstartGuide for a page written for several choices', () => {
  const agentGuide = `# LangChain Quickstart

## Register an Agent

Add the agent in the Console.

## Create the Project

<!-- variant lang=python -->

\`\`\`bash
pip install langchain
\`\`\`

<!-- /variant -->

<!-- variant lang=typescript -->

\`\`\`bash
npm install langchain
\`\`\`

<!-- /variant -->

## Build the Agent

<!-- variant mode=own -->

The tool gets an agent token.

<!-- variant lang=python -->

\`\`\`python title="agent_own.py"
# <!-- variant mode=obo -->
token = get_agent_token()
\`\`\`

<!-- /variant -->

<!-- variant lang=typescript -->

\`\`\`ts title="agent_own.ts"
const token = await getAgentToken();
\`\`\`

<!-- /variant -->

<!-- /variant -->

<!-- variant mode=obo -->

The tool signs the user in.

<!-- /variant -->

## What's Next

- More guides
`;

  it('starts after the agent is registered', () => {
    const steps = parseQuickstartGuide(agentGuide)?.steps;

    expect(steps?.map((step) => step.title)).toEqual(['Create the Project', 'Build the Agent']);
  });

  it('keeps the content of the choices named and drops the rest, also where choices nest', () => {
    const steps = parseQuickstartGuide(agentGuide, undefined, {mode: 'own', lang: 'python'})?.steps;

    expect(steps?.[0].blocks).toEqual([
      {type: 'code', language: 'bash', title: undefined, code: 'pip install langchain'},
    ]);
    expect(steps?.[1].blocks).toEqual([
      {type: 'paragraph', text: 'The tool gets an agent token.'},
      {
        type: 'code',
        language: 'python',
        title: 'agent_own.py',
        code: '# <!-- variant mode=obo -->\ntoken = get_agent_token()',
      },
    ]);
  });

  it('keeps every choice of a kind that is not named, without its markers', () => {
    const steps = parseQuickstartGuide(agentGuide, undefined, {mode: 'obo'})?.steps;

    expect(steps?.[0].blocks).toEqual([
      {type: 'code', language: 'bash', title: undefined, code: 'pip install langchain'},
      {type: 'code', language: 'bash', title: undefined, code: 'npm install langchain'},
    ]);
    expect(steps?.[1].blocks).toEqual([{type: 'paragraph', text: 'The tool signs the user in.'}]);
  });

  it('returns null when choices are named and the page marks none', () => {
    const unmarked = '## Register an Agent\n\nAdd it.\n\n## Build the Agent\n\nBuild it.';

    expect(parseQuickstartGuide(unmarked, undefined, {mode: 'own'})).toBeNull();
    expect(parseQuickstartGuide(unmarked)?.steps).toHaveLength(1);
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
