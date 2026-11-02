// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Element} from './elements';
import type {Step} from './steps';
import type {Template} from './templates';
import type {Widget} from './widget';

export type Resource = Element | Step | Widget;

export const ResourceTypes = {
  Step: 'STEP',
  Element: 'ELEMENT',
  Widget: 'WIDGET',
  Template: 'TEMPLATE',
} as const;

export type ResourceTypes = (typeof ResourceTypes)[keyof typeof ResourceTypes];

/**
 * Interface for the entire JSON structure.
 */
export interface Resources {
  /**
   * List of Elements.
   */
  elements: Element[];
  /**
   * List of Steps.
   */
  steps: Step[];
  /**
   * List of widgets.
   */
  widgets: Widget[];
  /**
   * List of templates.
   */
  templates: Template[];
  /**
   * List of executors (EXECUTION step resources).
   */
  executors: Step[];
}
