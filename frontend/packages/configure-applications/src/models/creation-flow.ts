// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ApplicationCreateFlowStep} from './application-create-flow';

/**
 * Wizard step ordering for creating an application.
 *
 * Each application template declares its own creation flow inline via
 * `ApplicationTemplate.creationFlow`. Templates without an explicit flow
 * default to the user-facing flow (see `resolveCreationFlow`).
 *
 * @public
 */
export interface CreationFlow {
  /** Ordered list of wizard steps for this flow. */
  steps: ApplicationCreateFlowStep[];
  /** Whether the application supports authentication for end users. */
  allowsUserLogins?: boolean;
  /**
   * Which of `steps` render the live sign-in preview panel. Templates with no hosted sign-in
   * screen at all (e.g. machine-to-machine backends) declare this as an empty array so the
   * wizard never renders a preview, instead of the page guessing from the current step.
   */
  previewSteps: ApplicationCreateFlowStep[];
}
