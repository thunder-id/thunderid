// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * UI Messages
 *
 * Static Admin Console copy the specs assert on. The details pages are the React SDK's admin
 * components, so this copy matches the SDK's en-US catalog (the `admin.*` keys in
 * @thunderid/javascript's i18n translations). Grouped by feature.
 *
 * @example
 * import { UIMessages } from "../../constants/ui-messages";
 * await expect(accountStatus.status).toHaveText(UIMessages.accountStatus.status.suspended);
 */
export const UIMessages = {
  /** The lifecycle buttons, their inline confirmation and the status a details page shows. */
  accountStatus: {
    actions: {
      suspend: "Suspend",
      unsuspend: "Unsuspend",
      unlock: "Unlock",
    },
    cancel: "Cancel",
    operatorNoteLabel: "Note (optional)",
    status: {
      active: "Active",
      locked: "Locked",
      suspended: "Suspended",
    },
    suspendQuestion: (name: string) =>
      `Suspend ${name}? It can no longer sign in, and its sessions and tokens are revoked.`,
    unsuspendQuestion: (name: string) => `Unsuspend ${name}? Its access is restored, unless it is also locked.`,
  },
  /** The inline confirmation a details page's delete action asks for. */
  deletion: {
    confirm: "Confirm",
    question: (name: string) => `Delete ${name}?`,
    user: "Delete user",
    agent: "Delete agent",
  },
} as const;
