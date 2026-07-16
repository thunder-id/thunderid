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

import {Box, Button, Menu, MenuItem, type BoxProps} from '@wso2/oxygen-ui';
import {
  ChevronDownIcon,
  ChevronUpIcon,
  GripVertical,
  PencilLineIcon,
  PlusIcon,
  TrashIcon,
} from '@wso2/oxygen-ui-icons-react';
import {useNodeId, useReactFlow, type Node} from '@xyflow/react';
import classNames from 'classnames';
import {useRef, useState, useMemo, memo, type MouseEvent, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import dashedAddButtonSx from './dashedAddButtonSx';
import Handle from '../../../dnd/Handle';
import Sortable from '../../../dnd/Sortable';
import type {SortableProps} from '../../../dnd/Sortable';
import ValidationErrorBoundary from '../../../validation-panel/ValidationErrorBoundary';
import VisualFlowConstants from '@/features/flows/constants/VisualFlowConstants';
import useComponentDelete from '@/features/flows/hooks/useComponentDelete';
import useFlowConfig from '@/features/flows/hooks/useFlowConfig';
import useFlowPlugins from '@/features/flows/hooks/useFlowPlugins';
import useInteractionState from '@/features/flows/hooks/useInteractionState';
import useUIPanelState from '@/features/flows/hooks/useUIPanelState';
import useValidationStatus from '@/features/flows/hooks/useValidationStatus';
import {BlockTypes, ElementCategories, type Element} from '@/features/flows/models/elements';
import type {Resource} from '@/features/flows/models/resources';
import type {StepData} from '@/features/flows/models/steps';

/**
 * Props interface of {@link ReorderableElement}
 */
export interface ReorderableComponentPropsInterface
  extends Omit<SortableProps, 'element'>,
    Omit<BoxProps, 'children' | 'id'> {
  /**
   * The element to be rendered.
   */
  element: Resource;
  /**
   * List of available elements that can be added.
   * @defaultValue undefined
   */
  availableElements?: Resource[];
  /**
   * Callback for adding an element to a form.
   * @param element - The element to add.
   * @param formId - The ID of the form to add to.
   * @defaultValue undefined
   */
  onAddElementToForm?: (element: Resource, formId: string) => void;
  /**
   * When true, hides all selection chrome (hover border and action toolbar) and
   * renders only the element content. Used for elements that are managed as part
   * of their parent unit, e.g. the single trigger button inside an action block.
   * @defaultValue false
   */
  hideChrome?: boolean;
  /**
   * When true, hides the drag grip handle.
   * @defaultValue false
   */
  hideDrag?: boolean;
  /**
   * When true, hides the edit action button.
   * @defaultValue false
   */
  hideEdit?: boolean;
  /**
   * When true, hides the delete action button.
   * @defaultValue false
   */
  hideDelete?: boolean;
  /**
   * Extra action nodes rendered inside the actions toolbar (after edit, before delete).
   * @defaultValue undefined
   */
  extraActions?: ReactNode;
  /**
   * Additional props to be passed to the Box component.
   */
  slotProps?: {
    ContentContainer?: {
      sx?: BoxProps['sx'];
    };
  };
  /**
   * Additional props to be passed to the Box component.
   */
  [key: string]: unknown;
}

/**
 * Re-orderable component inside a step node.
 *
 * @param props - Props injected to the component.
 * @returns ReorderableElement component.
 */
function ReorderableElement({
  id,
  index,
  element,
  className,
  availableElements = undefined,
  onAddElementToForm = undefined,
  hideChrome = false,
  hideDrag = false,
  hideEdit = false,
  hideDelete = false,
  extraActions = null,
  slotProps = {},
  ...rest
}: ReorderableComponentPropsInterface): ReactElement {
  const {t} = useTranslation();
  const handleRef = useRef<HTMLButtonElement>(null);
  const stepId: string | null = useNodeId();
  const {updateNodeData} = useReactFlow();
  const {deleteComponent} = useComponentDelete();
  const {ElementFactory} = useFlowConfig();
  const {setLastInteractedResource, setLastInteractedStepId} = useInteractionState();
  const {setIsOpenResourcePropertiesPanel} = useUIPanelState();
  const {setOpenValidationPanel, setSelectedNotification, addNotification} = useValidationStatus();
  const {emitNodeElementDelete} = useFlowPlugins();
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const menuOpen = Boolean(anchorEl);

  // Check if this element is a Form
  // Action-category blocks (e.g. social login trigger wrappers) share the BLOCK type
  // with forms but only group a trigger button — they must not accept extra fields.
  const isForm = element.type === BlockTypes.Form && element.category !== ElementCategories.Action;

  const depsRef = useRef({
    element,
    index,
    onAddElementToForm,
    availableElements,
    stepId,
    setOpenValidationPanel,
    setSelectedNotification,
    addNotification,
    setLastInteractedStepId,
    setLastInteractedResource,
    deleteComponent,
    setIsOpenResourcePropertiesPanel,
    setAnchorEl,
    updateNodeData,
    emitNodeElementDelete,
  });

  // Update refs every render (minimal overhead - just assignment)
  depsRef.current = {
    element,
    index,
    onAddElementToForm,
    availableElements,
    stepId,
    setOpenValidationPanel,
    setSelectedNotification,
    addNotification,
    setLastInteractedStepId,
    setLastInteractedResource,
    deleteComponent,
    setIsOpenResourcePropertiesPanel,
    setAnchorEl,
    updateNodeData,
    emitNodeElementDelete,
  };

  // Store stable references to handler functions
  const handlersRef = useRef<{
    handlePropertyPanelOpen: (event: React.MouseEvent<HTMLElement>) => void;
    handleElementDelete: () => void;
    handleMenuOpen: (event: MouseEvent<HTMLElement>) => void;
    handleMenuClose: () => void;
    handleAddFieldToForm: (fieldElement: Resource) => void;
    handleMoveUp: (event: React.MouseEvent<HTMLElement>) => void;
    handleMoveDown: (event: React.MouseEvent<HTMLElement>) => void;
  } | null>(null);

  // Create handlers only once using lazy initialization - reads ALL deps from ref at call time
  handlersRef.current ??= {
    handlePropertyPanelOpen: (event: React.MouseEvent<HTMLElement>): void => {
      event.stopPropagation();
      const deps = depsRef.current;
      deps.setOpenValidationPanel?.(false);
      deps.setSelectedNotification?.(null);
      if (deps.stepId) {
        deps.setLastInteractedStepId(deps.stepId);
      }

      // Action blocks (e.g. social login wrappers) carry no meaningful properties of
      // their own — open the properties of the wrapped trigger button instead.
      const {element} = deps;
      const isActionBlock = element.type === BlockTypes.Form && element.category === ElementCategories.Action;
      const innerAction = isActionBlock
        ? (element as Element).components?.find((child: Element) => child.category === ElementCategories.Action)
        : undefined;

      deps.setLastInteractedResource(innerAction ?? element);
    },

    handleElementDelete: (): void => {
      const deps = depsRef.current;
      deps.emitNodeElementDelete(deps.stepId ?? '', deps.element);
      if (deps.stepId) {
        deps.deleteComponent(deps.stepId, deps.element);
      }
      deps.setIsOpenResourcePropertiesPanel(false);
    },

    handleMenuOpen: (event: MouseEvent<HTMLElement>): void => {
      event.stopPropagation();
      depsRef.current.setAnchorEl(event.currentTarget);
    },

    handleMenuClose: (): void => {
      depsRef.current.setAnchorEl(null);
    },

    handleAddFieldToForm: (fieldElement: Resource): void => {
      const deps = depsRef.current;
      if (deps.onAddElementToForm) {
        deps.onAddElementToForm(fieldElement, deps.element.id);
      }
      deps.setAnchorEl(null);
    },

    handleMoveUp: (event: React.MouseEvent<HTMLElement>): void => {
      event.stopPropagation();
      const deps = depsRef.current;
      if (!deps.stepId || deps.index <= 0) return;

      deps.updateNodeData(deps.stepId, (node: Node) => {
        const components = [...((node.data as StepData)?.components ?? [])];

        // Check if the element is a top-level component
        const topIdx = components.findIndex((c: Element) => c.id === deps.element.id);
        if (topIdx > 0) {
          const result = [...components];
          [result[topIdx - 1], result[topIdx]] = [result[topIdx], result[topIdx - 1]];
          return {components: result};
        }

        // Otherwise search nested containers — only mutate the one that holds the element
        return {
          components: components.map((c: Element) => {
            if (!c.components) return c;
            const childIdx = c.components.findIndex((child: Element) => child.id === deps.element.id);
            if (childIdx <= 0) return c; // not found or already first
            const result = [...c.components];
            [result[childIdx - 1], result[childIdx]] = [result[childIdx], result[childIdx - 1]];
            return {...c, components: result};
          }),
        };
      });
    },

    handleMoveDown: (event: React.MouseEvent<HTMLElement>): void => {
      event.stopPropagation();
      const deps = depsRef.current;
      if (!deps.stepId) return;

      deps.updateNodeData(deps.stepId, (node: Node) => {
        const components = [...((node.data as StepData)?.components ?? [])];

        // Check if the element is a top-level component
        const topIdx = components.findIndex((c: Element) => c.id === deps.element.id);
        if (topIdx >= 0 && topIdx < components.length - 1) {
          const result = [...components];
          [result[topIdx], result[topIdx + 1]] = [result[topIdx + 1], result[topIdx]];
          return {components: result};
        }

        // Otherwise search nested containers — only mutate the one that holds the element
        return {
          components: components.map((c: Element) => {
            if (!c.components) return c;
            const childIdx = c.components.findIndex((child: Element) => child.id === deps.element.id);
            if (childIdx < 0 || childIdx >= c.components.length - 1) return c; // not found or already last
            const result = [...c.components];
            [result[childIdx], result[childIdx + 1]] = [result[childIdx + 1], result[childIdx]];
            return {...c, components: result};
          }),
        };
      });
    },
  };

  // Extract stable handlers
  const {
    handlePropertyPanelOpen,
    handleElementDelete,
    handleMenuOpen,
    handleMenuClose,
    handleAddFieldToForm,
    handleMoveUp,
    handleMoveDown,
  } = handlersRef.current;

  // Filter available elements to only show form-compatible types that are visible on resource panel
  const formCompatibleElements = useMemo(
    () =>
      isForm && depsRef.current.availableElements
        ? depsRef.current.availableElements.filter(
            (el: Resource) =>
              VisualFlowConstants.FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES.includes(el.type) &&
              el.display?.showOnResourcePanel !== false,
          )
        : [],
    [isForm],
  );

  return (
    <Sortable
      id={id}
      index={index}
      handleRef={handleRef}
      data={{isReordering: true, resource: element, stepId}}
      {...rest}
    >
      <ValidationErrorBoundary resource={element} key={element.id}>
        <Box
          display="flex"
          alignItems="center"
          className={classNames({'reorderable-component': !hideChrome}, className)}
          onDoubleClick={handlePropertyPanelOpen}
          sx={{
            position: 'relative',
            ...(hideChrome
              ? {}
              : {
                  border: '2px dashed transparent',
                  py: 2,
                  px: 1,
                  '&:hover, &:focus, &:active': {
                    borderColor: 'primary.main',
                    bgcolor: 'action.hover',
                    '& > .flow-builder-dnd-actions': {
                      visibility: 'visible',
                    },
                  },
                  // When a nested reorderable is hovered, hide this element's toolbar
                  // so only the innermost element's toolbar is visible.
                  '&:has(.reorderable-component:hover) > .flow-builder-dnd-actions': {
                    visibility: 'hidden',
                  },
                }),
          }}
        >
          {!hideChrome && (
            <Box
              className="flow-builder-dnd-actions"
              sx={{
                visibility: 'hidden',
                position: 'absolute',
                bgcolor: 'background.default',
                right: 0,
                top: 0,
                height: 32,
                display: 'flex',
                flexDirection: 'row',
                alignItems: 'center',
                gap: 0,
                borderBottomLeftRadius: 4,
                zIndex: 10,
                pointerEvents: 'none',
                '& svg': {pointerEvents: 'auto'},
              }}
            >
              {!hideDrag && (
                <>
                  <Handle label="Drag" cursor="grab" ref={handleRef}>
                    <GripVertical size={16} />
                  </Handle>
                  <Handle label="Move up" onClick={handleMoveUp}>
                    <ChevronUpIcon size={16} />
                  </Handle>
                  <Handle label="Move down" onClick={handleMoveDown}>
                    <ChevronDownIcon size={16} />
                  </Handle>
                </>
              )}
              {!hideEdit && (
                <Handle label="Edit" onClick={handlePropertyPanelOpen}>
                  <PencilLineIcon size={16} />
                </Handle>
              )}
              {extraActions}
              {isForm && formCompatibleElements.length > 0 && (
                <Handle label="Add Field" onClick={handleMenuOpen}>
                  <PlusIcon size={16} />
                </Handle>
              )}
              {!hideDelete && (
                <Handle label="Delete" onClick={handleElementDelete}>
                  <TrashIcon size={16} color="red" />
                </Handle>
              )}
            </Box>
          )}
          <Box
            data-testid="element-content"
            onClick={handlePropertyPanelOpen}
            sx={{
              width: '100%',
              display: 'flex',
              flexDirection: 'column',
              gap: 1,
              '& .adapter': {position: 'relative'},
              ...slotProps?.ContentContainer?.sx,
            }}
          >
            <ElementFactory
              stepId={stepId ?? ''}
              resource={element}
              elementIndex={index}
              availableElements={availableElements}
              onAddElementToForm={onAddElementToForm}
            />
            {isForm && formCompatibleElements.length > 0 && (
              <Button
                fullWidth
                size="small"
                className="nodrag"
                data-testid="form-add-field-button"
                startIcon={<PlusIcon size={15} />}
                onClick={(event: MouseEvent<HTMLElement>) => {
                  event.stopPropagation();
                  handleMenuOpen(event);
                }}
                sx={dashedAddButtonSx}
              >
                {t('flows:core.steps.view.addField', 'Add Field')}
              </Button>
            )}
          </Box>
        </Box>
      </ValidationErrorBoundary>

      {/* Menu for adding fields to Form */}
      {isForm && (
        <Menu
          anchorEl={anchorEl}
          open={menuOpen}
          onClose={handleMenuClose}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'right',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'right',
          }}
        >
          {formCompatibleElements && formCompatibleElements.length > 0 ? (
            formCompatibleElements.map((fieldElement: Resource) => (
              <MenuItem
                key={`${fieldElement.type}-${fieldElement.id}-${typeof fieldElement.variant === 'string' ? fieldElement.variant : ''}`}
                onClick={() => handleAddFieldToForm(fieldElement)}
                sx={{
                  minWidth: 200,
                }}
              >
                {fieldElement.display?.label ?? fieldElement.type}
              </MenuItem>
            ))
          ) : (
            <MenuItem disabled sx={{minWidth: 200}}>
              No fields available
            </MenuItem>
          )}
        </Menu>
      )}
    </Sortable>
  );
}

// Only re-render if element.id or element properties actually change
const MemoizedReorderableElement = memo(ReorderableElement, (prevProps, nextProps) => {
  // Re-render if element changed (compare by reference and key props)
  if (prevProps.element !== nextProps.element) {
    return false;
  }
  // Re-render if id or index changed
  if (prevProps.id !== nextProps.id || prevProps.index !== nextProps.index) {
    return false;
  }
  // Re-render if className changed
  if (prevProps.className !== nextProps.className) {
    return false;
  }
  // Re-render if any chrome-visibility flag changed — the comparator is
  // load-bearing for these; a missed flag would freeze stale chrome.
  if (
    prevProps.hideChrome !== nextProps.hideChrome ||
    prevProps.hideDrag !== nextProps.hideDrag ||
    prevProps.hideEdit !== nextProps.hideEdit ||
    prevProps.hideDelete !== nextProps.hideDelete
  ) {
    return false;
  }
  // Re-render if availableElements changed — the persistent add-field button
  // and its menu render from it, so a late-loaded list must not be swallowed.
  if (prevProps.availableElements !== nextProps.availableElements) {
    return false;
  }
  // Don't re-render for onAddElementToForm changes
  // (handlers read from refs, so they don't need to trigger re-renders)
  return true;
});

export {MemoizedReorderableElement as ReorderableElement};
export default MemoizedReorderableElement;
