// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Context, createContext} from 'react';
import type {AdministrationMode} from './constants';

/**
 * How one operation runs: a mode name, or a function that carries it out.
 *
 * A function lets a distributor change the behaviour of a single operation without forking the
 * package that owns it.
 *
 * @public
 */
export type AdministrationOperation<TArgs extends unknown[] = never[], TResult = unknown> =
  | AdministrationMode
  | ((...args: TArgs) => Promise<TResult>);

/**
 * How the operations of one feature run.
 *
 * @public
 */
export interface FeatureAdministrationConfig {
  /** The mode every operation of this feature uses unless it names its own. */
  mode?: AdministrationMode;
  /** How individual operations run, keyed by operation name, e.g. `delete` or `add`. */
  operations?: Record<string, AdministrationOperation>;
}

/**
 * How administration operations run across the console.
 *
 * This is deliberately not a plane setting. Which plane a console serves decides *which features
 * exist*, by composition; this decides *how an operation runs* when it does. A single flag driving
 * both turns back into a plane check that every package has to read.
 *
 * @public
 */
export interface AdministrationConfig {
  /** The mode used when neither the feature nor the operation names one. */
  mode?: AdministrationMode;
  /** Per-feature overrides, keyed by feature name, e.g. `users` or `applications`. */
  features?: Record<string, FeatureAdministrationConfig>;
}

/**
 * React context carrying how administration operations run.
 *
 * Defaults to an empty object rather than `undefined`, so a feature package renders standalone,
 * in a test or a Storybook story, without an `AdministrationProvider` above it. Read it through
 * `useAdministration` rather than directly.
 *
 * @public
 */
const AdministrationContext: Context<AdministrationConfig> = createContext<AdministrationConfig>({});

export default AdministrationContext;
