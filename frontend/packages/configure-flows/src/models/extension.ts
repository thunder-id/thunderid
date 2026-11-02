// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Event types in the flow builder extension.
 */
const EventTypes = {
  /**
   * Event triggered when a node is removed.
   */
  ON_NODE_DELETE: 'onNodeDelete',
  /**
   * Event triggered when an edge is removed.
   */
  ON_EDGE_DELETE: 'onEdgeDelete',
  /**
   * Event triggered when an element of a node is deleted.
   */
  ON_NODE_ELEMENT_DELETE: 'onNodeElementDelete',
  /**
   * Event triggered when a flow is updated.
   */
  ON_FLOW_UPDATE: 'onFlowUpdate',
  /**
   * Event triggered when the property panel is opened.
   */
  ON_PROPERTY_PANEL_OPEN: 'onPropertyPanelOpen',
  /**
   * Event triggered when a property is changed.
   */
  ON_PROPERTY_CHANGE: 'onPropertyChange',
  /**
   * Event triggered before a node element is rendered.
   */
  ON_NODE_ELEMENT_RENDER: 'onNodeElementRender',
  /**
   * Event triggered to filter node elements.
   */
  ON_NODE_ELEMENT_FILTER: 'onNodeElementFilter',
  /**
   * Event triggered when a template is loaded.
   */
  ON_TEMPLATE_LOAD: 'onTemplateLoad',
  /**
   * Event triggered when a custom nodes are registered to the flow.
   */
  ON_CUSTOM_NODE_REGISTER: 'onCustomNodeRegister',
} as const;

export default EventTypes;
export type EventTypes = (typeof EventTypes)[keyof typeof EventTypes];
