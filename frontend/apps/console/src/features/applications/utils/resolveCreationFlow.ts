/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {ApplicationCreateFlowStep} from '../models/application-create-flow';
import type {ApplicationTemplate} from '../models/application-templates';
import type {CreationFlow} from '../models/creation-flow';

const DEFAULT_USER_FACING_FLOW: CreationFlow = {
  steps: [
    ApplicationCreateFlowStep.STACK,
    ApplicationCreateFlowStep.NAME,
    ApplicationCreateFlowStep.ORGANIZATION_UNIT,
    ApplicationCreateFlowStep.DESIGN,
    ApplicationCreateFlowStep.OPTIONS,
    ApplicationCreateFlowStep.EXPERIENCE,
    ApplicationCreateFlowStep.CONFIGURE,
    ApplicationCreateFlowStep.COMPLETE,
  ],
};

/**
 * Resolve the creation flow for the given template. Templates that don't declare a
 * `creationFlow` fall back to the default user-facing flow.
 */
export default function resolveCreationFlow(template: ApplicationTemplate | null): CreationFlow {
  return template?.creationFlow ?? DEFAULT_USER_FACING_FLOW;
}
