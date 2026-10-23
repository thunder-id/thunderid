// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Remote entry for mounting the console inside a host application as a microfrontend, without an
 * iframe. The host imports this ES module at runtime and calls `mount()`; the console renders into
 * the host's node with its own React root and router, and the host's theme.
 *
 * Built by `pnpm build:remote` (vite.config.remote.ts) into `dist-remote/`.
 */

import {createRoot, type Root} from 'react-dom/client';
import type {MountContext, RemoteHandle, RemoteManifest} from './contract';
import consoleManifest from './manifest';

export const manifest: RemoteManifest = consoleManifest;

export async function mount(el: HTMLElement, ctx: MountContext): Promise<RemoteHandle> {
  const navigateRef: {current: ((path: string) => void) | null} = {current: null};
  const colorModeRef: {current: ((mode: 'light' | 'dark') => void) | null} = {current: null};
  const disposeRef: {current: (() => void) | null} = {current: null};

  // Loaded only now: the console's modules read the runtime config, and set up i18n, when they are
  // evaluated, and the config is only known once the host hands over its context.
  const {renderEmbedded} = await import('./render');

  const root: Root = createRoot(el);
  await renderEmbedded(root, ctx, {colorModeRef, disposeRef, navigateRef});

  return {
    navigate: (path: string) => navigateRef.current?.(path),
    setColorMode: (mode: 'light' | 'dark') => colorModeRef.current?.(mode),
    unmount: () => {
      disposeRef.current?.();
      disposeRef.current = null;
      root.unmount();
    },
  };
}
