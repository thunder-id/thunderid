// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppConfig} from '@thunderid/system-apps';

/**
 * The Gate design of the application an app signs in with: what its sign-in screens look like.
 * The builder edits it next to the app, but it is saved on the application.
 *
 * @public
 */
export interface AppGateDesign {
  layoutId?: string;
  themeId?: string;
}

/**
 * What the builder edits: an app's configuration, plus the application the app signs in with and
 * that application's Gate design, which are saved on the application rather than with the app.
 *
 * @public
 */
export interface BuilderAppConfig extends AppConfig {
  applicationId?: string;
  gate?: AppGateDesign;
}
