// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import {memo, type ReactElement} from 'react';
import CommonStepFactory, {type CommonStepFactoryPropsInterface} from './CommonStepFactory';
import type {Element} from '../../../models/elements';
import type {Resources} from '../../../models/resources';

/**
 * Props interface of {@link StepFactory}
 */
export interface StepFactoryPropsInterface extends CommonStepFactoryPropsInterface {
  /**
   * All available resources in the flow.
   * @defaultValue undefined
   */
  allResources?: Resources;
  /**
   * Callback for adding an element to the view.
   * @defaultValue undefined
   */
  onAddElement?: (element: Element) => void;
  /**
   * Callback for adding an element to a form.
   * @param element - The element to add.
   * @param formId - The ID of the form to add to.
   * @defaultValue undefined
   */
  onAddElementToForm?: (element: Element, formId: string) => void;
}

/**
 * Factory for creating steps.
 * Extends the {@link CommonStepFactory} component.
 *
 * @param props - Props injected to the component.
 * @returns The StepFactory component.
 */
function StepFactory({
  resourceId,
  resources,
  allResources = undefined,
  onAddElement = undefined,
  onAddElementToForm = undefined,
  ...rest
}: StepFactoryPropsInterface & Node): ReactElement {
  return (
    <CommonStepFactory
      resourceId={resourceId}
      resources={resources}
      allResources={allResources}
      onAddElement={onAddElement}
      onAddElementToForm={onAddElementToForm}
      {...rest}
    />
  );
}

// Memoize to prevent re-renders during drag operations
export default memo(
  StepFactory,
  (prevProps, nextProps) =>
    prevProps.id === nextProps.id &&
    prevProps.data === nextProps.data &&
    prevProps.resources === nextProps.resources &&
    prevProps.allResources === nextProps.allResources &&
    prevProps.onAddElement === nextProps.onAddElement &&
    prevProps.onAddElementToForm === nextProps.onAddElementToForm,
);
