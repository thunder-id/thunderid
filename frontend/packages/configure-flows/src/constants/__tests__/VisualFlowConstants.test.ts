// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import widgets from '../../data/widgets.json';
import {ElementTypes} from '../../models/elements';
import {WidgetTypes} from '../../models/widget';
import VisualFlowConstants from '../VisualFlowConstants';

describe('VisualFlowConstants', () => {
  describe.each([
    {name: 'SELECT', type: ElementTypes.Select, follows: ElementTypes.Dropdown},
    {name: 'USER_SELECT', type: ElementTypes.UserSelect, follows: ElementTypes.Select},
  ])('$name droppable targets', ({name, type, follows}) => {
    it(`should allow ${name} on the canvas, a view and a form`, () => {
      expect(VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES).toContain(type);
      expect(VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES).toContain(type);
      expect(VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES).toContain(type);
    });

    it(`should not allow ${name} in stack or display only containers`, () => {
      expect(VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES).not.toContain(type);
      expect(VisualFlowConstants.FLOW_BUILDER_FLOW_COMPLETION_VIEW_ALLOWED_RESOURCE_TYPES).not.toContain(type);
      expect(VisualFlowConstants.FLOW_BUILDER_STATIC_CONTENT_ALLOWED_RESOURCE_TYPES).not.toContain(type);
    });

    it(`should allow ${name} wherever ${follows} is allowed`, () => {
      const lists = [
        VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_FLOW_COMPLETION_VIEW_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_STATIC_CONTENT_ALLOWED_RESOURCE_TYPES,
      ];

      lists.forEach((list) => {
        expect(list.includes(type)).toBe(list.includes(follows));
      });
    });
  });

  describe('RESEND droppable targets', () => {
    it('should allow RESEND in a view, a form and a stack', () => {
      expect(VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES).toContain(ElementTypes.Resend);
      expect(VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES).toContain(ElementTypes.Resend);
      expect(VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES).toContain(ElementTypes.Resend);
    });

    it('should not allow RESEND on the canvas or in display only containers', () => {
      expect(VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES).not.toContain(ElementTypes.Resend);
      expect(VisualFlowConstants.FLOW_BUILDER_FLOW_COMPLETION_VIEW_ALLOWED_RESOURCE_TYPES).not.toContain(
        ElementTypes.Resend,
      );
      expect(VisualFlowConstants.FLOW_BUILDER_STATIC_CONTENT_ALLOWED_RESOURCE_TYPES).not.toContain(ElementTypes.Resend);
    });

    it('should allow RESEND wherever ACTION is allowed', () => {
      const lists = [
        VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_FLOW_COMPLETION_VIEW_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_STATIC_CONTENT_ALLOWED_RESOURCE_TYPES,
      ];

      lists.forEach((list) => {
        expect(list.includes(ElementTypes.Resend)).toBe(list.includes(ElementTypes.Action));
      });
    });
  });

  describe('widget droppable targets', () => {
    interface WidgetDefinition {
      type: string;
      config: {data: {steps?: {__generationMeta__?: {strategy?: string}}[]}};
    }

    const definitions = widgets as unknown as WidgetDefinition[];
    const mergesIntoAView = (widget: WidgetDefinition): boolean =>
      (widget.config.data.steps ?? []).some((step) => step.__generationMeta__?.strategy === 'MERGE_WITH_DROP_POINT');

    it('should know every widget the resource panel offers', () => {
      const known: string[] = Object.values(WidgetTypes);

      definitions.forEach((widget) => {
        expect(known, `${widget.type} is missing from WidgetTypes`).toContain(widget.type);
      });
    });

    it('should accept every widget on the canvas', () => {
      definitions.forEach((widget) => {
        expect(
          VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES,
          `${widget.type} cannot be dropped on the canvas`,
        ).toContain(widget.type);
      });
    });

    // A widget with a merge step attaches into an existing view. One without is a standalone
    // flow fragment, which only makes sense on the canvas.
    it('should accept a widget in a view only when it merges into one', () => {
      definitions.forEach((widget) => {
        expect(
          VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES.includes(widget.type),
          `${widget.type} view drop target`,
        ).toBe(mergesIntoAView(widget));
      });
    });

    it('should accept the agent onboarding widgets on the canvas', () => {
      ['AGENT_OWNER_RESOLUTION', 'AGENT_DELEGATION', 'AGENT_OBO_DETAILS'].forEach((type) => {
        expect(VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES).toContain(type);
        expect(VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES).not.toContain(type);
      });
    });
  });
});
