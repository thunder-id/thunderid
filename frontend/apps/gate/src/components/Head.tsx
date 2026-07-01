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

import {Helmet} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';

/**
 * Resolves a configured favicon path against the Vite base URL so it loads
 * correctly when the gate is served under a sub-path such as `/gate`.
 * Already-resolved values (absolute, protocol-relative or root-relative URLs,
 * and `data:` URIs) are returned untouched.
 */
function resolveFaviconHref(path: string): string {
  if (/^([a-z]+:|\/)/i.test(path)) {
    return path;
  }
  return `${import.meta.env.BASE_URL.replace(/\/$/, '')}/${path}`;
}

/**
 * Manages document head metadata, specifically the favicon, with support for
 * separate light and dark variants.
 *
 * Both variants are registered with a `prefers-color-scheme` media query so the
 * browser picks the matching favicon based on the operating system / browser
 * color scheme. This is independent of the in-app color scheme toggle — the tab
 * icon tracks the OS theme, switching live when the OS theme changes.
 */
export default function Head() {
  const {config} = useConfig();
  const {favicon} = config.brand;

  return (
    <Helmet>
      <link rel="icon" href={resolveFaviconHref(favicon.light)} media="(prefers-color-scheme: light)" />
      <link rel="icon" href={resolveFaviconHref(favicon.dark)} media="(prefers-color-scheme: dark)" />
    </Helmet>
  );
}
