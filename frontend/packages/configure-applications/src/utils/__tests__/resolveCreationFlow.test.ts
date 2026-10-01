// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import CustomPlatformTemplateJson from '../../data/application-templates/platform-based/custom.json';
import MCPClientTemplateJson from '../../data/application-templates/technology-based/mcp-client.json';
import {ApplicationCreateFlowStep} from '../../models/application-create-flow';
import type {ApplicationTemplate} from '../../models/application-templates';
import resolveCreationFlow from '../resolveCreationFlow';

const CustomPlatformTemplate = CustomPlatformTemplateJson as ApplicationTemplate;
const MCPClientTemplate = MCPClientTemplateJson as ApplicationTemplate;

describe('resolveCreationFlow', () => {
  it('returns the default user-facing flow (6 steps) when template is null', () => {
    const flow = resolveCreationFlow(null);
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.DETAILS,
      ApplicationCreateFlowStep.SECURITY,
      ApplicationCreateFlowStep.DESIGN,
      ApplicationCreateFlowStep.CONFIGURE,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
    expect(flow.previewSteps).toEqual([
      ApplicationCreateFlowStep.DETAILS,
      ApplicationCreateFlowStep.SECURITY,
      ApplicationCreateFlowStep.DESIGN,
    ]);
  });

  it('returns the default user-facing flow when the template has no creationFlow field', () => {
    const flow = resolveCreationFlow({id: 'react', displayName: 'React'});
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.DETAILS,
      ApplicationCreateFlowStep.SECURITY,
      ApplicationCreateFlowStep.DESIGN,
      ApplicationCreateFlowStep.CONFIGURE,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
  });

  it('returns the inline creationFlow from the template when present', () => {
    const flow = resolveCreationFlow({
      id: 'backend',
      creationFlow: {
        steps: [
          ApplicationCreateFlowStep.ORGANIZATION_UNIT,
          ApplicationCreateFlowStep.DETAILS,
          ApplicationCreateFlowStep.COMPLETE,
        ],
        previewSteps: [],
      },
    });
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.DETAILS,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
  });

  it('returns ORGANIZATION_UNIT, DETAILS and COMPLETE steps, with no preview steps, for the custom platform template', () => {
    const flow = resolveCreationFlow(CustomPlatformTemplate);
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.DETAILS,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
    expect(flow.previewSteps).toEqual([]);
  });

  it('returns ORGANIZATION_UNIT, DETAILS, CLIENT_TYPE, and COMPLETE steps, with no preview steps, for the mcp-client template', () => {
    const flow = resolveCreationFlow(MCPClientTemplate);
    expect(flow.steps).toEqual([
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.DETAILS,
      ApplicationCreateFlowStep.CLIENT_TYPE,
      ApplicationCreateFlowStep.COMPLETE,
    ]);
    expect(flow.previewSteps).toEqual([]);
  });
});
