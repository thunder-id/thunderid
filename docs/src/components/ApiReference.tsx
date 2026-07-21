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

import BrowserOnly from '@docusaurus/BrowserOnly';
import {ApiReferenceReact, type AnyApiReferenceConfiguration} from '@scalar/api-reference-react';
import '@scalar/api-reference-react/style.css';
import {Box, CircularProgress} from '@wso2/oxygen-ui';
import {JSX, useEffect, useRef} from 'react';

export type ApiReferenceProps = AnyApiReferenceConfiguration & {
  specUrl: string;
};

type IconNode = [string, Record<string, string>][];

const CATEGORY_ICONS: Record<string, IconNode> = {
  'applications': [
    ['rect', {width: '7', height: '7', x: '3', y: '3', rx: '1'}],
    ['rect', {width: '7', height: '7', x: '14', y: '3', rx: '1'}],
    ['rect', {width: '7', height: '7', x: '14', y: '14', rx: '1'}],
    ['rect', {width: '7', height: '7', x: '3', y: '14', rx: '1'}],
  ],
  'connections': [
    ['path', {d: 'M12 22v-5'}],
    ['path', {d: 'M9 8V2'}],
    ['path', {d: 'M15 8V2'}],
    ['path', {d: 'M18 8v5a4 4 0 0 1-4 4h-4a4 4 0 0 1-4-4V8Z'}],
  ],
  'identities': [
    ['path', {d: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2'}],
    ['path', {d: 'M16 3.128a4 4 0 0 1 0 7.744'}],
    ['path', {d: 'M22 21v-2a4 4 0 0 0-3-3.87'}],
    ['circle', {cx: '9', cy: '7', r: '4'}],
  ],
  'access-control': [
    ['path', {d: 'M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z'}],
    ['path', {d: 'm9 12 2 2 4-4'}],
  ],
  'authentication': [
    ['path', {d: 'M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z'}],
    ['circle', {cx: '16.5', cy: '7.5', r: '.5', fill: 'currentColor'}],
  ],
  'login-flows': [
    ['line', {x1: '6', x2: '6', y1: '3', y2: '15'}],
    ['circle', {cx: '18', cy: '6', r: '3'}],
    ['circle', {cx: '6', cy: '18', r: '3'}],
    ['path', {d: 'M18 9a9 9 0 0 1-9 9'}],
  ],
  'oauth2-oidc': [
    ['rect', {width: '18', height: '11', x: '3', y: '11', rx: '2', ry: '2'}],
    ['path', {d: 'M7 11V7a5 5 0 0 1 10 0v4'}],
  ],
  'branding-localization': [
    ['path', {d: 'M12 22a1 1 0 0 1 0-20 10 9 0 0 1 10 9 5 5 0 0 1-5 5h-2.25a1.75 1.75 0 0 0-1.4 2.8l.3.4a1.75 1.75 0 0 1-1.4 2.8z'}],
    ['circle', {cx: '13.5', cy: '6.5', r: '.5', fill: 'currentColor'}],
    ['circle', {cx: '17.5', cy: '10.5', r: '.5', fill: 'currentColor'}],
    ['circle', {cx: '6.5', cy: '12.5', r: '.5', fill: 'currentColor'}],
    ['circle', {cx: '8.5', cy: '7.5', r: '.5', fill: 'currentColor'}],
  ],
  'system': [
    ['path', {d: 'M20 7h-9'}],
    ['path', {d: 'M14 17H5'}],
    ['circle', {cx: '17', cy: '17', r: '3'}],
    ['circle', {cx: '7', cy: '7', r: '3'}],
  ],
};

const TAG_ICONS: Record<string, IconNode> = {
  'models': [
    ['path', {d: 'M8 3H7a2 2 0 0 0-2 2v5a2 2 0 0 1-2 2 2 2 0 0 1 2 2v5c0 1.1.9 2 2 2h1'}],
    ['path', {d: 'M16 21h1a2 2 0 0 0 2-2v-5c0-1.1.9-2 2-2a2 2 0 0 1-2-2V5a2 2 0 0 0-2-2h-1'}],
  ],
};

function buildSvg(nodes: IconNode): string {
  const children = nodes
    .map(([tag, attrs]) => {
      const attrStr = Object.entries(attrs)
        .filter(([k]) => k !== 'key')
        .map(([k, v]) => `${k}="${v}"`)
        .join(' ');
      return `<${tag} ${attrStr}/>`;
    })
    .join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="14" height="14">${children}</svg>`;
}

function ApiReferenceContent({specUrl, ...rest}: ApiReferenceProps): JSX.Element {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const injectIcon = (labelDiv: HTMLElement, nodes: IconNode) => {
      if (labelDiv.querySelector('.apis-category-icon')) return;
      const span = document.createElement('span');
      span.className = 'apis-category-icon';
      span.style.cssText = 'display:inline-flex;align-items:center;margin-right:6px;flex-shrink:0;';
      span.innerHTML = buildSvg(nodes);
      labelDiv.prepend(span);
    };

    const inject = () => {
      container.querySelectorAll('[data-sidebar-id*="/tag-group/"]').forEach((el) => {
        const id = el.getAttribute('data-sidebar-id') ?? '';
        const slug = id.replace(/.*\/tag-group\//, '');
        const nodes = CATEGORY_ICONS[slug];
        if (!nodes) return;
        const labelDiv = el.querySelector(':scope > div[aria-selected] > div:last-child');
        if (!(labelDiv instanceof HTMLElement)) return;
        injectIcon(labelDiv, nodes);
      });

      container.querySelectorAll('[data-sidebar-id$="/models"]').forEach((el) => {
        const nodes = TAG_ICONS.models;
        if (!nodes) return;
        const labelDiv = el.querySelector('[class*="button-label"]');
        if (!(labelDiv instanceof HTMLElement)) return;
        injectIcon(labelDiv, nodes);
      });
    };

    const observer = new MutationObserver(inject);
    observer.observe(container, {childList: true, subtree: true});
    inject();

    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={containerRef}
      className="apis-page"
      style={{
        position: 'fixed',
        top: 'var(--ifm-navbar-height)',
        left: 0,
        right: 0,
        bottom: 0,
        height: 'calc(100vh - var(--ifm-navbar-height))',
        overflowY: 'scroll',
        overflowX: 'hidden',
        WebkitOverflowScrolling: 'touch',
        background: 'var(--oxygen-palette-background-default)',
      }}
    >
      <ApiReferenceReact
        configuration={{
          url: specUrl,
          theme: 'default',
          layout: 'modern',
          // Hides the `Open in Client` button that takes to Scalar Hosted workspace.
          hideClientButton: true,
          hideDarkModeToggle: true,
          ...rest,
        }}
      />
    </div>
  );
}

export default function ApiReference({specUrl, ...rest}: ApiReferenceProps): JSX.Element {
  return (
    <BrowserOnly
      fallback={
        <Box sx={{display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh'}}>
          <CircularProgress />
        </Box>
      }
    >
      {() => <ApiReferenceContent specUrl={specUrl} {...rest} />}
    </BrowserOnly>
  );
}
