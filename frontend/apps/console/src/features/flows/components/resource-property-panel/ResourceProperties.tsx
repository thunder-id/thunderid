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

import {Stack, Typography} from '@wso2/oxygen-ui';
import {useReactFlow, type Node as FlowNode} from '@xyflow/react';
import cloneDeep from 'lodash-es/cloneDeep';
import debounce from 'lodash-es/debounce';
import isEmpty from 'lodash-es/isEmpty';
import merge from 'lodash-es/merge';
import set from 'lodash-es/set';
import {useRef, useEffect, useMemo, useCallback, memo, type ReactElement} from 'react';
import ResourcePropertyPanelConstants from '../../constants/ResourcePropertyPanelConstants';
import useFlowConfig from '../../hooks/useFlowConfig';
import useFlowPlugins from '../../hooks/useFlowPlugins';
import useInteractionState from '../../hooks/useInteractionState';
import type {Properties} from '../../models/base';
import type {Element} from '../../models/elements';
import {ElementTypes} from '../../models/elements';
import type {Resource} from '../../models/resources';
import type {StepData} from '../../models/steps';

/**
 * Props interface of {@link ResourceProperties}
 */
/**
 * Callback signature for property changes in the flow builder property panel.
 * @param propertyKey - Dot-path key to the property (e.g. 'data.properties.idpId').
 * @param newValue - The new value.
 * @param resource - The resource being changed.
 * @param debounce - When true, batches the update with a 300ms delay. Use for continuous
 *                   inputs (text fields, number inputs). Defaults to false (immediate).
 */
export type PropertyChangeHandler = (
  propertyKey: string,
  newValue: unknown,
  resource: Resource,
  debounce?: boolean,
) => void;

export interface CommonResourcePropertiesPropsInterface {
  properties?: Properties;
  /**
   * The resource associated with the property.
   */
  resource: Resource;
  /**
   * The event handler for the property change. Applies immediately by default.
   * Pass `true` as the 4th argument for text/number inputs to enable 300ms debouncing.
   */
  onChange: PropertyChangeHandler;
  /**
   * The event handler for the variant change.
   * @param variant - The variant of the element.
   * @param resource - Partial resource properties to override.
   */
  onVariantChange?: (variant: string, resource?: Partial<Resource>) => void;
}

/**
 * Top-level properties that users can edit directly on a resource.
 * Used for property extraction, variant preservation, and resource updates.
 */
const TOP_LEVEL_EDITABLE_PROPS = [
  'label',
  'hint',
  'placeholder',
  'required',
  'src',
  'alt',
  'width',
  'height',
  'startIcon',
  'endIcon',
  'eventType',
  'items',
  'direction',
  'gap',
  'align',
  'justify',
  'name',
  'size',
  'color',
  'classes',
] as const;

function ResourceProperties(): ReactElement {
  const {updateNodeData} = useReactFlow();
  const {lastInteractedResource, setLastInteractedResource, lastInteractedStepId} = useInteractionState();
  const {ResourceProperties: ResourcePropertiesComponent} = useFlowConfig();
  const {emitPropertyPanelOpen, emitPropertyChange} = useFlowPlugins();

  // Use a ref to track the current resource ID for debounced functions
  const lastInteractedResourceIdRef = useRef<string>(lastInteractedResource?.id);

  const lastInteractedResourceRef = useRef(lastInteractedResource);
  const lastInteractedStepIdRef = useRef(lastInteractedStepId);
  const setLastInteractedResourceRef = useRef(setLastInteractedResource);
  const updateNodeDataRef = useRef(updateNodeData);

  // Keep refs in sync
  useEffect(() => {
    lastInteractedResourceIdRef.current = lastInteractedResource?.id;
    lastInteractedResourceRef.current = lastInteractedResource;
    lastInteractedStepIdRef.current = lastInteractedStepId;
    setLastInteractedResourceRef.current = setLastInteractedResource;
    updateNodeDataRef.current = updateNodeData;
  });

  /**
   * Memoize filtered properties to avoid expensive operations on every render.
   * Only recomputes when lastInteractedResource or lastInteractedStepId changes.
   */
  const filteredProperties = useMemo((): Properties => {
    if (!lastInteractedResource) {
      return {} as Properties;
    }

    const accumulated: Properties = {} as Properties;

    const resourceWithProps = lastInteractedResource as Resource & Record<string, unknown>;
    TOP_LEVEL_EDITABLE_PROPS.forEach((key) => {
      if (resourceWithProps[key] !== undefined && !ResourcePropertyPanelConstants.EXCLUDED_PROPERTIES.includes(key)) {
        (accumulated as Record<string, unknown>)[key] = resourceWithProps[key];
      }
    });

    // Ensure TEXT elements always expose `align` so the dropdown is visible
    // even for elements that were created before `align` was added as a default.
    if (
      lastInteractedResource.type === ElementTypes.Text &&
      (accumulated as Record<string, unknown>).align === undefined
    ) {
      (accumulated as Record<string, unknown>).align = 'left';
    }

    // Also extract from config for backwards compatibility
    if (lastInteractedResource.config) {
      Object.keys(lastInteractedResource.config).forEach((key: string) => {
        if (!ResourcePropertyPanelConstants.EXCLUDED_PROPERTIES.includes(key)) {
          (accumulated as Record<string, unknown>)[key] = (
            lastInteractedResource.config as unknown as Record<string, unknown>
          )[key];
        }
      });
    }

    const stepProperties = (
      lastInteractedResource as Resource & {
        data?: {properties?: Record<string, unknown>};
      }
    ).data?.properties;

    if (stepProperties) {
      Object.entries(stepProperties).forEach(([key, value]) => {
        (accumulated as Record<string, unknown>)[`data.properties.${key}`] = value;
      });
    }

    emitPropertyPanelOpen(lastInteractedResource, accumulated, lastInteractedStepId);

    return cloneDeep(accumulated);
  }, [lastInteractedResource, lastInteractedStepId, emitPropertyPanelOpen]);

  const changeSelectedVariant = useCallback((selected: string, element?: Partial<Element>) => {
    const currentResource = lastInteractedResourceRef.current;
    const currentStepId = lastInteractedStepIdRef.current;

    if (!currentResource) return;

    let selectedVariant: Element | undefined = cloneDeep(
      currentResource.variants?.find((resource: Element) => resource.variant === selected),
    );

    if (!selectedVariant) {
      return;
    }

    if (element) {
      selectedVariant = merge(selectedVariant, element);
    }

    // Preserve user-modified properties when changing variants.
    // Variant definitions carry default values for these fields that would
    // overwrite the user's customizations via the merge below.
    for (const key of TOP_LEVEL_EDITABLE_PROPS) {
      const currentValue = (currentResource as unknown as Record<string, unknown>)[key];
      if (currentValue !== undefined) {
        (selectedVariant as unknown as Record<string, unknown>)[key] = currentValue;
      }
    }

    // Preserve the current text value when changing variants
    const currentText = (currentResource.config as {text?: string})?.text;
    if (currentText && selectedVariant.config) {
      (selectedVariant.config as {text?: string}).text = currentText;
    }

    const updateComponent = (components: Element[]): Element[] =>
      components.map((component: Element) => {
        if (component.id === currentResource.id) {
          return merge(cloneDeep(component), selectedVariant);
        }

        if (component.components) {
          return {
            ...component,
            components: updateComponent(component.components),
          };
        }

        return component;
      });

    updateNodeDataRef.current(currentStepId, (node: FlowNode<StepData>) => {
      const components: Element[] = updateComponent(cloneDeep(node?.data?.components) ?? []);

      setLastInteractedResourceRef.current(merge(cloneDeep(currentResource), selectedVariant));

      return {
        components,
      };
    });
  }, []);

  /**
   * Core property change logic shared by both debounced and immediate paths.
   * Handles plugin interception, ReactFlow node updates, and interaction state sync.
   */
  const applyPropertyChangeRef = useRef<
    ((propertyKey: string, newValue: string | boolean | number | object, element: Element) => void) | null
  >(null);

  useEffect(() => {
    applyPropertyChangeRef.current = (
      propertyKey: string,
      newValue: string | boolean | number | object,
      element: Element,
    ): void => {
      const currentStepId = lastInteractedStepIdRef.current;
      const currentResource = lastInteractedResourceRef.current;

      // Execute plugins for property change event.
      const pluginResult = emitPropertyChange(propertyKey, newValue, element, currentStepId);

      // If plugin handled the change (returned false), still update the resource to trigger re-render
      // This ensures properties panel updates after plugin modifications (e.g., adding confirm password field)
      if (!pluginResult) {
        if (element.id === lastInteractedResourceIdRef.current && currentResource) {
          const updatedResource: Resource = cloneDeep(currentResource);
          set(updatedResource as unknown as Record<string, unknown>, propertyKey, newValue);
          setLastInteractedResourceRef.current(updatedResource);
        }
        return;
      }

      const updateComponent = (components: Element[]): Element[] =>
        components.map((component: Element) => {
          if (component.id === element.id) {
            const updated = {...component};

            set(updated, propertyKey, newValue);

            return updated;
          }

          if (component.components) {
            return {
              ...component,
              components: updateComponent(component.components),
            };
          }

          return component;
        });

      updateNodeDataRef.current(currentStepId, (node: FlowNode<StepData>) => {
        const data: StepData = node?.data ?? {};

        if (!isEmpty(node?.data?.components)) {
          data.components = updateComponent(cloneDeep(node?.data?.components) ?? []);
        } else if (propertyKey === 'data') {
          // When propertyKey is exactly 'data', replace the entire data object
          return {...(newValue as StepData)};
        } else {
          // Strip 'data.' prefix if present since we're already setting on the data object
          const actualKey = propertyKey.startsWith('data.') ? propertyKey.slice(5) : propertyKey;
          set(data as Record<string, unknown>, actualKey, newValue);
        }

        return {...data};
      });

      // Only update lastInteractedResource if the element being changed is still the currently selected one.
      // This prevents stale updates from overwriting the heading when user switches to a different element.
      // Use the ref to get the current resource ID at execution time (not from the stale closure).
      if (propertyKey !== 'action' && element.id === lastInteractedResourceIdRef.current && currentResource) {
        const updatedResource: Resource = cloneDeep(currentResource);

        if (propertyKey === 'data') {
          // When propertyKey is exactly 'data', replace the entire data object
          updatedResource.data = newValue as StepData;
        } else if (propertyKey === 'id' || (TOP_LEVEL_EDITABLE_PROPS as readonly string[]).includes(propertyKey)) {
          set(updatedResource as unknown as Record<string, unknown>, propertyKey, newValue);
        } else if (propertyKey.startsWith('config.') || propertyKey.startsWith('data.')) {
          // Properties starting with 'config.' or 'data.' should be set on the resource directly
          set(updatedResource, propertyKey, newValue);
        } else {
          set(updatedResource.data as Record<string, unknown>, propertyKey, newValue);
        }
        setLastInteractedResourceRef.current(updatedResource);
      }
    };
  }, [emitPropertyChange]);

  /**
   * Debounced handler for continuous inputs (text fields, number inputs, rich text).
   * Batches rapid keystrokes with a 300ms delay before committing to ReactFlow state.
   */
  const handlePropertyChangeDebouncedRef = useRef<
    ((propertyKey: string, newValue: string | boolean | number | object, element: Element) => void) | null
  >(null);

  useEffect(() => {
    const debouncedFn = debounce(
      (propertyKey: string, newValue: string | boolean | number | object, element: Element): void => {
        applyPropertyChangeRef.current?.(propertyKey, newValue, element);
      },
      300,
    );

    handlePropertyChangeDebouncedRef.current = debouncedFn;

    return () => {
      debouncedFn.cancel();
    };
  }, []);

  /**
   * Unified property change handler.
   * - Default (debounce=false): applies immediately for discrete inputs (dropdowns, checkboxes).
   * - debounce=true: batches with 300ms delay for continuous inputs (text fields, number inputs).
   */
  const handlePropertyChange = useCallback(
    (propertyKey: string, newValue: string | boolean | number | object, element: Element, shouldDebounce?: boolean) => {
      if (shouldDebounce) {
        void handlePropertyChangeDebouncedRef.current?.(propertyKey, newValue, element);
      } else {
        // Flush any pending debounced change to prevent it from overwriting this immediate change
        (handlePropertyChangeDebouncedRef.current as ReturnType<typeof debounce> | null)?.flush();
        applyPropertyChangeRef.current?.(propertyKey, newValue, element);
      }
    },
    [],
  );

  if (!lastInteractedResource) {
    return (
      <Typography variant="body2" color="textSecondary" sx={{padding: 2}}>
        No properties available.
      </Typography>
    );
  }

  return (
    <Stack gap={2}>
      <ResourcePropertiesComponent
        resource={lastInteractedResource}
        properties={filteredProperties as Record<string, unknown>}
        onChange={handlePropertyChange}
        onVariantChange={changeSelectedVariant}
      />
    </Stack>
  );
}

export default memo(ResourceProperties);
