// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import RuntimeContext from './RuntimeContext';
import useConfig from '../Config/useConfig';

/**
 * React hook returning the base URL to build an application's runtime endpoints from: the
 * OAuth2/OIDC endpoints, the flow execution and metadata endpoints, and the passkey
 * registration endpoints.
 *
 * Use it for URLs the console displays for someone to copy into their own application. Calls
 * the console itself makes against the management API keep using `getServerUrl`, which is the
 * deployment the console is configured against.
 *
 * Falls back to `getServerUrl()` when no runtime URL has been supplied, so a deployment that
 * serves both the console and the runtime needs no provider at all.
 *
 * @returns The runtime base URL, without a trailing slash
 *
 * @example
 * ```tsx
 * const runtimeUrl = useRuntimeUrl();
 * const tokenEndpoint = `${runtimeUrl}/oauth2/token`;
 * ```
 *
 * @public
 */
export default function useRuntimeUrl(): string {
  const runtime = useContext(RuntimeContext);
  const {getServerUrl} = useConfig();

  return runtime?.runtimeUrl ?? getServerUrl();
}
