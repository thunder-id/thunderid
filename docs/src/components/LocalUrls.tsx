// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import type {ReactNode} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

function useProductConfig(): DocusaurusProductConfig {
  const {siteConfig} = useDocusaurusContext();
  return siteConfig.customFields?.product as DocusaurusProductConfig;
}

/**
 * Props for URL components that render links to local product instances.
 */
interface UrlComponentProps {
  /**
   * Optional path appended to the base URL. Must start with a slash.
   */
  path?: string;
  /**
   * When true, render the URL as plain text instead of an anchor.
   */
  plain?: boolean;
}

/**
 * Renders a URL to a local product instance, based on the `local` configurations.
 * The URL is rendered as a link by default, but can be rendered as plain text if `plain` is true.
 */
function renderUrl(baseUrl: string, {path = '', plain = false}: UrlComponentProps): ReactNode {
  const href = `${baseUrl}${path}`;
  return plain ? href : <Link to={href}>{href}</Link>;
}

/**
 * Renders a URL to the local <ProductName /> Console, based on the `local.consoleUrl` configuration.
 */
export function ConsoleUrl(props: UrlComponentProps): ReactNode {
  return renderUrl(useProductConfig().local.consoleUrl, props);
}

/**
 * Renders the local Control Plane Console URL.
 */
export function ControlPlaneConsoleUrl(props: UrlComponentProps): ReactNode {
  return renderUrl(useProductConfig().local.controlPlaneConsoleUrl, props);
}

/**
 * Renders a URL to the local Wayfinder sample, based on the `local.samples.wayfinderUrl` configuration.
 */
export function WayFinderSampleUrl(props: UrlComponentProps): ReactNode {
  return renderUrl(useProductConfig().local.samples.wayfinderUrl, props);
}

/**
 * Renders a URL to the local Wayfinder sample mail inbox, based on the `local.samples.wayfinderMailUrl` configuration.
 */
export function WayFinderMailUrl(props: UrlComponentProps): ReactNode {
  return renderUrl(useProductConfig().local.samples.wayfinderMailUrl, props);
}
