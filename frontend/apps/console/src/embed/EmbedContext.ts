// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, useContext} from 'react';
import type {MountContext} from './contract';

export interface EmbedState {
  ctx: MountContext;
  /**
   * Registers the console's navigate function with the host, so the host's sidebar can navigate
   * inside the console. Called by the router bridge with `null` when the console unmounts.
   */
  setNavigate(navigate: ((path: string) => void) | null): void;
}

/** Present only when the console runs inside a host application. */
export const EmbedContext = createContext<EmbedState | null>(null);

export const useEmbed = (): EmbedState | null => useContext(EmbedContext);
