// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CollisionPriority} from '@dnd-kit/abstract';
import {getStackGridSx, parseStackItems} from '@thunderid/design';
import {Box, Button, Menu, MenuItem, Typography, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronLeft, ChevronRight, ChevronUp, PlusIcon} from '@wso2/oxygen-ui-icons-react';
import {useReactFlow, type Node} from '@xyflow/react';
import {useMemo, useCallback, useState, type MouseEvent, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import VisualFlowConstants from '../../../../constants/VisualFlowConstants';
import useFlowPlugins from '../../../../hooks/useFlowPlugins';
import {type Element as FlowElement} from '../../../../models/elements';
import generateResourceId from '../../../../utils/generateResourceId';
import Droppable from '../../../dnd/Droppable';
import Handle from '../../../dnd/Handle';
import dashedAddButtonSx from '../../steps/view/dashedAddButtonSx';
import ReorderableFlowElement from '../../steps/view/ReorderableElement';

/**
 * Stack element type with layout configuration at top level.
 * With a valid `items` count of two or more the stack uses CSS Grid; anything else
 * (absent, 1, or malformed) keeps the flex layout.
 */
export type StackElement = FlowElement & {
  direction?: 'row' | 'column';
  gap?: number;
  align?: string;
  justify?: string;
  /** Number of slots across the main axis. Absent keeps the flex layout. */
  items?: number;
};

/**
 * Props interface of {@link StackAdapter}
 */
export interface StackAdapterPropsInterface {
  /**
   * The step id the resource resides on.
   */
  stepId: string;
  /**
   * The stack element properties.
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
 * sx applied to every grid cell — browser-inspector-style tinted slot.
 */
const SLOT_SX: SxProps<Theme> = {
  borderRadius: 1,
  border: '0px dashed',
  borderColor: 'divider',
  backgroundColor: 'rgba(var(--mui-palette-primary-mainChannel) / 0.04)',
  overflow: 'visible',
  width: '100%',
  // Floating action toolbar: reposition above the cell so it never obscures
  // compact content (buttons, short labels, etc.).
  '& .reorderable-component': {
    position: 'relative',
    p: '4px 6px',
    borderRadius: 1,
    border: '2px dashed transparent',
    '& .flow-builder-dnd-actions': {
      visibility: 'hidden',
      position: 'absolute',
      top: '-30px',
      right: 0,
      height: '26px',
      width: 'auto',
      minWidth: '72px',
      display: 'flex',
      flexDirection: 'row',
      alignItems: 'center',
      justifyContent: 'center',
      gap: '2px',
      background: 'var(--flow-builder-dragging-form-field-action-panel-background-color)',
      borderRadius: '4px 4px 4px 0',
      zIndex: 20,
      pointerEvents: 'none',
      '& svg': {pointerEvents: 'auto'},
    },
    '&:hover, &:focus-within': {
      borderColor: 'var(--flow-builder-dragging-form-field-hover-border-color)',
      backgroundColor: 'var(--flow-builder-dragging-form-field-hover-background-color)',
      '& .flow-builder-dnd-actions': {visibility: 'visible'},
    },
  },
};

/**
 * sx for empty placeholder slots.
 */
/**
 * Chrome that outlines the stack itself, mirroring the Form adapter so a stack's
 * children visibly belong to it.
 */
const CONTAINER_SX: SxProps<Theme> = {
  borderRadius: 'calc(2 * var(--oxygen-shape-borderRadius))',
  border: '1px dashed var(--oxygen-palette-divider)',
  backgroundColor: 'var(--oxygen-palette-background-paper)',
  boxSizing: 'border-box',
  px: 2,
  py: 1.5,
};

// An empty slot marks reserved space rather than an action, so it reads as a dashed
// outline instead of a filled box. That keeps it distinct from the "Add Component"
// button, which is filled and carries a label.
const EMPTY_SLOT_SX: SxProps<Theme> = {
  ...SLOT_SX,
  backgroundColor: 'transparent',
  border: '1.5px dashed',
  borderColor: 'divider',
  minHeight: '40px',
  padding: '8px',
  transition: 'border-color 150ms ease, background-color 150ms ease',
  '&:hover': {borderColor: 'primary.main', backgroundColor: 'action.hover'},
};

/**
 * Adapter for the Stack layout container.
 * Renders children in a configurable flex or grid layout with a droppable
 * zone for adding elements via drag-and-drop — same drop behaviour as Form.
 *
 * @param props - Props injected to the component.
 * @returns The StackAdapter component.
 */
function StackAdapter({
  stepId,
  resource,
  availableElements = [],
  onAddElementToForm = undefined,
}: StackAdapterPropsInterface): ReactElement {
  const stackElement = resource as StackElement;
  const gridSx = getStackGridSx(stackElement);
  const items = parseStackItems(stackElement?.items);
  const useGrid = gridSx !== null;
  // Grid resolves any non-column direction to the row axis, so the move controls have
  // to follow the same rule or a custom direction shows Up/Down beside a row grid.
  const isColumnAxis = (stackElement?.direction ?? '').startsWith('column');
  const isRow = useGrid ? !isColumnAxis : (stackElement?.direction ?? 'row') === 'row';

  const {t} = useTranslation();
  const {updateNodeData} = useReactFlow();
  const {emitElementFilter} = useFlowPlugins();
  const [addAnchorEl, setAddAnchorEl] = useState<null | HTMLElement>(null);

  const handleMove = useCallback(
    (componentId: string, delta: -1 | 1): void => {
      updateNodeData(stepId, (node: Node) => {
        const nodeData = node?.data as {components?: FlowElement[]};
        const components = nodeData?.components ?? [];

        const reorderInStack = (elements: FlowElement[]): FlowElement[] =>
          elements.map((el) => {
            if (el.id === resource.id) {
              const children = [...(el.components ?? [])];
              const idx = children.findIndex((c) => c.id === componentId);
              const newIdx = idx + delta;
              if (idx === -1 || newIdx < 0 || newIdx >= children.length) return el;
              [children[idx], children[newIdx]] = [children[newIdx], children[idx]];
              return {...el, components: children};
            }
            if (el.components) {
              return {...el, components: reorderInStack(el.components)};
            }
            return el;
          });

        return {components: reorderInStack(components)};
      });
    },
    [stepId, resource.id, updateNodeData],
  );

  const filteredComponents = useMemo(() => {
    if (!resource?.components) return [];
    return resource.components.filter((component: FlowElement) => emitElementFilter(component));
  }, [resource?.components, emitElementFilter]);

  // In grid mode the placeholders convey the slot structure, so every unoccupied cell
  // gets one, including the trailing cells of a partially filled last track.
  // A flex stack has no slots to convey, and an empty one already shows the
  // "Add Component" button, so a placeholder there would just be a second empty box.
  const slots = items ?? 1;
  const occupiedTracks = Math.max(1, Math.ceil(filteredComponents.length / slots));
  const emptySlotCount: number = useGrid ? occupiedTracks * slots - filteredComponents.length : 0;

  const layoutSx: SxProps<Theme> = gridSx ?? {
    display: 'flex',
    flexDirection: stackElement?.direction ?? 'row',
    flexWrap: 'wrap',
    gap: stackElement?.gap ?? 2,
    alignItems: stackElement?.align ?? 'center',
    justifyContent: stackElement?.justify ?? 'center',
  };

  const addableElements: FlowElement[] = availableElements.filter(
    (element: FlowElement) =>
      VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES.includes(element.type) &&
      element.display?.showOnResourcePanel !== false,
  );

  const handleAddComponent = (element: FlowElement): void => {
    setAddAnchorEl(null);
    onAddElementToForm?.(element, resource.id);
  };

  const droppable = (
    <Droppable
      id={generateResourceId(`${VisualFlowConstants.FLOW_BUILDER_STACK_ID}_${resource.id}`)}
      data={{droppedOn: resource, stepId}}
      collisionPriority={CollisionPriority.High}
      type={VisualFlowConstants.FLOW_BUILDER_DROPPABLE_STACK_ID}
      accept={[
        VisualFlowConstants.FLOW_BUILDER_DRAGGABLE_ID,
        ...VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES,
      ]}
      sx={layoutSx}
      hideDropZones
    >
      {filteredComponents.map((component: FlowElement, index: number) => {
        const isFirst = index === 0;
        const isLast = index === filteredComponents.length - 1;
        const moveActions = isRow ? (
          <>
            {!isFirst && (
              <Handle label="Move Left" onClick={() => handleMove(component.id, -1)}>
                <ChevronLeft size={16} />
              </Handle>
            )}
            {!isLast && (
              <Handle label="Move Right" onClick={() => handleMove(component.id, 1)}>
                <ChevronRight size={16} />
              </Handle>
            )}
          </>
        ) : (
          <>
            {!isFirst && (
              <Handle label="Move Up" onClick={() => handleMove(component.id, -1)}>
                <ChevronUp size={16} />
              </Handle>
            )}
            {!isLast && (
              <Handle label="Move Down" onClick={() => handleMove(component.id, 1)}>
                <ChevronDown size={16} />
              </Handle>
            )}
          </>
        );

        return (
          <ReorderableFlowElement
            key={component.id}
            id={component.id}
            index={index}
            element={component}
            group={resource.id}
            type={resource.id}
            accept={[resource.id, ...VisualFlowConstants.FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES]}
            availableElements={availableElements}
            onAddElementToForm={onAddElementToForm}
            hideDrag
            hideEdit
            extraActions={moveActions}
            sx={SLOT_SX}
            dropIndicatorStyles={{
              width: '100%',
            }}
            slotProps={{
              ContentContainer: {
                sx: {
                  alignItems: 'center',
                },
              },
            }}
          />
        );
      })}
      {Array.from({length: emptySlotCount}, (_, i) => (
        <Box key={`stack-empty-${i}`} sx={EMPTY_SLOT_SX} display="flex" alignItems="center" justifyContent="center">
          <Typography variant="caption" color="text.disabled">
            Drop here
          </Typography>
        </Box>
      ))}
    </Droppable>
  );

  // The add affordance lives inside the container chrome so the stack's outline
  // encloses it, the same way a form encloses its own add button.
  return (
    <Box sx={CONTAINER_SX}>
      {droppable}
      {addableElements.length > 0 && (
        <Button
          fullWidth
          size="small"
          className="nodrag"
          data-testid="stack-add-component-button"
          startIcon={<PlusIcon size={15} />}
          onClick={(event: MouseEvent<HTMLElement>) => {
            event.stopPropagation();
            setAddAnchorEl(event.currentTarget);
          }}
          sx={{...dashedAddButtonSx, mt: 1.5}}
        >
          {t('flows:core.steps.view.addComponent', 'Add Component')}
        </Button>
      )}
      <Menu
        anchorEl={addAnchorEl}
        open={Boolean(addAnchorEl)}
        onClose={() => setAddAnchorEl(null)}
        anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
        transformOrigin={{vertical: 'top', horizontal: 'right'}}
      >
        {addableElements.map((element: FlowElement) => (
          <MenuItem key={element.id} onClick={() => handleAddComponent(element)} sx={{minWidth: 200}}>
            {element.display?.label ?? element.type}
          </MenuItem>
        ))}
      </Menu>
    </Box>
  );
}

export default StackAdapter;
