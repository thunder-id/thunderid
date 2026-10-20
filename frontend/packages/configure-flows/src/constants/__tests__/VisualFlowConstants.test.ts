// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {ElementTypes} from '../../models/elements';
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
});
