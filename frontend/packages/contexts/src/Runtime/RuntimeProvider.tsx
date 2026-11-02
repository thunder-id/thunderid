// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo, type JSX, type PropsWithChildren} from 'react';
import RuntimeContext, {type RuntimeContextType} from './RuntimeContext';

/**
 * Props for the RuntimeProvider component.
 *
 * @public
 */
export interface RuntimeProviderProps extends PropsWithChildren {
  /**
   * Base URL an application's own traffic reaches. Leave it unset while the host application is
   * still resolving it, or when the deployment serving the console is also the one serving the
   * runtime: descendants then fall back to the server URL.
   */
  url?: string;
}

/**
 * React context provider that tells feature packages where an application's runtime endpoints
 * live, when that is not the deployment the console itself talks to.
 *
 * A console serving a separate runtime shows OAuth2/OIDC, flow and passkey URLs that a
 * developer is meant to copy into their own application, so those URLs have to name the
 * deployment that will answer them. Resolving which one that is belongs to the host
 * application: packages only read the answer through `useRuntimeUrl`.
 *
 * @example
 * ```tsx
 * <RuntimeProvider url={gateway?.baseUrl}>
 *   <Routes />
 * </RuntimeProvider>
 * ```
 *
 * @public
 */
export default function RuntimeProvider({url = undefined, children}: RuntimeProviderProps): JSX.Element {
  // An unset or empty URL leaves the context undefined rather than holding an empty string, so
  // `useRuntimeUrl` falls back instead of building endpoint URLs with no origin.
  const value: RuntimeContextType | undefined = useMemo(
    () => (url ? {runtimeUrl: url.replace(/\/+$/, '')} : undefined),
    [url],
  );

  return <RuntimeContext.Provider value={value}>{children}</RuntimeContext.Provider>;
}
