// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, type Context} from 'react';

/**
 * Wiring between the compact-mode canvas nodes and FlowBuilder's per-stack
 * expansion state: stack chips expand into their member chips, and the head
 * member chip of an expanded group offers restacking.
 */
export interface CompactStacksContextValue {
  /**
   * Collapse an expanded group back into its stack chip.
   */
  collapseStack: (stackId: string) => void;
  /**
   * Expand a stack chip into its individual member chips.
   */
  expandStack: (stackId: string, memberIds: string[]) => void;
  /**
   * Lookup from the head member node id of each expanded group to its stack
   * id, used by the head chip to offer the restack affordance.
   */
  expandedHeadIdToStackId: ReadonlyMap<string, string>;
}

const CompactStacksContext: Context<CompactStacksContextValue> = createContext<CompactStacksContextValue>({
  collapseStack: () => undefined,
  expandStack: () => undefined,
  expandedHeadIdToStackId: new Map<string, string>(),
});

CompactStacksContext.displayName = 'CompactStacksContext';

export default CompactStacksContext;
