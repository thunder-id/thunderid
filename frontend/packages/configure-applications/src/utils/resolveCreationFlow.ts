// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ApplicationCreateFlowStep} from '../models/application-create-flow';
import type {ApplicationTemplate} from '../models/application-templates';
import type {CreationFlow} from '../models/creation-flow';

const DEFAULT_USER_FACING_FLOW: CreationFlow = {
  steps: [
    ApplicationCreateFlowStep.ORGANIZATION_UNIT,
    ApplicationCreateFlowStep.DETAILS,
    ApplicationCreateFlowStep.SECURITY,
    ApplicationCreateFlowStep.DESIGN,
    ApplicationCreateFlowStep.CONFIGURE,
    ApplicationCreateFlowStep.COMPLETE,
  ],
  allowsUserLogins: true,
  previewSteps: [
    ApplicationCreateFlowStep.DETAILS,
    ApplicationCreateFlowStep.SECURITY,
    ApplicationCreateFlowStep.DESIGN,
  ],
};

/**
 * Resolve the creation flow for the given template. Templates that don't declare a
 * `creationFlow` fall back to the default user-facing flow.
 */
export default function resolveCreationFlow(template: ApplicationTemplate | null): CreationFlow {
  return template?.creationFlow ?? DEFAULT_USER_FACING_FLOW;
}
