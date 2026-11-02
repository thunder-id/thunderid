// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {NodeProps} from '@xyflow/react';
import type {ReactElement} from 'react';
import Call from './call/Call';
import End from './end/End';
import Execution from './execution/Execution';
import Rule from './rule/Rule';
import View from './view/View';
import type {Element} from '../../../models/elements';
import {ElementCategories, ElementTypes} from '../../../models/elements';
import type {Resources} from '../../../models/resources';
import {StepTypes, type Step} from '../../../models/steps';

/**
 * Props interface of {@link CommonStepFactory}
 */
export interface CommonStepFactoryPropsInterface extends NodeProps {
  /**
   * The flow id of the resource.
   */
  resourceId: string;
  /**
   * All the resources corresponding to the type.
   */
  resources: Step[];
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
 * Recursively checks whether a component tree contains any ACTION or RESEND elements.
 */
function hasActionElements(components: Element[] | undefined): boolean {
  if (!components) return false;
  return components.some((comp) => {
    if (
      comp.category === ElementCategories.Action ||
      comp.type === ElementTypes.Action ||
      comp.type === ElementTypes.Resend
    ) {
      return true;
    }
    const nested = comp.components;
    return hasActionElements(nested);
  });
}

/**
 * Factory for creating common steps.
 *
 * @param props - Props injected to the component.
 * @returns The CommonStepFactory component.
 */
function CommonStepFactory({
  resources,
  data,
  allResources = undefined,
  onAddElement = undefined,
  onAddElementToForm = undefined,
  ...rest
}: CommonStepFactoryPropsInterface): ReactElement | null {
  if (resources?.[0].type === StepTypes.View) {
    const isDisplayOnly = !hasActionElements(data?.['components'] as Element[] | undefined);
    return (
      <View
        resources={resources}
        data={data}
        availableElements={allResources?.elements}
        enableSourceHandle={isDisplayOnly}
        onAddElement={onAddElement}
        onAddElementToForm={onAddElementToForm}
        {...rest}
      />
    );
  }

  if (resources[0].type === StepTypes.Rule) {
    return <Rule resources={resources} data={data} {...rest} />;
  }

  if (resources[0].type === StepTypes.Execution) {
    return <Execution resources={resources} data={data} {...rest} />;
  }

  if (resources?.[0].type === StepTypes.End) {
    return <End resources={resources} data={data} {...rest} />;
  }

  if (resources?.[0].type === StepTypes.Call) {
    return <Call resources={resources} data={data} {...rest} />;
  }

  return null;
}

export default CommonStepFactory;
