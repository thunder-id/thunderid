// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import isEmpty from 'lodash-es/isEmpty';
import type {Element} from '../models/elements';
import generateResourceId from '../utils/generateResourceId';
/**
 * Props interface of {@link useGenerateStepElement}
 */
export interface UseGenerateStepElement {
  generateStepElement: (element: Element) => Element;
}

/**
 * Hook that provides a function to generate a step element with a unique ID.
 *
 * This hook allows components to generate a step element with a unique ID and default variant if applicable.
 *
 * @returns An object containing the `generateStepElement` function.
 *
 * @example
 * ```tsx
 * const { generateStepElement } = useGenerateStepElement();
 * const element = generateStepElement({ category: "ACTION", variants: [...] });
 * ```
 */
const useGenerateStepElement = (): UseGenerateStepElement => {
  const generateStepElement = (element: Element): Element => {
    let updatedElement: Element = {
      ...element,
      id: generateResourceId(element.category.toLowerCase()),
    };

    // If the component has variants, add the default variant to the root.
    if (!isEmpty(updatedElement?.variants)) {
      const defaultVariantType: string =
        updatedElement?.display?.defaultVariant ?? (updatedElement.variants?.[0]?.variant as string);
      const defaultVariant: Element | undefined = updatedElement.variants?.find(
        (variant: Element) => variant.variant === defaultVariantType,
      );

      if (defaultVariant) {
        updatedElement = {
          ...updatedElement,
          ...defaultVariant,
        };
      }
    }

    return updatedElement;
  };

  return {
    generateStepElement,
  };
};

export default useGenerateStepElement;
