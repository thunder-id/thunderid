// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * UI Messages
 *
 * Static Console copy the specs assert on, matching the en-US catalog in
 * frontend/packages/i18n/src/locales/en-US.ts. Grouped by feature.
 *
 * @example
 * import { UIMessages } from "../../constants/ui-messages";
 * await expect(dialogTitle).toHaveText(UIMessages.accountStatus.user.suspendTitle);
 */
export const UIMessages = {
  /** The lifecycle Actions menu, its dialogs, banners and toasts on the details pages. */
  accountStatus: {
    actions: {
      suspend: "Suspend",
      unsuspend: "Unsuspend",
      unsuspendAndUnlock: "Unsuspend and unlock",
      unlock: "Unlock",
    },
    cancel: "Cancel",
    operatorNoteLabel: "Operator note (optional)",
    operatorNote: (note: string) => `Operator note: ${note}`,
    suspendedLabel: "Suspended",
    lockedLabel: "Locked",
    user: {
      toasts: {
        suspended: "User suspended",
        unsuspended: "User unsuspended",
        unsuspendedAndUnlocked: "User unsuspended and unlocked",
        unlocked: "User unlocked",
      },
      suspendTitle: "Suspend user",
      suspendBody: "The account will no longer be able to sign in. Existing sessions and tokens will be revoked.",
      suspendedHint: "The account is suspended and cannot sign in until an administrator unsuspends it.",
      unsuspendTitle: "Unsuspend user",
      unsuspendGuidance:
        "Before unsuspending, verify the user's contact information and reset any potentially compromised credentials.",
      unsuspendBody:
        "The user's account will remain locked after unsuspension. To regain access, the user must complete account recovery or an administrator must unlock the account.",
      releasedHint:
        "Suspension removed. The account remains locked until the user completes account recovery or an administrator unlocks it.",
      unsuspendAndUnlockTitle: "Unsuspend and unlock user",
      unsuspendAndUnlockGuidance:
        "Before restoring access, verify the user's contact information and reset any potentially compromised credentials.",
      unsuspendAndUnlockBody:
        "This action will remove the suspension and unlock the account, allowing the user to sign in again.",
    },
    agent: {
      toasts: {
        suspended: "Agent suspended",
        unsuspended: "Agent unsuspended",
      },
      suspendTitle: "Suspend agent",
      suspendBody:
        "The agent will no longer be able to authenticate or obtain new tokens. Existing sessions and tokens will be revoked. You can still update the agent details and credentials while it is suspended.",
      suspendedHint: "The agent is suspended and cannot authenticate until an administrator unsuspends it.",
      unsuspendTitle: "Unsuspend agent",
      unsuspendGuidance:
        "Before unsuspending, rotate any potentially compromised agent secrets to prevent their reuse.",
      unsuspendBody:
        "This action will remove the suspension and restore the agent's access, unless the agent is also locked.",
    },
    application: {
      toasts: {
        suspended: "Application suspended",
        unsuspended: "Application unsuspended",
      },
      suspendTitle: "Suspend application",
      suspendBody:
        "The application will no longer be able to authenticate or obtain new tokens. Existing sessions and tokens will be revoked.",
      suspendedHint: "The application is suspended and cannot authenticate until an administrator unsuspends it.",
      unsuspendTitle: "Unsuspend application",
      unsuspendGuidance:
        "Before unsuspending, rotate any potentially compromised application secrets to prevent their reuse.",
      unsuspendBody: "This action will remove the suspension and restore the application's access.",
    },
  },
} as const;
