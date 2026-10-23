// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The contract between a host application and a console mounted inside it as a microfrontend.
 *
 * A remote is a plain ES module served from the console's own origin. The host imports it at
 * runtime and mounts it into a DOM node: no iframe, and no shared runtime, so the host and the
 * console each keep their own React, router and UI library versions. The host supplies the chrome
 * (top bar, sidebar), the theme and the session; the console renders its pages inside.
 *
 * This file is the whole integration surface. A host copies it, or generates its types from it.
 */

export interface RemoteNavItem {
  label: string;
  /** Path inside the remote, relative to its basename, for example `/users`. */
  path: string;
  /** Icon name from `@wso2/oxygen-ui-icons-react`, for example `Users`. */
  icon?: string;
}

export interface RemoteNavSection {
  /** Section heading, such as `Resources`. Omit for the leading, unlabelled group. */
  section?: string;
  items: RemoteNavItem[];
}

export interface RemoteManifest {
  id: string;
  title: string;
  /** Rendered by the host's sidebar while the console is open. */
  nav: RemoteNavSection[];
  /** Extra chrome the host shows in its top bar while the console is open. */
  header?: {chips?: {label: string; color?: 'default' | 'warning' | 'info' | 'primary'}[]};
}

/** Which ThunderID server the mounted console administers, and as which application. */
export interface RemoteServerConfig {
  /** Base URL of the ThunderID server. */
  serverUrl: string;
  /** The application registered for this console. */
  clientId: string;
  scopes?: string[];
}

/**
 * The host runs a backend for frontend on the page's origin. The console signs in through it with
 * the SDK's backend mode, so no token reaches the browser and API calls go through the proxy.
 * The paths follow the SDK's `endpoints.backend` contract.
 */
export interface RemoteBackendAuth {
  signIn: string;
  session: string;
  checkSession: string;
  refresh: string;
  signOut: string;
  proxy: string;
}

/**
 * The host has no backend. It signed the user in once and exchanged that session for a token minted
 * for this console (RFC 8693), bound to `serverUrl` and carrying only this console's scopes. The
 * console signs nobody in: it runs on the token the host hands it and asks `renew()` when it nears
 * expiry.
 */
export interface RemoteHostToken {
  accessToken: string;
  /** Epoch milliseconds. */
  expiresAt: number;
  scopes: string[];
  /** The authorization server that minted the token, which the console treats as a trusted issuer. */
  issuerUrl: string;
  renew(): Promise<{accessToken: string; expiresAt: number}>;
}

export interface MountContext {
  /** Where the host mounted the console, for example `/identity`. The console's router uses it as basename. */
  basename: string;
  colorMode: 'light' | 'dark';
  /**
   * The host's theme (an Oxygen UI / MUI theme object). The console renders with it instead of its
   * own, so the whole page has one look and one set of CSS variables.
   */
  theme: object;
  config: RemoteServerConfig;
  /** Exactly one of `auth` and `token` is supplied. */
  auth?: RemoteBackendAuth;
  token?: RemoteHostToken;
  /** The console reports in-app navigation so the host can highlight its sidebar. */
  onNavigate(path: string): void;
}

export interface RemoteHandle {
  /** Navigate inside the console. The host's sidebar calls this. */
  navigate(path: string): void;
  setColorMode?(mode: 'light' | 'dark'): void;
  unmount(): void;
}

export interface RemoteModule {
  manifest: RemoteManifest;
  mount(el: HTMLElement, ctx: MountContext): RemoteHandle | Promise<RemoteHandle>;
}

/**
 * Builds the backend endpoints for a console mounted at `basename`, for a host that serves its
 * backend for frontend under the mount path.
 */
export function backendEndpoints(basename: string): RemoteBackendAuth {
  return {
    checkSession: `${basename}/auth/checksession`,
    proxy: `${basename}/proxy`,
    refresh: `${basename}/auth/refresh`,
    session: `${basename}/auth/session`,
    signIn: `${basename}/auth/signin`,
    signOut: `${basename}/auth/signout`,
  };
}
