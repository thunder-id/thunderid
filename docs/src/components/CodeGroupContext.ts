// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, useContext, type MutableRefObject} from 'react';

export interface CodeGroupTabInfo {
  value: string;
  label: string;
  icon?: string;
  active: boolean;
  onSelect: () => void;
}

export interface CodeGroupTabsContextValue {
  tabs: CodeGroupTabInfo[];
  /**
   * Set right before a tab switch, read once by the newly mounted code block so it can
   * restore focus to its (now active) tab button. `CodeGroup` renders only the active
   * child, so switching tabs unmounts the whole block and mounts a new one — which would
   * otherwise silently drop focus to `<body>` instead of following the selection, breaking
   * keyboard and screen-reader navigation between tabs.
   */
  focusOnMountRef: MutableRefObject<boolean>;
}

const CodeGroupTabsContext = createContext<CodeGroupTabsContextValue | null>(null);

export const CodeGroupTabsProvider = CodeGroupTabsContext.Provider;

/**
 * The active `<CodeGroup>`'s tab switcher, when the code block currently rendering is one
 * of its children — read by the swizzled `CodeBlock/Layout` so the package-manager tabs
 * render inside the block's own `.tid-codeblock__bar` instead of floating above it as a
 * separate control.
 */
export function useCodeGroupTabs(): CodeGroupTabsContextValue | null {
  return useContext(CodeGroupTabsContext);
}
