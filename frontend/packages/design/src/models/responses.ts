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

import type {ApiPaginationLink} from '@thunderid/types';
import type {LayoutConfig} from './layout';
import type {Theme} from './theme';

/**
 * Theme item in list responses.
 * The full theme configuration is not included; only the metadata needed for display.
 */
export interface ThemeListItem {
  id: string;
  handle: string;
  displayName: string;
  description?: string;
  defaultColorScheme?: string;
  primaryColor?: string;
  createdAt?: string;
  updatedAt?: string;
  isReadOnly?: boolean;
}

/**
 * Layout item in list responses (layout data may be null in list view)
 */
export interface LayoutListItem {
  id: string;
  handle: string;
  displayName: string;
  layout: LayoutConfig | null;
}

/**
 * Response for listing theme configurations
 */
export interface ThemeListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  themes: ThemeListItem[];
  links: ApiPaginationLink[];
}

/**
 * Response for a single theme configuration
 */
export interface ThemeResponse {
  id: string;
  handle: string;
  displayName: string;
  description?: string;
  theme: Theme;
  isReadOnly?: boolean;
}

/**
 * Response for listing layout configurations
 */
export interface LayoutListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  layouts: LayoutListItem[];
  links: ApiPaginationLink[];
}

/**
 * Response for a single layout configuration
 */
export interface LayoutResponse {
  id: string;
  handle: string;
  displayName: string;
  layout: LayoutConfig;
}

/**
 * Response from the design resolve endpoint.
 * Returns the merged theme and layout for a given entity.
 */
export interface DesignResolveResponse {
  theme: Theme;
  layout: LayoutConfig;
}

/**
 * A single resource that references a theme.
 */
export interface ThemeUsage {
  resourceType: string;
  id: string;
  displayName: string;
  behaviorOnDelete: 'fallback' | 'cascade';
}

/**
 * Per-resource-type count of usages, keyed by resource type (e.g. application, agent).
 * Null when the counts could not be determined.
 */
export type ThemeUsagesSummary = Record<string, number> | null;

/**
 * Response for the theme usages endpoint, aggregated across all resource types.
 * totalResults is null when usage data is unavailable (e.g. registry not wired).
 */
export interface ThemeUsagesResponse {
  totalResults: number | null;
  count: number;
  summary: ThemeUsagesSummary;
  usages: ThemeUsage[];
}
