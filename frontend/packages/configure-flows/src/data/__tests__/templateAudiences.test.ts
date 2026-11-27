// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {type FlowTemplate, TemplateAudiences} from '../../models/templates';
import {isFlowTypeOffered} from '../../utils/templateAudience';
import rawTemplates from '../templates.json';

const templates = rawTemplates as FlowTemplate[];

// The flow types the Create Flow wizard offers.
const wizardFlowTypes = [
  'AUTHENTICATION',
  'REGISTRATION',
  'RECOVERY',
  'SIGNOUT',
  'ADMINISTRATION',
  'INVITATION',
  'DEVICE_GRANT',
  'SELF',
  'ADMIN_INITIATED',
];

const typesOf = (audience: TemplateAudiences): string[] =>
  templates
    .filter((template) => template.audience === audience && template.type !== 'BLANK')
    .map((template) => template.type)
    .sort();

describe('template audiences', () => {
  it('should label every template with an audience', () => {
    const audiences: string[] = Object.values(TemplateAudiences);

    const unlabelled = templates
      .filter((template) => !audiences.includes(template.audience))
      .map((template) => `${template.flowType}/${template.type}`);

    expect(unlabelled).toEqual([]);
  });

  it('should write every blank template for everyone', () => {
    const blanks = templates.filter((template) => template.type === 'BLANK');

    expect(blanks.length).toBeGreaterThan(0);
    blanks.forEach((template) => expect(template.audience).toBe(TemplateAudiences.All));
  });

  it('should write only the agent onboarding templates for agents', () => {
    expect(typesOf(TemplateAudiences.Agent)).toEqual([
      'AGENT_ONBOARDING_GENERIC',
      'AGENT_ONBOARDING_OBO',
      'AGENT_ONBOARDING_OWN_IDENTITY',
    ]);
  });

  // An agent's Flows tab selects any authentication flow, an agent signs in itself through a sign-in flow built for
  // people (password, code, magic link or passkey), and its device grant flow calls its sign-in flow.
  it('should write every sign-in and device authorization template for everyone', () => {
    const agentFacing = templates.filter((template) => ['AUTHENTICATION', 'DEVICE_GRANT'].includes(template.flowType));

    expect(agentFacing.length).toBeGreaterThan(0);
    agentFacing.forEach((template) => expect(template.audience).toBe(TemplateAudiences.All));
  });

  it('should write the templates that agents are known to use for everyone', () => {
    expect(typesOf(TemplateAudiences.All)).toEqual(
      expect.arrayContaining(['BASIC_WITH_CONSENT', 'CIBA_EMAIL_NOTIFICATION', 'DEVICE_GRANT', 'PERSONAL_AGENT']),
    );
  });

  // Only the personal assistant template signs an agent up. It alone has an Agent Type Resolver step, and it registers
  // users as well, so it is written for everyone. Every other sign-up creates a user only.
  it('should write a sign-up for everyone only when it can create an agent', () => {
    const executorName = (node: unknown): string | undefined => (node as {executor?: {name?: string}}).executor?.name;
    const signUps = templates.filter((template) => template.flowType === 'REGISTRATION' && template.type !== 'BLANK');

    expect(signUps.length).toBeGreaterThan(1);
    signUps.forEach((template) => {
      const createsAgent = template.config.nodes.some((node) => executorName(node) === 'AgentTypeResolver');

      expect(template.audience).toBe(createsAgent ? TemplateAudiences.All : TemplateAudiences.User);
    });
    expect(signUps.filter((template) => template.audience === TemplateAudiences.All).map((t) => t.type)).toEqual([
      'PERSONAL_AGENT',
    ]);
  });

  it('should write for people the flow types that no agent setting takes', () => {
    const peopleOnlyFlowTypes = ['ADMIN_INITIATED', 'INVITATION', 'RECOVERY', 'SELF', 'SIGNOUT', 'USER_ONBOARDING'];

    templates
      .filter((template) => peopleOnlyFlowTypes.includes(template.flowType) && template.type !== 'BLANK')
      .forEach((template) => expect(template.audience).toBe(TemplateAudiences.User));

    const flowTypesWithUserTemplates = [
      ...new Set(
        templates
          .filter((template) => template.audience === TemplateAudiences.User)
          .map((template) => template.flowType),
      ),
    ].sort();
    expect(flowTypesWithUserTemplates).toEqual([...peopleOnlyFlowTypes, 'REGISTRATION'].sort());
  });

  describe('flow types the wizard offers', () => {
    const offeredTo = (audience: TemplateAudiences): string[] =>
      wizardFlowTypes.filter((flowType) => isFlowTypeOffered(templates, flowType, audience)).sort();

    it('should offer every flow type when the audience is All', () => {
      expect(offeredTo(TemplateAudiences.All)).toEqual([...wizardFlowTypes].sort());
    });

    it('should leave out only the agent-only flow type when the audience is Users', () => {
      expect(offeredTo(TemplateAudiences.User)).toEqual(
        wizardFlowTypes.filter((flowType) => flowType !== 'ADMINISTRATION').sort(),
      );
    });

    it('should offer only the flow types with an agent or shared template when the audience is Agents', () => {
      expect(offeredTo(TemplateAudiences.Agent)).toEqual([
        'ADMINISTRATION',
        'AUTHENTICATION',
        'DEVICE_GRANT',
        'REGISTRATION',
      ]);
    });
  });
});
