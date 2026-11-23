// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type FlowTemplate, TemplateAudiences, TemplateTypes} from '../models/templates';

/**
 * Whether a template written for `audience` belongs in a list filtered by `filter`. `ALL` on either side matches.
 */
export const matchesTemplateAudience = (audience: TemplateAudiences, filter: TemplateAudiences): boolean =>
  filter === TemplateAudiences.All || audience === TemplateAudiences.All || audience === filter;

/**
 * Whether a flow type has a template to offer under `filter`. A blank template is written for every audience, so it
 * does not count: counting it would keep every flow type in every filter.
 */
export const isFlowTypeOffered = (templates: FlowTemplate[], flowType: string, filter: TemplateAudiences): boolean =>
  filter === TemplateAudiences.All ||
  templates.some(
    (template) =>
      template.flowType === flowType &&
      template.type !== TemplateTypes.Blank &&
      matchesTemplateAudience(template.audience, filter),
  );
