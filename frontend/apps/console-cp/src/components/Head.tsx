// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Helmet} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';

/**
 * Resolves a configured favicon path against the Vite base URL so it loads
 * correctly when the console is served under a sub-path such as `/console`.
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
