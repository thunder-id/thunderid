// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import RuntimeEnvironmentsContext, {type RuntimeEnvironmentsContextType} from './RuntimeEnvironmentsContext';

/**
 * React hook returning the environments an application's configuration is deployed to, in the order a
 * change moves through them, with a way to read what each holds.
 *
 * Use it to show what differs between environments, such as an application's client ID and runtime
 * endpoints. A console serving a single deployment provides none, so callers fall back to
 * `useRuntimeUrl` and the resource as the console reads it.
 *
 * @returns The environments, or undefined where the console provides none
 *
 * @example
 * ```tsx
 * const environments = useRuntimeEnvironments()?.environments ?? [];
 * ```
 *
 * @public
 */
export default function useRuntimeEnvironments(): RuntimeEnvironmentsContextType | undefined {
  return useContext(RuntimeEnvironmentsContext);
}
