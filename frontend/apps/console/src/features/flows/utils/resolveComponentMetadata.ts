// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import merge from 'lodash-es/merge';
import type {Element} from '../models/elements';
import type {Resources} from '../models/resources';

/**
 * Type-safe wrapper for lodash merge function.
 *
 * @param target - The target object to merge into.
 * @param sources - Source objects to merge from.
 * @returns The merged object.
 */
const safeMerge = <T>(...sources: Partial<T>[]): T => (merge as (...args: Partial<T>[]) => T)(...sources);

const resolveComponentMetadata = (resources: Resources, components?: Element[]): Element[] => {
  if (!components) {
    return [];
  }

  const updateComponentResourceType = (component: Element): Element => {
    let updatedComponent = {...component};

    resources?.elements?.forEach((componentWithMeta: Element) => {
      // Match by type only - element types are unique across categories (e.g., TEXT_INPUT only exists
      // in FIELD category, ACTION only in ACTION category). This allows proper metadata resolution
      // when components from templates have correct type but different category values.
      if (component.type === componentWithMeta.type) {
        if (component.variant) {
          // If the component metadata has a variants array, merge.
          if (componentWithMeta.variants) {
            updatedComponent = safeMerge<Element>({}, componentWithMeta, updatedComponent);

            return;
          }

          // If the component metadata has a high level variant, merge.
          if (componentWithMeta.variant && component.variant === componentWithMeta.variant) {
            updatedComponent = safeMerge<Element>({}, componentWithMeta, updatedComponent);
          }
        } else {
          updatedComponent = safeMerge<Element>({}, componentWithMeta, updatedComponent);
        }
      }
    });

    if (updatedComponent?.components) {
      updatedComponent.components = updatedComponent.components.map(updateComponentResourceType);
    }

    if (updatedComponent.variants?.length) {
      const selectedVariant = updatedComponent.variants.find(
        (variant: Element) => variant.variant === updatedComponent.variant,
      );
      const defaultVariant =
        selectedVariant ??
        updatedComponent.variants.find(
          (variant: Element) => variant.variant === updatedComponent.display?.defaultVariant,
        ) ??
        updatedComponent.variants[0];

      updatedComponent = safeMerge<Element>({}, defaultVariant, updatedComponent, {
        variant: defaultVariant.variant,
      });
    }

    return updatedComponent;
  };

  // Filter out non-element resources (like widgets, templates, steps) that may have been merged in
  // Only process actual elements (resourceType === 'ELEMENT' or undefined for template components)
  // Template components won't have resourceType set yet, so we include undefined values
  return components
    ?.filter((component) => !component.resourceType || component.resourceType === 'ELEMENT')
    .map(updateComponentResourceType);
};

export default resolveComponentMetadata;
