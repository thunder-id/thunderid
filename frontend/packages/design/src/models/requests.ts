// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {LayoutConfig} from './layout';
import type {Theme} from './theme';

/**
 * Request payload for creating a new theme configuration
 */
export interface CreateThemeRequest {
  handle: string;
  displayName: string;
  description?: string;
  /** ID of the project the theme belongs to. Required on the Control Plane. */
  projectId?: string;
  theme: Theme;
}

/**
 * Request payload for updating an existing theme configuration
 */
export interface UpdateThemeRequest {
  handle: string;
  displayName: string;
  description?: string;
  theme: Theme;
}

/**
 * Request payload for creating a new layout configuration
 */
export interface CreateLayoutRequest {
  handle: string;
  displayName: string;
  layout: LayoutConfig;
}

/**
 * Request payload for updating an existing layout configuration
 */
export interface UpdateLayoutRequest {
  handle: string;
  displayName: string;
  layout: LayoutConfig;
}
