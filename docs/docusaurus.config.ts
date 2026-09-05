// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Options as DocsOptions} from '@docusaurus/plugin-content-docs';
import type * as Preset from '@docusaurus/preset-classic';
import type {Config} from '@docusaurus/types';
import {themes as prismThemes} from 'prism-react-renderer';
import productConfig from './docusaurus.product.config';
import personaPlugin from './plugins/personaPlugin';
import rehypeProductName from './plugins/rehypeProductName';
import webpackPlugin from './plugins/webpackPlugin';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Recursively replaces `{{ProductName}}` and `{{productSlug}}` in every string
 * value inside a frontmatter object so authors can use these placeholders in
 * frontmatter fields (e.g. `title`, `description`) without hard-coding the
 * product name or slug.
 */
function replaceProductNameInObject(value: unknown, productName: string, productSlug: string): unknown {
  if (typeof value === 'string') {
    return value.replaceAll('{{ProductName}}', productName).replaceAll('{{productSlug}}', productSlug);
  }
  if (Array.isArray(value)) {
    return value.map((item) => replaceProductNameInObject(item, productName, productSlug));
  }
  if (value !== null && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([k, v]) => [
        k,
        replaceProductNameInObject(v, productName, productSlug),
      ]),
    );
  }
  return value;
}

const baseUrl =
  // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing
  process.env.DOCUSAURUS_BASE_URL ||
  (productConfig.documentation.deployment.production.baseUrl
    ? `/${productConfig.documentation.deployment.production.baseUrl}/`
    : '/');

// eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing
const siteUrl = process.env.DOCUSAURUS_URL || productConfig.documentation.deployment.production.url;

// Replace {{ProductName}}, {{productSlug}}, and local-URL placeholders inside code blocks at
// build time. Shared by every docs plugin instance.
const docsRehypePlugins: DocsOptions['rehypePlugins'] = [
  [
    rehypeProductName,
    {
      productName: productConfig.project.name,
      productSlug: productConfig.project.name.toLowerCase(),
      replacements: {
        '{{ConsoleUrl}}': productConfig.local.consoleUrl,
        '{{WayFinderSampleUrl}}': productConfig.local.samples.wayfinderUrl,
        '{{WayFinderMailUrl}}': productConfig.local.samples.wayfinderMailUrl,
      },
    },
  ],
];

const config: Config = {
  title: productConfig.project.name,
  tagline: productConfig.project.description,

  noIndex: false,

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  url: siteUrl,
  baseUrl,
  trailingSlash: true,

  // GitHub pages deployment config.
  organizationName: productConfig.project.source.github.owner.name, // Usually your GitHub org/user name.
  projectName: productConfig.project.source.github.name, // Usually your repo name.

  onBrokenLinks: 'throw',

  markdown: {
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
    // Replace {{ProductName}} placeholders in frontmatter values at build time.
    // This applies globally to all content (docs, pages, etc.).
    // See: https://docusaurus.io/docs/api/docusaurus-config#markdown
    parseFrontMatter: async (params) => {
      const result = await params.defaultParseFrontMatter(params);
      result.frontMatter = replaceProductNameInObject(
        result.frontMatter,
        productConfig.project.name,
        productConfig.project.name.toLowerCase(),
      ) as Record<string, unknown>;
      return result;
    },
  },

  themes: ['@docusaurus/theme-mermaid'],

  // Internationalization (i18n) configuration.
  // See: https://docusaurus.io/docs/i18n/introduction
  i18n: {
    defaultLocale: 'en-US',
    locales: ['en-US'],
    localeConfigs: {
      'en-US': {
        label: 'English (US)',
        direction: 'ltr',
        htmlLang: 'en-US',
        calendar: 'gregory',
      },
    },
  },

  clientModules: [require.resolve('./src/clientModules/tabTocSync.js')],

  headTags: [
    {
      tagName: 'script',
      attributes: {},
      // Reads the same "theme" localStorage key as Docusaurus' own no-flash script, but
      // stamps the attribute the MUI/Oxygen-UI theme reads (colorSchemeSelector:
      // "data-color-scheme"). Without this, a hard refresh paints MUI-styled surfaces with
      // their light-scheme fallback for one frame before OxygenUIThemeProvider mounts and
      // syncs to the already-correct Docusaurus theme. Docusaurus' stored value can be the
      // literal string "system" (its tri-state toggle), which must resolve through
      // prefers-color-scheme here rather than being stamped as-is, since Oxygen-UI's CSS
      // only defines variables for "dark"/"light".
      innerHTML: `(function(){try{var t=new URLSearchParams(window.location.search).get("docusaurus-theme")||window.localStorage.getItem("theme");var dark=t==="dark"||(t!=="light"&&window.matchMedia("(prefers-color-scheme: dark)").matches);document.documentElement.setAttribute("data-color-scheme",dark?"dark":"light");}catch(e){}})();`,
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'icon',
        href: '/assets/images/logo-mini.svg',
        media: '(prefers-color-scheme: light)',
        type: 'image/svg+xml',
      },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'icon',
        href: '/assets/images/logo-mini-inverted.svg',
        media: '(prefers-color-scheme: dark)',
        type: 'image/svg+xml',
      },
    },
    {
      tagName: 'script',
      attributes: {},
      innerHTML: `(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
})(window,document,'script','dataLayer','GTM-PTKWJGJL');`,
    },
    {
      tagName: 'script',
      attributes: {
        src: 'https://cookie-cdn.cookiepro.com/scripttemplates/otSDKStub.js',
        type: 'text/javascript',
        charset: 'UTF-8',
        'data-domain-script': '019e40cb-79a0-7395-aa5d-5d887b4b8d2d',
      },
    },
    {
      tagName: 'script',
      attributes: {type: 'text/javascript'},
      innerHTML: 'function OptanonWrapper() { }',
    },
  ],

  plugins: [
    '@docsearch/docusaurus-adapter',
    webpackPlugin,
    personaPlugin,
    './plugins/docusaurus-plugin-llms-txt',
    './plugins/docusaurus-plugin-markdown-export',
    // Community docs are a separate, unversioned plugin instance. They describe how to
    // contribute to the project as it stands today, so they are not snapshotted per
    // release and are served from /community/ instead of /docs/<version>/community/.
    [
      '@docusaurus/plugin-content-docs',
      {
        id: 'community',
        path: 'community',
        routeBasePath: 'community',
        sidebarPath: './sidebarsCommunity.ts',
        editUrl: productConfig.project.source.github.editUrls.content,
        rehypePlugins: docsRehypePlugins,
      } satisfies DocsOptions,
    ],
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          path: 'content',
          sidebarPath: './sidebars.ts',
          // Edit URL for the "edit this page" feature.
          editUrl: productConfig.project.source.github.editUrls.content,
          // Versioning.
          lastVersion: 'v1.0.x',
          versions: {
            current: {
              label: 'Next',
              path: 'next',
              // The current docs are the future/upcoming version, not an archive.
              banner: 'unreleased',
              // No "Version: Next" pill at the top of every doc page.
              badge: false,
            },
            'v1.0.x': {
              label: 'v1.0.x',
              // Explicit URL segment so the stable release lives at /docs/v1.0.x/
              // instead of the bare doc root. The version tracks the 1.0 minor line
              // (1.0.0, 1.0.1, ...), so patch releases reuse these docs.
              path: 'v1.0.x',
              // Current stable release: not archived, so no "unmaintained" banner.
              banner: 'none',
              // No "Version: v1.0.x" pill at the top of every doc page.
              badge: false,
            },
          },
          rehypePlugins: docsRehypePlugins,
        },
        blog: {
          path: 'blog',
          routeBasePath: 'blog',
          showReadingTime: true,
          blogSidebarTitle: 'All posts',
          blogSidebarCount: 'ALL',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'assets/images/og-image.png',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    // Mermaid measures label widths with this font, so it must match the CSS
    // theme in custom.css, otherwise labels get clipped. `base` keeps Mermaid's
    // own styling minimal and lets custom.css drive the palette for both modes.
    mermaid: {
      theme: {light: 'base', dark: 'base'},
      options: {
        fontFamily: "'Plus Jakarta Sans', 'Inter', system-ui, sans-serif",
        // More room between nodes and ranks so edge-label chips stop overlapping
        // on wide fan-outs. Defaults are 50/50.
        flowchart: {
          nodeSpacing: 50,
          rankSpacing: 46,
          padding: 18,
          // breathing room around subgraph titles so they don't hug the border
          subGraphTitleMargin: {top: 12, bottom: 14},
          // 'basis' (default) overshoots and looks loose; monotoneY gives clean,
          // non-overshooting curves for a top-down flow.
          curve: 'monotoneY',
        },
        sequence: {
          // Notes carry request/code detail, render them monospace. Use the generic
          // `monospace` keyword (NOT a web font or `ui-monospace`): Mermaid measures
          // note width with this exact string, and `monospace` resolves identically
          // for measurement and render, so the text can't overflow the box.
          noteFontFamily: 'monospace',
          noteFontSize: 12,
          noteAlign: 'left',
          // inner padding so the code text never touches the panel edge
          noteMargin: 16,
        },
      },
    },
    navbar: {
      title: '',
      logo: {
        href: '/',
        src: '/assets/images/logo.svg',
        srcDark: '/assets/images/logo-inverted.svg',
        alt: `${productConfig.project.name} Logo`,
        height: '40',
        width: '150',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'right',
          label: 'Docs',
          className: 'navbar__link--docs',
        },
        {
          type: 'doc',
          docId: 'apis',
          position: 'right',
          label: 'APIs',
        },
        {
          to: '/sdks',
          position: 'right',
          label: 'SDKs & Tools',
        },
        {
          to: '/blog',
          label: 'Blog',
          position: 'right',
        },
        {
          label: 'Releases',
          to: productConfig.project.source.github.releasesUrl,
          position: 'right',
        },
        {
          type: 'docSidebar',
          sidebarId: 'communitySidebar',
          docsPluginId: 'community',
          position: 'right',
          label: 'Community',
        },
        {
          type: 'custom-GitHubStarButton',
          position: 'right',
        },
        ...(productConfig.documentation.versioning.enabled
          ? [
              {
                type: 'docsVersionDropdown',
                position: 'right' as const,
              },
            ]
          : []),
      ],
    },
    footer: {
      style: 'dark',
      links: [],
      copyright: `Copyright © ${new Date().getFullYear()} ${productConfig.project.name}.`,
    },
    prism: {
      theme: prismThemes.nightOwlLight,
      darkTheme: prismThemes.nightOwl,
    },
    docsearch: {
      appId: 'PML8PAKD9O',
      apiKey: '04e88f06bc04c51b7f246d180438cf25',
      indexName: 'thunderid-docs-prod',
      askAi: {
        assistantId: "3e6fb420-3ffa-4b8b-9f59-5d8fc76a6236",
        indexName: "thunderid-llm-md",
        sidePanel: true,
        agentStudio: true,
      }
    },
  } satisfies Preset.ThemeConfig,

  /* -------------------------------- Product Config ------------------------------- */
  customFields: {
    product: productConfig,
  },
};

export default config;
