/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {Box} from '@wso2/oxygen-ui';
import classNames from 'classnames';
import {useMemo, type ReactElement} from 'react';
import {ReorderableElement} from '../../steps/view/ReorderableElement';
import useFlowPlugins from '@/features/flows/hooks/useFlowPlugins';
import {type Element as FlowElement} from '@/features/flows/models/elements';
import './BlockAdapter.scss';

/**
 * Props interface of {@link BlockAdapter}
 */
export interface BlockAdapterPropsInterface {
  /**
   * The block element properties.
   */
  resource: FlowElement;
  /**
   * List of available elements that can be added.
   */
  availableElements?: FlowElement[];
  /**
   * Callback for adding an element to a form.
   * @param element - The element to add.
   * @param formId - The ID of the form to add to.
   */
  onAddElementToForm?: (element: FlowElement, formId: string) => void;
}

/**
 * Adapter for rendering BLOCK containers without form styling.
 * Used for blocks that contain action buttons (like social login buttons)
 * where form-specific UI (badge, placeholder, droppable) is not desired.
 *
 * @param props - Props injected to the component.
 * @returns The BlockAdapter component.
 */
function BlockAdapter({
  resource,
  availableElements = [],
  onAddElementToForm = undefined,
}: BlockAdapterPropsInterface): ReactElement {
  const {emitElementFilter} = useFlowPlugins();

  const filteredComponents = useMemo(() => {
    if (!resource?.components) return [];
    return resource.components.filter((component: FlowElement) => emitElementFilter(component));
  }, [resource?.components, emitElementFilter]);

  return (
    <Box className="adapter block-adapter">
      {filteredComponents.map((component: FlowElement, index: number) => (
        <ReorderableElement
          key={component.id}
          id={component.id}
          index={index}
          element={component}
          className={classNames('flow-builder-step-content-form-field')}
          availableElements={availableElements}
          onAddElementToForm={onAddElementToForm}
          // Action blocks are managed as a single unit via the parent's chrome —
          // the nested trigger button must not render its own border and toolbar.
          hideChrome
        />
      ))}
    </Box>
  );
}

export default BlockAdapter;
