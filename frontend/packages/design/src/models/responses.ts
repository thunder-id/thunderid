// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
  /** ID of the project the theme belongs to, when it belongs to one */
  projectId?: string;
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
  /** The project the layout belongs to. */
  projectId?: string;
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
  /** ID of the project the theme belongs to, when it belongs to one */
  projectId?: string;
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
  /** The project the layout belongs to. */
  projectId?: string;
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
