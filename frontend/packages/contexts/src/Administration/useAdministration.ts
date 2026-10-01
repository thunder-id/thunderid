// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import AdministrationContext, {type AdministrationConfig, type AdministrationOperation} from './AdministrationContext';
import {AdministrationModes, type AdministrationMode} from './constants';

/**
 * The mode used when nothing names one.
 *
 * `flow` rather than `native`, because that is what every console did before this option existed:
 * an operation ran through the administration flow when one was configured. Defaulting the other
 * way would silently stop revoking tokens and ending sessions on deployments that never asked for
 * the change.
 */
export const DEFAULT_ADMINISTRATION_MODE: AdministrationMode = AdministrationModes.FLOW;

/**
 * Resolves how one operation runs, from the most specific setting to the least.
 *
 * The order is operation, then feature, then the console-wide default: naming an operation is
 * always more specific than naming the feature it belongs to.
 *
 * @param administration - The configuration to read.
 * @param feature - The feature the operation belongs to, e.g. `users`.
 * @param operation - The operation, e.g. `delete`.
 * @returns A mode name, or the function configured to carry the operation out.
 *
 * @public
 */
export function resolveAdministrationOperation(
  administration: AdministrationConfig,
  feature: string,
  operation: string,
): AdministrationOperation {
  const forFeature = administration.features?.[feature];

  return forFeature?.operations?.[operation] ?? forFeature?.mode ?? administration.mode ?? DEFAULT_ADMINISTRATION_MODE;
}

/**
 * Reads how administration operations run beneath the nearest `AdministrationProvider`.
 *
 * Returns an empty configuration when there is no provider, so every operation falls back to the
 * default mode and a package renders standalone.
 *
 * @public
 */
export default function useAdministration(): AdministrationConfig {
  return useContext(AdministrationContext);
}

/**
 * Reads how one operation runs.
 *
 * A package calls this with its own feature name and the operation it is about to perform, and
 * branches on the answer. It never asks which plane it is on.
 *
 * The caller states the signature it will call a supplied function with, because only it knows
 * what the operation takes. What is stored is the bottom function type, so that any function a
 * console supplies is accepted; that type cannot be called, and this is where it is given a shape.
 *
 * @typeParam TArgs - The arguments this operation is carried out with.
 * @typeParam TResult - What carrying it out resolves to.
 *
 * @example
 * ```ts
 * const mode = useAdministrationOperation<[string], void>('users', 'delete');
 *
 * if (typeof mode === 'function') {
 *   await mode(userId);
 * } else if (mode === AdministrationModes.NATIVE) {
 *   await deleteUserNatively(http, serverUrl, userId);
 * } else {
 *   await deleteUserViaFlow(http, serverUrl, userId);
 * }
 * ```
 *
 * @public
 */
export function useAdministrationOperation<TArgs extends unknown[] = never[], TResult = unknown>(
  feature: string,
  operation: string,
): AdministrationOperation<TArgs, TResult> {
  return resolveAdministrationOperation(useAdministration(), feature, operation) as AdministrationOperation<
    TArgs,
    TResult
  >;
}
