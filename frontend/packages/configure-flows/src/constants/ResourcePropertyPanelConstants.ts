// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Constants related to the flow builder elements.
 *
 * @remarks
 * This class is not meant to be instantiated. It only provides static constants.
 *
 * @example
 * ```typescript
 * const excludedProperties = ResourcePropertyPanelConstants.EXCLUDED_PROPERTIES;
 * ```
 */
class ResourcePropertyPanelConstants {
  /**
   * Private constructor to avoid object instantiation from outside the class.
   */
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private constructor() {}

  /**
   * Ordered sections the property panel groups fields into. Keys not listed here fall
   * into the trailing "Other" section, so a new element property still shows up.
   */
  public static readonly PROPERTY_SECTIONS: {title: string; keys: string[]}[] = [
    {title: 'Content', keys: ['label', 'text', 'title', 'subtitle', 'message', 'placeholder', 'hint', 'alt', 'src']},
    {title: 'Appearance', keys: ['variant', 'size', 'color', 'width', 'height']},
    {title: 'Layout', keys: ['items', 'direction', 'gap', 'align', 'justify']},
    {title: 'Validation', keys: ['required', 'minLength', 'maxLength', 'min', 'max', 'pattern']},
  ];

  // `classes` is excluded because the panel renders it through the dedicated
  // ClassesPropertyField; leaving it here too would show the same value twice.
  public static readonly EXCLUDED_PROPERTIES: string[] = [
    'ref',
    'type',
    'startIcon',
    'endIcon',
    'eventType',
    'actionType',
    'classes',
  ];
}

export default ResourcePropertyPanelConstants;
