// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

window.__THUNDERID_RUNTIME_CONFIG__ = {
  brand: {
    product_name: 'ThunderID',
    favicon: {
      light: 'assets/images/favicon.ico',
      dark: 'assets/images/favicon-inverted.ico',
    },
  },
  documentation: {
    baseUrl: 'https://thunderid.dev/docs/v1.0.x',
    releasesUrl: 'https://thunderid.dev/data/releases.json',
    links: {
      users: '',
      applications: '',
      agents: '',
      design: '',
      flows: '',
      roles: '',
      groups: '',
      'verifiableCredentials.credentials': '',
      'verifiableCredentials.presentations': '',
      settings: '',
      importExport: '',
      'deployment.csp': 'deployment/configuration/#content-security-policy',
      'applications.templates.react.docs': 'getting-started/connect-your-application/react/',
      'applications.templates.react.playground': '',
      'applications.templates.react.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/react/redirect-based.txt',
      'applications.templates.react.llmPrompt.embedded':
        'getting-started/connect-your-application/prompts/react/embedded.txt',
      'applications.templates.nextjs.docs': 'getting-started/connect-your-application/nextjs/',
      'applications.templates.nextjs.playground': '',
      'applications.templates.nextjs.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/nextjs/redirect-based.txt',
      'applications.templates.nextjs.llmPrompt.embedded':
        'getting-started/connect-your-application/prompts/nextjs/embedded.txt',
      'applications.templates.nuxt.docs': 'getting-started/connect-your-application/nuxt/',
      'applications.templates.nuxt.playground': '',
      'applications.templates.nuxt.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/nuxt/redirect-based.txt',
      'applications.templates.nuxt.llmPrompt.embedded':
        'getting-started/connect-your-application/prompts/nuxt/embedded.txt',
      'applications.templates.vue.docs': 'getting-started/connect-your-application/vue/',
      'applications.templates.vue.playground': '',
      'applications.templates.vue.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/vue/redirect-based.txt',
      'applications.templates.vue.llmPrompt.embedded':
        'getting-started/connect-your-application/prompts/vue/embedded.txt',
      'applications.templates.express.docs': 'getting-started/connect-your-application/express/',
      'applications.templates.express.playground': '',
      'applications.templates.express.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/express/redirect-based.txt',
      'applications.templates.express.llmPrompt.embedded':
        'getting-started/connect-your-application/prompts/express/embedded.txt',
      'applications.templates.node.docs': 'getting-started/connect-your-application/node/',
      'applications.templates.node.playground': '',
      'applications.templates.node.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/node/redirect-based.txt',
      'applications.templates.node.llmPrompt.embedded':
        'getting-started/connect-your-application/prompts/node/embedded.txt',
      'applications.templates.browser.docs': 'getting-started/connect-your-application/browser/',
      'applications.templates.browser.playground': '',
      'applications.templates.browser.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/browser/redirect-based.txt',
      'applications.templates.browser.llmPrompt.embedded':
        'getting-started/connect-your-application/prompts/browser/embedded.txt',
      'applications.templates.android.docs': 'getting-started/connect-your-application/android/',
      'applications.templates.android.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/android/redirect-based.txt',
      'applications.templates.ios.docs': 'getting-started/connect-your-application/ios/',
      'applications.templates.ios.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/ios/redirect-based.txt',
      'applications.templates.flutter.docs': 'getting-started/connect-your-application/flutter/',
      'applications.templates.flutter.llmPrompt.redirectBased':
        'getting-started/connect-your-application/prompts/flutter/redirect-based.txt',
      'applications.templates.mcpClient.docs': 'getting-started/connect-your-mcp/python/',
      'agents.quickstarts.langchain.docs': 'getting-started/connect-your-agent/langchain/',
    },
  },
  client: {
    base: '/console',
    client_id: 'CONSOLE',
    resource_identifier: 'https://localhost:8090/mcp',
    scopes: ['openid', 'profile', 'email', 'ou', 'system'],
  },
  // Defaults to the origin this app is served from. Add a `server` block with `public_url`
  // (or `hostname`, `port`, `http_only`) to target a different backend.

  // Optional: location of the login gate, used to build the OAuth redirect URI shown when
  // configuring social/OIDC connections. Omit to default to `${served origin}/gate/callback`.
  // gate_client: {
  //   public_url: 'https://gate.example.com',   // or hostname/port/scheme
  // },
};
