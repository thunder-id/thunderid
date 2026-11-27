// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {FlowTemplate, TemplateAudiences} from '../../models/templates';
import {isFlowTypeOffered, matchesTemplateAudience} from '../templateAudience';

const template = (flowType: FlowTemplate['flowType'], type: string, audience: TemplateAudiences): FlowTemplate => ({
  resourceType: 'TEMPLATE',
  category: 'PASSWORD',
  type,
  flowType,
  audience,
  display: {label: type, image: '', showOnResourcePanel: true},
  config: {name: '', handle: '', nodes: []},
});

describe('matchesTemplateAudience', () => {
  it.each<[TemplateAudiences, TemplateAudiences, boolean]>([
    ['ALL', 'ALL', true],
    ['USER', 'ALL', true],
    ['AGENT', 'ALL', true],
    ['ALL', 'USER', true],
    ['USER', 'USER', true],
    ['AGENT', 'USER', false],
    ['ALL', 'AGENT', true],
    ['USER', 'AGENT', false],
    ['AGENT', 'AGENT', true],
  ])('should return %s-template-under-%s-filter as %s', (audience, filter, expected) => {
    expect(matchesTemplateAudience(audience, filter)).toBe(expected);
  });
});

describe('isFlowTypeOffered', () => {
  const templates = [
    template('AUTHENTICATION', 'BLANK', 'ALL'),
    template('AUTHENTICATION', 'CREDENTIALS_AUTH', 'USER'),
    template('ADMINISTRATION', 'BLANK', 'ALL'),
    template('ADMINISTRATION', 'AGENT_ONBOARDING_OBO', 'AGENT'),
    template('DEVICE_GRANT', 'DEVICE_GRANT', 'ALL'),
  ];

  it('should offer every flow type under the All filter, even one with no template', () => {
    expect(isFlowTypeOffered([], 'RECOVERY', 'ALL')).toBe(true);
  });

  it('should offer a flow type that has a template for the audience', () => {
    expect(isFlowTypeOffered(templates, 'AUTHENTICATION', 'USER')).toBe(true);
    expect(isFlowTypeOffered(templates, 'ADMINISTRATION', 'AGENT')).toBe(true);
  });

  it('should not count a blank template, which is written for every audience', () => {
    expect(isFlowTypeOffered(templates, 'AUTHENTICATION', 'AGENT')).toBe(false);
    expect(isFlowTypeOffered(templates, 'ADMINISTRATION', 'USER')).toBe(false);
  });

  it('should count a non-blank template written for everyone under both audiences', () => {
    expect(isFlowTypeOffered(templates, 'DEVICE_GRANT', 'USER')).toBe(true);
    expect(isFlowTypeOffered(templates, 'DEVICE_GRANT', 'AGENT')).toBe(true);
  });

  it('should not offer a flow type that has no template under a narrowed filter', () => {
    expect(isFlowTypeOffered(templates, 'RECOVERY', 'USER')).toBe(false);
    expect(isFlowTypeOffered(templates, 'RECOVERY', 'AGENT')).toBe(false);
  });
});
