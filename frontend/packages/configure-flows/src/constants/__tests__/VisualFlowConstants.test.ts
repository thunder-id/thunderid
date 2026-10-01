// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {ElementTypes} from '../../models/elements';
import VisualFlowConstants from '../VisualFlowConstants';

describe('VisualFlowConstants', () => {
  describe('SELECT droppable targets', () => {
    it('should allow SELECT on the canvas, a view and a form', () => {
      expect(VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES).toContain(ElementTypes.Select);
      expect(VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES).toContain(ElementTypes.Select);
      expect(VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES).toContain(ElementTypes.Select);
    });

    it('should not allow SELECT in stack or display only containers', () => {
      expect(VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES).not.toContain(ElementTypes.Select);
      expect(VisualFlowConstants.FLOW_BUILDER_FLOW_COMPLETION_VIEW_ALLOWED_RESOURCE_TYPES).not.toContain(
        ElementTypes.Select,
      );
      expect(VisualFlowConstants.FLOW_BUILDER_STATIC_CONTENT_ALLOWED_RESOURCE_TYPES).not.toContain(ElementTypes.Select);
    });

    it('should allow SELECT wherever DROPDOWN is allowed', () => {
      const lists = [
        VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_FLOW_COMPLETION_VIEW_ALLOWED_RESOURCE_TYPES,
        VisualFlowConstants.FLOW_BUILDER_STATIC_CONTENT_ALLOWED_RESOURCE_TYPES,
      ];

      lists.forEach((list) => {
        expect(list.includes(ElementTypes.Select)).toBe(list.includes(ElementTypes.Dropdown));
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
