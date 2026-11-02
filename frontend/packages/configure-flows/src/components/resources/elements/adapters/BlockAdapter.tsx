// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';
import {useMemo, type ReactElement} from 'react';
import useFlowPlugins from '../../../../hooks/useFlowPlugins';
import {type Element as FlowElement} from '../../../../models/elements';
import {ReorderableElement} from '../../steps/view/ReorderableElement';

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
    <Box data-testid="block-adapter" sx={{display: 'block', width: '100%'}}>
      {filteredComponents.map((component: FlowElement, index: number) => (
        <ReorderableElement
          key={component.id}
          id={component.id}
          index={index}
          element={component}
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
