// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext} from 'react';

/**
 * The minimal shape of the HTTP client an administration action needs, so an action can be written
 * and tested without a React tree.
 */
export interface AdministrationHttpLike {
  request: (config: unknown) => Promise<{data?: unknown}>;
}

/**
 * How a console carries out the administrative operations that a plane may perform differently.
 *
 * Every entry is optional, and an absent entry means the plain management API. That default is the
 * Control Plane's path: it holds configuration and serves no runtime, so a deletion is a write and
 * nothing more. A Data Plane console installs the entries below, where the same operation runs an
 * administration flow that revokes tokens and detaches sessions before the record is removed.
 *
 * The actions are supplied rather than chosen, so neither console reads a setting to decide which
 * path it is on. A console links the actions it can perform and no others.
 */
export interface AdministrationActions {
  /**
   * Deletes an application, having first carried out whatever must happen to live traffic.
   */
  deleteApplication?: (http: AdministrationHttpLike, serverUrl: string, applicationId: string) => Promise<void>;
  /**
   * Rotates an application's client secret and returns the new value, or null when the caller
   * should fall back to the plain management path.
   */
  regenerateClientSecret?: (
    http: AdministrationHttpLike,
    serverUrl: string,
    applicationId: string,
  ) => Promise<string | null>;
  /**
   * Deletes a user, having first carried out whatever must happen to their live sessions.
   */
  deleteUser?: (http: AdministrationHttpLike, serverUrl: string, userId: string) => Promise<void>;
}

/**
 * Defaults to no actions, which is the plain management path for every operation.
 */
const AdministrationActionsContext = createContext<AdministrationActions>({});

AdministrationActionsContext.displayName = 'AdministrationActionsContext';

export default AdministrationActionsContext;
