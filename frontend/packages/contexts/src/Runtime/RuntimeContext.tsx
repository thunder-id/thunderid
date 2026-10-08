// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Context, createContext} from 'react';

/**
 * Runtime context interface that carries the base URL of the deployment serving the
 * runtime endpoints (OAuth2/OIDC, flow execution, passkey registration).
 *
 * @public
 */
export interface RuntimeContextType {
  /**
   * Base URL an application's own traffic reaches, without a trailing slash.
   * @example "https://gateway.example.com:8090"
   */
  runtimeUrl: string;
}

/**
 * React context holding the runtime base URL for the component tree beneath it.
 *
 * The value is undefined when no `RuntimeProvider` is present and when the host application
 * has not resolved a runtime URL yet, which is why `useRuntimeUrl` falls back to the server
 * URL rather than reading this context directly.
 *
 * @public
 */
const RuntimeContext: Context<RuntimeContextType | undefined> = createContext<RuntimeContextType | undefined>(
  undefined,
);

export default RuntimeContext;
