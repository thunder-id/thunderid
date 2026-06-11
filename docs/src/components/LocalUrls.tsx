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
