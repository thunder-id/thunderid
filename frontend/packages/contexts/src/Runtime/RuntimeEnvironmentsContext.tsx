// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Context, createContext} from 'react';

/**
 * One environment an application runs in, such as Development or Production: a place its configuration
 * is deployed to, with runtime endpoints of its own.
 *
 * @public
 */
export interface RuntimeEnvironment {
  /** Identifies the environment when reading what it holds. */
  id: string;
  /** What the environment is called, such as Development. */
  name: string;
  /**
   * Base URL an application's own traffic reaches in this environment, without a trailing slash.
   * @example "https://dev.gateway.example.com"
   */
  runtimeUrl: string;
  /** The color the environment is marked with across the console, for each color scheme. */
  color?: {light: string; dark: string};
}

/**
 * A field of a resource whose value can differ between environments, such as an application's client ID.
 *
 * @public
 */
export interface EnvironmentValueField {
  /** The resource's type, as a deploy names it, such as `application`. */
  resourceType: string;
  resourceId: string;
  /** The field, such as `clientId`. */
  field: string;
}

/**
 * What a field holds in one environment: a value, nothing (`unset`), or nothing because the environment
 * does not run the resource yet (`absent`).
 *
 * @public
 */
export type EnvironmentValue = {status: 'set'; value: string} | {status: 'unset'} | {status: 'absent'};

/**
 * The environments a console deploys to, and a way to read what each holds.
 *
 * @public
 */
export interface RuntimeEnvironmentsContextType {
  /** The environments in the order a change moves through them: a deploy enters at the first. */
  environments: RuntimeEnvironment[];
  /** Where configuration is deployed to the environments, when the console has such a page in scope. */
  deployPath?: string;
  /** Reads what a resource's field holds in one environment. */
  getValue: (environmentId: string, field: EnvironmentValueField) => Promise<EnvironmentValue>;
}

/**
 * React context holding the environments an application's configuration is deployed to.
 *
 * Only a console that deploys to environments provides it, so the value is undefined in a console
 * serving a single deployment. Read it with `useRuntimeEnvironments`.
 *
 * @public
 */
const RuntimeEnvironmentsContext: Context<RuntimeEnvironmentsContextType | undefined> = createContext<
  RuntimeEnvironmentsContextType | undefined
>(undefined);

export default RuntimeEnvironmentsContext;
