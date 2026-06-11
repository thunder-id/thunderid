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

import {describe, expect, it} from 'vitest';
import CustomPlatformTemplateJson from '../../data/application-templates/platform-based/custom.json';
import {ApplicationCreateFlowStep} from '../../models/application-create-flow';
import type {ApplicationTemplate} from '../../models/application-templates';
import resolveCreationFlow from '../resolveCreationFlow';

const CustomPlatformTemplate = CustomPlatformTemplateJson as ApplicationTemplate;

describe('resolveCreationFlow', () => {
  it('returns the default user-facing flow (8 steps) when template is null', () => {
    const flow = resolveCreationFlow(null);
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.STACK,
      ApplicationCreateFlowStep.NAME,
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.DESIGN,
      ApplicationCreateFlowStep.OPTIONS,
      ApplicationCreateFlowStep.EXPERIENCE,
      ApplicationCreateFlowStep.CONFIGURE,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
  });

  it('returns the default user-facing flow when the template has no creationFlow field', () => {
    const flow = resolveCreationFlow({id: 'react', displayName: 'React'});
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.STACK,
      ApplicationCreateFlowStep.NAME,
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.DESIGN,
      ApplicationCreateFlowStep.OPTIONS,
      ApplicationCreateFlowStep.EXPERIENCE,
      ApplicationCreateFlowStep.CONFIGURE,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
  });

  it('returns the inline creationFlow from the template when present', () => {
    const flow = resolveCreationFlow({
      id: 'backend',
      creationFlow: {
        steps: [ApplicationCreateFlowStep.STACK, ApplicationCreateFlowStep.NAME, ApplicationCreateFlowStep.COMPLETE],
      },
    });
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.STACK,
      ApplicationCreateFlowStep.NAME,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
  });

  it('returns only NAME and COMPLETE steps for the custom platform template', () => {
    const flow = resolveCreationFlow(CustomPlatformTemplate);
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.NAME,
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
  });
});
