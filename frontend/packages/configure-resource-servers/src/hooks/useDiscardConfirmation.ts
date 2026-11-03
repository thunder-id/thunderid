// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useState} from 'react';

/** Asks before leaving a form with unsaved changes through the page's own back or close control. */
export interface DiscardConfirmation {
  /** Runs the action now when nothing is unsaved, or once the user confirms discarding the changes. */
  confirm: (action: () => void) => void;
  /** Whether the confirmation is open. */
  open: boolean;
  /** Runs the action the user confirmed. */
  onConfirm: () => void;
  /** Keeps editing. */
  onClose: () => void;
}

/**
 * Asks before the page's own back or close control leaves a form with unsaved changes, as the flow
 * builder does for its back button.
 */
export default function useDiscardConfirmation(hasChanges: boolean): DiscardConfirmation {
  const [pending, setPending] = useState<(() => void) | null>(null);

  return {
    confirm: (action) => {
      if (hasChanges) setPending(() => action);
      else action();
    },
    open: pending !== null,
    onConfirm: () => {
      const action = pending;
      setPending(null);
      action?.();
    },
    onClose: () => setPending(null),
  };
}
