/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

const config: Config = {
  title: productConfig.project.name,
  tagline: productConfig.project.description,
  favicon: 'assets/images/favicon-inverted.ico',

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

  plugins: [webpackPlugin, personaPlugin],

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
          lastVersion: 'current',
          versions: {
            current: {
              label: 'Next',
              path: 'next',
            },
          },
          // Replace {{ProductName}}, {{productSlug}}, and local-URL placeholders inside code blocks at build time.
          rehypePlugins: [
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
          ],
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
    image: 'assets/images/social-card.png',
    colorMode: {
      respectPrefersColorScheme: true,
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
          type: 'doc',
          docId: 'sdks/overview',
          position: 'right',
          label: 'SDKs',
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
    algolia: {
      appId: 'PML8PAKD9O',
      apiKey: '04e88f06bc04c51b7f246d180438cf25',
      indexName: 'thunderid-docs-prod',
    },
  } satisfies Preset.ThemeConfig,

  /* -------------------------------- Product Config ------------------------------- */
  customFields: {
    product: productConfig,
  },
};

export default config;
